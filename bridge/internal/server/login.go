package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"zero-bridge/bridge/internal/auth"
)

const sessionCookie = "zb_session"

// bridgeSession is the cookie record. The map key is the cookie value and
// never goes into an audit line or a JSON body. handle is a separate public
// id, not derived from the cookie.
type bridgeSession struct {
	handle  string
	created time.Time
	last    time.Time
	expires time.Time
}

type loginState struct {
	mu       sync.Mutex
	sessions map[string]*bridgeSession
	byHandle map[string]string
	fails    map[string][]time.Time
}

func newLoginState() *loginState {
	return &loginState{
		sessions: map[string]*bridgeSession{},
		byHandle: map[string]string{},
		fails:    map[string][]time.Time{},
	}
}

func (st *loginState) forgetLocked(id string) {
	sess, ok := st.sessions[id]
	if !ok {
		return
	}
	if sess.handle != "" {
		delete(st.byHandle, sess.handle)
	}
	delete(st.sessions, id)
}

func (s *Server) now() time.Time {
	if s.cfg.Now != nil {
		return s.cfg.Now()
	}
	return time.Now()
}

func (s *Server) clientKey(r *http.Request) string {
	if s.cfg.TrustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			first := strings.TrimSpace(strings.Split(xff, ",")[0])
			if first != "" {
				return first
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Server) cookieSecure(r *http.Request) bool {
	return s.cfg.TrustProxy && strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func (s *Server) sessionOK(r *http.Request) bool {
	c, err := r.Cookie(sessionCookie)
	if err != nil || c.Value == "" {
		return false
	}
	alive, reason := s.touchSession(c.Value)
	if reason != "" {
		s.recordAudit(auth.KindSessionExpired, r, reason)
	}
	return alive
}

func (s *Server) touchSession(id string) (bool, string) {
	s.login.mu.Lock()
	defer s.login.mu.Unlock()
	sess, ok := s.login.sessions[id]
	if !ok {
		return false, ""
	}
	now := s.now()
	if !now.Before(sess.expires) {
		s.login.forgetLocked(id)
		return false, "ttl"
	}
	if !now.Before(sess.last.Add(s.cfg.SessionIdle)) {
		s.login.forgetLocked(id)
		return false, "idle"
	}
	sess.last = now
	return true, ""
}

func (s *Server) dropSession(id string) {
	s.login.mu.Lock()
	s.login.forgetLocked(id)
	s.login.mu.Unlock()
}

func (s *Server) dropAllSessions() {
	s.login.mu.Lock()
	s.login.sessions = map[string]*bridgeSession{}
	s.login.byHandle = map[string]string{}
	s.login.mu.Unlock()
}

func (s *Server) putSession(id string, now time.Time) (bool, error) {
	s.login.mu.Lock()
	defer s.login.mu.Unlock()
	handle, err := newPublicHandle()
	if err != nil {
		return false, err
	}
	for s.login.byHandle[handle] != "" {
		handle, err = newPublicHandle()
		if err != nil {
			return false, err
		}
	}
	evicted := false
	if len(s.login.sessions) >= s.cfg.MaxSessions {
		var oldestID string
		var oldest time.Time
		for sid, sess := range s.login.sessions {
			if oldestID == "" || sess.created.Before(oldest) {
				oldestID = sid
				oldest = sess.created
			}
		}
		s.login.forgetLocked(oldestID)
		evicted = true
	}
	s.login.sessions[id] = &bridgeSession{
		handle:  handle,
		created: now,
		last:    now,
		expires: now.Add(s.cfg.SessionTTL),
	}
	s.login.byHandle[handle] = id
	return evicted, nil
}

func (s *Server) reserveLogin(key string) (at time.Time, limited bool, retryAfter int) {
	now := s.now()
	s.login.mu.Lock()
	defer s.login.mu.Unlock()
	kept := recentFailures(s.login.fails[key], now, s.cfg.AuthFailWindow)
	loginFailMax := s.cfg.AuthFailLimit
	if len(kept) >= loginFailMax {
		s.login.fails[key] = kept
		retry := int(kept[0].Add(s.cfg.AuthFailWindow).Sub(now).Seconds())
		if retry < 1 {
			retry = 1
		}
		return time.Time{}, true, retry
	}
	s.login.fails[key] = append(kept, now)
	return now, false, 0
}

func (s *Server) releaseLogin(key string, at time.Time) {
	s.login.mu.Lock()
	defer s.login.mu.Unlock()
	prev := s.login.fails[key]
	for i, ts := range prev {
		if ts.Equal(at) {
			s.login.fails[key] = append(append([]time.Time{}, prev[:i]...), prev[i+1:]...)
			return
		}
	}
}

func recentFailures(prev []time.Time, now time.Time, window time.Duration) []time.Time {
	cutoff := now.Add(-window)
	kept := make([]time.Time, 0, len(prev))
	for _, ts := range prev {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}
	return kept
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	authOn := s.currentToken() != ""
	ok := !authOn || s.credentialOK(r)
	s.writeJSON(w, http.StatusOK, map[string]any{"auth": authOn, "authenticated": ok})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if s.currentToken() == "" {
		s.writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
		return
	}
	if !requireJSONContentType(r) {
		s.writeJSON(w, http.StatusUnsupportedMediaType, map[string]any{"error": "content-type must be application/json"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	var body struct {
		Token string `json:"token"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	key := s.clientKey(r)
	at, limited, retry := s.reserveLogin(key)
	if limited {
		s.recordAudit(auth.KindRateLimited, r, "window full")
		s.sleepLoginFailure()
		w.Header().Set("Retry-After", strconv.Itoa(retry))
		s.writeJSON(w, http.StatusTooManyRequests, map[string]any{"error": "too many requests"})
		return
	}
	if !auth.Equal(s.currentToken(), body.Token) {
		s.recordAudit(auth.KindLoginFailure, r, "mismatch")
		s.sleepLoginFailure()
		s.writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	s.releaseLogin(key, at)
	s.recordAudit(auth.KindLoginSuccess, r, "ok")
	id, err := newSessionID()
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "unavailable"})
		return
	}
	now := s.now()
	evicted, err := s.putSession(id, now)
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "unavailable"})
		return
	}
	if evicted {
		s.recordAudit(auth.KindSessionEvicted, r, "oldest")
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    id,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(s.cfg.SessionTTL.Seconds()),
		Secure:   s.cookieSecure(r),
	})
	s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) sleepLoginFailure() {
	if s.loginDelay > 0 {
		time.Sleep(s.loginDelay)
	}
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		s.dropSession(c.Value)
	}
	s.recordAudit(auth.KindLogout, r, "ok")
	s.clearSessionCookie(w, r)
	s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) cookieCSRF(r *http.Request) bool {
	if s.currentToken() == "" || !s.sessionOK(r) || s.hasBearer(r) {
		return true
	}
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		return true
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	return originAllowed(origin, s.allowed, s.cfg.PublicOrigins)
}

// bearerGate counts a presented Bearer on a protected route before the
// comparison. A full window rejects the request, including a later correct
// token. A valid session cookie does not consume the counter. A missing
// Authorization header stays a plain 401 so an empty probe is not a Bearer
// failure.
func (s *Server) bearerGate(r *http.Request) (bool, int) {
	if s.currentToken() == "" || isPublic(r) {
		return false, 0
	}
	if r.URL.Path == "/v1/auth/status" && r.Method == http.MethodGet {
		return false, 0
	}
	if r.URL.Path == "/v1/login" && r.Method == http.MethodPost {
		return false, 0
	}
	if s.sessionOK(r) {
		return false, 0
	}
	tok, ok := presentedBearer(r)
	if !ok {
		return false, 0
	}
	key := s.clientKey(r)
	at, limited, retry := s.reserveLogin(key)
	if limited {
		s.recordAudit(auth.KindRateLimited, r, "window full")
		s.sleepLoginFailure()
		return true, retry
	}
	if !auth.Equal(s.currentToken(), tok) {
		s.recordAudit(auth.KindBearerFailure, r, "mismatch")
		s.sleepLoginFailure()
		return false, 0
	}
	s.releaseLogin(key, at)
	return false, 0
}

func (s *Server) recordAudit(kind auth.Kind, r *http.Request, reason string) {
	if s.audit == nil {
		return
	}
	ev := auth.Event{Time: s.now(), Kind: kind, Reason: reason}
	if r != nil {
		ev.Source = s.clientKey(r)
		ev.Method = r.Method
		ev.Path = r.URL.Path
	}
	_ = s.audit.Record(ev)
}

func newSessionID() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// newPublicHandle returns an id that is not a prefix of the cookie. The
// leading "s." is outside the hex alphabet of the cookie value.
func newPublicHandle() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "s." + hex.EncodeToString(b[:]), nil
}

type authSessionView struct {
	Handle  string    `json:"handle"`
	Created time.Time `json:"created"`
	Last    time.Time `json:"last"`
	Expires time.Time `json:"expires"`
}

func (s *Server) authSessionViews() []authSessionView {
	s.login.mu.Lock()
	defer s.login.mu.Unlock()
	out := make([]authSessionView, 0, len(s.login.sessions))
	for _, sess := range s.login.sessions {
		out = append(out, authSessionView{
			Handle:  sess.handle,
			Created: sess.created,
			Last:    sess.last,
			Expires: sess.expires,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Created.Equal(out[j].Created) {
			return out[i].Handle < out[j].Handle
		}
		return out[i].Created.Before(out[j].Created)
	})
	return out
}

func (s *Server) revokeSession(handle string) bool {
	s.login.mu.Lock()
	defer s.login.mu.Unlock()
	id, ok := s.login.byHandle[handle]
	if !ok {
		return false
	}
	s.login.forgetLocked(id)
	return true
}

func (s *Server) clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
		Secure:   s.cookieSecure(r),
	})
}

func (s *Server) handleLogoutAll(w http.ResponseWriter, r *http.Request) {
	s.dropAllSessions()
	s.recordAudit(auth.KindLogoutAll, r, "ok")
	s.clearSessionCookie(w, r)
	s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleListAuthSessions(w http.ResponseWriter, _ *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]any{"sessions": s.authSessionViews()})
}

func (s *Server) handleRevokeAuthSession(w http.ResponseWriter, r *http.Request) {
	if !s.revokeSession(r.PathValue("handle")) {
		s.writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
		return
	}
	s.recordAudit(auth.KindLogout, r, "revoked")
	s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
