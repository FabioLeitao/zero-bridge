package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"zero-bridge/bridge/internal/sessionmeta"
)

func (s *Server) handleSessionMeta(w http.ResponseWriter, r *http.Request) {
	caps, ok := s.agentCaps()
	if !ok || !caps.List() {
		s.notSupported(w)
		return
	}
	if s.meta == nil {
		s.writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "session metadata unavailable"})
		return
	}
	if !requireJSONContentType(r) {
		s.writeJSON(w, http.StatusUnsupportedMediaType, map[string]any{"error": "content-type must be application/json"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	var body struct {
		Alias    json.RawMessage `json:"alias"`
		Archived *bool           `json:"archived"`
		Pinned   *bool           `json:"pinned"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	var extra struct{}
	if err := dec.Decode(&extra); err != io.EOF {
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	upd := sessionmeta.Update{Archived: body.Archived, Pinned: body.Pinned}
	if body.Alias != nil {
		if string(body.Alias) == "null" {
			empty := ""
			upd.Alias = &empty
		} else {
			var alias string
			if err := json.Unmarshal(body.Alias, &alias); err != nil {
				s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid alias"})
				return
			}
			upd.Alias = &alias
		}
	}
	entry, err := s.meta.Set(r.PathValue("id"), upd)
	if err != nil {
		if errors.Is(err, sessionmeta.ErrFull) {
			s.writeJSON(w, http.StatusInsufficientStorage, map[string]any{"error": err.Error()})
			return
		}
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	s.writeJSON(w, http.StatusOK, entry)
}
