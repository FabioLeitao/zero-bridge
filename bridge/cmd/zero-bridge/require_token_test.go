package main

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

const plantedSecret = "planted-secret-0123456789abcdef0123456789abcdef"

func TestRequireTokenRefusesBeforeListen(t *testing.T) {
	t.Setenv("ZERO_BRIDGE_TOKEN", "")
	t.Setenv("PLANTED_SECRET", plantedSecret)
	cases := []struct {
		name string
		args []string
		env  string
	}{
		{name: "flag", args: []string{"--listen", "127.0.0.1:9", "--require-token", "--agent-cmd", "/bin/true"}},
		{name: "env", args: []string{"--listen", "127.0.0.1:9", "--agent-cmd", "/bin/true"}, env: "true"},
		{name: "trust-proxy", args: []string{"--listen", "127.0.0.1:9", "--trust-proxy", "--agent-cmd", "/bin/true"}},
		{name: "public-origin", args: []string{"--listen", "127.0.0.1:9", "--public-origin", "https://bridge.example", "--agent-cmd", "/bin/true"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ZERO_BRIDGE_REQUIRE_TOKEN", tc.env)
			var stderr bytes.Buffer
			code := runArgs(tc.args, &stderr)
			got := stderr.String()
			if code != 2 {
				t.Fatalf("exit %d, want 2\n%s", code, got)
			}
			if !strings.Contains(got, requireTokenMessage) {
				t.Fatalf("stderr %q", got)
			}
			if strings.Contains(got, "no token on TCP") || strings.Contains(got, plantedSecret) {
				t.Fatalf("stderr %q", got)
			}
		})
	}
}

func TestPlainListenWithoutTokenStillWarns(t *testing.T) {
	if os.Getenv("ZB_REQUIRE_CHILD") == "1" {
		os.Exit(runRequireChild())
	}
	t.Setenv("ZERO_BRIDGE_TOKEN", "")
	t.Setenv("ZERO_BRIDGE_REQUIRE_TOKEN", "")
	status, stderr := serveRequire(t, nil, "")
	if status != http.StatusOK {
		t.Fatalf("GET /v1/capabilities = %d, want 200\n%s", status, stderr)
	}
	if !strings.Contains(stderr, "no token on TCP") || !strings.Contains(stderr, "POST /a2a/message/send") {
		t.Fatalf("stderr %q", stderr)
	}
	if strings.Contains(stderr, requireTokenMessage) {
		t.Fatalf("plain listen refused: %s", stderr)
	}
}

func TestRequireTokenWithTokenRejectsAnonymous(t *testing.T) {
	if os.Getenv("ZB_REQUIRE_CHILD") == "1" {
		os.Exit(runRequireChild())
	}
	t.Setenv("ZERO_BRIDGE_TOKEN", "")
	status, stderr := serveRequire(t, []string{"--require-token", "--token-file", writeRequireToken(t)}, "")
	if status != http.StatusUnauthorized {
		t.Fatalf("GET /v1/capabilities = %d, want 401\n%s", status, stderr)
	}
	if strings.Contains(stderr, "no token on TCP") || strings.Contains(stderr, plantedSecret) {
		t.Fatalf("stderr %q", stderr)
	}
}

func runRequireChild() int {
	raw, err := os.ReadFile(os.Getenv("ZB_REQUIRE_ARGFILE"))
	if err != nil {
		os.Stderr.WriteString(err.Error() + "\n")
		return 2
	}
	var args []string
	if err := json.Unmarshal(raw, &args); err != nil {
		os.Stderr.WriteString(err.Error() + "\n")
		return 2
	}
	return runArgs(args, os.Stderr)
}

func writeRequireToken(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "token.txt")
	if err := os.WriteFile(path, []byte(strings.Repeat("ab", 16)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func serveRequire(t *testing.T, extra []string, requireEnv string) (int, string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	agent := buildAcpfake(t, repoRoot(t))
	args := []string{
		"--listen", addr,
		"--agent-cmd", agent,
		"--agent-arg", "acp",
		"--profile", "generic",
		"--state-dir", t.TempDir(),
	}
	args = append(args, extra...)
	argFile := filepath.Join(t.TempDir(), "args.json")
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(argFile, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestPlainListenWithoutTokenStillWarns$|^TestRequireTokenWithTokenRejectsAnonymous$")
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + t.TempDir(),
		"ZB_REQUIRE_CHILD=1",
		"ZB_REQUIRE_ARGFILE=" + argFile,
		"ZERO_BRIDGE_REQUIRE_TOKEN=" + requireEnv,
		"GOFLAGS=-buildvcs=false",
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	url := "http://" + addr + "/v1/capabilities"
	deadline := time.Now().Add(15 * time.Second)
	var last error
	status := 0
	answered := false
	for time.Now().Before(deadline) {
		res, err := http.Get(url)
		if err != nil {
			last = err
			time.Sleep(50 * time.Millisecond)
			continue
		}
		res.Body.Close()
		status = res.StatusCode
		answered = true
		break
	}
	if !stopRequire(t, cmd) {
		t.Fatal("bridge did not exit after SIGTERM")
	}
	got := stderr.String()
	if !answered {
		t.Fatalf("bridge did not answer: %v\n%s", last, got)
	}
	return status, got
}

// stopRequire signals the child and waits until cmd.Wait returns.
// Wait joins the stderr copy, so the buffer is safe to read afterwards.
func stopRequire(t *testing.T, cmd *exec.Cmd) bool {
	t.Helper()
	if cmd.Process == nil {
		return true
	}
	_ = cmd.Process.Signal(syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(5 * time.Second):
		return false
	}
}
