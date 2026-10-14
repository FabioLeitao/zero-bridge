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
	httpSrv := srv.newHTTPServer()
	go httpSrv.Serve(ln)
	return srv, "http://" + addr, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(ctx)
		_ = srv.Shutdown(ctx)
	}
}

func TestInfoAndWorkspace(t *testing.T) {
	srv, base, cleanup := startTestServer(t, nil)
	if err := srv.PrepareWorkspace(); err != nil {
		t.Fatal(err)
	}
	res, err := http.Get(base + "/v1/info")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var body map[string]any
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["agent"] != "up" || body["version"] != "0" {
		t.Fatalf("info %v", body)
	}
	ws, _ := body["defaultWorkspace"].(string)
	if ws == "" || ws != srv.DefaultWorkspace() {
		t.Fatalf("workspace %q", ws)
	}
	st, err := os.Stat(ws)
	if err != nil || !st.IsDir() {
		t.Fatal(err)
	}
	cleanup()
	if _, err := os.Stat(ws); !os.IsNotExist(err) {
		t.Fatalf("temp workspace still present: %v", err)
	}
}

func TestExplicitWorkspace(t *testing.T) {
	dir := t.TempDir()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv, err := New(Config{Listen: ln.Addr().String(), Workspace: dir, Logger: log.New(io.Discard, "", 0)})
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.PrepareWorkspace(); err != nil {
		t.Fatal(err)
	}
	if srv.DefaultWorkspace() != dir {
		t.Fatalf("got %s", srv.DefaultWorkspace())
	}
	_ = ln.Close()
	missing, err := New(Config{Listen: "127.0.0.1:1", Workspace: dir + "/nope", Logger: log.New(io.Discard, "", 0)})
	if err != nil {
		t.Fatal(err)
	}
	if err := missing.PrepareWorkspace(); err == nil {
		t.Fatal("expected missing workspace error")
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
	lines := collectSSE(t, base+"/v1/sessions/"+sid+"/events")
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

// collectSSE reads until a stop event arrives. also lists event types that
// must be present as well (permission tests pass permission_resolved).
// The deadline is 15s so a loaded host can still finish a turn in flight.
func collectSSE(t *testing.T, url string, also ...string) []string {
	t.Helper()
	const deadline = 15 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), deadline)
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
		if sseHasType(lines, "stop") && sseHasAll(lines, also) {
			return lines
		}
	}
	t.Fatalf("sse missing stop within %s (also %v): %v", deadline, also, lines)
	return nil
}

func sseHasAll(lines []string, types []string) bool {
	for _, typ := range types {
		if !sseHasType(lines, typ) {
			return false
		}
	}
	return true
}

func TestConcurrentSessionsWhileSlowPrompt(t *testing.T) {
	_, base, cleanup := startTestServer(t, []string{"ZB_FAKE_AGENT_SLOW_MS=800"})
	defer cleanup()
	dir := t.TempDir()
	sidA := createSession(t, base, dir)
	postPromptAsync(base, sidA, "a")
	time.Sleep(50 * time.Millisecond)
	start := time.Now()
	sidB := createSession(t, base, dir)
	if time.Since(start) > time.Second {
		t.Fatalf("session B blocked for %s", time.Since(start))
	}
	res, err := http.Post(base+"/v1/sessions/"+sidB+"/prompt", "application/json", strings.NewReader(`{"text":"b"}`))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusAccepted {
		t.Fatalf("prompt B status %d", res.StatusCode)
	}
	time.Sleep(2 * time.Second)
	for _, sid := range []string{sidA, sidB} {
		lines := collectSSE(t, base+"/v1/sessions/"+sid+"/events")
		if !sseContainsType(lines, "stop") {
			t.Fatalf("session %s missing stop: %v", sid, lines)
		}
	}
}

func TestShutdownUnblocksSSE(t *testing.T) {
	srv, base, _ := startTestServer(t, nil)
	dir := t.TempDir()
	sid := createSession(t, base, dir)
	done := make(chan struct{})
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+"/v1/sessions/"+sid+"/events", nil)
		res, err := http.DefaultClient.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, res.Body)
			res.Body.Close()
		}
		close(done)
	}()
	time.Sleep(100 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("sse handler still running")
	}
}

func TestJSONContentTypeExact(t *testing.T) {
	_, base, cleanup := startTestServer(t, nil)
	defer cleanup()
	dir := t.TempDir()
	body := `{"workspace":"` + dir + `"}`
	ok, err := http.Post(base+"/v1/sessions", "application/json; charset=utf-8", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	ok.Body.Close()
	if ok.StatusCode != http.StatusOK {
		t.Fatalf("charset status %d", ok.StatusCode)
	}
	bad, err := http.Post(base+"/v1/sessions", "application/jsonx", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	bad.Body.Close()
	if bad.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("jsonx status %d", bad.StatusCode)
	}
}

func sseHasType(lines []string, typ string) bool {
	needle := `"type":"` + typ + `"`
	needleSp := `"type": "` + typ + `"`
	for _, line := range lines {
		if strings.Contains(line, needle) || strings.Contains(line, needleSp) {
			return true
		}
	}
	return false
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
