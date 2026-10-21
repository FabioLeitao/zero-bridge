package server

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zero-bridge/bridge/internal/openai"
)

func TestOpenAIModelsListsServedProfiles(t *testing.T) {
	_, base, stop := startTestServer(t, nil)
	defer stop()
	res, err := http.Get(base + "/v1/models")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d %s", res.StatusCode, raw)
	}
	var doc struct {
		Object string `json:"object"`
		Data   []struct {
			ID     string `json:"id"`
			Object string `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Object != "list" {
		t.Fatalf("object %q", doc.Object)
	}
	var ids []string
	for _, item := range doc.Data {
		if item.Object != "model" || item.ID == "" {
			t.Fatalf("item %+v", item)
		}
		ids = append(ids, item.ID)
	}
	if len(ids) != 1 || ids[0] != openai.ModelID {
		t.Fatalf("profiles %v, served %s", ids, openai.ModelID)
	}
}

func TestOpenAIStreamSuccessChunk(t *testing.T) {
	const marker = "chunk-ok"
	_, base, stop := startConfigured(t, Config{ExtraEnv: []string{"ZB_FAKE_TEXT=" + marker}})
	defer stop()
	req, err := http.NewRequest(http.MethodPost, base+"/v1/chat/completions", strings.NewReader(`{"model":"zero","stream":true,"messages":[{"role":"user","content":"hi"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("status %d type %q body %s", res.StatusCode, res.Header.Get("Content-Type"), raw)
	}
	sawChunk := false
	sawDone := false
	for _, line := range strings.Split(string(raw), "\n") {
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			t.Fatalf("bare line %q in %s", line, raw)
		}
		if line == "data: [DONE]" {
			sawDone = true
			continue
		}
		var chunk struct {
			Object  string `json:"object"`
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &chunk); err != nil {
			t.Fatalf("chunk %s", line)
		}
		if chunk.Object == "chat.completion.chunk" && len(chunk.Choices) == 1 && chunk.Choices[0].Delta.Content == marker {
			sawChunk = true
		}
	}
	if !sawChunk || !sawDone || strings.Contains(string(raw), "request failed") {
		t.Fatalf("stream %s", raw)
	}
}

func TestInitializeDisablesFSAndTerminal(t *testing.T) {
	capture := filepath.Join(t.TempDir(), "init")
	_, _, stop := startTestServer(t, []string{"ZB_FAKE_CAPTURE_INIT=" + capture})
	defer stop()
	raw, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"method":"initialize"`) {
		t.Fatalf("not initialize: %s", raw)
	}
	for _, needle := range []string{
		`"readTextFile":false`,
		`"writeTextFile":false`,
		`"terminal":false`,
	} {
		if !strings.Contains(string(raw), needle) {
			t.Fatalf("missing %s in %s", needle, raw)
		}
	}
}
