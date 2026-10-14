package server

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestPermissionAllowAndReplay(t *testing.T) {
	_, base, cleanup := startTestServer(t, []string{"ZB_FAKE_PERMISSION=1"})
	defer cleanup()
	dir := t.TempDir()
	sid := createSession(t, base, dir)
	postPromptAsync(base, sid, "go")
	reqID := waitPending(t, base, sid)
	res := postDecision(t, base, sid, reqID, "allow")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("allow %d", res.StatusCode)
	}
	again := postDecision(t, base, sid, reqID, "allow")
	if again.StatusCode != http.StatusConflict {
		t.Fatalf("repeat %d", again.StatusCode)
	}
	missing := postDecision(t, base, sid, "deadbeef", "deny")
	if missing.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown %d", missing.StatusCode)
	}
	lines := collectSSE(t, base+"/v1/sessions/"+sid+"/events", 3*time.Second)
	blob := strings.Join(lines, "\n")
	if !strings.Contains(blob, "ALLOWED optionId=allow") || !strings.Contains(blob, "permission_request") || !strings.Contains(blob, "permission_resolved") {
		t.Fatalf("sse %s", blob)
	}
}

func TestPermissionDenyAndOtherSession(t *testing.T) {
	_, base, cleanup := startTestServer(t, []string{"ZB_FAKE_PERMISSION=1"})
	defer cleanup()
	dir := t.TempDir()
	a := createSession(t, base, dir)
	b := createSession(t, base, dir)
	postPromptAsync(base, a, "go")
	reqID := waitPending(t, base, a)
	other := postDecision(t, base, b, reqID, "deny")
	if other.StatusCode != http.StatusNotFound {
		t.Fatalf("other session %d", other.StatusCode)
	}
	res := postDecision(t, base, a, reqID, "deny")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("deny %d", res.StatusCode)
	}
	lines := collectSSE(t, base+"/v1/sessions/"+a+"/events", 3*time.Second)
	if !strings.Contains(strings.Join(lines, "\n"), "DENIED optionId=deny") {
		t.Fatalf("sse %v", lines)
	}
}

func TestPermissionTimeout(t *testing.T) {
	_, base, cleanup := startTestServerTimeout(t, []string{"ZB_FAKE_PERMISSION=1"}, 300*time.Millisecond)
	defer cleanup()
	dir := t.TempDir()
	sid := createSession(t, base, dir)
	postPromptAsync(base, sid, "go")
	reqID := waitPending(t, base, sid)
	time.Sleep(500 * time.Millisecond)
	late := postDecision(t, base, sid, reqID, "allow")
	if late.StatusCode != http.StatusConflict {
		b, _ := io.ReadAll(late.Body)
		t.Fatalf("late %d %s", late.StatusCode, b)
	}
	late.Body.Close()
}

func TestAllowDoesNotSelectAlways(t *testing.T) {
	_, base, cleanup := startTestServer(t, []string{"ZB_FAKE_PERMISSION=1", "ZB_FAKE_PERMISSION_OPTIONS=always"})
	defer cleanup()
	dir := t.TempDir()
	sid := createSession(t, base, dir)
	postPromptAsync(base, sid, "go")
	reqID := waitPending(t, base, sid)
	res := postDecision(t, base, sid, reqID, "allow")
	res.Body.Close()
	lines := collectSSE(t, base+"/v1/sessions/"+sid+"/events", 3*time.Second)
	blob := strings.Join(lines, "\n")
	if strings.Contains(blob, "always_allow") && strings.Contains(blob, "ALLOWED") {
		t.Fatalf("widened grant: %s", blob)
	}
	if !strings.Contains(blob, "DENIED optionId=deny") {
		t.Fatalf("sse %s", blob)
	}
}

func TestPermissionBadDecision(t *testing.T) {
	_, base, cleanup := startTestServer(t, []string{"ZB_FAKE_PERMISSION=1"})
	defer cleanup()
	dir := t.TempDir()
	sid := createSession(t, base, dir)
	postPromptAsync(base, sid, "go")
	reqID := waitPending(t, base, sid)
	res := postDecision(t, base, sid, reqID, "always")
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func waitPending(t *testing.T, base, sid string) string {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		res, err := http.Get(base + "/v1/sessions/" + sid + "/permissions/pending")
		if err != nil {
			t.Fatal(err)
		}
		var body struct {
			Pending *struct {
				ReqID string `json:"reqId"`
			} `json:"pending"`
		}
		_ = json.NewDecoder(res.Body).Decode(&body)
		res.Body.Close()
		if body.Pending != nil && body.Pending.ReqID != "" {
			return body.Pending.ReqID
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("no pending permission")
	return ""
}

func postDecision(t *testing.T, base, sid, reqID, decision string) *http.Response {
	t.Helper()
	res, err := http.Post(base+"/v1/sessions/"+sid+"/permissions/"+reqID, "application/json", strings.NewReader(`{"decision":"`+decision+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func startTestServerTimeout(t *testing.T, extraEnv []string, timeout time.Duration) (*Server, string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	env := append([]string{"ZB_FAKE_AGENT=1"}, extraEnv...)
	srv, err := New(Config{
		Listen:            addr,
		ZeroBin:           os.Args[0],
		ExtraEnv:          env,
		PermissionTimeout: timeout,
		Logger:            log.New(io.Discard, "", 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.Boot(context.Background()); err != nil {
		t.Fatal(err)
	}
	httpSrv := srv.newHTTPServer()
	go httpSrv.Serve(ln)
	return srv, "http://" + addr, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(ctx)
		_ = srv.Shutdown(ctx)
	}
}
