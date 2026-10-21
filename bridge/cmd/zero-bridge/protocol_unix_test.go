//go:build unix

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestInitializeSilenceExits(t *testing.T) {
	bin := buildAcpFake(t)
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	t.Setenv("ZB_FAKE_SILENT_INIT", "1")
	t.Setenv("ZB_FAKE_PID_FILE", pidFile)
	t.Cleanup(func() { stopRecordedChild(pidFile) })
	var stderr bytes.Buffer
	code := make(chan int, 1)
	go func() {
		code <- runArgs([]string{"--listen", "127.0.0.1:0", "--agent-cmd", bin, "--init-timeout", "1s"}, &stderr)
	}()
	select {
	case got := <-code:
		if got == 0 {
			t.Fatal("exit 0")
		}
		line := timeoutLine(stderr.String())
		if !strings.Contains(line, "initialize timed out") || strings.Contains(line, "/") || strings.Contains(line, "acptest") || strings.Contains(line, "sess-") {
			t.Fatalf("stderr %q", stderr.String())
		}
		if childAlive(pidFile) {
			t.Fatal("child still running")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("boot still running")
	}
}

func timeoutLine(stderr string) string {
	for _, line := range strings.Split(stderr, "\n") {
		if strings.Contains(line, "initialize timed out") {
			return line
		}
	}
	return ""
}

func childAlive(pidFile string) bool {
	b, err := os.ReadFile(pidFile)
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 0 {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}

func stopRecordedChild(pidFile string) {
	b, err := os.ReadFile(pidFile)
	if err != nil {
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 0 {
		return
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
}
