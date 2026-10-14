// Package permissions queues human decisions for ACP session/request_permission.
package permissions

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"
)

const (
	queueLimit   = 8
	historyLimit = 256
)

// Clock is injectable so tests can fire timeouts without sleeping.
type Clock interface {
	Now() time.Time
	After(d time.Duration) <-chan time.Time
}

type realClock struct{}

func (realClock) Now() time.Time                         { return time.Now() }
func (realClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

// Event is emitted when a permission is exposed or resolved.
type Event struct {
	Type      string
	SessionID string
	ReqID     string
	Decision  string
	ToolCall  json.RawMessage
	Decisions []string
	ExpiresAt time.Time
}

// Broker tracks at most one exposed permission per session.
type Broker struct {
	Timeout time.Duration
	Clock   Clock
	OnEvent func(Event)

	mu       sync.Mutex
	sessions map[string]*sessionState
	stopped  bool
}

type sessionState struct {
	exposed *item
	queue   []*item
	hist    []histEntry
}

type histEntry struct {
	id       string
	decision string
}

type item struct {
	id        string
	sessionID string
	tool      json.RawMessage
	options   []option
	expires   time.Time
	done      chan struct{}
	decision  string
	res       any
	resMu     sync.Mutex
	once      sync.Once
}

type option struct {
	OptionID string `json:"optionId"`
	Kind     string `json:"kind"`
}

// Pending is the single exposed permission, if any.
type Pending struct {
	ReqID     string
	ToolCall  json.RawMessage
	Decisions []string
	ExpiresAt time.Time
	Queued    int
}

func New(timeout time.Duration) *Broker {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &Broker{
		Timeout:  timeout,
		Clock:    realClock{},
		sessions: make(map[string]*sessionState),
	}
}

func (b *Broker) clock() Clock {
	if b.Clock == nil {
		return realClock{}
	}
	return b.Clock
}

// Wait blocks until the permission is decided. The returned value is the ACP result.
func (b *Broker) Wait(sessionID string, params json.RawMessage) any {
	it := b.enqueue(sessionID, params)
	timer := b.clock().After(b.Timeout)
	select {
	case <-it.done:
	case <-timer:
		b.finish(it, "expired")
	}
	return it.getResult()
}

func (it *item) getResult() any {
	<-it.done
	it.resMu.Lock()
	defer it.resMu.Unlock()
	return it.res
}

func (b *Broker) enqueue(sessionID string, params json.RawMessage) *item {
	var parsed struct {
		ToolCall json.RawMessage `json:"toolCall"`
		Options  []option        `json:"options"`
	}
	_ = json.Unmarshal(params, &parsed)
	it := &item{
		id:        newID(),
		sessionID: sessionID,
		tool:      parsed.ToolCall,
		options:   parsed.Options,
		done:      make(chan struct{}),
		expires:   b.clock().Now().Add(b.Timeout),
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.stopped {
		b.finishLocked(it, "aborted")
		return it
	}
	st := b.session(sessionID)
	if st.exposed != nil && len(st.queue) >= queueLimit {
		b.finishLocked(it, "deny")
		return it
	}
	if st.exposed == nil {
		st.exposed = it
		b.emitLocked(Event{
			Type: "permission_request", SessionID: sessionID, ReqID: it.id,
			ToolCall: it.tool, Decisions: []string{"allow", "deny"}, ExpiresAt: it.expires,
		})
		return it
	}
	st.queue = append(st.queue, it)
	return it
}

// Resolve applies allow or deny to the exposed request.
func (b *Broker) Resolve(sessionID, reqID, decision string) string {
	if decision != "allow" && decision != "deny" {
		return "bad"
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	st := b.sessions[sessionID]
	if st == nil {
		return "unknown"
	}
	if d, ok := st.findHist(reqID); ok {
		return "resolved:" + d
	}
	if st.exposed == nil || st.exposed.id != reqID {
		return "unknown"
	}
	b.finishLocked(st.exposed, decision)
	b.promoteLocked(st)
	return "ok"
}

// Pending returns the exposed permission. Queued ids stay hidden.
func (b *Broker) Pending(sessionID string) Pending {
	b.mu.Lock()
	defer b.mu.Unlock()
	st := b.sessions[sessionID]
	if st == nil || st.exposed == nil {
		q := 0
		if st != nil {
			q = len(st.queue)
		}
		return Pending{Queued: q}
	}
	return Pending{
		ReqID:     st.exposed.id,
		ToolCall:  st.exposed.tool,
		Decisions: []string{"allow", "deny"},
		ExpiresAt: st.exposed.expires,
		Queued:    len(st.queue),
	}
}

// AbortSession denies every open permission for the session.
func (b *Broker) AbortSession(sessionID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	st := b.sessions[sessionID]
	if st == nil {
		return
	}
	if st.exposed != nil {
		b.finishLocked(st.exposed, "aborted")
		st.exposed = nil
	}
	for _, it := range st.queue {
		b.finishLocked(it, "aborted")
	}
	st.queue = nil
}

// Stop aborts every session. Used on shutdown.
func (b *Broker) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.stopped = true
	for _, st := range b.sessions {
		if st.exposed != nil {
			b.finishLocked(st.exposed, "aborted")
			st.exposed = nil
		}
		for _, it := range st.queue {
			b.finishLocked(it, "aborted")
		}
		st.queue = nil
	}
}

func (b *Broker) session(id string) *sessionState {
	st := b.sessions[id]
	if st == nil {
		st = &sessionState{}
		b.sessions[id] = st
	}
	return st
}

func (b *Broker) finish(it *item, decision string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	st := b.sessions[it.sessionID]
	if st != nil && st.exposed == it {
		b.finishLocked(it, decision)
		b.promoteLocked(st)
		return
	}
	b.finishLocked(it, decision)
}

func (b *Broker) finishLocked(it *item, decision string) {
	it.once.Do(func() {
		it.resMu.Lock()
		it.decision = decision
		it.res = acpResult(decision, it.options)
		it.resMu.Unlock()
		close(it.done)
		if st := b.sessions[it.sessionID]; st != nil {
			st.pushHist(it.id, decision)
		}
		b.emitLocked(Event{
			Type: "permission_resolved", SessionID: it.sessionID, ReqID: it.id, Decision: decision,
		})
	})
}

func (b *Broker) promoteLocked(st *sessionState) {
	st.exposed = nil
	if len(st.queue) == 0 || b.stopped {
		return
	}
	next := st.queue[0]
	st.queue = st.queue[1:]
	next.expires = b.clock().Now().Add(b.Timeout)
	st.exposed = next
	b.emitLocked(Event{
		Type: "permission_request", SessionID: next.sessionID, ReqID: next.id,
		ToolCall: next.tool, Decisions: []string{"allow", "deny"}, ExpiresAt: next.expires,
	})
}

func (b *Broker) emitLocked(ev Event) {
	if b.OnEvent != nil {
		b.OnEvent(ev)
	}
}

func (st *sessionState) findHist(id string) (string, bool) {
	for _, h := range st.hist {
		if h.id == id {
			return h.decision, true
		}
	}
	return "", false
}

func (st *sessionState) pushHist(id, decision string) {
	st.hist = append(st.hist, histEntry{id: id, decision: decision})
	if len(st.hist) > historyLimit {
		st.hist = st.hist[len(st.hist)-historyLimit:]
	}
}

func acpResult(decision string, options []option) any {
	if decision == "allow" {
		for _, opt := range options {
			if opt.Kind == "allow_once" && opt.OptionID == "allow" {
				return selected(opt.OptionID)
			}
		}
		decision = "deny"
	}
	if decision == "deny" || decision == "expired" || decision == "aborted" {
		for _, opt := range options {
			if opt.Kind == "reject_once" && opt.OptionID != "" {
				return selected(opt.OptionID)
			}
		}
	}
	return map[string]any{"outcome": map[string]string{"outcome": "cancelled"}}
}

func selected(id string) any {
	return map[string]any{"outcome": map[string]string{"outcome": "selected", "optionId": id}}
}

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
