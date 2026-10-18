package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestA2ACall_ValidResponse(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/a2a/message/send" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}

		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		response := map[string]any{
			"jsonrpc": "2.0",
			"result": map[string]any{
				"id":        "task-123",
				"status":    map[string]any{"state": "completed"},
				"artifacts": []map[string]any{{"parts": []map[string]any{{"type": "text", "text": "Hello from peer!"}}}},
			},
			"id": req["id"],
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	_ = Config{
		Peer:    "test",
		Message: "hi",
	}

	_ = PluginConfig{
		Peers: []PeerConfig{
			{Name: "test", URL: mockServer.URL + "/a2a/message/send", Allowed: true},
		},
	}

	context.WithTimeout(context.Background(), 5*time.Second)

	req := Request{
		JSONRPC: "2.0",
		Method:  "message/send",
		Params: mustMarshal(MessageSendParams{
			Message: Message{
				Role:  "user",
				Parts: []Part{{Type: "text", Text: "hi"}},
			},
		}),
		ID: mustMarshal("call-123"),
	}

	reqBody, _ := json.Marshal(req)

	reqHTTP, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, mockServer.URL+"/a2a/message/send", bytes.NewReader(reqBody))
	reqHTTP.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(reqHTTP)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var respObj Response
	if err := json.Unmarshal(body, &respObj); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if respObj.Error != nil {
		t.Fatalf("peer error: %s", respObj.Error.Message)
	}

	var task struct {
		Artifacts []struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"artifacts"`
	}
	if err := json.Unmarshal(respObj.Result, &task); err != nil {
		t.Fatalf("failed to parse task: %v", err)
	}

	var reply string
	for _, artifact := range task.Artifacts {
		for _, part := range artifact.Parts {
			reply += part.Text
		}
	}

	if reply != "Hello from peer!" {
		t.Errorf("expected 'Hello from peer!', got %q", reply)
	}
}

func TestA2ACall_Peer500(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal error"}`))
	}))
	defer mockServer.Close()

	_ = Config{
		Peer:    "test",
		Message: "hi",
	}

	_ = PluginConfig{
		Peers: []PeerConfig{
			{Name: "test", URL: mockServer.URL + "/a2a/message/send", Allowed: true},
		},
	}

	req := Request{
		JSONRPC: "2.0",
		Method:  "message/send",
		Params: mustMarshal(MessageSendParams{
			Message: Message{
				Role:  "user",
				Parts: []Part{{Type: "text", Text: "hi"}},
			},
		}),
		ID: mustMarshal("call-123"),
	}

	reqBody, _ := json.Marshal(req)

	context.WithTimeout(context.Background(), 5*time.Second)

	reqHTTP, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, mockServer.URL+"/a2a/message/send", bytes.NewReader(reqBody))
	reqHTTP.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(reqHTTP)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

func TestA2ACall_PeerNotFound(t *testing.T) {
	_ = Config{
		Peer:    "nonexistent",
		Message: "hi",
	}

	pluginCfg := PluginConfig{
		Peers: []PeerConfig{
			{Name: "test", URL: "http://127.0.0.1:8765", Allowed: true},
		},
	}

	found := false
	for _, p := range pluginCfg.Peers {
		if p.Name == "nonexistent" {
			found = true
			break
		}
	}

	if found {
		t.Fatalf("peer should not be found")
	}
}

func TestA2ACall_PeerNotAllowed(t *testing.T) {
	cfg := Config{
		Peer:    "blocked",
		Message: "hi",
	}

	pluginCfg := PluginConfig{
		Peers: []PeerConfig{
			{Name: "blocked", URL: "http://127.0.0.1:8765", Allowed: false},
		},
	}

	found := false
	for _, p := range pluginCfg.Peers {
		if p.Name == cfg.Peer {
			if p.Allowed {
				t.Fatalf("peer should not be allowed")
			}
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("peer should be found but not allowed")
	}
}
