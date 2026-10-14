package auth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const longToken = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestLoadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "token")
	if err := os.WriteFile(path, []byte(longToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != longToken {
		t.Fatalf("token len %d", len(got))
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFile(path); err == nil || !strings.Contains(err.Error(), "group or others") {
		t.Fatalf("mode: %v", err)
	}
	short := filepath.Join(dir, "short")
	if err := os.WriteFile(short, []byte("too-short\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFile(short); err == nil || !strings.Contains(err.Error(), "at least") {
		t.Fatalf("short: %v", err)
	}
}

func TestEqual(t *testing.T) {
	if !Equal(longToken, longToken) {
		t.Fatal("same")
	}
	if Equal(longToken, longToken+"x") || Equal("", longToken) {
		t.Fatal("different")
	}
}
