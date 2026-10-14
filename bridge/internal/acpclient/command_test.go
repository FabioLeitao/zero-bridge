package acpclient

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveCommandRejectsEmpty(t *testing.T) {
	_, err := ResolveCommand("", []string{"acp"})
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("got %v", err)
	}
}

func TestResolveCommandMissingName(t *testing.T) {
	_, err := ResolveCommand("zero-bridge-no-such-agent", []string{"acp"})
	if err == nil || !strings.Contains(err.Error(), "agent command") {
		t.Fatalf("got %v", err)
	}
}

func TestResolveCommandAbsoluteMissing(t *testing.T) {
	_, err := ResolveCommand("/no/such/zero-bridge-agent", []string{"acp"})
	if err == nil || !strings.Contains(err.Error(), "agent command") {
		t.Fatalf("got %v", err)
	}
}

func TestResolveCommandKeepsSeparateArgs(t *testing.T) {
	cmd, err := ResolveCommand("/bin/true", []string{"acp", "a b"})
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Path != "/bin/true" {
		t.Fatalf("path %q", cmd.Path)
	}
	if len(cmd.Args) != 2 || cmd.Args[0] != "acp" || cmd.Args[1] != "a b" {
		t.Fatalf("args %#v", cmd.Args)
	}
	for _, a := range append([]string{cmd.Path}, cmd.Args...) {
		if strings.Contains(a, "ZERO_BRIDGE_TOKEN") || strings.Contains(a, "secret") {
			t.Fatalf("argv carries a secret-shaped value: %q", a)
		}
	}
}

func TestResolveCommandLookPath(t *testing.T) {
	cmd, err := ResolveCommand("true", []string{"acp"})
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(cmd.Path) {
		t.Fatalf("path %q", cmd.Path)
	}
	if len(cmd.Args) != 1 || cmd.Args[0] != "acp" {
		t.Fatalf("args %#v", cmd.Args)
	}
}

func TestResolveCommandRejectsDirectory(t *testing.T) {
	dir := t.TempDir()
	_, err := ResolveCommand(dir, nil)
	if err == nil || !strings.Contains(err.Error(), "directory") {
		t.Fatalf("got %v", err)
	}
}

func TestResolveCommandRejectsNonExecutable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent")
	if err := os.WriteFile(path, []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ResolveCommand(path, nil)
	if err == nil || !strings.Contains(err.Error(), "not executable") {
		t.Fatalf("got %v", err)
	}
}
