package server

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentCardNamesTheFakeAgent(t *testing.T) {
	_, base, stop := startConfigured(t, Config{})
	defer stop()
	res, err := http.Get(base + "/.well-known/agent-card.json")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d %s", res.StatusCode, raw)
	}
	body := string(raw)
	for _, want := range []string{`"name":"acptest"`, `"version":"0"`, `"id":"chat"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s in %s", want, body)
		}
	}
}

func TestA2ARoutesRequireTokenWhenSet(t *testing.T) {
	_, base, stop := startConfigured(t, Config{Token: bearerToken})
	defer stop()
	card, err := http.Get(base + "/.well-known/agent-card.json")
	if err != nil {
		t.Fatal(err)
	}
	card.Body.Close()
	if card.StatusCode != http.StatusUnauthorized {
		t.Fatalf("card without credential: %d", card.StatusCode)
	}
	send := postA2A(t, base, a2aUser("hi"), "")
	if send.StatusCode != http.StatusUnauthorized {
		t.Fatalf("send without credential: %d", send.StatusCode)
	}
	okCard, err := http.NewRequest(http.MethodGet, base+"/.well-known/agent-card.json", nil)
	if err != nil {
		t.Fatal(err)
	}
	okCard.Header.Set("Authorization", "Bearer "+bearerToken)
	okRes, err := http.DefaultClient.Do(okCard)
	if err != nil {
		t.Fatal(err)
	}
	defer okRes.Body.Close()
	if okRes.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(okRes.Body)
		t.Fatalf("card with bearer: %d %s", okRes.StatusCode, raw)
	}
	okSend := postA2A(t, base, a2aUser("hi"), bearerToken)
	if okSend.StatusCode != http.StatusOK || !strings.Contains(okSend.Body, `"completed"`) {
		t.Fatalf("send with bearer: %d %s", okSend.StatusCode, okSend.Body)
	}
}

func TestA2ARejectsRolePartAndSize(t *testing.T) {
	_, base, stop := startConfigured(t, Config{A2AMaxBodyBytes: 160})
	defer stop()
	agent := postA2A(t, base, `{"jsonrpc":"2.0","id":1,"method":"message/send","params":{"message":{"role":"agent","parts":[{"type":"text","text":"hi"}]}}}`, "")
	if agent.StatusCode != http.StatusBadRequest || !strings.Contains(agent.Body, "only user role") {
		t.Fatalf("role: %d %s", agent.StatusCode, agent.Body)
	}
	file := postA2A(t, base, `{"jsonrpc":"2.0","id":1,"method":"message/send","params":{"message":{"role":"user","parts":[{"type":"file","text":"x"}]}}}`, "")
	if file.StatusCode != http.StatusBadRequest || !strings.Contains(file.Body, "only text parts") {
		t.Fatalf("part: %d %s", file.StatusCode, file.Body)
	}
	huge := `{"jsonrpc":"2.0","id":1,"method":"message/send","params":{"message":{"role":"user","parts":[{"type":"text","text":"` + strings.Repeat("a", 200) + `"}]}}}`
	over := postA2A(t, base, huge, "")
	if over.StatusCode != http.StatusBadRequest || !strings.Contains(over.Body, "parse error") {
		t.Fatalf("size: %d %s", over.StatusCode, over.Body)
	}
}

func TestA2APromptReachesAgent(t *testing.T) {
	capture := filepath.Join(t.TempDir(), "prompt")
	_, base, stop := startConfigured(t, Config{ExtraEnv: []string{"ZB_FAKE_CAPTURE_PROMPT=" + capture}})
	defer stop()
	const marker = "bridge-a2a-prompt-marker"
	got := postA2A(t, base, a2aUser(marker), "")
	if got.StatusCode != http.StatusOK {
		t.Fatalf("status %d %s", got.StatusCode, got.Body)
	}
	raw, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, marker) || !strings.Contains(text, "session/prompt") {
		t.Fatalf("capture %s", text)
	}
}

type a2aResult struct {
	StatusCode int
	Body       string
}

func a2aUser(text string) string {
	return `{"jsonrpc":"2.0","id":1,"method":"message/send","params":{"message":{"role":"user","parts":[{"type":"text","text":"` + text + `"}]}}}`
}

func postA2A(t *testing.T, base, body, token string) a2aResult {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, base+"/a2a/message/send", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	return a2aResult{StatusCode: res.StatusCode, Body: string(raw)}
}
