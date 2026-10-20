package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTCPWithoutTokenWarns(t *testing.T) {
	t.Setenv("ZERO_BRIDGE_TOKEN", "")
	var stderr bytes.Buffer
	code := runArgs([]string{"--listen", "127.0.0.1:0", "--agent-cmd", "/bin/true"}, &stderr)
	if code == 0 {
		t.Fatal("server stayed up")
	}
	got := stderr.String()
	if !strings.Contains(got, "no token") || !strings.Contains(got, "POST /a2a/message/send") || !strings.Contains(got, "POST /v1/chat/completions") || !strings.Contains(got, "local user") {
		t.Fatalf("stderr %q", got)
	}
}

func TestUnixWithoutTokenDoesNotWarn(t *testing.T) {
	t.Setenv("ZERO_BRIDGE_TOKEN", "")
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	code := runArgs([]string{"--listen", "unix:" + filepath.Join(dir, "bridge.sock"), "--agent-cmd", "/bin/true"}, &stderr)
	if code == 0 {
		t.Fatal("server stayed up")
	}
	if strings.Contains(stderr.String(), "no token") {
		t.Fatalf("unix warned: %s", stderr.String())
	}
}

func TestTCPWithTokenDoesNotWarn(t *testing.T) {
	t.Setenv("ZERO_BRIDGE_TOKEN", "0123456789abcdef0123456789abcdef")
	var stderr bytes.Buffer
	code := runArgs([]string{"--listen", "127.0.0.1:0", "--agent-cmd", "/bin/true"}, &stderr)
	if code == 0 {
		t.Fatal("server stayed up")
	}
	if strings.Contains(stderr.String(), "no token") {
		t.Fatalf("token warned: %s", stderr.String())
	}
}
