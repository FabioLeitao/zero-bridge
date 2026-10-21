package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestUnknownFlagExitsNamingTheArgument(t *testing.T) {
	const secret = "sk-test-secret-value-not-a-real-key"
	// One-character typos of the security flags that exist. Plurals are
	// not flags: --allowed-hosts is --allowed-host plus one s.
	typos := []string{
		"--tokn-file",
		"--allowed-hosts",
		"--public-origins",
		"--trust-prox",
		"--audit-lg",
		"--require-tokn",
	}
	for _, flag := range typos {
		var stderr bytes.Buffer
		code := runArgs([]string{"--agent-cmd", "/bin/true", flag, secret}, &stderr)
		msg := stderr.String()
		if code != 2 || !strings.Contains(msg, flag) || strings.Contains(msg, secret) {
			t.Fatalf("%s code %d stderr %q", flag, code, msg)
		}
	}
	const canary = "SEGREDO_CANARIO"
	var stderr bytes.Buffer
	code := runArgs([]string{"--agent-cmd", "/bin/true", "--tokn=" + canary}, &stderr)
	msg := stderr.String()
	if code != 2 || !strings.Contains(msg, "--tokn") || strings.Contains(msg, canary) {
		t.Fatalf("glued code %d stderr %q", code, msg)
	}
	stderr.Reset()
	code = runArgs([]string{"--agent-cmd", "/bin/true", canary}, &stderr)
	msg = stderr.String()
	if code != 2 || strings.Contains(msg, canary) || !strings.Contains(msg, "unexpected positional argument") {
		t.Fatalf("positional code %d stderr %q", code, msg)
	}
}

func TestHealthcheckStaysBeforeFlagParse(t *testing.T) {
	var stderr bytes.Buffer
	code := runArgs([]string{"--healthcheck"}, &stderr)
	msg := stderr.String()
	if code != 2 || !strings.Contains(msg, "needs one URL") || strings.Contains(msg, "unknown argument") {
		t.Fatalf("healthcheck code %d stderr %q", code, msg)
	}
}

func TestShippedFlagsStillParse(t *testing.T) {
	sets := [][]string{
		{"--listen", "127.0.0.1:8765", "--agent-cmd", "/bin/true", "--agent-arg", "acp", "--token-file", "/tmp/token", "--permission-timeout", "60s", "--workspace-root", "/tmp"},
		{"--listen", "127.0.0.1:8765", "--token-file", "/tmp/token", "--require-token", "--agent-cmd", "/bin/true"},
		{"--agent-cmd", "/bin/true", "--exit-with-parent"},
		{"--listen", "127.0.0.1:8765", "--zero-bin", "/bin/true", "--permission-timeout", "3s", "--state-dir", "/tmp/state", "--token-file", "/tmp/token", "--workspace", "/tmp/ws", "--workspace-root", "/tmp/ws"},
		{"--agent-cmd", "/bin/true", "--trust-proxy", "--allowed-host", "bridge.test", "--public-origin", "https://bridge.test", "--audit-log", "/tmp/audit"},
	}
	for _, args := range sets {
		if _, err := parseCLI(args, &bytes.Buffer{}); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
}
