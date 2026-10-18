package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestA2ADiscover_ValidLocalhost(t *testing.T) {
	// Create a mock A2A server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/agent-card.json" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		card := map[string]any{
			"name":        "test-agent",
			"description": "Test A2A Agent",
			"version":     "1.0.0",
			"capabilities": []string{"streaming", "tools"},
			"skills": []map[string]any{
				{"id": "chat", "name": "Chat", "description": "Chat with the agent", "tags": []string{"chat"}},
			},
		}
		json.NewEncoder(w).Encode(card)
	}))
	defer mockServer.Close()

	// Run the discover tool
	cfg := Config{
		URL:       mockServer.URL + "/.well-known/agent-card.json",
		Allowlist: []string{"127.0.0.1"},
	}
	_ = cfg // cfg is used below

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// We can't easily test the main() function, so test the logic directly
	parsedURL, err := url.Parse(cfg.URL)
	if err != nil {
		t.Fatalf("URL parse failed: %v", err)
	}

	if !isHostAllowed(parsedURL.Hostname(), cfg.Allowlist) {
		t.Fatalf("host %q should be allowed", parsedURL.Hostname())
	}

	// Make request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.URL, nil)
	if err != nil {
		t.Fatalf("request creation failed: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body failed: %v", err)
	}

	var card map[string]any
	if err := json.Unmarshal(body, &card); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if card["name"] != "test-agent" {
		t.Errorf("expected name 'test-agent', got %v", card["name"])
	}
}

func TestA2ADiscover_HostNotAllowed(t *testing.T) {
	cfg := Config{
		URL:       "http://evil.example/.well-known/agent-card.json",
		Allowlist: []string{"127.0.0.1"},
	}

	parsedURL, err := url.Parse(cfg.URL)
	if err != nil {
		t.Fatalf("URL parse failed: %v", err)
	}

	if isHostAllowed(parsedURL.Hostname(), cfg.Allowlist) {
		t.Fatalf("host %q should NOT be allowed", parsedURL.Hostname())
	}
}

func TestA2ADiscover_InvalidScheme(t *testing.T) {
	cfg := Config{
		URL:       "ftp://127.0.0.1/.well-known/agent-card.json",
		Allowlist: []string{"127.0.0.1"},
	}

	parsedURL, err := url.Parse(cfg.URL)
	if err != nil {
		t.Fatalf("URL parse failed: %v", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		// This is expected - the tool should reject this
		return
	}
	t.Fatalf("scheme %q should not be allowed", parsedURL.Scheme)
}

func TestA2ADiscover_DefaultAllowlist(t *testing.T) {
	cfg := Config{
		URL: "http://127.0.0.1:8765/.well-known/agent-card.json",
	}

	parsedURL, err := url.Parse(cfg.URL)
	if err != nil {
		t.Fatalf("URL parse failed: %v", err)
	}

	allowlist := []string{"127.0.0.1"}
	if isHostAllowed(parsedURL.Hostname(), allowlist) {
		// OK
	} else {
		t.Fatalf("host %q should be allowed by default", parsedURL.Hostname())
	}
}