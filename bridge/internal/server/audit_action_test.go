package server

import (
	"bytes"
	"log"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPermissionDecisionsLeaveOneAuditLine(t *testing.T) {
	t.Run("allow", func(t *testing.T) {
		buf, base, stop := startAuditServer(t, []string{"ZB_FAKE_PERMISSION=1"}, 0)
		defer stop()
		dir := t.TempDir()
		sid := createSession(t, base, dir)
		const prompt = "prompt-marker-allow"
		postPromptAsync(base, sid, prompt)
		reqID := waitPending(t, base, sid)
		res := postDecision(t, base, sid, reqID, "allow")
		res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("allow %d", res.StatusCode)
		}
		line := oneAuditKind(t, buf, "permission_allow")
		assertAuditOmits(t, line, sid, dir, prompt, reqID, "tc-perm")
		assertPermissionSource(t, line, true)
	})
	t.Run("deny", func(t *testing.T) {
		buf, base, stop := startAuditServer(t, []string{"ZB_FAKE_PERMISSION=1"}, 0)
		defer stop()
		dir := t.TempDir()
		sid := createSession(t, base, dir)
		const prompt = "prompt-marker-deny"
		postPromptAsync(base, sid, prompt)
		reqID := waitPending(t, base, sid)
		res := postDecision(t, base, sid, reqID, "deny")
		res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("deny %d", res.StatusCode)
		}
		line := oneAuditKind(t, buf, "permission_deny")
		assertAuditOmits(t, line, sid, dir, prompt, reqID, "tc-perm")
		assertPermissionSource(t, line, true)
	})
	t.Run("expire", func(t *testing.T) {
		var buf safeBuf
		_, base, stop := startConfigured(t, Config{
			ExtraEnv:          []string{"ZB_FAKE_PERMISSION=1"},
			Logger:            log.New(&buf, "", 0),
			PermissionTimeout: 300 * time.Millisecond,
		})
		defer stop()
		dir := t.TempDir()
		sid := createSession(t, base, dir)
		const prompt = "prompt-marker-expire"
		postPromptAsync(base, sid, prompt)
		reqID := waitPending(t, base, sid)
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) && !strings.Contains(buf.String(), `"kind":"permission_expire"`) {
			time.Sleep(20 * time.Millisecond)
		}
		snap := bytes.NewBufferString(buf.String())
		line := oneAuditKind(t, snap, "permission_expire")
		assertAuditOmits(t, line, sid, dir, prompt, reqID, "tc-perm")
		assertPermissionSource(t, line, false)
		if strings.Contains(buf.String(), `"kind":"permission_allow"`) {
			t.Fatalf("timeout also allowed: %s", buf.String())
		}
	})
}

// safeBuf is a bytes.Buffer the audit logger and a polling test can share.
type safeBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *safeBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *safeBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func TestHTTPAndExpireEachLeaveOnePermissionLine(t *testing.T) {
	var buf bytes.Buffer
	clk := &permClock{}
	srv, base, stop := startConfigured(t, Config{
		ExtraEnv:          []string{"ZB_FAKE_PERMISSION=1"},
		Logger:            log.New(&buf, "", 0),
		PermissionTimeout: time.Hour,
	})
	defer stop()
	srv.perms.Clock = clk
	dir := t.TempDir()
	// Both hubs exist before either permission. The fake agent answers one
	// permission at a time, so the expire side is enqueued on the broker.
	allowSid := createSession(t, base, dir)
	expireSid := createSession(t, base, dir)
	waited := make(chan struct{})
	go func() {
		srv.perms.Wait(expireSid, []byte(`{"toolCall":{"toolCallId":"tc-perm"}}`))
		close(waited)
	}()
	expireReq := waitPending(t, base, expireSid)
	postPromptAsync(base, allowSid, "prompt-allow")
	allowReq := waitPending(t, base, allowSid)
	fired := make(chan struct{})
	go func() {
		clk.fire(0)
		close(fired)
	}()
	res := postDecision(t, base, allowSid, allowReq, "allow")
	res.Body.Close()
	<-fired
	select {
	case <-waited:
	case <-time.After(2 * time.Second):
		t.Fatal("expire wait did not return")
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("allow %d log %s", res.StatusCode, buf.String())
	}
	allow := oneAuditKind(t, &buf, "permission_allow")
	expire := oneAuditKind(t, &buf, "permission_expire")
	assertAuditOmits(t, allow, allowSid, expireSid, allowReq, expireReq, dir, "prompt-allow")
	assertAuditOmits(t, expire, allowSid, expireSid, allowReq, expireReq, dir, "prompt-allow")
	assertPermissionSource(t, allow, true)
	assertPermissionSource(t, expire, false)
}

type permClock struct {
	mu sync.Mutex
	ch []chan time.Time
}

func (c *permClock) Now() time.Time { return time.Now() }

func (c *permClock) After(time.Duration) <-chan time.Time {
	ch := make(chan time.Time, 1)
	c.mu.Lock()
	c.ch = append(c.ch, ch)
	c.mu.Unlock()
	return ch
}

func (c *permClock) fire(i int) {
	c.mu.Lock()
	ch := c.ch[i]
	c.mu.Unlock()
	ch <- time.Now()
}

func TestPromptAndSessionAuditOmitSecrets(t *testing.T) {
	buf, base, stop := startAuditServer(t, nil, 0)
	defer stop()
	dir := t.TempDir()
	sid := createSession(t, base, dir)
	const prompt = "prompt-marker-body"
	postPrompt(t, base, sid, prompt)
	sessionLine := oneAuditKind(t, buf, "session_created")
	promptLine := oneAuditKind(t, buf, "prompt_submitted")
	assertAuditOmits(t, sessionLine, sid, dir, prompt)
	assertAuditOmits(t, promptLine, sid, dir, prompt)
}

func TestIngressAuditNamesRouteAndCredential(t *testing.T) {
	token := strings.Repeat("z", 32)
	const prompt = "ingress-marker-body"
	buf, base, stop := startAuditServer(t, nil, 0, token)
	defer stop()
	chatBody := `{"model":"zero","messages":[{"role":"user","content":"` + prompt + `"}]}`
	req, err := http.NewRequest(http.MethodPost, base+"/v1/chat/completions", strings.NewReader(chatBody))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("chat %d", res.StatusCode)
	}
	line := oneAuditKind(t, buf, "ingress")
	if !strings.Contains(line, "/v1/chat/completions") || !strings.Contains(line, "bearer") {
		t.Fatalf("ingress line %s", line)
	}
	assertAuditOmits(t, line, token, prompt)
}

func startAuditServer(t *testing.T, extra []string, timeout time.Duration, token ...string) (*bytes.Buffer, string, func()) {
	t.Helper()
	var buf bytes.Buffer
	cfg := Config{
		ExtraEnv: extra,
		Logger:   log.New(&buf, "", 0),
	}
	if timeout > 0 {
		cfg.PermissionTimeout = timeout
	}
	if len(token) > 0 {
		cfg.Token = token[0]
	}
	_, base, stop := startConfigured(t, cfg)
	return &buf, base, stop
}

func oneAuditKind(t *testing.T, buf *bytes.Buffer, kind string) string {
	t.Helper()
	needle := `"kind":"` + kind + `"`
	var lines []string
	for _, line := range strings.Split(buf.String(), "\n") {
		if strings.Contains(line, needle) {
			lines = append(lines, line)
		}
	}
	if len(lines) != 1 {
		t.Fatalf("%s lines %d in %s", kind, len(lines), buf.String())
	}
	return lines[0]
}

func assertPermissionSource(t *testing.T, line string, fromHTTP bool) {
	t.Helper()
	const route = "/v1/sessions/{id}/permissions/{reqId}"
	if fromHTTP {
		if !strings.Contains(line, `"source":"127.0.0.1"`) || !strings.Contains(line, route) {
			t.Fatalf("http decision line %s", line)
		}
		return
	}
	if !strings.Contains(line, `"source":""`) || strings.Contains(line, route) {
		t.Fatalf("expire line %s", line)
	}
}

func assertAuditOmits(t *testing.T, line string, forbidden ...string) {
	t.Helper()
	for _, secret := range forbidden {
		if secret != "" && strings.Contains(line, secret) {
			t.Fatalf("line contains %q: %s", secret, line)
		}
	}
}
