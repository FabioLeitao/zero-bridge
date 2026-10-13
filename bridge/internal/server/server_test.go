package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"log"

	"zero-bridge/bridge/internal/acptest"
)

func TestMain(m *testing.M) {
	acptest.MaybeMain()
	os.Exit(m.Run())
}

func startTestServer(t *testing.T, extraEnv []string) (*Server, string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	env := []string{"ZB_FAKE_AGENT=1"}
	env = append(env, extraEnv...)
	srv, err := New(Config{
		Listen:   addr,
		ZeroBin:  os.Args[0],
		ExtraEnv: env,
		Logger:   log.New(io.Discard, "", 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.Boot(context.Background()); err != nil {
		t.Fatal(err)
	}
	httpSrv := &http.Server{Handler: srv.Handler()}
	go httpSrv.Serve(ln)
	return srv, "http://" + addr, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(ctx)
		_ = srv.Shutdown(ctx)
	}
}

func TestHealthAndSecurityHeaders(t *testing.T) {
	_, base, cleanup := startTestServer(t, nil)
	defer cleanup()
	res, err := http.Get(base + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	if res.Header.Get("Content-Security-Policy") != headerCSP {
		t.Fatal("missing CSP")
	}
	if res.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("unexpected CORS")
	}
}

func TestHealthAgentDown(t *testing.T) {
	srv, base, cleanup := startTestServer(t, nil)
	_ = srv.acp.Close()
	time.Sleep(200 * time.Millisecond)
	res, err := http.Get(base + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status %d", res.StatusCode)
	}
	cleanup()
}

func TestForbiddenHost(t *testing.T) {
	_, base, cleanup := startTestServer(t, nil)
	defer cleanup()
	req, _ := http.NewRequest(http.MethodGet, base+"/health", nil)
	req.Host = "evil.example:8765"
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestPromptRequiresJSON(t *testing.T) {
	_, base, cleanup := startTestServer(t, nil)
	defer cleanup()
	dir := t.TempDir()
	sid := createSession(t, base, dir)
	req, _ := http.NewRequest(http.MethodPost, base+"/v1/sessions/"+sid+"/prompt", strings.NewReader(`{"text":"hi"}`))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestWorkspaceValidation(t *testing.T) {
	_, base, cleanup := startTestServer(t, nil)
	defer cleanup()
	res, err := http.Post(base+"/v1/sessions", "application/json", strings.NewReader(`{"workspace":"relative"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestDirectPromptCall(t *testing.T) {
	srv, _, cleanup := startTestServer(t, nil)
	defer cleanup()
	dir := t.TempDir()
	sid := createSessionHTTP(t, srv, dir)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var res struct {
		StopReason string `json:"stopReason"`
	}
	if err := srv.acpCall(ctx, "session/prompt", map[string]any{
		"sessionId": sid,
		"prompt":    []map[string]string{{"type": "text", "text": "hi"}},
	}, &res); err != nil {
		t.Fatal(err)
	}
	if res.StopReason != "end_turn" {
		t.Fatalf("stop %q", res.StopReason)
	}
	srv.mu.RLock()
	h := srv.sessions[sid]
	srv.mu.RUnlock()
	if h == nil || h.logLen() == 0 {
		t.Fatal("expected events in session log")
	}
}

func createSessionHTTP(t *testing.T, srv *Server, dir string) string {
	t.Helper()
	var res struct {
		SessionID string `json:"sessionId"`
	}
	if err := srv.acpCall(context.Background(), "session/new", map[string]any{
		"cwd":        dir,
		"mcpServers": []any{},
	}, &res); err != nil {
		t.Fatal(err)
	}
	logCap := defaultEventLogCap
	subQ := defaultSubQueue
	srv.mu.Lock()
	srv.sessions[res.SessionID] = newSessionHub(logCap, subQ)
	srv.mu.Unlock()
	return res.SessionID
}

func TestSSEReplayHandler(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	srv, err := New(Config{Listen: addr, ZeroBin: "zero", Logger: log.New(io.Discard, "", 0)})
	if err != nil {
		t.Fatal(err)
	}
	h := newSessionHub(10, 10)
	h.emit(map[string]any{"type": "stop", "stopReason": "end_turn"})
	srv.mu.Lock()
	srv.sessions["s1"] = h
	srv.mu.Unlock()
	httpSrv := &http.Server{Handler: srv.Handler()}
	go httpSrv.Serve(ln)
	defer func() {
		_ = httpSrv.Shutdown(context.Background())
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/v1/sessions/s1/events", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), "data:") {
		t.Fatalf("body %q", body)
	}
}

func TestPromptEventsAndReplay(t *testing.T) {
	srv, base, cleanup := startTestServer(t, nil)
	defer cleanup()
	dir := t.TempDir()
	sid := createSession(t, base, dir)
	postPrompt(t, base, sid, "hello")
	time.Sleep(500 * time.Millisecond)
	if srv.sessions[sid].logLen() == 0 {
		t.Fatal("expected session log entries after prompt")
	}
	lines := collectSSE(t, base+"/v1/sessions/"+sid+"/events", 5*time.Second)
	if !sseContainsType(lines, "session/update") || !sseContainsType(lines, "stop") {
		t.Fatalf("events %v", lines)
	}
}

func TestConcurrentPrompt409(t *testing.T) {
	_, base, cleanup := startTestServer(t, []string{"ZB_FAKE_AGENT_SLOW_MS=300"})
	defer cleanup()
	dir := t.TempDir()
	sid := createSession(t, base, dir)
	postPromptAsync(base, sid, "one")
	time.Sleep(30 * time.Millisecond)
	res, err := http.Post(base+"/v1/sessions/"+sid+"/prompt", "application/json", strings.NewReader(`{"text":"two"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestWS404(t *testing.T) {
	_, base, cleanup := startTestServer(t, nil)
	defer cleanup()
	res, err := http.Get(base + "/ws")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestShutdownClean(t *testing.T) {
	srv, base, cleanup := startTestServer(t, nil)
	_ = base
	cleanup()
	if srv.acp != nil {
		select {
		case <-srv.acp.Done():
		case <-time.After(2 * time.Second):
			t.Fatal("acp still running")
		}
	}
}

func createSession(t *testing.T, base, dir string) string {
	t.Helper()
	body := `{"workspace":"` + dir + `"}`
	res, err := http.Post(base+"/v1/sessions", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	var out struct {
		SessionID string `json:"sessionId"`
	}
	_ = json.NewDecoder(res.Body).Decode(&out)
	return out.SessionID
}

func postPrompt(t *testing.T, base, sid, text string) {
	t.Helper()
	payload := `{"text":"` + text + `"}`
	res, err := http.Post(base+"/v1/sessions/"+sid+"/prompt", "application/json", strings.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusAccepted {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func postPromptAsync(base, sid, text string) {
	payload := `{"text":"` + text + `"}`
	go func() {
		_, _ = http.Post(base+"/v1/sessions/"+sid+"/prompt", "application/json", strings.NewReader(payload))
	}()
}

func collectSSE(t *testing.T, url string, timeout time.Duration) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("events status %d: %s", res.StatusCode, b)
	}
	var lines []string
	sc := bufio.NewScanner(res.Body)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "data: ") {
			lines = append(lines, line)
		}
		if sseContainsType(lines, "stop") {
			break
		}
	}
	return lines
}

func sseContainsType(lines []string, typ string) bool {
	for _, line := range lines {
		if strings.Contains(line, `"type":"`+typ+`"`) || strings.Contains(line, `"type": "`+typ+`"`) {
			return true
		}
	}
	var buf bytes.Buffer
	for _, l := range lines {
		buf.WriteString(l)
	}
	return strings.Contains(buf.String(), typ)
}
