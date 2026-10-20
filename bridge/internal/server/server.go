package server

import (
	"bytes"
	"context"
	"encoding/base64"
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
	"strings"
	"sync"
	"time"

	"zero-bridge/bridge/a2a"
	"zero-bridge/bridge/internal/acpclient"
	"zero-bridge/bridge/internal/auth"
	"zero-bridge/bridge/internal/observability"
	"zero-bridge/bridge/internal/permissions"
	"zero-bridge/bridge/internal/sessionmeta"
	"zero-bridge/bridge/internal/sessionstate"
	"zero-bridge/bridge/internal/workroot"
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
	// InitTimeout bounds the initialize call. Zero uses defaultInitTimeout.
	InitTimeout       time.Duration
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
	// Meta is the local session label store. Nil makes the meta route answer 503.
	Meta *sessionmeta.Store

	// Image attachment limits (S25).
	MaxImageBytes  int64    // 0 = default (4 MiB)
	MaxImages      int      // 0 = default (4)
	AllowImageMIME []string // empty = default (PNG, JPEG, WebP, GIF)

	// Automatic mode routing (S27).
	AutoModeEnabled bool   // off by default
	AutoModeCeiling string // empty = reduce-only (never raise)
	AutoModeApply   bool   // if true, apply without click (requires ceiling)

	// A2A ingress (S7).
	A2AMaxBodyBytes int64 // 0 = default (1 MiB)
}

// Server bridges HTTP to a single long-lived zero acp child.
type Server struct {
	cfg        Config
	listen     string
	network    string
	sockPath   string
	ownsSocket bool
	allowed    []string
	acp        *acpclient.Client
	acpMu      sync.RWMutex
	sessions   map[string]*sessionHub
	mu         sync.RWMutex
	states     map[string]sessionstate.Machine
	stateMu    sync.Mutex
	agentDown  bool
	httpSrv    *http.Server

	serveCtx    context.Context
	serveCancel context.CancelFunc

	workspace      string
	workspaceOwned bool
	roots          *workroot.Set
	perms          *permissions.Broker
	login          *loginState
	loginDelay     time.Duration
	audit          auth.Sink
	meta           *sessionmeta.Store
	tokenMu        sync.RWMutex
	token          string
	prevToken      string
	reloadDone     chan struct{}
	openai         openaiGate
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
	if cfg.InitTimeout <= 0 {
		cfg.InitTimeout = defaultInitTimeout
	}
	if cfg.Logger == nil {
		cfg.Logger = log.New(io.Discard, "", 0)
	}
	if cfg.Audit == nil {
		cfg.Audit = auth.NewLogSink(cfg.Logger)
	}
	// Image attachment defaults (S25).
	const (
		defaultMaxImageBytes = 4 << 20 // 4 MiB
		defaultMaxImages     = 4
	)
	if cfg.MaxImageBytes <= 0 {
		cfg.MaxImageBytes = defaultMaxImageBytes
	}
	if cfg.MaxImageBytes > 16<<20 { // hard cap 16 MiB
		return nil, fmt.Errorf("max-image-bytes exceeds hard cap 16MiB")
	}
	if cfg.MaxImages <= 0 {
		cfg.MaxImages = defaultMaxImages
	}
	if cfg.MaxImages > 16 { // hard cap
		return nil, fmt.Errorf("max-images exceeds hard cap 16")
	}
	if len(cfg.AllowImageMIME) == 0 {
		cfg.AllowImageMIME = []string{"image/png", "image/jpeg", "image/webp", "image/gif"}
	}
	for _, m := range cfg.AllowImageMIME {
		switch m {
		case "image/png", "image/jpeg", "image/webp", "image/gif":
		default:
			return nil, fmt.Errorf("unsupported image MIME %q", m)
		}
	}
	// A2A ingress defaults (S7).
	const (
		defaultA2AMaxBodyBytes = 1 << 20 // 1 MiB
	)
	if cfg.A2AMaxBodyBytes <= 0 {
		cfg.A2AMaxBodyBytes = defaultA2AMaxBodyBytes
	}
	if cfg.A2AMaxBodyBytes > 16<<20 { // hard cap 16 MiB
		return nil, fmt.Errorf("a2a-max-body-bytes exceeds hard cap 16MiB")
	}
	// Automatic mode routing defaults (S27).
	if cfg.AutoModeEnabled {
		if cfg.AutoModeApply && cfg.AutoModeCeiling == "" {
			return nil, fmt.Errorf("auto-mode-apply requires auto-mode-ceiling")
		}
		// Validate ceiling if provided.
		if cfg.AutoModeCeiling != "" {
			switch cfg.AutoModeCeiling {
			case "plan", "ask", "auto":
			default:
				return nil, fmt.Errorf("auto-mode-ceiling must be plan, ask, or auto")
			}
		}
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
	ep, err := ResolveListen(cfg.Listen)
	if err != nil {
		return nil, err
	}
	var allowed []string
	if ep.Network == "tcp" {
		allowed, err = allowedHosts(ep.Addr)
		if err != nil {
			return nil, err
		}
	}
	serveCtx, serveCancel := context.WithCancel(context.Background())
	s := &Server{
		cfg:         cfg,
		listen:      ep.Addr,
		network:     ep.Network,
		allowed:     allowed,
		sessions:    make(map[string]*sessionHub),
		serveCtx:    serveCtx,
		serveCancel: serveCancel,
		perms:       permissions.New(cfg.PermissionTimeout),
		login:       newLoginState(),
		loginDelay:  delay,
		audit:       cfg.Audit,
		meta:        cfg.Meta,
		token:       cfg.Token,
	}
	if ep.Network == "unix" {
		s.sockPath = ep.Addr
		s.listen = ""
	}
	s.perms.OnEvent = s.onPermission
	return s, nil
}

// childExtra is the environment added for the agent. The zero profile may
// set ZERO_PROVIDER. The generic profile adds nothing unless the caller
// passed ExtraEnv. A non-nil ExtraEnv, including an empty slice, is used
// as given so tests can disable the probe.
func (s *Server) childExtra() []string {
	if s.cfg.ExtraEnv != nil {
		return s.cfg.ExtraEnv
	}
	if s.cfg.Profile == "zero" {
		return zeroenv.ProviderEnv()
	}
	return []string{}
}

// acpProtocolVersion is the only initialize protocolVersion this process accepts.
const acpProtocolVersion = 1

// defaultInitTimeout is how long Boot waits for initialize when the
// caller did not set InitTimeout. A silent peer fails inside this bound.
const defaultInitTimeout = 30 * time.Second

// Boot starts the agent child and performs initialize.
// A successful initialize whose protocolVersion is not acpProtocolVersion, or
// whose field is absent or not a whole number, is a mismatch. The error names
// the sent and received versions and nothing else.
func (s *Server) Boot(ctx context.Context) error {
	extra := s.childExtra()
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
		"protocolVersion": acpProtocolVersion,
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
	initCtx, initCancel := context.WithTimeout(ctx, s.cfg.InitTimeout)
	err = s.acpCall(initCtx, "initialize", initParams, &initRes)
	initCancel()
	if err != nil {
		_ = client.Close()
		if errors.Is(err, context.DeadlineExceeded) {
			return errors.New("initialize timed out")
		}
		return fmt.Errorf("initialize: %w", err)
	}
	if err := protocolVersionError(initRes, acpProtocolVersion); err != nil {
		_ = client.Close()
		return err
	}
	go func() {
		<-client.Done()
		s.perms.Stop()
		s.markAgentDown()
	}()
	return nil
}

// protocolVersionError rejects an initialize result that is not sent.
// A missing field, null, or a value that is not a whole number is "none".
// A whole number other than sent is printed as that integer. The text never
// copies a string from the peer.
func protocolVersionError(res map[string]any, sent int) error {
	raw, ok := res["protocolVersion"]
	if !ok || raw == nil {
		return fmt.Errorf("protocol mismatch: sent %d, received none", sent)
	}
	got, ok := protocolNumber(raw)
	if !ok {
		return fmt.Errorf("protocol mismatch: sent %d, received none", sent)
	}
	if got != sent {
		return fmt.Errorf("protocol mismatch: sent %d, received %d", sent, got)
	}
	return nil
}

func protocolNumber(v any) (int, bool) {
	n, ok := v.(float64)
	if !ok || n != float64(int(n)) || n < -1<<20 || n > 1<<20 {
		return 0, false
	}
	return int(n), true
}

// cancelPrompt tells the agent to stop the running turn. The prompt call
// stays open until the agent answers it. OpenAI disconnect only cancels the
// HTTP request context; it does not send this notification.
func (s *Server) cancelPrompt(sessionID string) error {
	c, err := s.acpClient()
	if err != nil {
		return err
	}
	return c.Notify("session/cancel", map[string]string{"sessionId": sessionID})
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
	// A2A ingress (S7). With a token set, both routes use the same credential check as /v1.
	mux.HandleFunc("GET /.well-known/agent-card.json", s.handleAgentCard)
	mux.HandleFunc("POST /a2a/message/send", s.handleA2AMessageSend)
	// OpenAI ingress (S29). Not public. Same credential as /v1 when a token is set.
	mux.HandleFunc("GET /v1/models", s.handleModels)
	mux.HandleFunc("POST /v1/chat/completions", s.handleChatCompletions)
	mux.HandleFunc("POST /v1/sessions", s.handleNewSession)
	mux.HandleFunc("GET /v1/sessions", s.handleListSessions)
	mux.HandleFunc("POST /v1/sessions/{id}/meta", s.handleSessionMeta)
	mux.HandleFunc("POST /v1/sessions/{id}/load", s.handleLoad)
	mux.HandleFunc("POST /v1/sessions/{id}/resume", s.handleResume)
	mux.HandleFunc("POST /v1/sessions/{id}/prompt", s.handlePrompt)
	mux.HandleFunc("POST /v1/sessions/{id}/cancel", s.handleCancel)
	mux.HandleFunc("POST /v1/sessions/{id}/mode", s.handleMode)
	mux.HandleFunc("POST /v1/sessions/{id}/mode/recommend", s.handleModeRecommend)
	mux.HandleFunc("POST /v1/sessions/{id}/config_option", s.handleConfigOption)
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
// A unix listener does not open a TCP port. Shutdown removes the socket.
func (s *Server) ListenAndServe() error {
	httpSrv := s.newHTTPServer()
	httpSrv.Addr = ""
	s.mu.Lock()
	s.httpSrv = httpSrv
	s.mu.Unlock()
	ln, err := s.openListener()
	if err != nil {
		return err
	}
	return httpSrv.Serve(ln)
}

func (s *Server) openListener() (net.Listener, error) {
	if s.network == "unix" {
		ln, err := listenUnix(s.sockPath)
		if err != nil {
			return nil, err
		}
		s.mu.Lock()
		s.ownsSocket = true
		s.mu.Unlock()
		return ln, nil
	}
	return net.Listen("tcp", s.listen)
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
	s.mu.Lock()
	httpSrv := s.httpSrv
	ownsSocket := s.ownsSocket
	sockPath := s.sockPath
	s.mu.Unlock()
	var err error
	if httpSrv != nil {
		err = httpSrv.Shutdown(ctx)
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
	if ownsSocket {
		_ = removeUnixSocket(sockPath)
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
		if !s.requestAllowed(r) {
			s.writeRouteError(rec, r, http.StatusForbidden, "forbidden")
			s.cfg.Logger.Printf("%s %s %d %s", r.Method, r.URL.Path, http.StatusForbidden, time.Since(start))
			return
		}
		if !s.cookieCSRF(r) {
			s.writeRouteError(rec, r, http.StatusForbidden, "forbidden")
			s.cfg.Logger.Printf("%s %s %d %s", r.Method, r.URL.Path, http.StatusForbidden, time.Since(start))
			return
		}
		if blocked, retry := s.bearerGate(r); blocked {
			rec.Header().Set("Retry-After", strconv.Itoa(retry))
			s.writeRouteError(rec, r, http.StatusTooManyRequests, "too many requests")
			s.cfg.Logger.Printf("%s %s %d %s", r.Method, r.URL.Path, http.StatusTooManyRequests, time.Since(start))
			return
		}
		if !s.authorized(r) {
			rec.Header().Set("WWW-Authenticate", "Bearer")
			s.writeRouteError(rec, r, http.StatusUnauthorized, "unauthorized")
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
	kind := "tcp"
	if s.network == "unix" {
		kind = "unix"
	}
	s.writeJSON(w, http.StatusOK, map[string]any{
		"agent":            agent,
		"defaultWorkspace": s.workspace,
		"workspaceRoots":   s.workspaceRoots(),
		"version":          "0",
		"listen":           kind,
	})
}

// SetWorkspaceRoots installs the allowlist. An empty list is refused.
// The session routes do not consult it yet.
func (s *Server) SetWorkspaceRoots(paths []string) error {
	set, err := workroot.New(paths)
	if err != nil {
		return err
	}
	s.roots = set
	return nil
}

func (s *Server) workspaceRoots() []string {
	if s.roots != nil {
		return s.roots.Paths()
	}
	if s.workspace != "" {
		return []string{s.workspace}
	}
	return []string{}
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
	return s.credentialOK(r)
}

// credentialOK is Bearer or a session cookie. On a unix socket a request
// without Origin is Bearer only: curl and the Electron main process do not
// send Origin, and a cookie without Origin is not accepted. Cookie CSRF for
// POST and DELETE is unchanged.
func (s *Server) credentialOK(r *http.Request) bool {
	if s.network == "unix" && r.Header.Get("Origin") == "" {
		return s.hasBearer(r)
	}
	return s.hasBearer(r) || s.sessionOK(r)
}

func (s *Server) requestAllowed(r *http.Request) bool {
	if s.network != "unix" {
		return checkHostOrigin(r, s.allowed, s.cfg.AllowedHosts, s.cfg.PublicOrigins)
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	return originAllowed(origin, nil, s.cfg.PublicOrigins)
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
	// Include configOptions from the first session offer (they're per-session but usually same)
	var configOptions []acpclient.ConfigOption
	offer, ok := c.SessionOffer("")
	if ok {
		configOptions = offer.ConfigOptions
	}
	s.writeJSON(w, http.StatusOK, map[string]any{
		"profile":       s.cfg.Profile,
		"agent":         map[string]string{"name": snap.AgentInfo.Name, "version": snap.AgentInfo.Version},
		"loadSession":   snap.Capabilities.LoadSession,
		"sessionList":   snap.Capabilities.List(),
		"sessionResume": snap.Capabilities.Resume(),
		"prompt":        snap.Capabilities.PromptCapabilities,
		"modes":         modes,
		"sessionStatus": s.stateLevel(),
		"configOptions": configOptions,
		"workspaceRoots": func() []string {
			if s.roots != nil {
				return s.roots.Paths()
			}
			return []string{}
		}(),
	})
}

// handleAgentCard handles GET /.well-known/agent-card.json
func (s *Server) handleAgentCard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
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
	card := a2a.DefaultAgentCard(snap.AgentInfo.Name, snap.AgentInfo.Version)
	s.writeJSON(w, http.StatusOK, card)
}

// handleA2AMessageSend handles POST /a2a/message/send (JSON-RPC 2.0)
func (s *Server) handleA2AMessageSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.A2AMaxBodyBytes)

	var req struct {
		JSONRPC string          `json:"jsonrpc"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params,omitempty"`
		ID      json.RawMessage `json:"id,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]any{
			"jsonrpc": "2.0",
			"error":   map[string]any{"code": -32700, "message": "parse error"},
			"id":      nil,
		})
		return
	}

	if req.JSONRPC != "2.0" {
		s.writeJSON(w, http.StatusBadRequest, map[string]any{
			"jsonrpc": "2.0",
			"error":   map[string]any{"code": -32600, "message": "invalid request"},
			"id":      req.ID,
		})
		return
	}

	if req.Method != "message/send" {
		s.writeJSON(w, http.StatusNotFound, map[string]any{
			"jsonrpc": "2.0",
			"error":   map[string]any{"code": -32601, "message": "method not found"},
			"id":      req.ID,
		})
		return
	}

	var params struct {
		Message struct {
			Role  string `json:"role"`
			Parts []struct {
				Type string `json:"type"`
				Text string `json:"text,omitempty"`
			} `json:"parts"`
		} `json:"message"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]any{
			"jsonrpc": "2.0",
			"error":   map[string]any{"code": -32602, "message": "invalid params"},
			"id":      req.ID,
		})
		return
	}

	if params.Message.Role != "user" {
		s.writeJSON(w, http.StatusBadRequest, map[string]any{
			"jsonrpc": "2.0",
			"error":   map[string]any{"code": -32602, "message": "only user role supported"},
			"id":      req.ID,
		})
		return
	}

	if len(params.Message.Parts) == 0 {
		s.writeJSON(w, http.StatusBadRequest, map[string]any{
			"jsonrpc": "2.0",
			"error":   map[string]any{"code": -32602, "message": "at least one part required"},
			"id":      req.ID,
		})
		return
	}

	// Extract text from parts (only text parts supported for S7)
	var textParts []string
	for _, part := range params.Message.Parts {
		if part.Type != "text" {
			s.writeJSON(w, http.StatusBadRequest, map[string]any{
				"jsonrpc": "2.0",
				"error":   map[string]any{"code": -32602, "message": "only text parts supported"},
				"id":      req.ID,
			})
			return
		}
		textParts = append(textParts, part.Text)
	}
	text := strings.Join(textParts, "\n")

	// Get or create a session for this A2A interaction
	c, err := s.acpClient()
	if err != nil {
		s.writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"jsonrpc": "2.0",
			"error":   map[string]any{"code": -32603, "message": "agent down"},
			"id":      req.ID,
		})
		return
	}

	sessionID, err := s.getOrCreateA2ASession(r.Context(), c)
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]any{
			"jsonrpc": "2.0",
			"error":   map[string]any{"code": -32603, "message": "session error: " + err.Error()},
			"id":      req.ID,
		})
		return
	}

	// Send prompt via ACP
	start := time.Now()
	var res struct {
		StopReason string `json:"stopReason"`
	}
	err = s.acpCall(r.Context(), "session/prompt", map[string]any{
		"sessionId": sessionID,
		"prompt": []map[string]any{
			{"type": "text", "text": text},
		},
	}, &res)
	observability.RecordACPMetric("session/prompt", time.Since(start).Seconds()*1000, err)

	if err != nil {
		s.writeJSON(w, http.StatusBadGateway, map[string]any{
			"jsonrpc": "2.0",
			"error":   map[string]any{"code": -32603, "message": "ACP error: " + err.Error()},
			"id":      req.ID,
		})
		return
	}

	// Generate a task ID
	taskID := fmt.Sprintf("task-%d", time.Now().UnixNano())

	// Return a completed task
	task := map[string]any{
		"id":        taskID,
		"status":    map[string]any{"state": "completed"},
		"artifacts": []map[string]any{{"parts": []map[string]any{{"type": "text", "text": "Task completed"}}}},
	}
	result, _ := json.Marshal(task)

	s.writeJSON(w, http.StatusOK, map[string]any{
		"jsonrpc": "2.0",
		"result":  json.RawMessage(result),
		"id":      req.ID,
	})
}

func (s *Server) getOrCreateA2ASession(ctx context.Context, c *acpclient.Client) (string, error) {
	var res json.RawMessage
	err := c.Call(ctx, "session/new", map[string]any{
		"workspace": "/tmp",
	}, &res)
	if err != nil {
		return "", err
	}
	var result struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(res, &result); err != nil {
		return "", err
	}
	return result.SessionID, nil
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
	q := r.URL.Query()
	cwd := q.Get("cwd")
	if cwd != "" && !filepath.IsAbs(cwd) {
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "cwd must be an absolute path"})
		return
	}
	filter, err := parseListFilter(q.Get("q"), q.Get("archived"), q.Get("limit"))
	if err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	cur, err := decodeListCursor(q.Get("cursor"))
	if err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid cursor"})
		return
	}
	page, err := s.collectSessions(r.Context(), cwd, cur, filter)
	if err != nil {
		s.writeJSON(w, http.StatusBadGateway, map[string]any{"error": s.redact(err.Error())})
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{
		"sessions":   page.sessions,
		"nextCursor": page.next,
		"truncated":  page.truncated,
	})
}

// stripZeroMeta drops the Zero vendor object. Other profiles do not interpret
// _meta.modelId.
func stripZeroMeta(raw []json.RawMessage) []json.RawMessage {
	out := make([]json.RawMessage, 0, len(raw))
	for _, item := range raw {
		var obj map[string]json.RawMessage
		if json.Unmarshal(item, &obj) != nil {
			out = append(out, item)
			continue
		}
		delete(obj, "_meta")
		b, err := json.Marshal(obj)
		if err != nil {
			out = append(out, item)
			continue
		}
		out = append(out, b)
	}
	return out
}

func (s *Server) handleNewSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	workspace := s.workspace
	var extra []string
	if r.ContentLength != 0 {
		if !requireJSONContentType(r) {
			s.writeJSON(w, http.StatusUnsupportedMediaType, map[string]any{"error": "content-type must be application/json"})
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var body struct {
			Workspace             string   `json:"workspace"`
			AdditionalDirectories []string `json:"additionalDirectories"`
		}
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&body); err != nil && err != io.EOF {
			s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
			return
		}
		if body.Workspace != "" {
			workspace = body.Workspace
		}
		extra = body.AdditionalDirectories
	}
	cwd, dirs, err := s.resolveNewSession(workspace, extra)
	if err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	params := map[string]any{
		"cwd":        cwd,
		"mcpServers": []any{},
	}
	if len(dirs) > 0 {
		params["additionalDirectories"] = dirs
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
	if err := s.acpCall(r.Context(), "session/new", params, &res); err != nil {
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
	offer, _ := c.SessionOffer(id)
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
	allowed := map[string]bool{}
	fallback := false
	if len(offer.Modes) > 0 {
		for _, m := range offer.Modes {
			if m.ID != "" {
				allowed[m.ID] = true
			}
		}
	} else if s.cfg.Profile == "zero" {
		fallback = true
		allowed["auto"] = true
		allowed["ask"] = true
		allowed["plan"] = true
	} else {
		s.notSupported(w)
		return
	}
	if !allowed[body.Mode] {
		msg := "mode is not offered by the agent"
		if fallback {
			msg = "mode must be auto, ask, or plan"
		}
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": msg})
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

func (s *Server) handleConfigOption(w http.ResponseWriter, r *http.Request) {
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
	offer, _ := c.SessionOffer(id)
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	var body struct {
		OptionID string `json:"optionId"`
		Value    string `json:"value"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	if body.OptionID == "" || body.Value == "" {
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "optionId and value are required"})
		return
	}
	// Validate against the offered options
	allowed := false
	for _, opt := range offer.ConfigOptions {
		if opt.ID == body.OptionID && opt.Type == "select" {
			for _, v := range opt.Options {
				if v.Value == body.Value {
					allowed = true
					break
				}
			}
			break
		}
	}
	if !allowed {
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "value not offered by the agent"})
		return
	}
	var ignored map[string]any
	if err := s.acpCall(r.Context(), "session/set_config_option", map[string]any{
		"sessionId": id,
		"optionId":  body.OptionID,
		"value":     body.Value,
	}, &ignored); err != nil {
		s.writeJSON(w, http.StatusBadGateway, map[string]any{"error": s.redact(err.Error())})
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"ok": true, "optionId": body.OptionID, "value": body.Value})
}

func (s *Server) handleModeRecommend(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.AutoModeEnabled {
		s.writeJSON(w, http.StatusNotImplemented, map[string]any{"error": "automatic mode routing is disabled"})
		return
	}
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
	offer, _ := c.SessionOffer(id)
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	var body struct {
		Text string `json:"text"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	if body.Text == "" {
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "text is required"})
		return
	}
	// Get current mode and available modes.
	offerModes := offer.Modes
	if len(offerModes) == 0 && s.cfg.Profile == "zero" {
		offerModes = []acpclient.Mode{
			{ID: "auto", Name: "Auto", Description: "Agent decides"},
			{ID: "ask", Name: "Ask", Description: "Writes ask for approval"},
			{ID: "plan", Name: "Plan", Description: "Read-only planning"},
		}
	}
	if len(offerModes) == 0 {
		s.notSupported(w)
		return
	}
	// Build allowed map from offered modes.
	allowed := map[string]bool{}
	for _, m := range offerModes {
		if m.ID != "" {
			allowed[m.ID] = true
		}
	}
	// Determine current mode from session.
	var currentMode string
	if hub.currentMode != "" {
		currentMode = hub.currentMode
	} else {
		// Fallback to first offered mode.
		currentMode = offerModes[0].ID
	}
	// Run the routing rules.
	recommended, ruleID := s.routeMode(currentMode, body.Text, allowed)
	// Enforce ceiling.
	ceiling := s.cfg.AutoModeCeiling
	if ceiling != "" && !allowed[ceiling] {
		ceiling = ""
	}
	if ceiling != "" {
		// Check if recommended is above ceiling.
		ceilingRank := modeRank(ceiling)
		recommendedRank := modeRank(recommended)
		if recommendedRank > ceilingRank {
			recommended = ceiling
			ruleID = "ceiling"
		}
	} else {
		// Reduce-only: never raise authority.
		currentRank := modeRank(currentMode)
		recommendedRank := modeRank(recommended)
		if recommendedRank > currentRank {
			recommended = currentMode
			ruleID = "reduce-only"
		}
	}
	// Audit event (no prompt text).
	s.audit.Record(auth.Event{
		Kind:   auth.KindModeRouted,
		Source: "auto",
		Method: "POST",
		Path:   "/v1/sessions/" + id + "/mode/recommend",
		Reason: fmt.Sprintf("from=%s to=%s rule=%s", currentMode, recommended, ruleID),
	})
	// If apply is enabled and recommendation differs, apply it.
	applied := false
	if s.cfg.AutoModeApply && recommended != currentMode {
		var ignored map[string]any
		if err := s.acpCall(r.Context(), "session/set_mode", map[string]any{
			"sessionId": id,
			"modeId":    recommended,
		}, &ignored); err == nil {
			applied = true
			hub.currentMode = recommended
		}
	}
	s.writeJSON(w, http.StatusOK, map[string]any{
		"ok":          true,
		"currentMode": currentMode,
		"recommended": recommended,
		"rule":        ruleID,
		"applied":     applied,
	})
}

// modeRank returns an ordering: plan=0, ask=1, auto=2.
func modeRank(m string) int {
	switch m {
	case "plan":
		return 0
	case "ask":
		return 1
	case "auto":
		return 2
	default:
		return -1
	}
}

// routeMode applies deterministic, reduce-only rules.
func (s *Server) routeMode(current, text string, allowed map[string]bool) (string, string) {
	// Rules are applied in order; first match wins.
	// Lower rank = more restrictive.
	planID := "plan"
	askID := "ask"
	if !allowed[planID] {
		planID = ""
	}
	if !allowed[askID] {
		askID = ""
	}
	// Rule 1: Empty or whitespace-only -> plan (most restrictive).
	if strings.TrimSpace(text) == "" {
		if planID != "" {
			return planID, "empty"
		}
	}
	// Rule 2: Hostile/override attempts -> never raise.
	// "ignore the rules", "use auto", "switch to auto", etc.
	lower := strings.ToLower(text)
	overridePatterns := []string{
		"ignore the rules",
		"use auto",
		"switch to auto",
		"force auto",
		"auto mode",
		"bypass",
		"override",
	}
	for _, p := range overridePatterns {
		if strings.Contains(lower, p) {
			// Stay at or below current.
			return current, "override-attempt"
		}
	}
	// Rule 3: Keywords that suggest read-only/planning.
	planKeywords := []string{
		"explain",
		"analyze",
		"review",
		"audit",
		"describe",
		"how does",
		"what is",
		"summarize",
		"plan",
		"design",
		"architecture",
		"read only",
		"read-only",
		"just look",
		"only read",
	}
	for _, k := range planKeywords {
		if strings.Contains(lower, k) && planID != "" {
			return planID, "plan-keyword"
		}
	}
	// Rule 4: Keywords that suggest write/ask.
	askKeywords := []string{
		"create",
		"write",
		"edit",
		"modify",
		"change",
		"update",
		"delete",
		"remove",
		"add",
		"implement",
		"fix",
		"refactor",
		"rename",
		"move",
		"copy",
		"run",
		"execute",
		"test",
		"build",
		"deploy",
		"commit",
		"push",
		"merge",
	}
	for _, k := range askKeywords {
		if strings.Contains(lower, k) && askID != "" {
			return askID, "ask-keyword"
		}
	}
	// Rule 5: Code blocks or diffs in prompt -> ask (user showing code to edit).
	if strings.Contains(text, "```") || strings.Contains(text, "diff") {
		if askID != "" {
			return askID, "code-block"
		}
	}
	// Rule 6: Very short prompts without clear intent -> ask (default to safer).
	if len(strings.TrimSpace(text)) < 20 && askID != "" {
		return askID, "short"
	}
	// Default: stay at current.
	return current, "default"
}

func (s *Server) resolveNewSession(workspace string, extra []string) (string, []string, error) {
	if s.roots != nil {
		return s.roots.Resolve(workspace, extra)
	}
	if len(extra) > 0 {
		return "", nil, fmt.Errorf("directory is not allowed")
	}
	if err := validateWorkspace(workspace); err != nil {
		return "", nil, err
	}
	return filepath.Clean(workspace), nil, nil
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

func (s *Server) validateImage(data []byte, mime string) error {
	if int64(len(data)) > s.cfg.MaxImageBytes {
		return fmt.Errorf("image size %d exceeds limit %d", len(data), s.cfg.MaxImageBytes)
	}
	allowed := false
	for _, m := range s.cfg.AllowImageMIME {
		if m == mime {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Errorf("MIME type %q not allowed", mime)
	}
	// Magic byte check.
	if len(data) < 12 {
		return fmt.Errorf("image too small for magic byte check")
	}
	switch {
	case bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}): // PNG
		if mime != "image/png" {
			return fmt.Errorf("magic bytes indicate PNG but MIME is %q", mime)
		}
	case bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}): // JPEG
		if mime != "image/jpeg" {
			return fmt.Errorf("magic bytes indicate JPEG but MIME is %q", mime)
		}
	case bytes.HasPrefix(data, []byte{0x52, 0x49, 0x46, 0x46}) && bytes.HasPrefix(data[8:12], []byte{0x57, 0x45, 0x42, 0x50}): // WebP (RIFF + WEBP)
		if mime != "image/webp" {
			return fmt.Errorf("magic bytes indicate WebP but MIME is %q", mime)
		}
	case bytes.HasPrefix(data, []byte{0x47, 0x49, 0x46, 0x38, 0x37, 0x61}) || bytes.HasPrefix(data, []byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61}): // GIF87a / GIF89a
		if mime != "image/gif" {
			return fmt.Errorf("magic bytes indicate GIF but MIME is %q", mime)
		}
	default:
		return fmt.Errorf("unrecognized image format")
	}
	return nil
}

func (s *Server) handlePrompt(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ctx, endSpan := observability.StartHTTPSpan(r.Context(), "/v1/sessions/{id}/prompt", "POST")
	defer endSpan()

	r = r.WithContext(ctx)

	if !requireJSONContentType(r) {
		observability.RecordHTTPMetric("/v1/sessions/{id}/prompt", "POST", http.StatusUnsupportedMediaType, time.Since(start).Seconds()*1000)
		s.writeJSON(w, http.StatusUnsupportedMediaType, map[string]any{"error": "content-type must be application/json"})
		return
	}
	id := r.PathValue("id")
	hub := s.sessionHub(id)
	if hub == nil {
		observability.RecordHTTPMetric("/v1/sessions/{id}/prompt", "POST", http.StatusNotFound, time.Since(start).Seconds()*1000)
		s.writeJSON(w, http.StatusNotFound, map[string]any{"error": "session not found"})
		return
	}
	if !hub.tryStartPrompt() {
		observability.RecordHTTPMetric("/v1/sessions/{id}/prompt", "POST", http.StatusConflict, time.Since(start).Seconds()*1000)
		s.writeJSON(w, http.StatusConflict, map[string]any{"error": "prompt already in progress"})
		return
	}
	// Allow up to max images + text in body.
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxImageBytes*int64(s.cfg.MaxImages+1))
	var body struct {
		Text   string            `json:"text"`
		Images []ImageAttachment `json:"images,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		hub.endPrompt()
		observability.RecordHTTPMetric("/v1/sessions/{id}/prompt", "POST", http.StatusBadRequest, time.Since(start).Seconds()*1000)
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	if body.Text == "" && len(body.Images) == 0 {
		hub.endPrompt()
		observability.RecordHTTPMetric("/v1/sessions/{id}/prompt", "POST", http.StatusBadRequest, time.Since(start).Seconds()*1000)
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "text or images required"})
		return
	}
	if len(body.Images) > s.cfg.MaxImages {
		hub.endPrompt()
		observability.RecordHTTPMetric("/v1/sessions/{id}/prompt", "POST", http.StatusBadRequest, time.Since(start).Seconds()*1000)
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": fmt.Sprintf("too many images (max %d)", s.cfg.MaxImages)})
		return
	}
	// Validate each image.
	for i, img := range body.Images {
		data, err := base64.StdEncoding.DecodeString(img.Data)
		if err != nil {
			hub.endPrompt()
			observability.RecordHTTPMetric("/v1/sessions/{id}/prompt", "POST", http.StatusBadRequest, time.Since(start).Seconds()*1000)
			s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": fmt.Sprintf("image %d: invalid base64", i)})
			return
		}
		if err := s.validateImage(data, img.MIME); err != nil {
			hub.endPrompt()
			observability.RecordHTTPMetric("/v1/sessions/{id}/prompt", "POST", http.StatusBadRequest, time.Since(start).Seconds()*1000)
			s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": fmt.Sprintf("image %d: %v", i, err)})
			return
		}
		body.Images[i].Data = string(data) // Store decoded bytes for runPrompt
	}
	observability.RecordHTTPMetric("/v1/sessions/{id}/prompt", "POST", http.StatusAccepted, time.Since(start).Seconds()*1000)
	s.writeJSON(w, http.StatusAccepted, map[string]any{"ok": true})
	s.applyState(id, sessionstate.Event{Kind: sessionstate.KindPrompt})
	go s.runPrompt(id, hub, body.Text, body.Images)
}

type ImageAttachment struct {
	Data string `json:"data"` // base64 encoded
	MIME string `json:"mimeType"`
}

func (s *Server) runPrompt(sessionID string, hub *sessionHub, text string, images []ImageAttachment) {
	start := time.Now()
	ctx, endSpan := observability.StartACPSpan(context.Background(), "session/prompt")
	defer endSpan()

	defer func() {
		hub.endPrompt()
		if s.perms != nil {
			s.perms.AbortSession(sessionID)
		}
	}()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	var res struct {
		StopReason string `json:"stopReason"`
	}
	// Build prompt content blocks.
	prompt := make([]map[string]any, 0, 1+len(images))
	if text != "" {
		prompt = append(prompt, map[string]any{"type": "text", "text": text})
	}
	for _, img := range images {
		prompt = append(prompt, map[string]any{
			"type":     "image",
			"data":     img.Data,
			"mimeType": img.MIME,
		})
	}
	err := s.acpCall(ctx, "session/prompt", map[string]any{
		"sessionId": sessionID,
		"prompt":    prompt,
	}, &res)
	observability.RecordACPMetric("session/prompt", time.Since(start).Seconds()*1000, err)
	if err != nil {
		s.applyState(sessionID, sessionstate.Event{Kind: sessionstate.KindFail})
		hub.emit(map[string]any{"type": "error", "message": s.redact(err.Error())})
		return
	}
	s.applyState(sessionID, sessionstate.Event{Kind: sessionstate.KindStop, Stop: res.StopReason})
	hub.emit(map[string]any{"type": "stop", "stopReason": res.StopReason})
}

func (s *Server) handleCancel(w http.ResponseWriter, r *http.Request) {
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
	if !hub.promptInProgress() {
		s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	if err := s.cancelPrompt(id); err != nil {
		s.writeJSON(w, http.StatusBadGateway, map[string]any{"error": "request failed"})
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
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
