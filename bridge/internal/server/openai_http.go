package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"

	"zero-bridge/bridge/internal/openai"
)

const (
	openaiInflightPerCred   = 1
	openaiInflightProcess   = 4
	openaiPromptsPerSession = 32
)

// openaiGate bounds OpenAI calls. One ACP session per credential, with a
// prompt cap, so the session cannot grow without a limit.
type openaiGate struct {
	mu       sync.Mutex
	inflight map[string]int
	total    int
	slots    map[string]*openaiSlot
}

type openaiSlot struct {
	id      string
	prompts int
}

func openaiRoute(path string) bool {
	return path == "/v1/models" || path == "/v1/chat/completions"
}

func (s *Server) writeRouteError(w http.ResponseWriter, r *http.Request, status int, plain string) {
	if !openaiRoute(r.URL.Path) {
		s.writeJSON(w, status, map[string]any{"error": plain})
		return
	}
	typ := "permission_error"
	code := "forbidden"
	msg := "forbidden"
	switch status {
	case http.StatusUnauthorized:
		typ = "authentication_error"
		code = "unauthorized"
		msg = "unauthorized"
	case http.StatusTooManyRequests:
		typ = "rate_limit_error"
		code = "rate_limit"
		msg = "too many requests"
	}
	openai.WriteError(w, &openai.Error{Status: status, Type: typ, Code: code, Message: msg})
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]any{
		"object": "list",
		"data": []any{
			map[string]any{"id": openai.ModelID, "object": "model", "owned_by": "zero-bridge"},
		},
	})
}

func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if !requireJSONContentType(r) {
		openai.WriteError(w, &openai.Error{Status: http.StatusBadRequest, Type: "invalid_request_error", Code: "content_type", Message: "content-type must be application/json"})
		return
	}
	if !s.requestAllowed(r) {
		openai.WriteError(w, &openai.Error{Status: http.StatusForbidden, Type: "permission_error", Code: "forbidden", Message: "forbidden"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, openai.MaxBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		openai.WriteError(w, &openai.Error{Status: http.StatusBadRequest, Type: "invalid_request_error", Code: "invalid_body", Message: "invalid json"})
		return
	}
	chat, perr := openai.Parse(body)
	if perr != nil {
		openai.WriteError(w, perr)
		return
	}
	if !openai.KnownModel(chat.Model) {
		openai.WriteError(w, &openai.Error{Status: http.StatusBadRequest, Type: "invalid_request_error", Code: "model", Message: "unknown model"})
		return
	}
	key := openaiCredKey(r)
	if !s.openai.acquire(key) {
		openai.WriteError(w, &openai.Error{Status: http.StatusTooManyRequests, Type: "rate_limit_error", Code: "rate_limit", Message: "too many requests"})
		return
	}
	defer s.openai.release(key)
	sessionID, err := s.openaiSession(r.Context(), key)
	if err != nil || sessionID == "" {
		openai.WriteError(w, &openai.Error{Status: http.StatusBadGateway, Type: "api_error", Code: "agent", Message: "request failed"})
		return
	}
	if !s.openai.takePrompt(key) {
		openai.WriteError(w, &openai.Error{Status: http.StatusTooManyRequests, Type: "rate_limit_error", Code: "rate_limit", Message: "too many requests"})
		return
	}
	hub := s.sessionHub(sessionID)
	if hub == nil || !hub.tryStartPrompt() {
		openai.WriteError(w, &openai.Error{Status: http.StatusTooManyRequests, Type: "rate_limit_error", Code: "rate_limit", Message: "too many requests"})
		return
	}
	defer hub.endPrompt()
	s.finishChat(w, r, hub, sessionID, chat)
}

func (s *Server) finishChat(w http.ResponseWriter, r *http.Request, hub *sessionHub, sessionID string, chat openai.Chat) {
	compID := newCompletionID()
	subID, ch, _ := hub.subscribe()
	defer hub.unsubscribe(subID)
	var col strings.Builder
	var colMu sync.Mutex
	if chat.Stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		writePiece := func(raw []byte) {
			piece := agentChunkText(raw)
			if piece == "" {
				return
			}
			colMu.Lock()
			col.WriteString(piece)
			colMu.Unlock()
			if !chat.Stream {
				return
			}
			frame, err := openai.EncodeChunk(compID, openai.ModelID, piece)
			if err != nil {
				return
			}
			_, _ = w.Write(frame)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
		for {
			select {
			case raw := <-ch:
				writePiece(raw)
			case <-stop:
				for {
					select {
					case raw := <-ch:
						writePiece(raw)
					default:
						return
					}
				}
			}
		}
	}()
	var res struct {
		StopReason string `json:"stopReason"`
	}
	err := s.acpCall(r.Context(), "session/prompt", map[string]any{
		"sessionId": sessionID,
		"prompt":    []map[string]any{{"type": "text", "text": chat.Transcript()}},
	}, &res)
	close(stop)
	wg.Wait()
	if r.Context().Err() != nil {
		return
	}
	if err != nil {
		if chat.Stream {
			openai.WriteError(w, &openai.Error{Status: http.StatusBadGateway, Type: "api_error", Code: "agent", Message: "request failed"})
			return
		}
		openai.WriteError(w, &openai.Error{Status: http.StatusBadGateway, Type: "api_error", Code: "agent", Message: "request failed"})
		return
	}
	colMu.Lock()
	text := col.String()
	colMu.Unlock()
	if chat.Stream {
		_, _ = w.Write(openai.EncodeDone())
		return
	}
	raw, encErr := openai.EncodeCompletion(compID, openai.ModelID, text)
	if encErr != nil {
		openai.WriteError(w, &openai.Error{Status: http.StatusBadGateway, Type: "api_error", Code: "agent", Message: "request failed"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}

func (s *Server) openaiSession(ctx context.Context, key string) (string, error) {
	s.openai.mu.Lock()
	if slot := s.openai.slots[key]; slot != nil {
		id := slot.id
		s.openai.mu.Unlock()
		return id, nil
	}
	s.openai.mu.Unlock()
	var res struct {
		SessionID string `json:"sessionId"`
	}
	if err := s.acpCall(ctx, "session/new", map[string]any{
		"cwd":        s.workspace,
		"mcpServers": []any{},
	}, &res); err != nil || res.SessionID == "" {
		return "", err
	}
	logCap := s.cfg.EventLogCap
	if logCap <= 0 {
		logCap = defaultEventLogCap
	}
	subQ := s.cfg.SubQueueDepth
	if subQ <= 0 {
		subQ = defaultSubQueue
	}
	hub := newSessionHub(logCap, subQ)
	hub.logger = s.cfg.Logger
	s.mu.Lock()
	s.sessions[res.SessionID] = hub
	s.mu.Unlock()
	s.openai.mu.Lock()
	defer s.openai.mu.Unlock()
	if s.openai.slots == nil {
		s.openai.slots = map[string]*openaiSlot{}
	}
	if slot := s.openai.slots[key]; slot != nil {
		return slot.id, nil
	}
	s.openai.slots[key] = &openaiSlot{id: res.SessionID}
	return res.SessionID, nil
}

func (g *openaiGate) acquire(key string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.inflight == nil {
		g.inflight = map[string]int{}
	}
	if g.total >= openaiInflightProcess || g.inflight[key] >= openaiInflightPerCred {
		return false
	}
	g.inflight[key]++
	g.total++
	return true
}

func (g *openaiGate) release(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.inflight[key] > 0 {
		g.inflight[key]--
		g.total--
	}
}

func (g *openaiGate) takePrompt(key string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	slot := g.slots[key]
	if slot == nil || slot.prompts >= openaiPromptsPerSession {
		return false
	}
	slot.prompts++
	return true
}

func openaiCredKey(r *http.Request) string {
	if tok, ok := presentedBearer(r); ok && tok != "" {
		return "b:" + shortHash(tok)
	}
	c, err := r.Cookie("zb_session")
	if err == nil && c.Value != "" {
		return "c:" + shortHash(c.Value)
	}
	return "anon"
}

func shortHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8])
}

func newCompletionID() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "chatcmpl-local"
	}
	return "chatcmpl-" + hex.EncodeToString(b[:])
}

func agentChunkText(raw []byte) string {
	raw = bytes.TrimSpace(raw)
	raw = bytes.TrimPrefix(raw, []byte("data:"))
	raw = bytes.TrimSpace(raw)
	var ev struct {
		Params struct {
			Update struct {
				SessionUpdate string `json:"sessionUpdate"`
				Content       struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"update"`
		} `json:"params"`
	}
	if json.Unmarshal(raw, &ev) != nil {
		return ""
	}
	if ev.Params.Update.SessionUpdate != "agent_message_chunk" {
		return ""
	}
	if ev.Params.Update.Content.Type != "" && ev.Params.Update.Content.Type != "text" {
		return ""
	}
	return ev.Params.Update.Content.Text
}
