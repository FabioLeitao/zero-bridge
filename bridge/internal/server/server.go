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
	"strconv"
	"sync"
	"time"

	"zero-bridge/bridge/internal/acpclient"
	"zero-bridge/bridge/internal/auth"
	"zero-bridge/bridge/internal/permissions"
	"zero-bridge/bridge/internal/zeroenv"
	"zero-bridge/bridge/web"
)

// Config configures the HTTP bridge server.
type Config struct {
	Listen            string
	ZeroBin           string
	AgentCmd          string
	AgentArgs         []string
	Profile           string
	Workspace         string
	ExtraEnv          []string
	EventLogCap       int
	ReplayLogCap      int
	SubQueueDepth     int
	Keepalive         time.Duration
	PermissionTimeout time.Duration
	Logger            *log.Logger
	Audit             auth.Sink
	Token             string
	TokenFile         string // set only for --token-file; empty when the token came from the environment
	AllowedHosts      []string
	PublicOrigins     []string
	TrustProxy        bool
	Now               func() time.Time
	LoginFailureDelay time.Duration
	SessionTTL        time.Duration
	SessionIdle       time.Duration
	MaxSessions       int
	AuthFailWindow    time.Duration
	AuthFailLimit     int
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
	login          *loginState
	loginDelay     time.Duration
	audit          auth.Sink
	tokenMu        sync.RWMutex
	token          string
	prevToken      string
	reloadDone     chan struct{}
}

// New builds a Server; call Boot before serving.
func New(cfg Config) (*Server, error) {
	if cfg.ZeroBin == "" {
		cfg.ZeroBin = "zero"
	}
	switch cfg.Profile {
	case "":
		cfg.Profile = "zero"
	case "zero", "generic":
	default:
		return nil, fmt.Errorf("unknown profile %q", cfg.Profile)
	}
	if cfg.Keepalive <= 0 {
		cfg.Keepalive = 15 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = log.New(io.Discard, "", 0)
	}
	if cfg.Audit == nil {
		cfg.Audit = auth.NewLogSink(cfg.Logger)
	}
	if err := applyLimitDefaults(&cfg); err != nil {
		return nil, err
	}
	if cfg.Token != "" {
		if err := auth.Validate(cfg.Token); err != nil {
			return nil, err
		}
	}
	if err := validateExposure(cfg); err != nil {
		return nil, err
	}
	delay := cfg.LoginFailureDelay
	if delay == 0 {
		delay = 200 * time.Millisecond
	}
	if delay < 0 {
		delay = 0
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
		login:       newLoginState(),
		loginDelay:  delay,
		audit:       cfg.Audit,
		token:       cfg.Token,
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
		ZeroBin:   s.cfg.ZeroBin,
		AgentCmd:  s.cfg.AgentCmd,
		AgentArgs: s.cfg.AgentArgs,
		ExtraEnv:  extra,
		Timeout:   0,
		Notify:    s.onNotify,
		Inbound:   permInbound{s},
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
	mux.HandleFunc("GET /v1/auth/status", s.handleAuthStatus)
	mux.HandleFunc("POST /v1/login", s.handleLogin)
	mux.HandleFunc("POST /v1/logout", s.handleLogout)
	mux.HandleFunc("POST /v1/logout-all", s.handleLogoutAll)
	mux.HandleFunc("GET /v1/auth/sessions", s.handleListAuthSessions)
	mux.HandleFunc("DELETE /v1/auth/sessions/{handle}", s.handleRevokeAuthSession)
	mux.HandleFunc("GET /v1/info", s.handleInfo)
	mux.HandleFunc("GET /v1/capabilities", s.handleCapabilities)
	mux.HandleFunc("POST /v1/sessions", s.handleNewSession)
	mux.HandleFunc("GET /v1/sessions", s.handleListSessions)
	mux.HandleFunc("POST /v1/sessions/{id}/load", s.handleLoad)
	mux.HandleFunc("POST /v1/sessions/{id}/resume", s.handleResume)
	mux.HandleFunc("POST /v1/sessions/{id}/prompt", s.handlePrompt)
	mux.HandleFunc("POST /v1/sessions/{id}/mode", s.handleMode)
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
	s.waitReload(ctx)
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
	if cl, ok := s.audit.(auth.Closer); ok {
		if cerr := cl.Close(); err == nil {
			err = cerr
		}
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
		if !checkHostOrigin(r, s.allowed, s.cfg.AllowedHosts, s.cfg.PublicOrigins) {
			s.writeJSON(rec, http.StatusForbidden, map[string]any{"error": "forbidden"})
			s.cfg.Logger.Printf("%s %s %d %s", r.Method, r.URL.Path, http.StatusForbidden, time.Since(start))
			return
		}
		if !s.cookieCSRF(r) {
			s.writeJSON(rec, http.StatusForbidden, map[string]any{"error": "forbidden"})
			s.cfg.Logger.Printf("%s %s %d %s", r.Method, r.URL.Path, http.StatusForbidden, time.Since(start))
			return
		}
		if blocked, retry := s.bearerGate(r); blocked {
			rec.Header().Set("Retry-After", strconv.Itoa(retry))
			s.writeJSON(rec, http.StatusTooManyRequests, map[string]any{"error": "too many requests"})
			s.cfg.Logger.Printf("%s %s %d %s", r.Method, r.URL.Path, http.StatusTooManyRequests, time.Since(start))
			return
		}
		if !s.authorized(r) {
			rec.Header().Set("WWW-Authenticate", "Bearer")
			s.writeJSON(rec, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
			s.cfg.Logger.Printf("%s %s %d %s", r.Method, r.URL.Path, http.StatusUnauthorized, time.Since(start))
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

func (s *Server) redact(msg string) string {
	s.tokenMu.RLock()
	cur, prev := s.token, s.prevToken
	s.tokenMu.RUnlock()
	return RedactSecrets(msg, cur, prev)
}

func (s *Server) authorized(r *http.Request) bool {
	if s.currentToken() == "" || isPublic(r) {
		return true
	}
	if r.URL.Path == "/v1/auth/status" && r.Method == http.MethodGet {
		return true
	}
	if r.URL.Path == "/v1/login" && r.Method == http.MethodPost {
		return true
	}
	return s.hasBearer(r) || s.sessionOK(r)
}

func isPublic(r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}
	switch r.URL.Path {
	case "/health", "/", "/app.js", "/style.css":
		return true
	default:
		return false
	}
}

func presentedBearer(r *http.Request) (string, bool) {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if len(h) <= len(prefix) || h[:len(prefix)] != prefix {
		return "", false
	}
	return h[len(prefix):], true
}

func (s *Server) hasBearer(r *http.Request) bool {
	tok, ok := presentedBearer(r)
	if !ok {
		return false
	}
	return auth.Equal(s.currentToken(), tok)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if s.currentToken() != "" && !s.hasBearer(r) && !s.sessionOK(r) {
		s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	if s.agentUp() {
		s.writeJSON(w, http.StatusOK, map[string]any{"ok": true, "agent": "up"})
		return
	}
	s.writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "agent": "down"})
}

var errSessionNotInList = errors.New("session not in list")

func (s *Server) handleCapabilities(w http.ResponseWriter, r *http.Request) {
	c, err := s.acpClient()
	if err != nil {
		s.writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "agent down"})
		return
	}
	snap, ok := c.InitializeSnapshot()
	if !ok {
		s.writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "agent down"})
		return
	}
	modes := c.DeclaredModes()
	if modes == nil {
		modes = []acpclient.Mode{}
	}
	s.writeJSON(w, http.StatusOK, map[string]any{
		"profile":       s.cfg.Profile,
		"agent":         map[string]string{"name": snap.AgentInfo.Name, "version": snap.AgentInfo.Version},
		"loadSession":   snap.Capabilities.LoadSession,
		"sessionList":   snap.Capabilities.List(),
		"sessionResume": snap.Capabilities.Resume(),
		"prompt":        snap.Capabilities.PromptCapabilities,
		"modes":         modes,
	})
}

func (s *Server) agentCaps() (acpclient.AgentCapabilities, bool) {
	c, err := s.acpClient()
	if err != nil {
		return acpclient.AgentCapabilities{}, false
	}
	snap, ok := c.InitializeSnapshot()
	if !ok {
		return acpclient.AgentCapabilities{}, false
	}
	return snap.Capabilities, true
}

func (s *Server) notSupported(w http.ResponseWriter) {
	s.writeJSON(w, http.StatusNotImplemented, map[string]any{"error": "not supported by agent"})
}

func (s *Server) handleLoad(w http.ResponseWriter, r *http.Request) {
	s.handleAttach(w, r, true)
}

func (s *Server) handleResume(w http.ResponseWriter, r *http.Request) {
	s.handleAttach(w, r, false)
}

func (s *Server) handleAttach(w http.ResponseWriter, r *http.Request, replay bool) {
	caps, ok := s.agentCaps()
	if !ok || (replay && !caps.LoadSession) || (!replay && !caps.Resume()) {
		s.notSupported(w)
		return
	}
	id := r.PathValue("id")
	if id == "" {
		s.writeJSON(w, http.StatusNotFound, map[string]any{"error": "session not found"})
		return
	}
	s.mu.Lock()
	_, attached := s.sessions[id]
	s.mu.Unlock()
	if attached {
		if replay {
			s.writeJSON(w, http.StatusConflict, map[string]any{"error": "already attached"})
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"attached": true})
		return
	}
	workspace, hasBody, ok := s.readAttachWorkspace(w, r)
	if !ok {
		return
	}
	var cwd string
	if hasBody {
		cwd = workspace
	} else {
		found, err := s.findSessionCwd(r.Context(), id)
		if err != nil {
			if errors.Is(err, errSessionNotInList) {
				s.writeJSON(w, http.StatusNotFound, map[string]any{"error": "session not found"})
				return
			}
			s.writeJSON(w, http.StatusBadGateway, map[string]any{"error": s.redact(err.Error())})
			return
		}
		cwd = found
	}
	hub := s.newAttachedHub(replay)
	s.mu.Lock()
	if _, exists := s.sessions[id]; exists {
		s.mu.Unlock()
		if replay {
			s.writeJSON(w, http.StatusConflict, map[string]any{"error": "already attached"})
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"attached": true})
		return
	}
	s.sessions[id] = hub
	s.mu.Unlock()
	if replay {
		hub.setReplay(true)
		defer hub.setReplay(false)
	}
	method := "session/resume"
	if replay {
		method = "session/load"
	}
	var res modePayload
	err := s.acpCall(r.Context(), method, map[string]any{
		"sessionId":  id,
		"cwd":        cwd,
		"mcpServers": []any{},
	}, &res)
	if err != nil {
		s.mu.Lock()
		delete(s.sessions, id)
		s.mu.Unlock()
		if code, ok := acpclient.JSONRPCCode(err); ok && code == -32602 {
			s.writeJSON(w, http.StatusNotFound, map[string]any{"error": "session not found"})
			return
		}
		s.writeJSON(w, http.StatusBadGateway, map[string]any{"error": s.redact(err.Error())})
		return
	}
	s.writeJSON(w, http.StatusOK, modeResponse(id, res))
}

func (s *Server) readAttachWorkspace(w http.ResponseWriter, r *http.Request) (string, bool, bool) {
	if r.ContentLength == 0 {
		return "", false, true
	}
	if !requireJSONContentType(r) {
		s.writeJSON(w, http.StatusUnsupportedMediaType, map[string]any{"error": "content-type must be application/json"})
		return "", false, false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	var body struct {
		Workspace string `json:"workspace"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return "", false, false
	}
	if body.Workspace == "" {
		return "", false, true
	}
	if !filepath.IsAbs(body.Workspace) {
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "workspace must be an absolute path"})
		return "", false, false
	}
	return filepath.Clean(body.Workspace), true, true
}

func (s *Server) findSessionCwd(ctx context.Context, id string) (string, error) {
	cursor := ""
	for page := 0; page < 20; page++ {
		params := map[string]any{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		var res struct {
			Sessions []struct {
				SessionID string `json:"sessionId"`
				Cwd       string `json:"cwd"`
			} `json:"sessions"`
			NextCursor string `json:"nextCursor"`
		}
		if err := s.acpCall(ctx, "session/list", params, &res); err != nil {
			return "", err
		}
		for _, sess := range res.Sessions {
			if sess.SessionID == id && sess.Cwd != "" {
				return sess.Cwd, nil
			}
		}
		if res.NextCursor == "" || res.NextCursor == cursor {
			return "", errSessionNotInList
		}
		cursor = res.NextCursor
	}
	return "", errSessionNotInList
}

func (s *Server) newAttachedHub(replay bool) *sessionHub {
	logCap := s.cfg.EventLogCap
	if replay {
		logCap = s.cfg.ReplayLogCap
		if logCap <= 0 {
			logCap = defaultReplayLogCap
		}
	} else if logCap <= 0 {
		logCap = defaultEventLogCap
	}
	subQ := s.cfg.SubQueueDepth
	if subQ <= 0 {
		subQ = defaultSubQueue
	}
	hub := newSessionHub(logCap, subQ)
	hub.logger = s.cfg.Logger
	return hub
}

type modePayload struct {
	Modes *struct {
		CurrentModeID  string `json:"currentModeId"`
		AvailableModes []struct {
			ID string `json:"id"`
		} `json:"availableModes"`
	} `json:"modes"`
}

func modeResponse(id string, res modePayload) map[string]any {
	out := map[string]any{"sessionId": id}
	if res.Modes == nil {
		return out
	}
	ids := make([]string, 0, len(res.Modes.AvailableModes))
	for _, m := range res.Modes.AvailableModes {
		if m.ID != "" {
			ids = append(ids, m.ID)
		}
	}
	out["mode"] = res.Modes.CurrentModeID
	out["modes"] = ids
	return out
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	caps, ok := s.agentCaps()
	if !ok || !caps.List() {
		s.notSupported(w)
		return
	}
	cwd := r.URL.Query().Get("cwd")
	if cwd != "" && !filepath.IsAbs(cwd) {
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "cwd must be an absolute path"})
		return
	}
	params := map[string]any{}
	if cwd != "" {
		params["cwd"] = cwd
	}
	if cursor := r.URL.Query().Get("cursor"); cursor != "" {
		params["cursor"] = cursor
	}
	var res struct {
		Sessions   []json.RawMessage `json:"sessions"`
		NextCursor string            `json:"nextCursor"`
	}
	if err := s.acpCall(r.Context(), "session/list", params, &res); err != nil {
		s.writeJSON(w, http.StatusBadGateway, map[string]any{"error": s.redact(err.Error())})
		return
	}
	if res.Sessions == nil {
		res.Sessions = []json.RawMessage{}
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"sessions": res.Sessions, "nextCursor": res.NextCursor})
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
		Modes     *struct {
			CurrentModeID  string `json:"currentModeId"`
			AvailableModes []struct {
				ID string `json:"id"`
			} `json:"availableModes"`
		} `json:"modes"`
	}
	if err := s.acpCall(r.Context(), "session/new", map[string]any{
		"cwd":        body.Workspace,
		"mcpServers": []any{},
	}, &res); err != nil {
		s.writeJSON(w, http.StatusBadGateway, map[string]any{"error": s.redact(err.Error())})
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
	out := map[string]any{"sessionId": res.SessionID}
	if res.Modes != nil {
		ids := make([]string, 0, len(res.Modes.AvailableModes))
		for _, m := range res.Modes.AvailableModes {
			if m.ID != "" {
				ids = append(ids, m.ID)
			}
		}
		out["mode"] = res.Modes.CurrentModeID
		out["modes"] = ids
	}
	s.writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleMode(w http.ResponseWriter, r *http.Request) {
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
	if hub.promptInProgress() {
		s.writeJSON(w, http.StatusConflict, map[string]any{"error": "prompt already in progress"})
		return
	}
	c, err := s.acpClient()
	if err != nil {
		s.notSupported(w)
		return
	}
	offer, hasOffer := c.SessionOffer(id)
	if !hasOffer || len(offer.Modes) == 0 {
		s.notSupported(w)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	var body struct {
		Mode string `json:"mode"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	switch body.Mode {
	case "auto", "ask", "plan":
	default:
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "mode must be auto, ask, or plan"})
		return
	}
	var ignored map[string]any
	if err := s.acpCall(r.Context(), "session/set_mode", map[string]any{
		"sessionId": id,
		"modeId":    body.Mode,
	}, &ignored); err != nil {
		s.writeJSON(w, http.StatusBadGateway, map[string]any{"error": s.redact(err.Error())})
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"ok": true, "mode": body.Mode})
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
		hub.emit(map[string]any{"type": "error", "message": s.redact(err.Error())})
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
	w.Header().Set("X-Accel-Buffering", "no")
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
