package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"zero-bridge/bridge/internal/acpclient"
	"zero-bridge/bridge/internal/permissions"
	"zero-bridge/bridge/internal/zeroenv"
	"zero-bridge/bridge/web"
)

// Config configures the HTTP bridge server.
type Config struct {
	Listen            string
	ZeroBin           string
	Workspace         string
	ExtraEnv          []string
	EventLogCap       int
	SubQueueDepth     int
	Keepalive         time.Duration
	PermissionTimeout time.Duration
	Logger            *log.Logger
}

// Server bridges HTTP to a single long-lived zero acp child.
type Server struct {
	cfg      Config
	listen   string
	allowed  []string
	acp      *acpclient.Client
	acpMu    sync.RWMutex
	sessions map[string]*sessionHub
	mu       sync.RWMutex
	httpSrv  *http.Server

	serveCtx    context.Context
	serveCancel context.CancelFunc

	workspace      string
	workspaceOwned bool
	perms          *permissions.Broker
}

// New builds a Server; call Boot before serving.
func New(cfg Config) (*Server, error) {
	if cfg.ZeroBin == "" {
		cfg.ZeroBin = "zero"
	}
	if cfg.Keepalive <= 0 {
		cfg.Keepalive = 15 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = log.New(io.Discard, "", 0)
	}
	listen, err := ResolveListen(cfg.Listen)
	if err != nil {
		return nil, err
	}
	allowed, err := allowedHosts(listen)
	if err != nil {
		return nil, err
	}
	serveCtx, serveCancel := context.WithCancel(context.Background())
	s := &Server{
		cfg:         cfg,
		listen:      listen,
		allowed:     allowed,
		sessions:    make(map[string]*sessionHub),
		serveCtx:    serveCtx,
		serveCancel: serveCancel,
		perms:       permissions.New(cfg.PermissionTimeout),
	}
	s.perms.OnEvent = s.onPermission
	return s, nil
}

// Boot starts the zero acp child and performs initialize.
func (s *Server) Boot(ctx context.Context) error {
	extra := s.cfg.ExtraEnv
	if extra == nil {
		extra = zeroenv.ProviderEnv()
	}
	client, err := acpclient.Start(ctx, acpclient.Options{
		ZeroBin:  s.cfg.ZeroBin,
		ExtraEnv: extra,
		Timeout:  0,
		Notify:   s.onNotify,
		Inbound:  permInbound{s},
	})
	if err != nil {
		return err
	}
	s.acpMu.Lock()
	s.acp = client
	s.acpMu.Unlock()
	initParams := map[string]any{
		"protocolVersion": 1,
		"clientCapabilities": map[string]any{
			"fs": map[string]bool{
				"readTextFile":  false,
				"writeTextFile": false,
			},
			"terminal": false,
		},
		"clientInfo": map[string]string{"name": "zero-bridge", "version": "0"},
	}
	var initRes map[string]any
	if err := s.acpCall(ctx, "initialize", initParams, &initRes); err != nil {
		_ = client.Close()
		return fmt.Errorf("initialize: %w", err)
	}
	go func() {
		<-client.Done()
		s.perms.Stop()
	}()
	return nil
}

func (s *Server) acpClient() (*acpclient.Client, error) {
	s.acpMu.RLock()
	c := s.acp
	s.acpMu.RUnlock()
	if c == nil {
		return nil, errors.New("agent not started")
	}
	return c, nil
}

func (s *Server) acpCall(ctx context.Context, method string, params any, result any) error {
	c, err := s.acpClient()
	if err != nil {
		return err
	}
	return c.Call(ctx, method, params, result)
}

func (s *Server) agentUp() bool {
	if s.acp == nil {
		return false
	}
	select {
	case <-s.acp.Done():
		return false
	default:
		return true
	}
}

func (s *Server) onNotify(method string, params json.RawMessage) {
	if method != "session/update" {
		return
	}
	var envelope struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(params, &envelope); err != nil || envelope.SessionID == "" {
		return
	}
	hub := s.sessionHub(envelope.SessionID)
	if hub == nil {
		return
	}
	var paramsObj any
	_ = json.Unmarshal(params, &paramsObj)
	hub.emit(map[string]any{
		"type":   "session/update",
		"params": paramsObj,
	})
}

func (s *Server) sessionHub(id string) *sessionHub {
	s.mu.RLock()
	h := s.sessions[id]
	s.mu.RUnlock()
	return h
}

// Handler returns the HTTP handler (after Boot).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("/{$}", s.methodNotAllowed)
	mux.HandleFunc("/app.js", s.methodNotAllowed)
	mux.HandleFunc("/style.css", s.methodNotAllowed)
	mux.HandleFunc("GET /app.js", s.handleFile("app.js", "text/javascript; charset=utf-8"))
	mux.HandleFunc("GET /style.css", s.handleFile("style.css", "text/css; charset=utf-8"))
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /v1/info", s.handleInfo)
	mux.HandleFunc("POST /v1/sessions", s.handleNewSession)
	mux.HandleFunc("POST /v1/sessions/{id}/prompt", s.handlePrompt)
	mux.HandleFunc("GET /v1/sessions/{id}/events", s.handleEvents)
	mux.HandleFunc("GET /v1/sessions/{id}/permissions/pending", s.handlePending)
	mux.HandleFunc("POST /v1/sessions/{id}/permissions/{reqId}", s.handlePermission)
	mux.HandleFunc("/ws", func(w http.ResponseWriter, _ *http.Request) {
		s.writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/", "/app.js", "/style.css":
			if r.Method != http.MethodGet {
				s.methodNotAllowed(w, r)
				return
			}
		}
		s.writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
	})
	return s.withMiddleware(mux)
}

// ListenAndServe blocks until the server stops.
func (s *Server) ListenAndServe() error {
	s.httpSrv = s.newHTTPServer()
	return s.httpSrv.ListenAndServe()
}

func (s *Server) newHTTPServer() *http.Server {
	return &http.Server{
		Addr:              s.listen,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		MaxHeaderBytes:    16 << 10,
		BaseContext: func(net.Listener) context.Context {
			return s.serveCtx
		},
	}
}

// Shutdown stops HTTP and closes the acp child.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.serveCancel != nil {
		s.serveCancel()
	}
	var err error
	if s.httpSrv != nil {
		err = s.httpSrv.Shutdown(ctx)
	}
	s.acpMu.Lock()
	c := s.acp
	s.acp = nil
	s.acpMu.Unlock()
	if s.perms != nil {
		s.perms.Stop()
	}
	if c != nil {
		if cerr := c.Close(); err == nil {
			err = cerr
		}
	}
	if s.workspaceOwned && s.workspace != "" {
		_ = os.RemoveAll(s.workspace)
	}
	return err
}

// PrepareWorkspace creates a temp directory or checks an explicit one.
func (s *Server) PrepareWorkspace() error {
	if s.cfg.Workspace == "" {
		dir, err := os.MkdirTemp("", "zbridge-workspace-*")
		if err != nil {
			return err
		}
		s.workspace = dir
		s.workspaceOwned = true
		return nil
	}
	if err := validateWorkspace(s.cfg.Workspace); err != nil {
		return err
	}
	s.workspace = filepath.Clean(s.cfg.Workspace)
	s.workspaceOwned = false
	return nil
}

// DefaultWorkspace returns the absolute workspace used when the client omits one.
func (s *Server) DefaultWorkspace() string { return s.workspace }

func (s *Server) withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		writeSecurityHeaders(rec)
		if !checkHostOrigin(r, s.allowed) {
			s.writeJSON(rec, http.StatusForbidden, map[string]any{"error": "forbidden"})
			s.cfg.Logger.Printf("%s %s %d %s", r.Method, r.URL.Path, http.StatusForbidden, time.Since(start))
			return
		}
		next.ServeHTTP(rec, r)
		if rec.status == 0 {
			rec.status = http.StatusOK
		}
		s.cfg.Logger.Printf("%s %s %d %s", r.Method, r.URL.Path, rec.status, time.Since(start))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *Server) methodNotAllowed(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Allow", "GET")
	s.writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	s.handleFile("index.html", "text/html; charset=utf-8")(w, r)
}

func (s *Server) handleFile(name, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		b, err := web.Files.ReadFile(name)
		if err != nil {
			s.writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
			return
		}
		writeSecurityHeaders(w)
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(b)
	}
}

func (s *Server) handleInfo(w http.ResponseWriter, _ *http.Request) {
	agent := "down"
	if s.agentUp() {
		agent = "up"
	}
	s.writeJSON(w, http.StatusOK, map[string]any{
		"agent":            agent,
		"defaultWorkspace": s.workspace,
		"version":          "0",
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	if s.agentUp() {
		s.writeJSON(w, http.StatusOK, map[string]any{"ok": true, "agent": "up"})
		return
	}
	s.writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "agent": "down"})
}

func (s *Server) handleNewSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	if !requireJSONContentType(r) {
		s.writeJSON(w, http.StatusUnsupportedMediaType, map[string]any{"error": "content-type must be application/json"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var body struct {
		Workspace string `json:"workspace"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	if err := validateWorkspace(body.Workspace); err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	var res struct {
		SessionID string `json:"sessionId"`
	}
	if err := s.acpCall(r.Context(), "session/new", map[string]any{
		"cwd":        body.Workspace,
		"mcpServers": []any{},
	}, &res); err != nil {
		s.writeJSON(w, http.StatusBadGateway, map[string]any{"error": Redact(err.Error())})
		return
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
	s.writeJSON(w, http.StatusOK, map[string]any{"sessionId": res.SessionID})
}

func validateWorkspace(p string) error {
	if p == "" || !filepath.IsAbs(p) {
		return fmt.Errorf("workspace must be an absolute path")
	}
	clean := filepath.Clean(p)
	st, err := os.Stat(clean)
	if err != nil {
		return fmt.Errorf("workspace not found")
	}
	if !st.IsDir() {
		return fmt.Errorf("workspace must be a directory")
	}
	return nil
}

func (s *Server) handlePrompt(w http.ResponseWriter, r *http.Request) {
	if !requireJSONContentType(r) {
		s.writeJSON(w, http.StatusUnsupportedMediaType, map[string]any{"error": "content-type must be application/json"})
		return
	}
	id := r.PathValue("id")
	hub := s.sessionHub(id)
	if hub == nil {
		s.writeJSON(w, http.StatusNotFound, map[string]any{"error": "session not found"})
		return
	}
	if !hub.tryStartPrompt() {
		s.writeJSON(w, http.StatusConflict, map[string]any{"error": "prompt already in progress"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		hub.endPrompt()
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	s.writeJSON(w, http.StatusAccepted, map[string]any{"ok": true})
	go s.runPrompt(id, hub, body.Text)
}

func (s *Server) runPrompt(sessionID string, hub *sessionHub, text string) {
	defer func() {
		hub.endPrompt()
		if s.perms != nil {
			s.perms.AbortSession(sessionID)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	var res struct {
		StopReason string `json:"stopReason"`
	}
	err := s.acpCall(ctx, "session/prompt", map[string]any{
		"sessionId": sessionID,
		"prompt": []map[string]string{
			{"type": "text", "text": text},
		},
	}, &res)
	if err != nil {
		hub.emit(map[string]any{"type": "error", "message": Redact(err.Error())})
		return
	}
	hub.emit(map[string]any{"type": "stop", "stopReason": res.StopReason})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	hub := s.sessionHub(id)
	if hub == nil {
		s.writeJSON(w, http.StatusNotFound, map[string]any{"error": "session not found"})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	writeSecurityHeaders(w)
	rc := http.NewResponseController(w)
	subID, ch, replay := hub.subscribe()
	defer hub.unsubscribe(subID)
	for _, frame := range replay {
		_, _ = w.Write(frame)
		_ = rc.Flush()
	}
	ticker := time.NewTicker(s.cfg.Keepalive)
	defer ticker.Stop()
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case frame, ok := <-ch:
			if !ok {
				return
			}
			_, _ = w.Write(frame)
			_ = rc.Flush()
		case <-ticker.C:
			_, _ = w.Write([]byte(": keepalive\n\n"))
			_ = rc.Flush()
		}
	}
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, v any) {
	writeSecurityHeaders(w)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
