package main

import (
	"bytes"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseCLIMissingValuesAndBadNumbers(t *testing.T) {
	flags := []string{
		"--listen", "--workspace-root", "--state-dir", "--workspace",
		"--permission-timeout", "--init-timeout", "--allowed-host", "--public-origin",
		"--token-file", "--audit-log", "--config", "--otlp-endpoint",
		"--otel-service", "--otel-sample", "--session-ttl", "--agent-cmd",
		"--agent-arg", "--profile", "--zero-bin",
	}
	for _, flag := range flags {
		if _, err := parseCLI([]string{"--agent-cmd", "/bin/true", flag}, &bytes.Buffer{}); err == nil {
			t.Fatalf("%s missing value accepted", flag)
		}
	}
	if _, err := parseCLI([]string{"--agent-cmd", "/bin/true", "--permission-timeout", "nope"}, &bytes.Buffer{}); err == nil {
		t.Fatal("bad timeout accepted")
	}
	if _, err := parseCLI([]string{"--agent-cmd", "/bin/true", "--otel-sample", "nope"}, &bytes.Buffer{}); err == nil {
		t.Fatal("bad sample accepted")
	}
	cfg, err := parseCLI([]string{
		"--agent-cmd", "/bin/true",
		"--permission-timeout", "2s",
		"--otel-sample", "0.25",
		"--trust-proxy",
		"--allowed-host", "bridge.test",
		"--public-origin", "https://bridge.test",
		"--session-ttl", "1h",
	}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.permTimeout.String() != "2s" || cfg.otelSample != 0.25 || !cfg.trustProxy || cfg.initTimeout.String() != "30s" || cfg.exitWithParent {
		t.Fatalf("cfg %+v", cfg)
	}
	withParent, err := parseCLI([]string{"--agent-cmd", "/bin/true", "--exit-with-parent"}, &bytes.Buffer{})
	if err != nil || !withParent.exitWithParent {
		t.Fatalf("exit with parent %+v %v", withParent.exitWithParent, err)
	}
	short, err := parseCLI([]string{"--agent-cmd", "/bin/true", "--init-timeout", "1s"}, &bytes.Buffer{})
	if err != nil || short.initTimeout.String() != "1s" {
		t.Fatalf("init timeout %+v %v", short.initTimeout, err)
	}
	if _, err := parseCLI([]string{"--zero-bin", "/bin/true", "--agent-cmd", "/bin/true"}, &bytes.Buffer{}); err == nil {
		t.Fatal("combined launchers accepted")
	}
}

func TestRunArgsRejectsBadConfigTokenAndAudit(t *testing.T) {
	if code := runArgs([]string{"--healthcheck"}, &bytes.Buffer{}); code != 2 {
		t.Fatalf("healthcheck argc %d", code)
	}
	t.Setenv("ZERO_BRIDGE_TOKEN", "short")
	if code := runArgs([]string{"--agent-cmd", "/bin/true"}, &bytes.Buffer{}); code != 2 {
		t.Fatal("short token accepted")
	}
	t.Setenv("ZERO_BRIDGE_TOKEN", "")
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "token")
	if err := os.WriteFile(tokenPath, []byte("0123456789abcdef0123456789abcdef\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := runArgs([]string{"--agent-cmd", "/bin/true", "--token-file", tokenPath}, &bytes.Buffer{}); code != 2 {
		t.Fatal("loose token file accepted")
	}
	cfgPath := filepath.Join(dir, "limits")
	if err := os.WriteFile(cfgPath, []byte("not-a-key 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := runArgs([]string{"--agent-cmd", "/bin/true", "--config", cfgPath}, &bytes.Buffer{}); code != 2 {
		t.Fatal("bad config accepted")
	}
	var stderr bytes.Buffer
	code := runArgs([]string{"--agent-cmd", "/bin/true", "--audit-log", dir}, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "audit") {
		t.Fatalf("audit %d %q", code, stderr.String())
	}
}

func TestStateDirFromEnvAndFreshMeta(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", filepath.Join(t.TempDir(), "xdg"))
	if !strings.HasSuffix(defaultStateDir(), filepath.Join("xdg", "zero-bridge")) {
		t.Fatalf("state dir %q", defaultStateDir())
	}
	dir := filepath.Join(t.TempDir(), "fresh")
	if openSessionMeta(dir, log.New(io.Discard, "", 0)) == nil {
		t.Fatal("fresh state dir refused")
	}
	if openSessionMeta("", log.New(io.Discard, "", 0)) != nil {
		t.Fatal("empty dir opened")
	}
}

func TestPlaintextTelemetryStaysOff(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "")
	var stderr bytes.Buffer
	code := runArgs([]string{"--agent-cmd", "/bin/true", "--otlp-endpoint", "http://127.0.0.1:9"}, &stderr)
	if code == 0 {
		t.Fatal("server stayed up")
	}
	if !strings.Contains(stderr.String(), "telemetry disabled") || !strings.Contains(stderr.String(), "plaintext") {
		t.Fatalf("stderr %q", stderr.String())
	}
	t.Setenv("OTLP_ENDPOINT", "http://127.0.0.1:9")
	stderr.Reset()
	code = runArgs([]string{"--agent-cmd", "/bin/true"}, &stderr)
	if code == 0 || !strings.Contains(stderr.String(), "telemetry disabled") {
		t.Fatalf("env endpoint %d %q", code, stderr.String())
	}
}
