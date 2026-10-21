package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"sync"
	"time"

	"zero-bridge/bridge/internal/redact"
)

// Kind is the audit event name. The set is closed so a caller cannot invent
// a field that carries a credential.
type Kind string

const (
	KindLoginSuccess      Kind = "login_success"
	KindLoginFailure      Kind = "login_failure"
	KindRateLimited       Kind = "rate_limited"
	KindBearerFailure     Kind = "bearer_failure"
	KindLogout            Kind = "logout"
	KindLogoutAll         Kind = "logout_all"
	KindSessionExpired    Kind = "session_expired"
	KindSessionEvicted    Kind = "session_evicted"
	KindTokenReloaded     Kind = "token_reloaded"
	KindTokenReloadFailed Kind = "token_reload_failed"
	KindModeRouted        Kind = "mode_routed"
	KindPermissionAllow   Kind = "permission_allow"
	KindPermissionDeny    Kind = "permission_deny"
	KindPermissionExpire  Kind = "permission_expire"
	KindPromptSubmitted   Kind = "prompt_submitted"
	KindSessionCreated    Kind = "session_created"
	KindIngress           Kind = "ingress"
)

// Event is one audit record. It has no token, no token prefix, and no session
// id. Those values must not be copied into Source, Path, or Reason either;
// Record applies the shared secret-shape list and then scrubs long hex runs.
type Event struct {
	Time   time.Time `json:"time"`
	Kind   Kind      `json:"kind"`
	Source string    `json:"source"`
	Method string    `json:"method"`
	Path   string    `json:"path"`
	Reason string    `json:"reason"`
}

// Sink receives audit events. Record is synchronous: it does not start a goroutine.
type Sink interface {
	Record(Event) error
}

// Closer releases a sink that owns a file. The log sink does not implement it.
type Closer interface {
	Close() error
}

var hexRun = regexp.MustCompile(`[A-Fa-f0-9]{32,}`)

func scrub(s string) string {
	s = redact.Apply(s)
	s = hexRun.ReplaceAllString(s, "[redacted]")
	return s
}

func line(e Event) (string, error) {
	e.Source = scrub(e.Source)
	e.Method = scrub(e.Method)
	e.Path = scrub(e.Path)
	e.Reason = scrub(e.Reason)
	if e.Time.IsZero() {
		e.Time = time.Now().UTC()
	} else {
		e.Time = e.Time.UTC()
	}
	b, err := json.Marshal(e)
	if err != nil {
		return "", err
	}
	return "audit " + string(b), nil
}

// LogSink writes audit lines to an existing logger. It does not close it.
type LogSink struct {
	mu sync.Mutex
	l  *log.Logger
}

// NewLogSink returns a sink that prints one line per event.
func NewLogSink(l *log.Logger) *LogSink {
	if l == nil {
		l = log.New(io.Discard, "", 0)
	}
	return &LogSink{l: l}
}

// Record writes one scrubbed audit line.
func (s *LogSink) Record(e Event) error {
	text, err := line(e)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.l.Print(text)
	return nil
}

// FileSink appends audit lines to a regular file. The mode must not grant
// group or other access. A new file is created at 0600.
type FileSink struct {
	mu sync.Mutex
	f  *os.File
}

// OpenFileSink opens path for append. An existing file whose mode is more
// open than 0600 is refused. Symlinks are refused.
func OpenFileSink(path string) (*FileSink, error) {
	fi, err := os.Lstat(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("audit log: %w", err)
	}
	if err == nil {
		if fi.Mode()&os.ModeSymlink != 0 || !fi.Mode().IsRegular() {
			return nil, fmt.Errorf("audit log %s is not a regular file", path)
		}
		if fi.Mode().Perm()&0o077 != 0 {
			return nil, fmt.Errorf("audit log %s is readable by group or others (mode %#o); use 0600", path, fi.Mode().Perm())
		}
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
		if err != nil {
			return nil, fmt.Errorf("audit log: %w", err)
		}
		return &FileSink{f: f}, nil
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, fmt.Errorf("audit log: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("audit log: %w", err)
	}
	return &FileSink{f: f}, nil
}

// Record appends one scrubbed audit line and a newline.
func (s *FileSink) Record(e Event) error {
	text, err := line(e)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.f.WriteString(text + "\n"); err != nil {
		return err
	}
	return nil
}

// Close closes the file.
func (s *FileSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.f == nil {
		return nil
	}
	err := s.f.Close()
	s.f = nil
	return err
}

// Multi records to every child. A child error stops the rest.
type Multi struct {
	children []Sink
}

// NewMulti returns a sink that fans out to children.
func NewMulti(children ...Sink) *Multi {
	return &Multi{children: children}
}

// Record writes the event to each child.
func (m *Multi) Record(e Event) error {
	for _, c := range m.children {
		if c == nil {
			continue
		}
		if err := c.Record(e); err != nil {
			return err
		}
	}
	return nil
}

// Close closes every child that implements Closer.
func (m *Multi) Close() error {
	var err error
	for _, c := range m.children {
		cl, ok := c.(Closer)
		if !ok {
			continue
		}
		if cerr := cl.Close(); err == nil {
			err = cerr
		}
	}
	return err
}
