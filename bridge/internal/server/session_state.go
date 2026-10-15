package server

import "zero-bridge/bridge/internal/sessionstate"

// stateLevel is full for every profile in this slice of the wiring.
// The capability gate narrows generic to basic.
func (s *Server) stateLevel() string {
	return sessionstate.LevelFull
}

func (s *Server) applyState(id string, ev sessionstate.Event) {
	s.stateMu.Lock()
	if s.states == nil {
		s.states = map[string]sessionstate.Machine{}
	}
	m, ok := s.states[id]
	if !ok {
		m = sessionstate.New(s.stateLevel())
		if s.agentDown {
			m = m.Apply(sessionstate.Event{Kind: sessionstate.KindExit})
		}
	}
	prev := m.State()
	m = m.Apply(ev)
	s.states[id] = m
	next := m.State()
	s.stateMu.Unlock()
	if next == prev {
		return
	}
	if hub := s.sessionHub(id); hub != nil {
		hub.emit(map[string]any{"type": "session_state", "state": next})
	}
}

func (s *Server) stateOf(id string) string {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if m, ok := s.states[id]; ok {
		return m.State()
	}
	if s.agentDown && s.stateLevel() == sessionstate.LevelFull {
		return sessionstate.Error
	}
	return sessionstate.Idle
}

func (s *Server) markAgentDown() {
	s.stateMu.Lock()
	s.agentDown = true
	s.stateMu.Unlock()
	s.mu.RLock()
	ids := make([]string, 0, len(s.sessions))
	for id := range s.sessions {
		ids = append(ids, id)
	}
	s.mu.RUnlock()
	for _, id := range ids {
		s.applyState(id, sessionstate.Event{Kind: sessionstate.KindExit})
	}
}
