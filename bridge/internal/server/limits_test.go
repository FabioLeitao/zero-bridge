package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolveLimitsEdges(t *testing.T) {
	ok, err := ResolveLimits(map[string]string{"session-ttl": "720h"}, nil, nil)
	if err != nil || ok.SessionTTL != MaxSessionTTL {
		t.Fatalf("cap: %v %s", err, ok.SessionTTL)
	}
	for _, raw := range []string{"0", "-1", "unlimited", "720h1m"} {
		if _, err := ResolveLimits(map[string]string{"session-ttl": raw}, nil, nil); err == nil {
			t.Fatalf("%s accepted", raw)
		}
	}
	if _, err := ResolveLimits(map[string]string{"session-ttl": "1h", "session-idle": "2h"}, nil, nil); err == nil {
		t.Fatal("idle longer than ttl accepted")
	}
	same, err := ResolveLimits(map[string]string{"session-ttl": "2h", "session-idle": "2h"}, nil, nil)
	if err != nil || same.SessionIdle != 2*time.Hour {
		t.Fatalf("idle at ttl: %v", err)
	}
	if _, err := ResolveLimits(map[string]string{"max-sessions": "64"}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveLimits(map[string]string{"max-sessions": "65"}, nil, nil); err == nil {
		t.Fatal("max-sessions cap")
	}
	if _, err := ResolveLimits(map[string]string{"max-sessions": "0"}, nil, nil); err == nil {
		t.Fatal("max-sessions zero")
	}
	if _, err := ResolveLimits(map[string]string{"auth-fail-window": "1s"}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveLimits(map[string]string{"auth-fail-window": "1ms"}, nil, nil); err == nil {
		t.Fatal("window floor")
	}
	if _, err := ResolveLimits(map[string]string{"auth-fail-window": "1h1s"}, nil, nil); err == nil {
		t.Fatal("window cap")
	}
	if _, err := ResolveLimits(map[string]string{"auth-fail-limit": "100"}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveLimits(map[string]string{"auth-fail-limit": "unlimited"}, nil, nil); err == nil {
		t.Fatal("limit unlimited")
	}
	got, err := ResolveLimits(
		map[string]string{"session-ttl": "3h"},
		map[string]string{"session-ttl": "2h"},
		map[string]string{"session-ttl": "1h"},
	)
	if err != nil || got.SessionTTL != 3*time.Hour {
		t.Fatalf("precedence %v %s", err, got.SessionTTL)
	}
}

func TestLoadLimitFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bridge.conf")
	body := "# note\n\nsession-ttl 4h\nmax-sessions\t3\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadLimitFile(path)
	if err != nil || got["session-ttl"] != "4h" || got["max-sessions"] != "3" {
		t.Fatalf("%v %v", err, got)
	}
	bad := filepath.Join(dir, "bad.conf")
	if err := os.WriteFile(bad, []byte("nope 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadLimitFile(bad); err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("unknown: %v", err)
	}
}
