package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestSessionStatusByProfile(t *testing.T) {
	t.Run("zero", func(t *testing.T) {
		_, base, cleanup := startTestServer(t, nil)
		defer cleanup()
		if got := capabilityStatus(t, base); got != "full" {
			t.Fatalf("sessionStatus %s", got)
		}
	})
	t.Run("generic", func(t *testing.T) {
		_, base, cleanup := startConfigured(t, Config{Profile: "generic"})
		defer cleanup()
		if got := capabilityStatus(t, base); got != "basic" {
			t.Fatalf("sessionStatus %s", got)
		}
	})
	t.Run("generic never blocked", func(t *testing.T) {
		_, base, cleanup := startConfigured(t, Config{
			Profile:  "generic",
			ExtraEnv: []string{"ZB_FAKE_HOLD_PERMISSION=1"},
		})
		defer cleanup()
		load := postAttach(t, base+"/v1/sessions/sess-a1/load", `{"workspace":"/tmp/zbridge-ws-a"}`)
		if load.StatusCode != http.StatusOK {
			t.Fatalf("load %d %s", load.StatusCode, load.Body)
		}
		postPromptAsync(base, "sess-a1", "secret-prompt")
		_ = waitPending(t, base, "sess-a1")
		row := sessionRow(t, base, "sess-a1")
		if row["state"] != "working" {
			t.Fatalf("list state %#v", row["state"])
		}
		for _, st := range statesSeen(t, base+"/v1/sessions/sess-a1/events") {
			if st == "blocked" || st == "done" || st == "error" {
				t.Fatalf("generic reported %s", st)
			}
		}
	})
}

func statesSeen(t *testing.T, url string) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
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
	var seen []string
	_ = collectEvents(t, res.Body, 0, func(ev sseEvent) bool {
		if ev.raw["type"] != "session_state" {
			return false
		}
		st, _ := ev.raw["state"].(string)
		seen = append(seen, st)
		return false
	})
	if len(seen) == 0 {
		t.Fatal("no session_state")
	}
	return seen
}

func capabilityStatus(t *testing.T, base string) string {
	t.Helper()
	res, err := http.Get(base + "/v1/capabilities")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var body struct {
		SessionStatus string `json:"sessionStatus"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("capabilities %d", res.StatusCode)
	}
	return body.SessionStatus
}

func TestSessionStateFromTraffic(t *testing.T) {
	t.Run("pending permission stays blocked", func(t *testing.T) {
		_, base, cleanup := startTestServer(t, []string{"ZB_FAKE_HOLD_PERMISSION=1"})
		defer cleanup()
		load := postAttach(t, base+"/v1/sessions/sess-a1/load", `{"workspace":"/tmp/zbridge-ws-a"}`)
		if load.StatusCode != http.StatusOK {
			t.Fatalf("load %d %s", load.StatusCode, load.Body)
		}
		postPromptAsync(base, "sess-a1", "secret-prompt")
		ev := waitState(t, base+"/v1/sessions/sess-a1/events", "blocked")
		if strings.Contains(ev, "secret-prompt") || strings.Contains(ev, "secret-arg") || strings.Contains(ev, "<") {
			t.Fatalf("state leaked %s", ev)
		}
		row := sessionRow(t, base, "sess-a1")
		if row["state"] != "blocked" {
			t.Fatalf("list state %#v", row["state"])
		}
		title, _ := row["title"].(string)
		if !strings.Contains(title, "<img") {
			t.Fatalf("title %q", title)
		}
		if strings.Contains(row["state"].(string), "<") || strings.Contains(row["state"].(string), "secret") {
			t.Fatalf("list state leaked %#v", row["state"])
		}
	})

	t.Run("turn error", func(t *testing.T) {
		_, base, cleanup := startTestServer(t, []string{"ZB_FAKE_TURN_ERROR=1"})
		defer cleanup()
		load := postAttach(t, base+"/v1/sessions/sess-a1/load", `{"workspace":"/tmp/zbridge-ws-a"}`)
		if load.StatusCode != http.StatusOK {
			t.Fatalf("load %d %s", load.StatusCode, load.Body)
		}
		postPromptAsync(base, "sess-a1", "secret-prompt")
		ev := waitState(t, base+"/v1/sessions/sess-a1/events", "error")
		if strings.Contains(ev, "secret-prompt") || strings.Contains(ev, "turn failed") {
			t.Fatalf("state leaked %s", ev)
		}
		row := sessionRow(t, base, "sess-a1")
		if row["state"] != "error" {
			t.Fatalf("list state %#v", row["state"])
		}
	})

	t.Run("agent exit", func(t *testing.T) {
		_, base, cleanup := startTestServer(t, []string{"ZB_FAKE_PROMPT_EXIT=1"})
		defer cleanup()
		load := postAttach(t, base+"/v1/sessions/sess-a1/load", `{"workspace":"/tmp/zbridge-ws-a"}`)
		if load.StatusCode != http.StatusOK {
			t.Fatalf("load %d %s", load.StatusCode, load.Body)
		}
		postPromptAsync(base, "sess-a1", "secret-prompt")
		ev := waitState(t, base+"/v1/sessions/sess-a1/events", "error")
		if strings.Contains(ev, "secret-prompt") {
			t.Fatalf("state leaked %s", ev)
		}
	})

	t.Run("no permission ends done", func(t *testing.T) {
		_, base, cleanup := startTestServer(t, []string{"ZB_FAKE_NO_PERMISSION=1"})
		defer cleanup()
		load := postAttach(t, base+"/v1/sessions/sess-a1/load", `{"workspace":"/tmp/zbridge-ws-a"}`)
		if load.StatusCode != http.StatusOK {
			t.Fatalf("load %d %s", load.StatusCode, load.Body)
		}
		before := sessionRow(t, base, "sess-a1")
		if before["state"] != "idle" {
			t.Fatalf("before %#v", before["state"])
		}
		postPromptAsync(base, "sess-a1", "secret-prompt")
		ev := waitState(t, base+"/v1/sessions/sess-a1/events", "done")
		if strings.Contains(ev, "blocked") || strings.Contains(ev, "secret-prompt") {
			t.Fatalf("state %s", ev)
		}
		row := sessionRow(t, base, "sess-a1")
		if row["state"] != "done" {
			t.Fatalf("list state %#v", row["state"])
		}
	})
}

func waitState(t *testing.T, url, want string) string {
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
	var seen []string
	found := collectEvents(t, res.Body, 0, func(ev sseEvent) bool {
		if ev.raw["type"] != "session_state" {
			return false
		}
		st, _ := ev.raw["state"].(string)
		seen = append(seen, st)
		blob, _ := json.Marshal(ev.raw)
		if strings.Contains(string(blob), "secret-prompt") || strings.Contains(string(blob), "secret-arg") || strings.Contains(st, "<") {
			t.Fatalf("state leaked %s", blob)
		}
		return st == want
	})
	if len(found) == 0 {
		t.Fatalf("missing state %s, saw %v", want, seen)
	}
	return strings.Join(seen, ",")
}

func sessionRow(t *testing.T, base, id string) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/v1/sessions", nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("list %d %s", res.StatusCode, body)
	}
	if strings.Contains(string(body), "secret-prompt") || strings.Contains(string(body), "secret-arg") {
		t.Fatalf("list leaked %s", body)
	}
	var page struct {
		Sessions []map[string]any `json:"sessions"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		t.Fatal(err)
	}
	for _, row := range page.Sessions {
		if row["sessionId"] == id {
			return row
		}
	}
	t.Fatalf("missing %s in %s", id, body)
	return nil
}
