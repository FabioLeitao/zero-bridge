package openai

import (
	"encoding/json"
	"net/http"
)

// Error is an OpenAI error object. Message is a fixed phrase, never a path,
// a token, a session id, or agent stderr.
type Error struct {
	Status  int
	Type    string
	Code    string
	Message string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

// WriteError writes the OpenAI error envelope and nothing else.
func WriteError(w http.ResponseWriter, e *Error) {
	if e == nil {
		e = &Error{Status: http.StatusBadGateway, Type: "api_error", Code: "internal", Message: "request failed"}
	}
	status := e.Status
	if status == 0 {
		status = http.StatusBadRequest
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"message": e.Message,
			"type":    e.Type,
			"code":    e.Code,
		},
	})
}
