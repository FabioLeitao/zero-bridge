package main

import (
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

const deployTokenPath = "/var/lib/zerobridge/.local/state/zero-bridge/token.txt"

// Issue 84 owns this name. Any other deploy env that no Go reads fails the test.
var knownUnreadDeployEnv = map[string]bool{"OTEL_SAMPLE": true}

func TestDeployStartRejectsAnonymous(t *testing.T) {
	if os.Getenv("ZB_DEPLOY_CHILD") == "1" {
		raw, err := os.ReadFile(os.Getenv("ZB_DEPLOY_ARGFILE"))
		if err != nil {
			os.Stderr.WriteString(err.Error() + "\n")
			os.Exit(2)
		}
		var args []string
		if err := json.Unmarshal(raw, &args); err != nil {
			os.Stderr.WriteString(err.Error() + "\n")
			os.Exit(2)
		}
		os.Exit(runArgs(args, os.Stderr))
	}

	root := repoRoot(t)
	starts := shippedStarts(t, root)
	agent := buildAcpfake(t, root)
	tokenPath := writeDeployToken(t)
	stateDir := t.TempDir()
	for _, start := range starts {
		t.Run(start.name, func(t *testing.T) {
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			addr := ln.Addr().String()
			ln.Close()
			args := hostArgs(start.args, addr, tokenPath, stateDir, agent)
			status := anonymousCapabilities(t, args, addr)
			if status != http.StatusUnauthorized {
				t.Fatalf("GET /v1/capabilities = %d, want 401", status)
			}
		})
	}
}

func TestDeploySpecsNameOnlyReadEnvAndLoadToken(t *testing.T) {
	root := repoRoot(t)
	names := deployEnvNames(t, root)
	read := goReadsEnv(t, filepath.Join(root, "bridge"), names)
	var unread []string
	for _, name := range names {
		if read[name] || knownUnreadDeployEnv[name] {
			continue
		}
		unread = append(unread, name)
	}
	if len(unread) > 0 {
		t.Fatalf("deploy env not read by Go: %s", strings.Join(unread, ", "))
	}
	for _, start := range shippedStarts(t, root) {
		if !tokenFlag(start.args) {
			t.Fatalf("%s start does not pass --token-file %s: %q", start.name, deployTokenPath, start.args)
		}
	}
}

type shippedStart struct {
	name string
	args []string
}

func shippedStarts(t *testing.T, root string) []shippedStart {
	t.Helper()
	docker := string(mustRead(t, filepath.Join(root, "Dockerfile")))
	compose := string(mustRead(t, filepath.Join(root, "docker-compose.yml")))
	quadlet := string(mustRead(t, filepath.Join(root, "zero-bridge.container")))
	kube := string(mustRead(t, filepath.Join(root, "pod-zero-bridge.kube")))
	image := dockerfileCmd(t, docker)
	return []shippedStart{
		{name: "dockerfile", args: image},
		{name: "compose", args: composeStart(t, compose, image)},
		{name: "quadlet", args: quadletExec(t, quadlet)},
		{name: "kube", args: kubeStart(t, kube, image)},
	}
}

func dockerfileCmd(t *testing.T, text string) []string {
	t.Helper()
	idx := strings.LastIndex(text, "\nCMD ")
	if idx < 0 {
		t.Fatal("Dockerfile has no CMD")
	}
	line := strings.TrimSpace(strings.Split(text[idx+1:], "\n")[0])
	line = strings.TrimPrefix(line, "CMD ")
	var args []string
	if err := json.Unmarshal([]byte(line), &args); err != nil {
		t.Fatalf("Dockerfile CMD: %v", err)
	}
	return args
}

func composeStart(t *testing.T, text string, image []string) []string {
	t.Helper()
	block := serviceBlock(text, "  zero-bridge:")
	for _, line := range strings.Split(block, "\n") {
		if !strings.HasPrefix(line, "    command:") {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(line, "    command:"))
		if rest == "" {
			t.Fatal("compose command is not a single line")
		}
		return parseCommand(t, rest)
	}
	return image
}

func quadletExec(t *testing.T, text string) []string {
	t.Helper()
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "Exec=") {
			return strings.Fields(strings.TrimPrefix(line, "Exec="))
		}
	}
	t.Fatal("quadlet has no Exec")
	return nil
}

func kubeStart(t *testing.T, text string, image []string) []string {
	t.Helper()
	block := serviceBlock(text, "    - name: zero-bridge")
	for _, line := range strings.Split(block, "\n") {
		if strings.HasPrefix(line, "      command:") || strings.HasPrefix(line, "      args:") {
			rest := strings.TrimSpace(line)
			rest = rest[strings.Index(rest, ":")+1:]
			rest = strings.TrimSpace(rest)
			if rest == "" {
				t.Fatal("kube command is not a single line")
			}
			return parseCommand(t, rest)
		}
	}
	return image
}

func parseCommand(t *testing.T, rest string) []string {
	t.Helper()
	if strings.HasPrefix(rest, "[") {
		var args []string
		if err := json.Unmarshal([]byte(rest), &args); err != nil {
			t.Fatalf("command: %v", err)
		}
		return args
	}
	return strings.Fields(rest)
}

func serviceBlock(text, header string) string {
	idx := strings.Index(text, header)
	if idx < 0 {
		return ""
	}
	rest := text[idx+len(header):]
	lines := strings.Split(rest, "\n")
	var b strings.Builder
	for _, line := range lines {
		if line != "" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			break
		}
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "   ") && strings.TrimSpace(line) != "" && header == "  zero-bridge:" {
			break
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func tokenFlag(args []string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--token-file" && args[i+1] == deployTokenPath {
			return true
		}
	}
	return false
}

func deployEnvNames(t *testing.T, root string) []string {
	t.Helper()
	var names []string
	names = append(names, composeEnv(string(mustRead(t, filepath.Join(root, "docker-compose.yml"))))...)
	names = append(names, quadletEnv(string(mustRead(t, filepath.Join(root, "zero-bridge.container"))))...)
	names = append(names, kubeEnv(string(mustRead(t, filepath.Join(root, "pod-zero-bridge.kube"))))...)
	names = append(names, dockerfileEnv(string(mustRead(t, filepath.Join(root, "Dockerfile"))))...)
	return names
}

func composeEnv(text string) []string {
	block := serviceBlock(text, "  zero-bridge:")
	var names []string
	in := false
	for _, line := range strings.Split(block, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "environment:" {
			in = true
			continue
		}
		if in && !strings.HasPrefix(trim, "-") {
			if trim != "" {
				in = false
			}
			continue
		}
		if in {
			item := strings.TrimPrefix(trim, "-")
			item = strings.TrimSpace(item)
			if key, _, ok := strings.Cut(item, "="); ok && key != "" {
				names = append(names, key)
			}
		}
	}
	return names
}

func quadletEnv(text string) []string {
	var names []string
	for _, line := range strings.Split(text, "\n") {
		if !strings.HasPrefix(line, "Environment=") {
			continue
		}
		item := strings.TrimPrefix(line, "Environment=")
		if key, _, ok := strings.Cut(item, "="); ok && key != "" {
			names = append(names, key)
		}
	}
	return names
}

func kubeEnv(text string) []string {
	block := serviceBlock(text, "    - name: zero-bridge")
	var names []string
	in := false
	for _, line := range strings.Split(block, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "env:" {
			in = true
			continue
		}
		if in && (strings.HasPrefix(trim, "volumeMounts:") || strings.HasPrefix(trim, "resources:")) {
			break
		}
		if in && strings.HasPrefix(trim, "- name:") {
			names = append(names, strings.TrimSpace(strings.TrimPrefix(trim, "- name:")))
		}
	}
	return names
}

func dockerfileEnv(text string) []string {
	var names []string
	for _, line := range strings.Split(text, "\n") {
		trim := strings.TrimSpace(line)
		if !strings.HasPrefix(trim, "ENV ") {
			continue
		}
		rest := strings.TrimPrefix(trim, "ENV ")
		if key, _, ok := strings.Cut(rest, "="); ok {
			names = append(names, key)
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) > 0 {
			names = append(names, fields[0])
		}
	}
	return names
}

func goReadsEnv(t *testing.T, root string, names []string) map[string]bool {
	t.Helper()
	found := map[string]bool{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(b)
		for _, name := range names {
			if strings.Contains(text, `"`+name+`"`) {
				found[name] = true
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return found
}

func hostArgs(spec []string, listen, tokenPath, stateDir, agent string) []string {
	args := append([]string{}, spec...)
	if len(args) > 0 && (args[0] == "zero-bridge" || strings.HasSuffix(args[0], "/zero-bridge")) {
		args = args[1:]
	}
	for i := 0; i < len(args)-1; i++ {
		switch args[i] {
		case "--listen":
			args[i+1] = listen
		case "--token-file":
			args[i+1] = tokenPath
		}
	}
	return append(args,
		"--agent-cmd", agent,
		"--agent-arg", "acp",
		"--profile", "generic",
		"--state-dir", stateDir,
	)
}

func writeDeployToken(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "token.txt")
	if err := os.WriteFile(path, []byte(strings.Repeat("ab", 16)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func buildAcpfake(t *testing.T, root string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "acpfake")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/acpfake")
	cmd.Dir = filepath.Join(root, "bridge")
	cmd.Env = append(os.Environ(), "GOFLAGS=-buildvcs=false")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build acpfake: %v\n%s", err, out)
	}
	return bin
}

func anonymousCapabilities(t *testing.T, args []string, addr string) int {
	t.Helper()
	argFile := filepath.Join(t.TempDir(), "args.json")
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(argFile, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestDeployStartRejectsAnonymous$")
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + t.TempDir(),
		"ZB_DEPLOY_CHILD=1",
		"ZB_DEPLOY_ARGFILE=" + argFile,
		"GOFLAGS=-buildvcs=false",
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer stopChild(t, cmd)
	url := "http://" + addr + "/v1/capabilities"
	deadline := time.Now().Add(15 * time.Second)
	var last error
	for time.Now().Before(deadline) {
		res, err := http.Get(url)
		if err != nil {
			last = err
			time.Sleep(50 * time.Millisecond)
			continue
		}
		res.Body.Close()
		return res.StatusCode
	}
	t.Fatalf("bridge did not answer: %v", last)
	return 0
}

func stopChild(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	if cmd.Process == nil {
		return
	}
	_ = cmd.Process.Signal(syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		_, _ = cmd.Process.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Errorf("bridge did not exit after SIGTERM")
	}
}
