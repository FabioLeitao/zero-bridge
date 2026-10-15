package acpclient

import (
	"encoding/json"
	"sync"
)

// PromptCapabilities is the promptCapabilities object from initialize.
type PromptCapabilities struct {
	Image           bool `json:"image"`
	Audio           bool `json:"audio"`
	EmbeddedContext bool `json:"embeddedContext"`
}

// SessionCapabilities advertises optional ACP session methods.
// A non-nil List or Resume means the agent sent that key (usually {}).
type SessionCapabilities struct {
	List   *struct{} `json:"list,omitempty"`
	Resume *struct{} `json:"resume,omitempty"`
}

// AgentCapabilities is the agentCapabilities object from initialize.
type AgentCapabilities struct {
	LoadSession         bool                 `json:"loadSession"`
	PromptCapabilities  PromptCapabilities   `json:"promptCapabilities"`
	SessionCapabilities *SessionCapabilities `json:"sessionCapabilities,omitempty"`
}

// List reports whether the agent advertised session/list.
func (c AgentCapabilities) List() bool {
	return c.SessionCapabilities != nil && c.SessionCapabilities.List != nil
}

// Resume reports whether the agent advertised session/resume.
func (c AgentCapabilities) Resume() bool {
	return c.SessionCapabilities != nil && c.SessionCapabilities.Resume != nil
}

// AgentInfo is the agentInfo object from initialize.
type AgentInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Mode is one entry of availableModes.
type Mode struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ConfigOptionValue is one choice inside a session config option.
type ConfigOptionValue struct {
	Value       string `json:"value"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ConfigOption is one configOptions entry from session/new, load, or resume.
type ConfigOption struct {
	ID           string              `json:"id"`
	Name         string              `json:"name"`
	Description  string              `json:"description,omitempty"`
	Category     string              `json:"category,omitempty"`
	Type         string              `json:"type"`
	CurrentValue string              `json:"currentValue"`
	Options      []ConfigOptionValue `json:"options"`
}

// SessionOffer is what one session declared: modes and config options.
type SessionOffer struct {
	CurrentModeID string
	Modes         []Mode
	ConfigOptions []ConfigOption
}

// InitializeSnapshot is the initialize result the client kept.
type InitializeSnapshot struct {
	ProtocolVersion int
	AgentInfo       AgentInfo
	Capabilities    AgentCapabilities
}

type agentState struct {
	mu       sync.Mutex
	seenInit bool
	snap     InitializeSnapshot
	sessions map[string]SessionOffer
	seenMode map[string]bool
	modes    []Mode
}

func (c *Client) noteResult(method string, params any, raw json.RawMessage) {
	switch method {
	case "initialize":
		var res struct {
			ProtocolVersion   int               `json:"protocolVersion"`
			AgentCapabilities AgentCapabilities `json:"agentCapabilities"`
			AgentInfo         *AgentInfo        `json:"agentInfo"`
		}
		if len(raw) > 0 && json.Unmarshal(raw, &res) != nil {
			return
		}
		snap := InitializeSnapshot{
			ProtocolVersion: res.ProtocolVersion,
			Capabilities:    res.AgentCapabilities,
		}
		if res.AgentInfo != nil {
			snap.AgentInfo = *res.AgentInfo
		}
		c.state.mu.Lock()
		c.state.seenInit = true
		c.state.snap = snap
		c.state.mu.Unlock()
	case "session/new", "session/load", "session/resume":
		var res struct {
			SessionID string `json:"sessionId"`
			Modes     *struct {
				CurrentModeID  string `json:"currentModeId"`
				AvailableModes []Mode `json:"availableModes"`
			} `json:"modes"`
			ConfigOptions []ConfigOption `json:"configOptions"`
		}
		if len(raw) > 0 && json.Unmarshal(raw, &res) != nil {
			return
		}
		id := res.SessionID
		if id == "" {
			id = sessionIDParam(params)
		}
		if id == "" {
			return
		}
		offer := SessionOffer{ConfigOptions: append([]ConfigOption(nil), res.ConfigOptions...)}
		if res.Modes != nil {
			offer.CurrentModeID = res.Modes.CurrentModeID
			offer.Modes = append([]Mode(nil), res.Modes.AvailableModes...)
		}
		c.state.mu.Lock()
		if c.state.sessions == nil {
			c.state.sessions = map[string]SessionOffer{}
		}
		c.state.sessions[id] = offer
		for _, m := range offer.Modes {
			if m.ID == "" || c.state.seenMode[m.ID] {
				continue
			}
			if c.state.seenMode == nil {
				c.state.seenMode = map[string]bool{}
			}
			c.state.seenMode[m.ID] = true
			c.state.modes = append(c.state.modes, m)
		}
		c.state.mu.Unlock()
	}
}

// DeclaredModes returns mode ids the agent announced, in first-seen order.
// It does not include a session id.
func (c *Client) DeclaredModes() []Mode {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	return append([]Mode(nil), c.state.modes...)
}

func sessionIDParam(params any) string {
	if params == nil {
		return ""
	}
	b, err := json.Marshal(params)
	if err != nil {
		return ""
	}
	var p struct {
		SessionID string `json:"sessionId"`
	}
	if json.Unmarshal(b, &p) != nil {
		return ""
	}
	return p.SessionID
}

// InitializeSnapshot returns what the agent declared in initialize.
// The bool is false until a successful initialize response has been read.
func (c *Client) InitializeSnapshot() (InitializeSnapshot, bool) {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	if !c.state.seenInit {
		return InitializeSnapshot{}, false
	}
	return c.state.snap, true
}

// SessionOffer returns modes and config options stored for id.
func (c *Client) SessionOffer(id string) (SessionOffer, bool) {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	offer, ok := c.state.sessions[id]
	if !ok {
		return SessionOffer{}, false
	}
	offer.Modes = append([]Mode(nil), offer.Modes...)
	offer.ConfigOptions = append([]ConfigOption(nil), offer.ConfigOptions...)
	return offer, true
}
