package server

import (
	"encoding/json"
	"regexp"
)

var (
	reSk         = regexp.MustCompile(`sk-[A-Za-z0-9_-]+`)
	reBearer     = regexp.MustCompile(`Bearer\s+[^\s"'\\]+`)
	reAuthHeader = regexp.MustCompile(`(?i)authorization:\s*[^\s"'\\]+`)
)

// Redact removes likely secrets from error text shown to clients or logs.
func Redact(s string) string {
	s = reSk.ReplaceAllString(s, "[redacted]")
	s = reBearer.ReplaceAllString(s, "[redacted]")
	s = reAuthHeader.ReplaceAllString(s, "authorization: [redacted]")
	return s
}

// RedactJSON applies Redact to every string in a JSON value and returns
// encoded JSON. Undecodable input becomes {}.
func RedactJSON(raw []byte) json.RawMessage {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return json.RawMessage(`{}`)
	}
	out, err := json.Marshal(redactValue(v))
	if err != nil || !json.Valid(out) {
		return json.RawMessage(`{}`)
	}
	return out
}

func redactValue(v any) any {
	switch t := v.(type) {
	case string:
		return Redact(t)
	case map[string]any:
		for k, val := range t {
			t[k] = redactValue(val)
		}
		return t
	case []any:
		for i := range t {
			t[i] = redactValue(t[i])
		}
		return t
	default:
		return v
	}
}
