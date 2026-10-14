package server

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"zero-bridge/bridge/internal/auth"
)

func (s *Server) currentToken() string {
	s.tokenMu.RLock()
	defer s.tokenMu.RUnlock()
	return s.token
}

// ArmReload listens for SIGHUP until Shutdown. A token that came from the
// environment has an empty TokenFile, so the signal keeps the current token.
// The goroutine exits when Shutdown cancels the server context.
func (s *Server) ArmReload() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGHUP)
	s.startReload(ch, func() { signal.Stop(ch) })
}

func (s *Server) startReload(signals <-chan os.Signal, stop func()) {
	s.tokenMu.Lock()
	if s.reloadDone != nil {
		s.tokenMu.Unlock()
		if stop != nil {
			stop()
		}
		return
	}
	done := make(chan struct{})
	s.reloadDone = done
	s.tokenMu.Unlock()
	go func() {
		defer close(done)
		if stop != nil {
			defer stop()
		}
		for {
			select {
			case <-s.serveCtx.Done():
				return
			case <-signals:
				s.reloadTokenFile()
			}
		}
	}()
}

func (s *Server) waitReload(ctx context.Context) {
	s.tokenMu.RLock()
	done := s.reloadDone
	s.tokenMu.RUnlock()
	if done == nil {
		return
	}
	select {
	case <-done:
	case <-ctx.Done():
	}
}

func (s *Server) reloadTokenFile() {
	if s.cfg.TokenFile == "" {
		s.recordAudit(auth.KindTokenReloadFailed, nil, "not a file")
		return
	}
	next, err := auth.LoadFile(s.cfg.TokenFile)
	if err != nil {
		s.recordAudit(auth.KindTokenReloadFailed, nil, tokenReloadReason(err))
		return
	}
	s.tokenMu.Lock()
	s.prevToken = s.token
	s.token = next
	s.tokenMu.Unlock()
	s.dropAllSessions()
	s.recordAudit(auth.KindTokenReloaded, nil, "ok")
}

func tokenReloadReason(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "group or others"):
		return "mode"
	case strings.Contains(msg, "at least"):
		return "entropy"
	case strings.Contains(msg, "no such file"):
		return "missing"
	default:
		return "read"
	}
}
