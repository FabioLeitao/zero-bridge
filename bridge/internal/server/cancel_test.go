package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCancelEndsHeldPrompt(t *testing.T) {
	ready := filepath.Join(t.TempDir(), "ready")
	srv, base, cleanup := startTestServer(t, []string{
		"ZB_FAKE_HOLD_PROMPT=1",
		"ZB_FAKE_HOLD_READY=" + ready,
	})
	defer cleanup()
	dir := t.TempDir()
	sid := createSession(t, base, dir)
	postPromptAsync(base, sid, "first-marker")
	waitFile(t, ready)
	if err := srv.cancelPrompt(sid); err != nil {
		t.Fatal(err)
	}
	if got := waitStopReason(t, srv, sid, "cancelled"); got != "cancelled" {
		t.Fatalf("stop %q", got)
	}
	postPrompt(t, base, sid, "second-marker")
	if got := waitStopReason(t, srv, sid, "end_turn"); got != "end_turn" {
		t.Fatalf("next stop %q", got)
	}
}

func waitFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("prompt did not hold")
}

func waitStopReason(t *testing.T, srv *Server, sid, reason string) string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if seen := stopReasons(srv, sid); len(seen) > 0 {
			for _, got := range seen {
				if got == reason {
					return got
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	return strings.Join(stopReasons(srv, sid), ",")
}

func stopReasons(srv *Server, sid string) []string {
	srv.mu.RLock()
	h := srv.sessions[sid]
	srv.mu.RUnlock()
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []string
	for _, frame := range h.log {
		text := string(frame)
		if !strings.Contains(text, `"type":"stop"`) {
			continue
		}
		switch {
		case strings.Contains(text, `"stopReason":"cancelled"`):
			out = append(out, "cancelled")
		case strings.Contains(text, `"stopReason":"end_turn"`):
			out = append(out, "end_turn")
		default:
			out = append(out, "other")
		}
	}
	return out
}
