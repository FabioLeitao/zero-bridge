package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"log"

	"zero-bridge/bridge/internal/acptest"
)

// testAgentBin is a non-race acpfake built once per package run.
// Pointing ZeroBin at the race-instrumented test binary costs about a second
// per server, and that start dominates the slow tests.
// -buildvcs=false keeps the build working when the module copy has no .git,
// which is how make mutate runs. GOFLAGS and GIT_* are dropped so the
// environment cannot turn stamping back on or point git at another tree.
var testAgentBin string

func TestMain(m *testing.M) {
	acptest.MaybeMain()
	bin, err := buildTestAgent()
	if err != nil {
		fmt.Fprintf(os.Stderr, "test agent: %v\n", err)
		os.Exit(1)
	}
	testAgentBin = bin
	code := m.Run()
	_ = os.RemoveAll(filepath.Dir(bin))
	os.Exit(code)
}

func buildTestAgent() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("test agent: no source path")
	}
	root := filepath.Dir(filepath.Dir(filepath.Dir(file)))
	dir, err := os.MkdirTemp("", "zbridge-acpfake-")
	if err != nil {
		return "", err
	}
	bin := filepath.Join(dir, "acpfake")
	cmd := exec.Command("go", "build", "-buildvcs=false", "-o", bin, "./cmd/acpfake")
	cmd.Dir = root
	cmd.Env = testAgentEnv(os.Environ())
	out, err := cmd.CombinedOutput()
	if err != nil {
		_ = os.RemoveAll(dir)
		return "", fmt.Errorf("%w: %s", err, out)
	}
	return bin, nil
}

func testAgentEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, e := range env {
		if strings.HasPrefix(e, "GOFLAGS=") || strings.HasPrefix(e, "GIT_") {
			continue
		}
		out = append(out, e)
	}
	return out
}

func startTestServer(t *testing.T, extraEnv []string) (*Server, string, func()) {
	t.Helper()
	return startConfigured(t, Config{ExtraEnv: extraEnv})
}

func startConfigured(t *testing.T, cfg Config) (*Server, string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	env := []string{"ZB_FAKE_AGENT=1"}
	env = append(env, cfg.ExtraEnv...)
	cfg.Listen = addr
	cfg.ZeroBin = testAgentBin
	cfg.ExtraEnv = env
	if cfg.Logger == nil {
		cfg.Logger = log.New(io.Discard, "", 0)
	}
	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.Boot(context.Background()); err != nil {
		t.Fatal(err)
	}
	httpSrv := srv.newHTTPServer()
	go httpSrv.Serve(ln)
	return srv, "http://" + addr, func() { stopTestHTTP(httpSrv, srv) }
}

// stopTestHTTP asks the listener to drain, then closes it. Shutdown with a
// long context waits out SSE and keep-alive connections. A short context
// plus Close drops those connections. The agent still stops through Server.Shutdown.
func stopTestHTTP(httpSrv *http.Server, srv *Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	err := httpSrv.Shutdown(ctx)
	cancel()
	if err != nil {
		_ = httpSrv.Close()
	}
	ctx, cancel = context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_ = srv.Shutdown(ctx)
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
	roots, _ := body["workspaceRoots"].([]any)
	if len(roots) != 1 || roots[0] != ws {
		t.Fatalf("roots %v", body["workspaceRoots"])
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

func TestListSessions(t *testing.T) {
	_, base, cleanup := startTestServer(t, nil)
	defer cleanup()
	rel, err := http.Get(base + "/v1/sessions?cwd=relative")
	if err != nil {
		t.Fatal(err)
	}
	rel.Body.Close()
	if rel.StatusCode != http.StatusBadRequest {
		t.Fatalf("relative %d", rel.StatusCode)
	}
	put, err := http.NewRequest(http.MethodPut, base+"/v1/sessions?cwd=/tmp/zbridge-ws-a", nil)
	if err != nil {
		t.Fatal(err)
	}
	putRes, err := http.DefaultClient.Do(put)
	if err != nil {
		t.Fatal(err)
	}
	putRes.Body.Close()
	if putRes.StatusCode != http.StatusNotFound {
		t.Fatalf("method %d", putRes.StatusCode)
	}
	hostReq, _ := http.NewRequest(http.MethodGet, base+"/v1/sessions?cwd=/tmp/zbridge-ws-a", nil)
	hostReq.Host = "evil.example:1"
	hostRes, err := http.DefaultClient.Do(hostReq)
	if err != nil {
		t.Fatal(err)
	}
	hostRes.Body.Close()
	if hostRes.StatusCode != http.StatusForbidden {
		t.Fatalf("host %d", hostRes.StatusCode)
	}
	originReq, _ := http.NewRequest(http.MethodGet, base+"/v1/sessions?cwd=/tmp/zbridge-ws-a", nil)
	originReq.Header.Set("Origin", "http://evil.example")
	originRes, err := http.DefaultClient.Do(originReq)
	if err != nil {
		t.Fatal(err)
	}
	originRes.Body.Close()
	if originRes.StatusCode != http.StatusForbidden {
		t.Fatalf("origin %d", originRes.StatusCode)
	}
	res, err := http.Get(base + "/v1/sessions?cwd=/tmp/zbridge-ws-a&limit=2")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var page struct {
		Sessions []struct {
			SessionID string `json:"sessionId"`
			Title     string `json:"title"`
			Cwd       string `json:"cwd"`
		} `json:"sessions"`
		NextCursor string `json:"nextCursor"`
	}
	if err := json.NewDecoder(res.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	if len(page.Sessions) != 2 || page.Sessions[0].Title != `<img src=x onerror="window.__xss=1"></script><script>window.__xss=2</script>` || page.NextCursor == "" {
		t.Fatalf("page %+v", page)
	}
	res2, err := http.Get(base + "/v1/sessions?cwd=/tmp/zbridge-ws-a&limit=2&cursor=" + url.QueryEscape(page.NextCursor))
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	var page2 struct {
		Sessions []struct {
			SessionID string `json:"sessionId"`
		} `json:"sessions"`
	}
	if err := json.NewDecoder(res2.Body).Decode(&page2); err != nil {
		t.Fatal(err)
	}
	if len(page2.Sessions) != 2 || page2.Sessions[0].SessionID != "sess-a3" {
		t.Fatalf("page2 %+v", page2.Sessions)
	}
	one, err := http.Get(base + "/v1/sessions?cwd=/tmp/zbridge-ws-b")
	if err != nil {
		t.Fatal(err)
	}
	defer one.Body.Close()
	var filtered struct {
		Sessions []struct {
			Cwd string `json:"cwd"`
		} `json:"sessions"`
	}
	if err := json.NewDecoder(one.Body).Decode(&filtered); err != nil {
		t.Fatal(err)
	}
	if len(filtered.Sessions) != 1 || filtered.Sessions[0].Cwd != "/tmp/zbridge-ws-b" {
		t.Fatalf("filter %+v", filtered.Sessions)
	}
}

func TestListSessionsRedactsError(t *testing.T) {
	_, base, cleanup := startTestServer(t, []string{"ZB_FAKE_LIST_ERROR=1"})
	defer cleanup()
	res, err := http.Get(base + "/v1/sessions")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusBadGateway {
		t.Fatalf("status %d %s", res.StatusCode, b)
	}
	if strings.Contains(string(b), "secret-token") || !strings.Contains(string(b), "[redacted]") {
		t.Fatalf("body %s", b)
	}
}

func TestSessionMode(t *testing.T) {
	_, base, cleanup := startTestServer(t, nil)
	defer cleanup()
	dir := t.TempDir()
	res, err := http.Post(base+"/v1/sessions", "application/json", strings.NewReader(`{"workspace":"`+dir+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var created struct {
		SessionID string   `json:"sessionId"`
		Mode      string   `json:"mode"`
		Modes     []string `json:"modes"`
	}
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.Mode != "auto" || len(created.Modes) != 3 {
		t.Fatalf("modes %+v", created)
	}
	sid := created.SessionID
	postMode := func(body, ctype string) *http.Response {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, base+"/v1/sessions/"+sid+"/mode", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		if ctype != "" {
			req.Header.Set("Content-Type", ctype)
		}
		got, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	plain := postMode(`{"mode":"ask"}`, "")
	plain.Body.Close()
	if plain.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("415 %d", plain.StatusCode)
	}
	bad := postMode(`{"mode":"unsafe"}`, "application/json")
	bad.Body.Close()
	if bad.StatusCode != http.StatusBadRequest {
		t.Fatalf("unsafe %d", bad.StatusCode)
	}
	extra := postMode(`{"mode":"ask","x":1}`, "application/json")
	extra.Body.Close()
	if extra.StatusCode != http.StatusBadRequest {
		t.Fatalf("unknown field %d", extra.StatusCode)
	}
	noSession, err := http.Post(base+"/v1/sessions/nope/mode", "application/json", strings.NewReader(`{"mode":"ask"}`))
	if err != nil {
		t.Fatal(err)
	}
	noSession.Body.Close()
	if noSession.StatusCode != http.StatusNotFound {
		t.Fatalf("404 %d", noSession.StatusCode)
	}
	hostReq, _ := http.NewRequest(http.MethodPost, base+"/v1/sessions/"+sid+"/mode", strings.NewReader(`{"mode":"ask"}`))
	hostReq.Header.Set("Content-Type", "application/json")
	hostReq.Host = "evil.example:1"
	hostRes, err := http.DefaultClient.Do(hostReq)
	if err != nil {
		t.Fatal(err)
	}
	hostRes.Body.Close()
	if hostRes.StatusCode != http.StatusForbidden {
		t.Fatalf("host %d", hostRes.StatusCode)
	}
	originReq, _ := http.NewRequest(http.MethodPost, base+"/v1/sessions/"+sid+"/mode", strings.NewReader(`{"mode":"ask"}`))
	originReq.Header.Set("Content-Type", "application/json")
	originReq.Header.Set("Origin", "http://evil.example")
	originRes, err := http.DefaultClient.Do(originReq)
	if err != nil {
		t.Fatal(err)
	}
	originRes.Body.Close()
	if originRes.StatusCode != http.StatusForbidden {
		t.Fatalf("origin %d", originRes.StatusCode)
	}
	okRes := postMode(`{"mode":"ask"}`, "application/json")
	defer okRes.Body.Close()
	if okRes.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(okRes.Body)
		t.Fatalf("ask %d %s", okRes.StatusCode, b)
	}
	var okBody struct {
		OK   bool   `json:"ok"`
		Mode string `json:"mode"`
	}
	if err := json.NewDecoder(okRes.Body).Decode(&okBody); err != nil {
		t.Fatal(err)
	}
	if !okBody.OK || okBody.Mode != "ask" {
		t.Fatalf("body %+v", okBody)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	evReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+"/v1/sessions/"+sid+"/events", nil)
	evRes, err := http.DefaultClient.Do(evReq)
	if err != nil && ctx.Err() == nil {
		t.Fatal(err)
	}
	if evRes != nil {
		b, _ := io.ReadAll(evRes.Body)
		evRes.Body.Close()
		if !strings.Contains(string(b), `"currentModeId":"ask"`) {
			t.Fatalf("sse %s", b)
		}
	}
}

func TestModeConflictsWithPrompt(t *testing.T) {
	_, base, cleanup := startTestServer(t, []string{"ZB_FAKE_AGENT_SLOW_MS=1500"})
	defer cleanup()
	dir := t.TempDir()
	sid := createSession(t, base, dir)
	postPromptAsync(base, sid, "one")
	deadline := time.Now().Add(2 * time.Second)
	var status int
	for time.Now().Before(deadline) {
		res, err := http.Post(base+"/v1/sessions/"+sid+"/mode", "application/json", strings.NewReader(`{"mode":"plan"}`))
		if err != nil {
			t.Fatal(err)
		}
		status = res.StatusCode
		res.Body.Close()
		if status == http.StatusConflict {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("status %d", status)
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

func TestLoadReplayAndResume(t *testing.T) {
	_, base, cleanup := startTestServer(t, nil)
	defer cleanup()
	rel := postAttach(t, base+"/v1/sessions/sess-a1/load", `{"workspace":"rel"}`)
	if rel.StatusCode != http.StatusBadRequest {
		t.Fatalf("relative %d", rel.StatusCode)
	}
	load := postAttach(t, base+"/v1/sessions/sess-a1/load", `{"workspace":"/tmp/zbridge-ws-a"}`)
	if load.StatusCode != http.StatusOK {
		t.Fatalf("load %d %s", load.StatusCode, load.Body)
	}
	var loaded map[string]any
	if err := json.Unmarshal(load.Body, &loaded); err != nil {
		t.Fatal(err)
	}
	if loaded["sessionId"] != "sess-a1" || loaded["mode"] != "ask" {
		t.Fatalf("loaded %v", loaded)
	}
	again := postAttach(t, base+"/v1/sessions/sess-a1/load", "")
	if again.StatusCode != http.StatusConflict {
		t.Fatalf("again %d %s", again.StatusCode, again.Body)
	}
	evs := readSessionEvents(t, base+"/v1/sessions/sess-a1/events", 3)
	want := []string{"replay-user", "replay-agent", "replay-tool"}
	for i, text := range want {
		if evs[i].text != text || !evs[i].replay || evs[i].seq != float64(i+1) {
			t.Fatalf("event %d %+v", i, evs[i])
		}
	}
	resumed := postAttach(t, base+"/v1/sessions/sess-a2/resume", "")
	if resumed.StatusCode != http.StatusOK || !strings.Contains(string(resumed.Body), `"mode":"auto"`) {
		t.Fatalf("resume %d %s", resumed.StatusCode, resumed.Body)
	}
	idem := postAttach(t, base+"/v1/sessions/sess-a2/resume", "")
	if idem.StatusCode != http.StatusOK || !strings.Contains(string(idem.Body), `"attached":true`) {
		t.Fatalf("idempotent %d %s", idem.StatusCode, idem.Body)
	}
	quiet := readSessionEvents(t, base+"/v1/sessions/sess-a2/events", 0)
	if len(quiet) != 0 {
		t.Fatalf("resume replayed %+v", quiet)
	}
	missing := postAttach(t, base+"/v1/sessions/missing/load", "")
	if missing.StatusCode != http.StatusNotFound {
		t.Fatalf("discover %d %s", missing.StatusCode, missing.Body)
	}
	unknown := postAttach(t, base+"/v1/sessions/missing/load", `{"workspace":"/tmp/zbridge-ws-a"}`)
	if unknown.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown %d %s", unknown.StatusCode, unknown.Body)
	}
	found := postAttach(t, base+"/v1/sessions/sess-b1/load", "")
	if found.StatusCode != http.StatusOK {
		t.Fatalf("discover cwd %d %s", found.StatusCode, found.Body)
	}
}

func TestLoadReplayIsLive(t *testing.T) {
	_, base, cleanup := startTestServer(t, []string{"ZB_FAKE_LOAD_SLOW_MS=400"})
	defer cleanup()
	done := make(chan attachResult, 1)
	go func() {
		done <- postAttach(t, base+"/v1/sessions/sess-a3/load", "")
	}()
	waitUntil(t, 2*time.Second, func() bool {
		res, err := http.Get(base + "/v1/sessions/sess-a3/events")
		if err != nil {
			return false
		}
		res.Body.Close()
		return res.StatusCode == http.StatusOK
	})
	evs := readSessionEvents(t, base+"/v1/sessions/sess-a3/events", 3)
	if evs[0].text != "replay-user" || evs[2].text != "replay-tool" || !evs[0].replay {
		t.Fatalf("live %+v", evs)
	}
	res := <-done
	if res.StatusCode != http.StatusOK {
		t.Fatalf("load %d %s", res.StatusCode, res.Body)
	}
}

func TestLoadRedactsAndViewOnly(t *testing.T) {
	_, base, cleanup := startTestServer(t, []string{"ZB_FAKE_LOAD_ERROR=1"})
	defer cleanup()
	bad := postAttach(t, base+"/v1/sessions/sess-a1/load", `{"workspace":"/tmp/zbridge-ws-a"}`)
	if bad.StatusCode != http.StatusBadGateway || strings.Contains(string(bad.Body), "secret-token") || !strings.Contains(string(bad.Body), "[redacted]") {
		t.Fatalf("load error %d %s", bad.StatusCode, bad.Body)
	}
	cleanup()

	_, base, cleanup = startTestServer(t, []string{"ZB_FAKE_VIEWONLY_SECRET=1"})
	defer cleanup()
	loaded := postAttach(t, base+"/v1/sessions/sess-view/load", "")
	if loaded.StatusCode != http.StatusOK {
		t.Fatalf("view load %d %s", loaded.StatusCode, loaded.Body)
	}
	req, err := http.NewRequest(http.MethodPost, base+"/v1/sessions/sess-view/prompt", strings.NewReader(`{"text":"hi"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusAccepted {
		t.Fatalf("prompt %d", res.StatusCode)
	}
	errEv := waitEvent(t, base+"/v1/sessions/sess-view/events", "error")
	msg, _ := errEv["message"].(string)
	if strings.Contains(msg, "secret-token") || !strings.Contains(msg, "[redacted]") || !strings.Contains(msg, "view-only") {
		t.Fatalf("view-only message %q", msg)
	}
}

func TestLoadReplayCapAndShutdown(t *testing.T) {
	srv, base, cleanup := startConfigured(t, Config{ReplayLogCap: 1})
	defer cleanup()
	loaded := postAttach(t, base+"/v1/sessions/sess-a1/load", `{"workspace":"/tmp/zbridge-ws-a"}`)
	if loaded.StatusCode != http.StatusOK {
		t.Fatalf("load %d %s", loaded.StatusCode, loaded.Body)
	}
	if n := srv.sessionHub("sess-a1").logLen(); n != 1 {
		t.Fatalf("log cap %d", n)
	}
	kept := readSessionEvents(t, base+"/v1/sessions/sess-a1/events", 1)
	if kept[0].text != "replay-tool" {
		t.Fatalf("kept %+v", kept[0])
	}

	before := runtime.NumGoroutine()
	for i := 0; i < 2; i++ {
		slow, slowBase, stop := startTestServer(t, []string{"ZB_FAKE_LOAD_SLOW_MS=200"})
		finished := make(chan struct{})
		go func() {
			defer close(finished)
			_ = postAttach(t, slowBase+"/v1/sessions/sess-a1/load", "")
		}()
		waitUntil(t, 2*time.Second, func() bool {
			res, err := http.Get(slowBase + "/v1/sessions/sess-a1/events")
			if err != nil {
				return false
			}
			res.Body.Close()
			return res.StatusCode == http.StatusOK
		})
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = slow.Shutdown(ctx)
		cancel()
		stop()
		select {
		case <-finished:
		case <-time.After(3 * time.Second):
			t.Fatal("load did not return on shutdown")
		}
	}
	time.Sleep(200 * time.Millisecond)
	if after := runtime.NumGoroutine(); after > before+8 {
		t.Fatalf("goroutines %d -> %d", before, after)
	}
}

type attachResult struct {
	StatusCode int
	Body       []byte
}

func postAttach(t *testing.T, url, body string) attachResult {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req, err := http.NewRequest(http.MethodPost, url, r)
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return attachResult{StatusCode: res.StatusCode, Body: b}
}

type sseEvent struct {
	raw    map[string]any
	text   string
	replay bool
	seq    float64
}

func readSessionEvents(t *testing.T, url string, n int) []sseEvent {
	t.Helper()
	wait := 3 * time.Second
	if n == 0 {
		wait = 300 * time.Millisecond
	}
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		if n == 0 {
			return nil
		}
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		if n == 0 {
			return nil
		}
		t.Fatalf("events %d", res.StatusCode)
	}
	return collectEvents(t, res.Body, n, func(sseEvent) bool { return false })
}

func waitEvent(t *testing.T, url, typ string) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	found := collectEvents(t, res.Body, 0, func(ev sseEvent) bool {
		return ev.raw["type"] == typ
	})
	if len(found) == 0 {
		t.Fatalf("missing %s", typ)
	}
	return found[0].raw
}

func collectEvents(t *testing.T, r io.Reader, n int, stop func(sseEvent) bool) []sseEvent {
	t.Helper()
	sc := bufio.NewScanner(r)
	var out []sseEvent
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &obj); err != nil {
			t.Fatal(err)
		}
		ev := sseEvent{raw: obj, replay: obj["replay"] == true, seq: num(obj["seq"]), text: eventText(obj)}
		if stop != nil && stop(ev) {
			return []sseEvent{ev}
		}
		out = append(out, ev)
		if n > 0 && len(out) >= n {
			return out
		}
	}
	if n == 0 {
		return out
	}
	if len(out) < n {
		t.Fatalf("got %d events, want %d (%v)", len(out), n, out)
	}
	return out
}

func eventText(obj map[string]any) string {
	params, _ := obj["params"].(map[string]any)
	upd, _ := params["update"].(map[string]any)
	if upd == nil {
		return ""
	}
	if title, ok := upd["title"].(string); ok && title != "" {
		return title
	}
	content, _ := upd["content"].(map[string]any)
	text, _ := content["text"].(string)
	return text
}

func num(v any) float64 {
	n, _ := v.(float64)
	return n
}

func waitUntil(t *testing.T, d time.Duration, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if ready() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("timeout")
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

const bearerToken = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestLoginRateLimitBlocksCorrectToken(t *testing.T) {
	now := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	_, base, cleanup := startConfigured(t, Config{
		Token:             bearerToken,
		LoginFailureDelay: -1,
		Now:               func() time.Time { return now },
	})
	defer cleanup()
	post := func(token string) (*http.Response, []byte) {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, base+"/v1/login", strings.NewReader(`{"token":"`+token+`"}`))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(res.Body)
		res.Body.Close()
		return res, b
	}
	for i := 0; i < 5; i++ {
		res, _ := post("wrong-token")
		if res.StatusCode != http.StatusUnauthorized {
			t.Fatalf("fail %d status %d", i, res.StatusCode)
		}
	}
	blocked, _ := post(bearerToken)
	if blocked.StatusCode != http.StatusTooManyRequests || blocked.Header.Get("Retry-After") == "" || len(blocked.Cookies()) != 0 {
		t.Fatalf("correct token while limited: %d retry %q cookies %d", blocked.StatusCode, blocked.Header.Get("Retry-After"), len(blocked.Cookies()))
	}
	now = now.Add(61 * time.Second)
	okRes, _ := post(bearerToken)
	if okRes.StatusCode != http.StatusOK {
		t.Fatalf("after window %d", okRes.StatusCode)
	}
	got := false
	for _, c := range okRes.Cookies() {
		if c.Name == sessionCookie && c.Value != "" {
			got = true
		}
	}
	if !got {
		t.Fatal("session cookie missing after the window")
	}
}

func TestLoginRateLimitConcurrentWrongToken(t *testing.T) {
	_, base, cleanup := startConfigured(t, Config{
		Token:             bearerToken,
		LoginFailureDelay: -1,
	})
	defer cleanup()
	var unauthorized atomic.Int32
	var limited atomic.Int32
	var other atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, err := http.NewRequest(http.MethodPost, base+"/v1/login", strings.NewReader(`{"token":"wrong-token"}`))
			if err != nil {
				other.Add(1)
				return
			}
			req.Header.Set("Content-Type", "application/json")
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				other.Add(1)
				return
			}
			res.Body.Close()
			switch res.StatusCode {
			case http.StatusUnauthorized:
				unauthorized.Add(1)
			case http.StatusTooManyRequests:
				limited.Add(1)
			default:
				other.Add(1)
			}
		}()
	}
	wg.Wait()
	if other.Load() != 0 || unauthorized.Load() > 5 || unauthorized.Load()+limited.Load() != 50 {
		t.Fatalf("401=%d 429=%d other=%d", unauthorized.Load(), limited.Load(), other.Load())
	}
}

func TestV1RoutesRequireCredential(t *testing.T) {
	_, base, cleanup := startConfigured(t, Config{
		Token:             bearerToken,
		LoginFailureDelay: -1,
	})
	defer cleanup()
	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/v1/info"},
		{http.MethodGet, "/v1/capabilities"},
		{http.MethodGet, "/v1/sessions"},
		{http.MethodPost, "/v1/sessions"},
		{http.MethodPost, "/v1/sessions/s1/prompt"},
		{http.MethodGet, "/v1/sessions/s1/events"},
		{http.MethodGet, "/v1/sessions/s1/permissions/pending"},
		{http.MethodPost, "/v1/sessions/s1/permissions/r1"},
		{http.MethodPost, "/v1/sessions/s1/mode"},
		{http.MethodPost, "/v1/sessions/s1/load"},
		{http.MethodPost, "/v1/sessions/s1/resume"},
		{http.MethodPost, "/v1/logout"},
		{http.MethodPost, "/v1/logout-all"},
		{http.MethodGet, "/v1/auth/sessions"},
		{http.MethodDelete, "/v1/auth/sessions/missing"},
	}
	call := func(method, path, bearer string, cookie *http.Cookie) int {
		t.Helper()
		req, err := http.NewRequest(method, base+path, strings.NewReader(`{}`))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		if cookie != nil {
			req.AddCookie(cookie)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}
	for _, rt := range routes {
		if got := call(rt.method, rt.path, "", nil); got != http.StatusUnauthorized {
			t.Fatalf("%s %s without credential: %d", rt.method, rt.path, got)
		}
		if got := call(rt.method, rt.path, bearerToken, nil); got == http.StatusUnauthorized {
			t.Fatalf("%s %s with bearer: 401", rt.method, rt.path)
		}
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
	for _, rt := range routes {
		if rt.method != http.MethodPost && rt.method != http.MethodDelete {
			continue
		}
		if got := call(rt.method, rt.path, "", session); got != http.StatusForbidden {
			t.Fatalf("%s %s cookie without Origin: %d", rt.method, rt.path, got)
		}
	}
	for i, rt := range routes {
		got := call(rt.method, rt.path, "wrong-bearer-token", nil)
		want := http.StatusUnauthorized
		if i >= 5 {
			want = http.StatusTooManyRequests
		}
		if got != want {
			t.Fatalf("%s %s wrong bearer %d: %d", rt.method, rt.path, i, got)
		}
	}
	if got := call(http.MethodGet, "/v1/info", bearerToken, nil); got != http.StatusTooManyRequests {
		t.Fatalf("correct bearer while limited: %d", got)
	}
	if got := call(http.MethodGet, "/v1/info", "", nil); got != http.StatusUnauthorized {
		t.Fatalf("missing credential while limited: %d", got)
	}
}

func TestBearerRateLimitConcurrentWrongToken(t *testing.T) {
	var logs bytes.Buffer
	_, base, cleanup := startConfigured(t, Config{
		Token:             bearerToken,
		LoginFailureDelay: -1,
		Logger:            log.New(&logs, "", 0),
	})
	defer cleanup()
	var unauthorized atomic.Int32
	var limited atomic.Int32
	var other atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, err := http.NewRequest(http.MethodGet, base+"/v1/info", nil)
			if err != nil {
				other.Add(1)
				return
			}
			req.Header.Set("Authorization", "Bearer wrong-bearer-token")
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				other.Add(1)
				return
			}
			res.Body.Close()
			switch res.StatusCode {
			case http.StatusUnauthorized:
				unauthorized.Add(1)
			case http.StatusTooManyRequests:
				limited.Add(1)
			default:
				other.Add(1)
			}
		}()
	}
	wg.Wait()
	if other.Load() != 0 || unauthorized.Load() > 5 || unauthorized.Load()+limited.Load() != 50 {
		t.Fatalf("401=%d 429=%d other=%d", unauthorized.Load(), limited.Load(), other.Load())
	}
	okReq, err := http.NewRequest(http.MethodGet, base+"/v1/info", nil)
	if err != nil {
		t.Fatal(err)
	}
	okReq.Header.Set("Authorization", "Bearer "+bearerToken)
	okRes, err := http.DefaultClient.Do(okReq)
	if err != nil {
		t.Fatal(err)
	}
	okRes.Body.Close()
	if okRes.StatusCode != http.StatusTooManyRequests || okRes.Header.Get("Retry-After") == "" {
		t.Fatalf("correct bearer while limited: %d retry %q", okRes.StatusCode, okRes.Header.Get("Retry-After"))
	}
	text := logs.String()
	if !strings.Contains(text, `"kind":"bearer_failure"`) || !strings.Contains(text, `"kind":"rate_limited"`) {
		t.Fatalf("audit: %s", text)
	}
	if strings.Contains(text, bearerToken) || strings.Contains(text, "wrong-bearer-token") {
		t.Fatal("secret in audit log")
	}
}

func TestLoginCookieSession(t *testing.T) {
	fixed := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	cfg := Config{
		Token:             bearerToken,
		LoginFailureDelay: -1,
		Now:               func() time.Time { return fixed },
		PublicOrigins:     []string{"http://bridge.test:8780"},
		AllowedHosts:      []string{"bridge.test"},
	}
	_, base, cleanup := startConfigured(t, cfg)
	defer cleanup()
	origin := "http://" + strings.TrimPrefix(base, "http://")
	login := func(token string, extra func(*http.Request)) (*http.Response, []byte) {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, base+"/v1/login", strings.NewReader(`{"token":"`+token+`"}`))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		if extra != nil {
			extra(req)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(res.Body)
		res.Body.Close()
		return res, b
	}
	_, base2, cleanup2 := startConfigured(t, cfg)
	defer cleanup2()
	okRes, _ := login(bearerToken, nil)
	if okRes.StatusCode != http.StatusOK {
		t.Fatalf("login %d", okRes.StatusCode)
	}
	var session *http.Cookie
	for _, c := range okRes.Cookies() {
		if c.Name == sessionCookie {
			session = c
		}
	}
	if session == nil || !session.HttpOnly || session.SameSite != http.SameSiteStrictMode || session.Secure || session.Path != "/" || session.MaxAge != 43200 || len(session.Value) != 64 {
		t.Fatalf("cookie %+v", session)
	}
	info := func(url string, c *http.Cookie, origin string) int {
		t.Helper()
		req, err := http.NewRequest(http.MethodGet, url+"/v1/info", nil)
		if err != nil {
			t.Fatal(err)
		}
		if c != nil {
			req.AddCookie(c)
		}
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}
	if info(base, session, "") != http.StatusOK {
		t.Fatal("cookie info")
	}
	if info(base2, session, "") != http.StatusUnauthorized {
		t.Fatal("restart kept the session")
	}
	stReq, _ := http.NewRequest(http.MethodGet, base+"/v1/auth/status", nil)
	stReq.AddCookie(session)
	stRes, err := http.DefaultClient.Do(stReq)
	if err != nil {
		t.Fatal(err)
	}
	stBody, _ := io.ReadAll(stRes.Body)
	stRes.Body.Close()
	if !strings.Contains(string(stBody), `"authenticated":true`) {
		t.Fatalf("status %s", stBody)
	}
	outReq, _ := http.NewRequest(http.MethodPost, base+"/v1/logout", nil)
	outReq.AddCookie(session)
	outRes, err := http.DefaultClient.Do(outReq)
	if err != nil {
		t.Fatal(err)
	}
	outRes.Body.Close()
	if outRes.StatusCode != http.StatusForbidden {
		t.Fatalf("csrf %d", outRes.StatusCode)
	}
	if info(base, session, "") != http.StatusOK {
		t.Fatal("csrf logout dropped the session")
	}
	outReq, _ = http.NewRequest(http.MethodPost, base+"/v1/logout", nil)
	outReq.AddCookie(session)
	outReq.Header.Set("Origin", origin)
	outRes, err = http.DefaultClient.Do(outReq)
	if err != nil {
		t.Fatal(err)
	}
	outRes.Body.Close()
	if outRes.StatusCode != http.StatusOK {
		t.Fatalf("logout %d", outRes.StatusCode)
	}
	if info(base, session, "") != http.StatusUnauthorized {
		t.Fatal("logout did not invalidate")
	}
	secRes, _ := login(bearerToken, func(r *http.Request) {
		r.Header.Set("X-Forwarded-Proto", "https")
	})
	for _, c := range secRes.Cookies() {
		if c.Name == sessionCookie && c.Secure {
			t.Fatal("Secure without trust-proxy")
		}
	}
	_, proxied, stop := startConfigured(t, Config{
		Token:             bearerToken,
		LoginFailureDelay: -1,
		Now:               func() time.Time { return fixed },
		TrustProxy:        true,
	})
	defer stop()
	req, _ := http.NewRequest(http.MethodPost, proxied+"/v1/login", strings.NewReader(`{"token":"`+bearerToken+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-Proto", "https")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	secure := false
	for _, c := range res.Cookies() {
		if c.Name == sessionCookie && c.Secure {
			secure = true
		}
	}
	if !secure {
		t.Fatal("Secure missing behind trusted https")
	}
	before := runtime.NumGoroutine()
	for i := 0; i < 2; i++ {
		_, u, done := startConfigured(t, Config{Token: bearerToken, LoginFailureDelay: -1, Now: func() time.Time { return fixed }})
		req, _ := http.NewRequest(http.MethodPost, u+"/v1/login", strings.NewReader(`{"token":"nope"}`))
		req.Header.Set("Content-Type", "application/json")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		done()
	}
	time.Sleep(200 * time.Millisecond)
	if after := runtime.NumGoroutine(); after > before+8 {
		t.Fatalf("goroutines %d -> %d", before, after)
	}
}

func TestProxyHostRules(t *testing.T) {
	if _, err := New(Config{Listen: "127.0.0.1:9", AllowedHosts: []string{"bridge.test"}}); err == nil || !strings.Contains(err.Error(), "token required when exposed via proxy") {
		t.Fatalf("missing token: %v", err)
	}
	_, base, cleanup := startConfigured(t, Config{
		Token:         bearerToken,
		AllowedHosts:  []string{"bridge.test"},
		PublicOrigins: []string{"http://bridge.test:8780"},
	})
	defer cleanup()
	get := func(host, origin string) int {
		t.Helper()
		req, err := http.NewRequest(http.MethodGet, base+"/health", nil)
		if err != nil {
			t.Fatal(err)
		}
		if host != "" {
			req.Host = host
		}
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}
	if get("bridge.test", "") != http.StatusOK {
		t.Fatal("listed host")
	}
	if get("", "") != http.StatusOK {
		t.Fatal("loopback host")
	}
	if get("evil.example", "") != http.StatusForbidden {
		t.Fatal("rebinding host")
	}
	if get("bridge.test", "http://bridge.test:8780") != http.StatusOK {
		t.Fatal("listed origin")
	}
	if get("bridge.test", "http://bridge.test:9") != http.StatusForbidden {
		t.Fatal("origin port")
	}
	if get("bridge.test", "https://bridge.test:8780") != http.StatusForbidden {
		t.Fatal("origin scheme")
	}
	if get("bridge.test", "http://evil.test:8780") != http.StatusForbidden {
		t.Fatal("origin host")
	}
}

func TestBearerAuth(t *testing.T) {
	if _, err := New(Config{Listen: "127.0.0.1:9", Token: "short"}); err == nil {
		t.Fatal("short token accepted")
	}
	var logs bytes.Buffer
	_, base, cleanup := startConfigured(t, Config{
		Token: bearerToken,
		ExtraEnv: []string{
			"ZB_FAKE_LIST_ERROR=1",
			"ZB_FAKE_ERROR_ECHO=" + bearerToken,
		},
		Logger: log.New(&logs, "", 0),
	})
	defer cleanup()
	res, err := http.Get(base + "/v1/info")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized || res.Header.Get("WWW-Authenticate") != "Bearer" || !strings.Contains(string(b), `"error":"unauthorized"`) {
		t.Fatalf("missing %d %s %s", res.StatusCode, res.Header.Get("WWW-Authenticate"), b)
	}
	wrong, err := http.NewRequest(http.MethodGet, base+"/v1/info", nil)
	if err != nil {
		t.Fatal(err)
	}
	wrong.Header.Set("Authorization", "Bearer not-the-token")
	wrongRes, err := http.DefaultClient.Do(wrong)
	if err != nil {
		t.Fatal(err)
	}
	wrongBody, _ := io.ReadAll(wrongRes.Body)
	wrongRes.Body.Close()
	if wrongRes.StatusCode != http.StatusUnauthorized || string(wrongBody) != string(b) {
		t.Fatalf("wrong token %d %s", wrongRes.StatusCode, wrongBody)
	}
	okReq, err := http.NewRequest(http.MethodGet, base+"/v1/info", nil)
	if err != nil {
		t.Fatal(err)
	}
	okReq.Header.Set("Authorization", "Bearer "+bearerToken)
	okRes, err := http.DefaultClient.Do(okReq)
	if err != nil {
		t.Fatal(err)
	}
	okRes.Body.Close()
	if okRes.StatusCode != http.StatusOK {
		t.Fatalf("bearer %d", okRes.StatusCode)
	}
	listReq, err := http.NewRequest(http.MethodGet, base+"/v1/sessions", nil)
	if err != nil {
		t.Fatal(err)
	}
	listReq.Header.Set("Authorization", "Bearer "+bearerToken)
	listRes, err := http.DefaultClient.Do(listReq)
	if err != nil {
		t.Fatal(err)
	}
	listBody, _ := io.ReadAll(listRes.Body)
	listRes.Body.Close()
	if strings.Contains(string(listBody), bearerToken) || !strings.Contains(string(listBody), "[redacted]") {
		t.Fatalf("error body leaked")
	}
	health, err := http.Get(base + "/health")
	if err != nil {
		t.Fatal(err)
	}
	healthBody, _ := io.ReadAll(health.Body)
	health.Body.Close()
	if health.StatusCode != http.StatusOK || strings.Contains(string(healthBody), "agent") {
		t.Fatalf("health %d %s", health.StatusCode, healthBody)
	}
	page, err := http.Get(base + "/")
	if err != nil {
		t.Fatal(err)
	}
	page.Body.Close()
	if page.StatusCode != http.StatusOK {
		t.Fatalf("static %d", page.StatusCode)
	}
	q, err := http.Get(base + "/v1/info?access_token=" + bearerToken)
	if err != nil {
		t.Fatal(err)
	}
	q.Body.Close()
	if strings.Contains(logs.String(), bearerToken) {
		t.Fatal("token appeared in the log")
	}
}

func TestLoginAuditOmitsSecret(t *testing.T) {
	var logs bytes.Buffer
	_, base, cleanup := startConfigured(t, Config{
		Token:             bearerToken,
		LoginFailureDelay: -1,
		Logger:            log.New(&logs, "", 0),
	})
	defer cleanup()
	body := `{"token":"` + bearerToken + `"}`
	bad, err := http.NewRequest(http.MethodPost, base+"/v1/login", strings.NewReader(`{"token":"wrong-token-value"}`))
	if err != nil {
		t.Fatal(err)
	}
	bad.Header.Set("Content-Type", "application/json")
	badRes, err := http.DefaultClient.Do(bad)
	if err != nil {
		t.Fatal(err)
	}
	badRes.Body.Close()
	if badRes.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status %d", badRes.StatusCode)
	}
	ok, err := http.NewRequest(http.MethodPost, base+"/v1/login", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	ok.Header.Set("Content-Type", "application/json")
	okRes, err := http.DefaultClient.Do(ok)
	if err != nil {
		t.Fatal(err)
	}
	okRes.Body.Close()
	if okRes.StatusCode != http.StatusOK {
		t.Fatalf("login %d", okRes.StatusCode)
	}
	text := logs.String()
	if !strings.Contains(text, `"kind":"login_failure"`) || !strings.Contains(text, `"kind":"login_success"`) {
		t.Fatalf("audit lines: %s", text)
	}
	if strings.Contains(text, bearerToken) || strings.Contains(text, "wrong-token-value") {
		t.Fatal("secret in audit log")
	}
	out, err := http.NewRequest(http.MethodPost, base+"/v1/logout", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range okRes.Cookies() {
		out.AddCookie(c)
	}
	out.Header.Set("Origin", base)
	outRes, err := http.DefaultClient.Do(out)
	if err != nil {
		t.Fatal(err)
	}
	outRes.Body.Close()
	if !strings.Contains(logs.String(), `"kind":"logout"`) {
		t.Fatalf("logout audit: %s", logs.String())
	}
}

func TestSessionLifetimeAndEviction(t *testing.T) {
	loginCookie := func(t *testing.T, base string) *http.Cookie {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, base+"/v1/login", strings.NewReader(`{"token":"`+bearerToken+`"}`))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("login %d", res.StatusCode)
		}
		for _, c := range res.Cookies() {
			if c.Name == sessionCookie {
				return c
			}
		}
		t.Fatal("cookie missing")
		return nil
	}
	status := func(t *testing.T, base string, c *http.Cookie) int {
		t.Helper()
		req, err := http.NewRequest(http.MethodGet, base+"/v1/info", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.AddCookie(c)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}

	t.Run("evict oldest", func(t *testing.T) {
		now := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
		var logs bytes.Buffer
		_, base, cleanup := startConfigured(t, Config{
			Token:             bearerToken,
			LoginFailureDelay: -1,
			Now:               func() time.Time { return now },
			SessionTTL:        2 * time.Hour,
			SessionIdle:       time.Hour,
			MaxSessions:       2,
			Logger:            log.New(&logs, "", 0),
		})
		defer cleanup()
		first := loginCookie(t, base)
		now = now.Add(time.Second)
		second := loginCookie(t, base)
		now = now.Add(time.Second)
		third := loginCookie(t, base)
		if status(t, base, first) != http.StatusUnauthorized {
			t.Fatal("oldest session still accepted")
		}
		if status(t, base, second) != http.StatusOK || status(t, base, third) != http.StatusOK {
			t.Fatal("newer session rejected")
		}
		text := logs.String()
		if !strings.Contains(text, `"kind":"session_evicted"`) || !strings.Contains(text, `"reason":"oldest"`) {
			t.Fatalf("evict audit: %s", text)
		}
		for _, c := range []*http.Cookie{first, second, third} {
			if strings.Contains(text, c.Value) {
				t.Fatal("session id in audit log")
			}
		}
	})

	t.Run("idle and ttl boundaries", func(t *testing.T) {
		now := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
		var logs bytes.Buffer
		_, base, cleanup := startConfigured(t, Config{
			Token:             bearerToken,
			LoginFailureDelay: -1,
			Now:               func() time.Time { return now },
			SessionTTL:        time.Hour,
			SessionIdle:       30 * time.Minute,
			Logger:            log.New(&logs, "", 0),
		})
		defer cleanup()
		idleCookie := loginCookie(t, base)
		now = now.Add(30*time.Minute - time.Nanosecond)
		if status(t, base, idleCookie) != http.StatusOK {
			t.Fatal("inside idle window rejected")
		}
		fresh := loginCookie(t, base)
		now = now.Add(30 * time.Minute)
		if status(t, base, fresh) != http.StatusUnauthorized {
			t.Fatal("exact idle boundary accepted")
		}
		if !strings.Contains(logs.String(), `"reason":"idle"`) {
			t.Fatalf("idle audit: %s", logs.String())
		}
		logs.Reset()
		ttlCookie := loginCookie(t, base)
		created := now
		now = created.Add(time.Hour)
		if status(t, base, ttlCookie) != http.StatusUnauthorized {
			t.Fatal("exact ttl boundary accepted")
		}
		text := logs.String()
		if !strings.Contains(text, `"reason":"ttl"`) || strings.Contains(text, ttlCookie.Value) {
			t.Fatalf("ttl audit: %s", text)
		}
	})

	t.Run("sliding idle", func(t *testing.T) {
		start := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
		now := start
		var logs bytes.Buffer
		_, base, cleanup := startConfigured(t, Config{
			Token:             bearerToken,
			LoginFailureDelay: -1,
			Now:               func() time.Time { return now },
			SessionTTL:        time.Hour,
			SessionIdle:       10 * time.Minute,
			Logger:            log.New(&logs, "", 0),
		})
		defer cleanup()
		cookie := loginCookie(t, base)
		for _, minute := range []int{8, 16, 24} {
			now = start.Add(time.Duration(minute) * time.Minute)
			if status(t, base, cookie) != http.StatusOK {
				t.Fatalf("use at %dm rejected", minute)
			}
		}
		logs.Reset()
		now = start.Add(24*time.Minute + 10*time.Minute + time.Nanosecond)
		if status(t, base, cookie) != http.StatusUnauthorized {
			t.Fatal("silence past idle still accepted")
		}
		text := logs.String()
		if !strings.Contains(text, `"reason":"idle"`) || strings.Contains(text, cookie.Value) {
			t.Fatalf("idle audit: %s", text)
		}
	})

	// Activity stays inside the idle window on purpose. Removing the TTL
	// check must fail here; the older boundary reached idle at the same instant.
	t.Run("absolute ttl", func(t *testing.T) {
		start := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
		now := start
		var logs bytes.Buffer
		_, base, cleanup := startConfigured(t, Config{
			Token:             bearerToken,
			LoginFailureDelay: -1,
			Now:               func() time.Time { return now },
			SessionTTL:        time.Hour,
			SessionIdle:       10 * time.Minute,
			Logger:            log.New(&logs, "", 0),
		})
		defer cleanup()
		cookie := loginCookie(t, base)
		for _, minute := range []int{8, 16, 24, 32, 40, 48, 56} {
			now = start.Add(time.Duration(minute) * time.Minute)
			if status(t, base, cookie) != http.StatusOK {
				t.Fatalf("use at %dm rejected", minute)
			}
		}
		logs.Reset()
		now = start.Add(time.Hour)
		if status(t, base, cookie) != http.StatusUnauthorized {
			t.Fatal("absolute ttl with a fresh idle still accepted")
		}
		text := logs.String()
		if !strings.Contains(text, `"reason":"ttl"`) || strings.Contains(text, `"reason":"idle"`) || strings.Contains(text, cookie.Value) {
			t.Fatalf("ttl audit: %s", text)
		}
	})
}

const bearerTokenNext = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"

func TestLogoutAllListRevokeAndReload(t *testing.T) {
	loginCookie := func(t *testing.T, base string) *http.Cookie {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, base+"/v1/login", strings.NewReader(`{"token":"`+bearerToken+`"}`))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("login %d", res.StatusCode)
		}
		for _, c := range res.Cookies() {
			if c.Name == sessionCookie {
				return c
			}
		}
		t.Fatal("cookie missing")
		return nil
	}
	infoStatus := func(t *testing.T, base string, c *http.Cookie, bearer string) int {
		t.Helper()
		req, err := http.NewRequest(http.MethodGet, base+"/v1/info", nil)
		if err != nil {
			t.Fatal(err)
		}
		if c != nil {
			req.AddCookie(c)
		}
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}
	writeToken := func(t *testing.T, path, token string, mode os.FileMode) {
		t.Helper()
		if err := os.WriteFile(path, []byte(token+"\n"), mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("list and revoke", func(t *testing.T) {
		now := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
		var logs bytes.Buffer
		_, base, cleanup := startConfigured(t, Config{
			Token:             bearerToken,
			LoginFailureDelay: -1,
			Now:               func() time.Time { return now },
			Logger:            log.New(&logs, "", 0),
		})
		defer cleanup()
		first := loginCookie(t, base)
		now = now.Add(time.Second)
		second := loginCookie(t, base)
		req, err := http.NewRequest(http.MethodGet, base+"/v1/auth/sessions", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.AddCookie(first)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("list %d %s", res.StatusCode, body)
		}
		var listed struct {
			Sessions []authSessionView `json:"sessions"`
		}
		if err := json.Unmarshal(body, &listed); err != nil {
			t.Fatal(err)
		}
		if len(listed.Sessions) != 2 {
			t.Fatalf("sessions %d", len(listed.Sessions))
		}
		text := string(body)
		for _, c := range []*http.Cookie{first, second} {
			if strings.Contains(text, c.Value) {
				t.Fatal("cookie value in session list")
			}
		}
		for _, view := range listed.Sessions {
			if strings.HasPrefix(first.Value, view.Handle) || strings.HasPrefix(second.Value, view.Handle) || !strings.HasPrefix(view.Handle, "s.") {
				t.Fatalf("handle %q derived from cookie", view.Handle)
			}
		}
		drop := listed.Sessions[1].Handle
		deny, err := http.NewRequest(http.MethodDelete, base+"/v1/auth/sessions/"+drop, nil)
		if err != nil {
			t.Fatal(err)
		}
		deny.AddCookie(first)
		denyRes, err := http.DefaultClient.Do(deny)
		if err != nil {
			t.Fatal(err)
		}
		denyRes.Body.Close()
		if denyRes.StatusCode != http.StatusForbidden {
			t.Fatalf("delete without origin %d", denyRes.StatusCode)
		}
		ok, err := http.NewRequest(http.MethodDelete, base+"/v1/auth/sessions/"+drop, nil)
		if err != nil {
			t.Fatal(err)
		}
		ok.AddCookie(first)
		ok.Header.Set("Origin", base)
		okRes, err := http.DefaultClient.Do(ok)
		if err != nil {
			t.Fatal(err)
		}
		okRes.Body.Close()
		if okRes.StatusCode != http.StatusOK {
			t.Fatalf("delete %d", okRes.StatusCode)
		}
		if infoStatus(t, base, second, "") != http.StatusUnauthorized {
			t.Fatal("revoked session still accepted")
		}
		if infoStatus(t, base, first, "") != http.StatusOK {
			t.Fatal("other session rejected")
		}
		audit := logs.String()
		if !strings.Contains(audit, `"reason":"revoked"`) || strings.Contains(audit, first.Value) || strings.Contains(audit, second.Value) {
			t.Fatalf("revoke audit: %s", audit)
		}
	})

	t.Run("logout all keeps bearer", func(t *testing.T) {
		var logs bytes.Buffer
		_, base, cleanup := startConfigured(t, Config{
			Token:             bearerToken,
			LoginFailureDelay: -1,
			Logger:            log.New(&logs, "", 0),
		})
		defer cleanup()
		first := loginCookie(t, base)
		second := loginCookie(t, base)
		deny, err := http.NewRequest(http.MethodPost, base+"/v1/logout-all", nil)
		if err != nil {
			t.Fatal(err)
		}
		deny.AddCookie(first)
		denyRes, err := http.DefaultClient.Do(deny)
		if err != nil {
			t.Fatal(err)
		}
		denyRes.Body.Close()
		if denyRes.StatusCode != http.StatusForbidden {
			t.Fatalf("logout-all without origin %d", denyRes.StatusCode)
		}
		if infoStatus(t, base, first, "") != http.StatusOK || infoStatus(t, base, second, "") != http.StatusOK {
			t.Fatal("csrf logout-all dropped a session")
		}
		all, err := http.NewRequest(http.MethodPost, base+"/v1/logout-all", nil)
		if err != nil {
			t.Fatal(err)
		}
		all.Header.Set("Authorization", "Bearer "+bearerToken)
		allRes, err := http.DefaultClient.Do(all)
		if err != nil {
			t.Fatal(err)
		}
		allRes.Body.Close()
		if allRes.StatusCode != http.StatusOK {
			t.Fatalf("logout-all %d", allRes.StatusCode)
		}
		if infoStatus(t, base, first, "") != http.StatusUnauthorized || infoStatus(t, base, second, "") != http.StatusUnauthorized {
			t.Fatal("session survived logout-all")
		}
		if infoStatus(t, base, nil, bearerToken) != http.StatusOK {
			t.Fatal("bearer rejected after logout-all")
		}
		audit := logs.String()
		if !strings.Contains(audit, `"kind":"logout_all"`) || strings.Contains(audit, first.Value) || strings.Contains(audit, bearerToken) {
			t.Fatalf("logout-all audit: %s", audit)
		}
	})

	t.Run("sighup rotates file token", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "token")
		writeToken(t, path, bearerToken, 0o600)
		var logs bytes.Buffer
		srv, base, cleanup := startConfigured(t, Config{
			Token:             bearerToken,
			TokenFile:         path,
			LoginFailureDelay: -1,
			Logger:            log.New(&logs, "", 0),
		})
		defer cleanup()
		cookie := loginCookie(t, base)
		ch := make(chan os.Signal, 1)
		srv.startReload(ch, nil)
		writeToken(t, path, bearerTokenNext, 0o600)
		ch <- syscall.SIGHUP
		waitUntil(t, 2*time.Second, func() bool { return srv.currentToken() == bearerTokenNext })
		if infoStatus(t, base, nil, bearerToken) != http.StatusUnauthorized {
			t.Fatal("old token still accepted")
		}
		if infoStatus(t, base, cookie, "") != http.StatusUnauthorized {
			t.Fatal("session survived rotation")
		}
		if infoStatus(t, base, nil, bearerTokenNext) != http.StatusOK {
			t.Fatal("new token rejected")
		}
		audit := logs.String()
		if !strings.Contains(audit, `"kind":"token_reloaded"`) || strings.Contains(audit, bearerToken) || strings.Contains(audit, bearerTokenNext) || strings.Contains(audit, cookie.Value) {
			t.Fatalf("reload audit: %s", audit)
		}
	})

	t.Run("failed reload keeps token and sessions", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "token")
		writeToken(t, path, bearerToken, 0o600)
		var logs bytes.Buffer
		srv, base, cleanup := startConfigured(t, Config{
			Token:             bearerToken,
			TokenFile:         path,
			LoginFailureDelay: -1,
			Logger:            log.New(&logs, "", 0),
		})
		defer cleanup()
		cookie := loginCookie(t, base)
		if err := os.Chmod(path, 0o644); err != nil {
			t.Fatal(err)
		}
		srv.reloadTokenFile()
		if srv.currentToken() != bearerToken {
			t.Fatal("token changed after a mode failure")
		}
		if infoStatus(t, base, cookie, "") != http.StatusOK || infoStatus(t, base, nil, bearerToken) != http.StatusOK {
			t.Fatal("credential rejected after a failed reload")
		}
		if !strings.Contains(logs.String(), `"reason":"mode"`) || strings.Contains(logs.String(), path) || strings.Contains(logs.String(), bearerToken) {
			t.Fatalf("mode reload: %s", logs.String())
		}
		logs.Reset()
		writeToken(t, path, "short", 0o600)
		srv.reloadTokenFile()
		if !strings.Contains(logs.String(), `"reason":"entropy"`) || srv.currentToken() != bearerToken {
			t.Fatalf("entropy reload: token changed or audit %s", logs.String())
		}
		logs.Reset()
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		srv.reloadTokenFile()
		if !strings.Contains(logs.String(), `"reason":"missing"`) || strings.Contains(logs.String(), path) {
			t.Fatalf("missing reload: %s", logs.String())
		}
		if infoStatus(t, base, cookie, "") != http.StatusOK {
			t.Fatal("session dropped after failed reloads")
		}
	})

	t.Run("env token is not rotated", func(t *testing.T) {
		var logs bytes.Buffer
		srv, base, cleanup := startConfigured(t, Config{
			Token:             bearerToken,
			LoginFailureDelay: -1,
			Logger:            log.New(&logs, "", 0),
		})
		defer cleanup()
		cookie := loginCookie(t, base)
		srv.reloadTokenFile()
		if srv.currentToken() != bearerToken || infoStatus(t, base, cookie, "") != http.StatusOK {
			t.Fatal("env token reload changed credentials")
		}
		if !strings.Contains(logs.String(), `"kind":"token_reload_failed"`) || !strings.Contains(logs.String(), `"reason":"not a file"`) {
			t.Fatalf("env reload audit: %s", logs.String())
		}
	})

	t.Run("reload loop stops", func(t *testing.T) {
		srv, err := New(Config{Listen: "127.0.0.1:0", Logger: log.New(io.Discard, "", 0), LoginFailureDelay: -1})
		if err != nil {
			t.Fatal(err)
		}
		before := runtime.NumGoroutine()
		ch := make(chan os.Signal, 1)
		srv.startReload(ch, nil)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			t.Fatal(err)
		}
		if after := runtime.NumGoroutine(); after > before {
			t.Fatalf("goroutines %d -> %d", before, after)
		}
	})
}
