package openai

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

const (
	ModelID         = "zero"
	MaxMessages     = 32
	MaxMessageBytes = 16 << 10
	MaxHistoryBytes = 64 << 10
	MaxBodyBytes    = 1 << 20
)

// Chat is a parsed completion request. Role text is conversation only.
// The struct has no mode, permission, workspace, or policy field.
type Chat struct {
	Model    string    `json:"model"`
	Stream   bool      `json:"stream"`
	Messages []Message `json:"messages"`
}

// Message is one turn. Text is the only payload the bridge keeps.
type Message struct {
	Role string `json:"role"`
	Text string `json:"text"`
}

// Transcript renders the turns for a single ACP prompt.
func (c Chat) Transcript() string {
	var b strings.Builder
	for i, m := range c.Messages {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteByte('[')
		b.WriteString(m.Role)
		b.WriteString("] ")
		b.WriteString(m.Text)
	}
	return b.String()
}

// KnownModel reports whether the bridge serves this model id.
func KnownModel(id string) bool {
	return id == ModelID
}

var forbiddenFields = map[string]struct{}{
	"tools":           {},
	"tool_choice":     {},
	"functions":       {},
	"function_call":   {},
	"logprobs":        {},
	"response_format": {},
	"audio":           {},
}

var allowedRoles = map[string]struct{}{
	"system":    {},
	"developer": {},
	"user":      {},
	"assistant": {},
}

// Parse reads one chat completion body. Unsupported fields are errors.
// A system or developer role stays inside the transcript.
func Parse(body []byte) (Chat, *Error) {
	if len(body) == 0 || len(body) > MaxBodyBytes {
		return Chat{}, invalid("invalid_body", "invalid json")
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var raw map[string]json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return Chat{}, invalid("invalid_body", "invalid json")
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return Chat{}, invalid("invalid_body", "invalid json")
	}
	for key := range raw {
		if _, bad := forbiddenFields[key]; bad {
			return Chat{}, invalid("unsupported_field", "unsupported field")
		}
	}
	if err := checkN(raw["n"]); err != nil {
		return Chat{}, err
	}
	var model string
	if err := json.Unmarshal(raw["model"], &model); err != nil || strings.TrimSpace(model) == "" {
		return Chat{}, invalid("model", "model is required")
	}
	var stream bool
	if rawStream, ok := raw["stream"]; ok {
		if err := json.Unmarshal(rawStream, &stream); err != nil {
			return Chat{}, invalid("stream", "stream must be a boolean")
		}
	}
	msgs, err := parseMessages(raw["messages"])
	if err != nil {
		return Chat{}, err
	}
	return Chat{Model: model, Stream: stream, Messages: msgs}, nil
}

func checkN(raw json.RawMessage) *Error {
	if len(raw) == 0 {
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err != nil {
		return invalid("unsupported_field", "unsupported field")
	}
	if n.String() != "1" {
		return invalid("unsupported_field", "unsupported field")
	}
	return nil
}

func parseMessages(raw json.RawMessage) ([]Message, *Error) {
	if len(raw) == 0 {
		return nil, invalid("messages", "messages are required")
	}
	var rows []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, invalid("messages", "messages are required")
	}
	if len(rows) == 0 || len(rows) > MaxMessages {
		return nil, invalid("messages", "too many messages")
	}
	out := make([]Message, 0, len(rows))
	total := 0
	for _, row := range rows {
		for key := range row {
			switch key {
			case "role", "content", "name":
			default:
				return nil, invalid("unsupported_field", "unsupported field")
			}
		}
		var role string
		if err := json.Unmarshal(row["role"], &role); err != nil {
			return nil, invalid("messages", "invalid role")
		}
		if _, ok := allowedRoles[role]; !ok {
			return nil, invalid("messages", "invalid role")
		}
		text, err := messageText(row["content"])
		if err != nil {
			return nil, err
		}
		if len(text) > MaxMessageBytes {
			return nil, invalid("messages", "message is too large")
		}
		total += len(text)
		if total > MaxHistoryBytes {
			return nil, invalid("messages", "history is too large")
		}
		out = append(out, Message{Role: role, Text: text})
	}
	return out, nil
}

func messageText(raw json.RawMessage) (string, *Error) {
	if len(raw) == 0 {
		return "", invalid("messages", "content is required")
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text, nil
	}
	var parts []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &parts); err != nil || len(parts) == 0 {
		return "", invalid("messages", "content is required")
	}
	var b strings.Builder
	for i, part := range parts {
		var kind string
		if err := json.Unmarshal(part["type"], &kind); err != nil || kind != "text" {
			return "", invalid("unsupported_field", "unsupported content")
		}
		var piece string
		if err := json.Unmarshal(part["text"], &piece); err != nil {
			return "", invalid("messages", "content is required")
		}
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(piece)
	}
	return b.String(), nil
}

func invalid(code, message string) *Error {
	return &Error{Status: 400, Type: "invalid_request_error", Code: code, Message: message}
}
