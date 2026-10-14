package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"zero-bridge/bridge/internal/auth"
)

const (
	sessionCookie   = "zb_session"
	sessionTTL      = 12 * time.Hour
	loginFailWindow = time.Minute
	loginFailMax    = 5
)

type loginState struct {
	mu       sync.Mutex
	sessions map[string]time.Time
	fails    map[string][]time.Time
}

func newLoginState() *loginState {
	return &loginState{
		sessions: map[string]time.Time{},
		fails:    map[string][]time.Time{},
	}
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
	s.login.mu.Lock()
	defer s.login.mu.Unlock()
	exp, ok := s.login.sessions[c.Value]
	if !ok || !s.now().Before(exp) {
		delete(s.login.sessions, c.Value)
		return false
	}
	return true
}

func (s *Server) dropSession(id string) {
	s.login.mu.Lock()
	delete(s.login.sessions, id)
	s.login.mu.Unlock()
}

func (s *Server) noteLoginFailure(key string) (limited bool, retryAfter int) {
	now := s.now()
	s.login.mu.Lock()
	defer s.login.mu.Unlock()
	cutoff := now.Add(-loginFailWindow)
	prev := s.login.fails[key]
	kept := make([]time.Time, 0, len(prev)+1)
	for _, ts := range prev {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}
	if len(kept) >= loginFailMax {
		s.login.fails[key] = kept
		retry := int(kept[0].Add(loginFailWindow).Sub(now).Seconds())
		if retry < 1 {
			retry = 1
		}
		return true, retry
	}
	s.login.fails[key] = append(kept, now)
	return false, 0
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	authOn := s.cfg.Token != ""
	ok := !authOn || s.hasBearer(r) || s.sessionOK(r)
	s.writeJSON(w, http.StatusOK, map[string]any{"auth": authOn, "authenticated": ok})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Token == "" {
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
	if !auth.Equal(s.cfg.Token, body.Token) {
		s.failLogin(w, r)
		return
	}
	id, err := newSessionID()
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "unavailable"})
		return
	}
	exp := s.now().Add(sessionTTL)
	s.login.mu.Lock()
	s.login.sessions[id] = exp
	s.login.mu.Unlock()
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    id,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(sessionTTL.Seconds()),
		Secure:   s.cookieSecure(r),
	})
	s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) failLogin(w http.ResponseWriter, r *http.Request) {
	if s.loginDelay > 0 {
		time.Sleep(s.loginDelay)
	}
	limited, retry := s.noteLoginFailure(s.clientKey(r))
	if limited {
		w.Header().Set("Retry-After", strconv.Itoa(retry))
		s.writeJSON(w, http.StatusTooManyRequests, map[string]any{"error": "too many requests"})
		return
	}
	s.writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		s.dropSession(c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
		Secure:   s.cookieSecure(r),
	})
	s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) cookieCSRF(r *http.Request) bool {
	if s.cfg.Token == "" || !s.sessionOK(r) || s.hasBearer(r) || r.Method != http.MethodPost {
		return true
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	return originAllowed(origin, s.allowed, s.cfg.PublicOrigins)
}

func newSessionID() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
