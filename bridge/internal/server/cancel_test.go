package server

import (
	"encoding/json"
	"io"
	"net/http"
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

func TestCancelRouteEndsHeldPrompt(t *testing.T) {
	ready := filepath.Join(t.TempDir(), "ready")
	srv, base, cleanup := startTestServer(t, []string{
		"ZB_FAKE_HOLD_PROMPT=1",
		"ZB_FAKE_HOLD_READY=" + ready,
	})
	defer cleanup()
	sid := createSession(t, base, t.TempDir())
	postPromptAsync(base, sid, "hold")
	waitFile(t, ready)
	got := postCancel(t, base, sid, "", "")
	if got.status != http.StatusOK || strings.Contains(got.body, sid) {
		t.Fatalf("cancel %#v", got)
	}
	if reason := waitStopReason(t, srv, sid, "cancelled"); reason != "cancelled" {
		t.Fatalf("stop %q", reason)
	}
}

func TestCancelRouteAuthAndDefinedStatus(t *testing.T) {
	_, base, cleanup := startConfigured(t, Config{
		Token:             bearerToken,
		LoginFailureDelay: -1,
	})
	defer cleanup()
	noAuth := postCancel(t, base, "missing-session", "", "")
	if noAuth.status != http.StatusUnauthorized || strings.Contains(noAuth.body, "missing-session") || !strings.Contains(noAuth.body, `"error":"unauthorized"`) {
		t.Fatalf("no auth %#v", noAuth)
	}
	loginReq, err := http.NewRequest(http.MethodPost, base+"/v1/login", strings.NewReader(`{"token":"`+bearerToken+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	loginReq.Header.Set("Content-Type", "application/json")
	loginRes, err := http.DefaultClient.Do(loginReq)
	if err != nil {
		t.Fatal(err)
	}
	loginRes.Body.Close()
	var session *http.Cookie
	for _, c := range loginRes.Cookies() {
		if c.Name == sessionCookie {
			session = c
		}
	}
	if session == nil {
		t.Fatal("login cookie missing")
	}
	cookie := postCancel(t, base, "missing-session", "", session.Name+"="+session.Value)
	if cookie.status != http.StatusForbidden || strings.Contains(cookie.body, "missing-session") || !strings.Contains(cookie.body, `"error":"forbidden"`) {
		t.Fatalf("cookie %#v", cookie)
	}
	missing := postCancel(t, base, "missing-session", bearerToken, "")
	if missing.status != http.StatusNotFound || missing.status >= 500 || !strings.Contains(missing.body, `"error":"session not found"`) || strings.Contains(missing.body, "missing-session") {
		t.Fatalf("missing %#v", missing)
	}
	dir := t.TempDir()
	sid := createSessionBearer(t, base, dir, bearerToken)
	idle := postCancel(t, base, sid, bearerToken, "")
	if idle.status != http.StatusOK || idle.status >= 500 || strings.Contains(idle.body, sid) || !strings.Contains(idle.body, `"ok":true`) {
		t.Fatalf("idle %#v", idle)
	}
}

type cancelHTTP struct {
	status int
	body   string
}

func createSessionBearer(t *testing.T, base, dir, bearer string) string {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, base+"/v1/sessions", strings.NewReader(`{"workspace":"`+dir+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bearer)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("session %d %s", res.StatusCode, b)
	}
	var out struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(b, &out); err != nil || out.SessionID == "" {
		t.Fatalf("session body %s", b)
	}
	return out.SessionID
}

func postCancel(t *testing.T, base, sid, bearer, cookie string) cancelHTTP {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, base+"/v1/sessions/"+sid+"/cancel", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return cancelHTTP{status: res.StatusCode, body: string(b)}
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
