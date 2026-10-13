package server

import (
	"encoding/json"
	"sync"
)

const defaultEventLogCap = 1000
const defaultSubQueue = 256

type sessionHub struct {
	mu         sync.Mutex
	seq        int64
	log        [][]byte
	logCap     int
	subQueue   int
	subs       map[int]*subscriber
	nextSub    int
	promptBusy bool
}

func newSessionHub(logCap, subQueue int) *sessionHub {
	if logCap <= 0 {
		logCap = defaultEventLogCap
	}
	if subQueue <= 0 {
		subQueue = defaultSubQueue
	}
	return &sessionHub{
		logCap:   logCap,
		subQueue: subQueue,
		subs:     make(map[int]*subscriber),
	}
}

type subscriber struct {
	mu          sync.Mutex
	ch          chan []byte
	pendingDrop int
}

func (h *sessionHub) tryStartPrompt() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.promptBusy {
		return false
	}
	h.promptBusy = true
	return true
}

func (h *sessionHub) endPrompt() {
	h.mu.Lock()
	h.promptBusy = false
	h.mu.Unlock()
}

func (h *sessionHub) logLen() int {
	h.mu.Lock()
	n := len(h.log)
	h.mu.Unlock()
	return n
}

func (h *sessionHub) subscribe() (int, <-chan []byte, [][]byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	replay := append([][]byte(nil), h.log...)
	id := h.nextSub
	h.nextSub++
	sub := &subscriber{ch: make(chan []byte, h.subQueue)}
	h.subs[id] = sub
	return id, sub.ch, replay
}

func (h *sessionHub) unsubscribe(id int) {
	h.mu.Lock()
	delete(h.subs, id)
	h.mu.Unlock()
}

func (h *sessionHub) emit(obj map[string]any) {
	h.mu.Lock()
	h.seq++
	obj["seq"] = h.seq
	b, _ := json.Marshal(obj)
	frame := append([]byte("data: "), b...)
	frame = append(frame, '\n', '\n')
	h.appendLog(frame)
	subs := make([]*subscriber, 0, len(h.subs))
	for _, s := range h.subs {
		subs = append(subs, s)
	}
	h.mu.Unlock()
	for _, s := range subs {
		s.deliver(frame)
	}
}

func (h *sessionHub) appendLog(frame []byte) {
	if len(h.log) < h.logCap {
		h.log = append(h.log, frame)
		return
	}
	if len(h.log) == h.logCap {
		h.log = h.log[1:]
	}
	h.log = append(h.log, frame)
}

func (s *subscriber) deliver(frame []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case s.ch <- frame:
		s.flushDroppedLocked()
	default:
		s.pendingDrop++
		s.flushDroppedLocked()
	}
}

func (s *subscriber) flushDroppedLocked() {
	for s.pendingDrop > 0 {
		msg := map[string]any{"type": "dropped", "count": s.pendingDrop}
		b, _ := json.Marshal(msg)
		frame := append([]byte("data: "), b...)
		frame = append(frame, '\n', '\n')
		select {
		case s.ch <- frame:
			s.pendingDrop = 0
		default:
			return
		}
	}
}
