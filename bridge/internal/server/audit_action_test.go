package server

import (
	"bytes"
	"log"
	"net/http"
	"strings"
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
	})
	t.Run("expire", func(t *testing.T) {
		buf, base, stop := startAuditServer(t, []string{"ZB_FAKE_PERMISSION=1"}, 300*time.Millisecond)
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
		line := oneAuditKind(t, buf, "permission_expire")
		assertAuditOmits(t, line, sid, dir, prompt, reqID, "tc-perm")
		if strings.Contains(buf.String(), `"kind":"permission_allow"`) {
			t.Fatalf("timeout also allowed: %s", buf.String())
		}
	})
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

func assertAuditOmits(t *testing.T, line string, forbidden ...string) {
	t.Helper()
	for _, secret := range forbidden {
		if secret != "" && strings.Contains(line, secret) {
			t.Fatalf("line contains %q: %s", secret, line)
		}
	}
}
