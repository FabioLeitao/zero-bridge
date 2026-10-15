package main

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestZeroBinAlias(t *testing.T) {
	var stderr bytes.Buffer
	cfg, err := parseCLI([]string{"--zero-bin", "/bin/true"}, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.agent.Path != "/bin/true" {
		t.Fatalf("path %q", cfg.agent.Path)
	}
	if len(cfg.agent.Args) != 1 || cfg.agent.Args[0] != "acp" {
		t.Fatalf("args %#v", cfg.agent.Args)
	}
	if cfg.profile != "zero" {
		t.Fatalf("profile %q", cfg.profile)
	}
	if !strings.Contains(stderr.String(), "deprecated") {
		t.Fatalf("stderr %q", stderr.String())
	}
}

func TestAgentArgRepeat(t *testing.T) {
	cfg, err := parseCLI([]string{
		"--agent-cmd", "/bin/true",
		"--agent-arg", "acp",
		"--agent-arg", "--verbose",
		"--profile", "generic",
	}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.agent.Path != "/bin/true" {
		t.Fatalf("path %q", cfg.agent.Path)
	}
	if len(cfg.agent.Args) != 2 || cfg.agent.Args[0] != "acp" || cfg.agent.Args[1] != "--verbose" {
		t.Fatalf("args %#v", cfg.agent.Args)
	}
	if cfg.profile != "generic" {
		t.Fatalf("profile %q", cfg.profile)
	}
}

func TestMissingAgentExit2(t *testing.T) {
	var stderr bytes.Buffer
	code := runArgs([]string{"--agent-cmd", "/no/such/zero-bridge-agent"}, &stderr)
	if code != 2 {
		t.Fatalf("exit %d stderr %q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "agent command") {
		t.Fatalf("stderr %q", stderr.String())
	}
}

func TestEmptyAgentCommandExit2(t *testing.T) {
	var stderr bytes.Buffer
	code := runArgs([]string{"--agent-cmd", ""}, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "empty") {
		t.Fatalf("exit %d stderr %q", code, stderr.String())
	}
}

func TestStateDirFlag(t *testing.T) {
	cfg, err := parseCLI([]string{"--agent-cmd", "/bin/true", "--state-dir", "/tmp/zb-state"}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.stateDir != "/tmp/zb-state" {
		t.Fatalf("state dir %q", cfg.stateDir)
	}
}

func TestOpenSessionMetaHidesPath(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "marker-state-path")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	if openSessionMeta(dir, logger) != nil {
		t.Fatal("loose directory was accepted")
	}
	if strings.Contains(buf.String(), "marker-state-path") {
		t.Fatalf("log leaked the path: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "not available") {
		t.Fatalf("log %q", buf.String())
	}
}

func TestUnknownProfileExit2(t *testing.T) {
	var stderr bytes.Buffer
	code := runArgs([]string{"--profile", "other", "--agent-cmd", "/bin/true"}, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "unknown profile") {
		t.Fatalf("exit %d stderr %q", code, stderr.String())
	}
}
