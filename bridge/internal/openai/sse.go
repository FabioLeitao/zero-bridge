package openai

import (
	"bytes"
	"encoding/json"
	"strings"
)

// EncodeCompletion writes one non-stream chat completion. usage is omitted.
func EncodeCompletion(id, model, text string) ([]byte, error) {
	if !safeID(id) || !safeID(model) {
		return nil, invalid("id", "invalid id")
	}
	doc := map[string]any{
		"id":     id,
		"object": "chat.completion",
		"model":  model,
		"choices": []any{
			map[string]any{
				"index": 0,
				"message": map[string]string{
					"role":    "assistant",
					"content": text,
				},
				"finish_reason": "stop",
			},
		},
	}
	return json.Marshal(doc)
}

// EncodeChunk writes one SSE data line. The payload is a single JSON line.
func EncodeChunk(id, model, delta string) ([]byte, error) {
	if !safeID(id) || !safeID(model) {
		return nil, invalid("id", "invalid id")
	}
	doc := map[string]any{
		"id":     id,
		"object": "chat.completion.chunk",
		"model":  model,
		"choices": []any{
			map[string]any{
				"index":         0,
				"delta":         map[string]string{"content": delta},
				"finish_reason": nil,
			},
		},
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	if bytes.Contains(raw, []byte("\n")) {
		return nil, invalid("id", "invalid id")
	}
	var out bytes.Buffer
	out.WriteString("data: ")
	out.Write(raw)
	out.WriteString("\n\n")
	return out.Bytes(), nil
}

// EncodeDone is the terminal SSE line.
func EncodeDone() []byte {
	return []byte("data: [DONE]\n\n")
}

func safeID(s string) bool {
	return s != "" && !strings.ContainsAny(s, "\r\n")
}
