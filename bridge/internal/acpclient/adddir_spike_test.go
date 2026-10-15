package acpclient

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestAdditionalDirectoriesSpike asks a real `zero acp` whether
// additionalDirectories changes session/new, session/load, or session/resume.
// The child uses a fresh HOME. No prompt is sent. The test skips when zero
// is not on PATH so the suite stays hermetic everywhere else.
func TestAdditionalDirectoriesSpike(t *testing.T) {
	bin, err := exec.LookPath("zero")
	if err != nil {
		t.Skip("zero is not on PATH")
	}
	root := t.TempDir()
	home := filepath.Join(root, "home")
	for _, dir := range []string{
		home,
		filepath.Join(home, "config"),
		filepath.Join(home, "cache"),
		filepath.Join(home, "data"),
		filepath.Join(home, "state"),
		filepath.Join(root, "ws"),
		filepath.Join(root, "other"),
	} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	ws := filepath.Join(root, "ws")
	other := filepath.Join(root, "other")
	missing := filepath.Join(root, "missing-add")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.Command(bin, "acp")
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + home,
		"XDG_CONFIG_HOME=" + filepath.Join(home, "config"),
		"XDG_CACHE_HOME=" + filepath.Join(home, "cache"),
		"XDG_DATA_HOME=" + filepath.Join(home, "data"),
		"XDG_STATE_HOME=" + filepath.Join(home, "state"),
		"TMPDIR=" + root,
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stopSpikeChild(t, cmd) })

	replies := make(chan spikeMsg, 8)
	go func() {
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 64*1024), 1<<20)
		for sc.Scan() {
			line := bytes.TrimSpace(sc.Bytes())
			if len(line) == 0 {
				continue
			}
			var msg spikeMsg
			if json.Unmarshal(line, &msg) != nil || msg.ID == 0 {
				continue
			}
			replies <- msg
		}
	}()

	var next int64
	rpc := func(method string, params any) (json.RawMessage, string) {
		t.Helper()
		next++
		id := next
		body, err := json.Marshal(map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"method":  method,
			"params":  params,
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := stdin.Write(append(body, '\n')); err != nil {
			t.Fatalf("%s write: %v", method, err)
		}
		timer := time.NewTimer(8 * time.Second)
		defer timer.Stop()
		for {
			select {
			case msg := <-replies:
				if msg.ID != id {
					continue
				}
				if msg.Error != nil {
					return nil, msg.Error.Message
				}
				return msg.Result, ""
			case <-timer.C:
				t.Fatalf("%s timed out; stderr=%q", method, redactSpike(stderr.String(), root))
			case <-ctx.Done():
				t.Fatalf("%s: %v", method, ctx.Err())
			}
		}
	}

	if _, errMsg := rpc("initialize", map[string]any{
		"protocolVersion": 1,
		"clientCapabilities": map[string]any{
			"fs":       map[string]bool{"readTextFile": false, "writeTextFile": false},
			"terminal": false,
		},
		"clientInfo": map[string]string{"name": "zero-bridge-adddir-spike", "version": "0"},
	}); errMsg != "" {
		t.Fatalf("initialize: %s", redactSpike(errMsg, root))
	}

	newParams := func(cwd string, add []string) map[string]any {
		p := map[string]any{"cwd": cwd, "mcpServers": []any{}}
		if add != nil {
			p["additionalDirectories"] = add
		}
		return p
	}
	var created struct {
		SessionID string `json:"sessionId"`
	}
	raw, errMsg := rpc("session/new", newParams(ws, []string{missing}))
	if errMsg != "" {
		t.Fatalf("session/new with a missing additional directory was refused: %s", redactSpike(errMsg, root))
	}
	if err := json.Unmarshal(raw, &created); err != nil || created.SessionID == "" {
		t.Fatal("session/new returned no session id")
	}

	loadParams := func(cwd string, add []string) map[string]any {
		p := map[string]any{
			"sessionId":  created.SessionID,
			"cwd":        cwd,
			"mcpServers": []any{},
		}
		if add != nil {
			p["additionalDirectories"] = add
		}
		return p
	}

	if _, errMsg = rpc("session/load", loadParams(ws, []string{missing})); errMsg != "" {
		t.Fatalf("session/load with a missing additional directory was refused: %s", redactSpike(errMsg, root))
	}
	if _, errMsg = rpc("session/load", loadParams(other, []string{missing})); errMsg == "" || !strings.Contains(errMsg, "does not match") {
		t.Fatalf("session/load cwd check = %q, want a workspace mismatch", redactSpike(errMsg, root))
	}
	if _, errMsg = rpc("session/load", loadParams(other, nil)); errMsg == "" || !strings.Contains(errMsg, "does not match") {
		t.Fatalf("session/load cwd check without the field = %q, want the same mismatch", redactSpike(errMsg, root))
	}
	if _, errMsg = rpc("session/resume", loadParams(ws, []string{missing})); errMsg != "" {
		t.Fatalf("session/resume with a missing additional directory was refused: %s", redactSpike(errMsg, root))
	}
	if _, errMsg = rpc("session/resume", loadParams(other, []string{missing})); errMsg == "" || !strings.Contains(errMsg, "does not match") {
		t.Fatalf("session/resume cwd check = %q, want a workspace mismatch", redactSpike(errMsg, root))
	}
	if _, errMsg = rpc("session/resume", loadParams(other, nil)); errMsg == "" || !strings.Contains(errMsg, "does not match") {
		t.Fatalf("session/resume cwd check without the field = %q, want the same mismatch", redactSpike(errMsg, root))
	}
}

type spikeMsg struct {
	ID     int64           `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func redactSpike(s, root string) string {
	return strings.ReplaceAll(s, root, "<tmp>")
}

func stopSpikeChild(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	if cmd.Process == nil {
		return
	}
	pid := cmd.Process.Pid
	raw, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		t.Errorf("read command line: %v", err)
		return
	}
	if !bytes.Contains(raw, []byte("zero")) || !bytes.Contains(raw, []byte("acp")) {
		t.Errorf("refuse to stop pid %d", pid)
		return
	}
	_ = cmd.Process.Kill()
	_, _ = cmd.Process.Wait()
}
