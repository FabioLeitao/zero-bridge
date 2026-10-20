package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestProtocolMismatchExits(t *testing.T) {
	bin := buildAcpFake(t)
	cases := []struct {
		env  string
		want string
	}{
		{env: "2", want: "sent 1, received 2"},
		{env: "omit", want: "sent 1, received none"},
	}
	for _, tc := range cases {
		t.Run(tc.env, func(t *testing.T) {
			t.Setenv("ZB_FAKE_PROTOCOL", tc.env)
			var stderr bytes.Buffer
			code := make(chan int, 1)
			go func() {
				code <- runArgs([]string{"--listen", "127.0.0.1:0", "--agent-cmd", bin}, &stderr)
			}()
			select {
			case got := <-code:
				if got == 0 {
					t.Fatal("exit 0")
				}
				line := mismatchLine(stderr.String())
				if !strings.Contains(line, tc.want) || strings.Contains(line, "acptest") || strings.Contains(line, "/") || strings.Contains(line, "sess-") {
					t.Fatalf("stderr %q", stderr.String())
				}
			case <-time.After(2 * time.Second):
				t.Fatal("boot hung")
			}
		})
	}
}

func TestInitTimeoutRejectsInvalid(t *testing.T) {
	for _, value := range []string{"0s", "-1s", "nope", "0"} {
		var stderr bytes.Buffer
		code := runArgs([]string{"--init-timeout", value, "--agent-cmd", "/bin/true"}, &stderr)
		if code != 2 {
			t.Fatalf("%s exit %d", value, code)
		}
		if strings.Contains(stderr.String(), "/") || strings.Contains(stderr.String(), "acptest") || strings.Contains(stderr.String(), "sess-") {
			t.Fatalf("%s stderr %q", value, stderr.String())
		}
		if !strings.Contains(stderr.String(), "init timeout must be greater than zero") {
			t.Fatalf("%s stderr %q", value, stderr.String())
		}
	}
}

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

func mismatchLine(stderr string) string {
	for _, line := range strings.Split(stderr, "\n") {
		if strings.Contains(line, "protocol mismatch:") {
			return line
		}
	}
	return ""
}

func buildAcpFake(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("no source path")
	}
	root := filepath.Dir(filepath.Dir(filepath.Dir(file)))
	bin := filepath.Join(t.TempDir(), "acpfake")
	cmd := exec.Command("go", "build", "-buildvcs=false", "-o", bin, "./cmd/acpfake")
	cmd.Dir = root
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("acpfake: %v %s", err, out)
	}
	return bin
}
