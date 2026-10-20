package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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
