package server

import (
	"encoding/json"
	"net/http"

	"zero-bridge/bridge/internal/acpclient"
	"zero-bridge/bridge/internal/permissions"
	"zero-bridge/bridge/internal/sessionstate"
)

type permInbound struct{ s *Server }

func (h permInbound) HandleRequest(method string, params json.RawMessage) (any, error) {
	if method == "session/request_permission" {
		var p struct {
			SessionID string `json:"sessionId"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return acpclient.DefaultInboundHandler{}.HandleRequest(method, params)
		}
		return h.s.perms.Wait(p.SessionID, params), nil
	}
	return acpclient.DefaultInboundHandler{}.HandleRequest(method, params)
}

func (s *Server) onPermission(ev permissions.Event) {
	hub := s.sessionHub(ev.SessionID)
	if hub == nil {
		return
	}
	tool := RedactJSON(ev.ToolCall)
	switch ev.Type {
	case "permission_request":
		s.applyState(ev.SessionID, sessionstate.Event{Kind: sessionstate.KindPermit})
		hub.emit(map[string]any{
			"type":      "permission_request",
			"reqId":     ev.ReqID,
			"toolCall":  tool,
			"decisions": ev.Decisions,
			"expiresAt": ev.ExpiresAt,
		})
	case "permission_resolved":
		s.applyState(ev.SessionID, sessionstate.Event{Kind: sessionstate.KindRelease})
		hub.emit(map[string]any{
			"type":     "permission_resolved",
			"reqId":    ev.ReqID,
			"decision": ev.Decision,
		})
	}
}

func (s *Server) handlePending(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if s.sessionHub(id) == nil {
		s.writeJSON(w, http.StatusNotFound, map[string]any{"error": "session not found"})
		return
	}
	p := s.perms.Pending(id)
	var pending any
	if p.ReqID != "" {
		pending = map[string]any{
			"reqId":     p.ReqID,
			"toolCall":  RedactJSON(p.ToolCall),
			"decisions": p.Decisions,
			"expiresAt": p.ExpiresAt,
		}
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"pending": pending, "queued": p.Queued})
}

func (s *Server) handlePermission(w http.ResponseWriter, r *http.Request) {
	if !requireJSONContentType(r) {
		s.writeJSON(w, http.StatusUnsupportedMediaType, map[string]any{"error": "content-type must be application/json"})
		return
	}
	id := r.PathValue("id")
	reqID := r.PathValue("reqId")
	if s.sessionHub(id) == nil {
		s.writeJSON(w, http.StatusNotFound, map[string]any{"error": "session not found"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	var body struct {
		Decision string `json:"decision"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	switch got := s.perms.Resolve(id, reqID, body.Decision); got {
	case "ok":
		s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	case "ok:deny":
		s.writeJSON(w, http.StatusOK, map[string]any{"ok": true, "effective": "deny"})
	case "bad":
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "decision must be allow or deny"})
	case "unknown":
		s.writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
	case "resolved:expired":
		s.writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "reason": "expired"})
	default:
		s.writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "reason": "already resolved"})
	}
}
