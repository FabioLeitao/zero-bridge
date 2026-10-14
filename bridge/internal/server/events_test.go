package server

import (
	"bytes"
	"log"
	"strings"
	"sync"
	"testing"
)

func TestConcurrentEmit(t *testing.T) {
	h := newSessionHub(100, 1)
	_, ch, _ := h.subscribe()
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			h.emit(map[string]any{"type": "session/update", "n": n})
		}(i)
	}
	wg.Wait()
	n := 0
	for {
		select {
		case <-ch:
			n++
		default:
			if n == 0 {
				t.Fatal("no events delivered")
			}
			return
		}
	}
}

func TestEmitMarshalErrorDropsWithoutPayload(t *testing.T) {
	var buf bytes.Buffer
	h := newSessionHub(10, 4)
	h.logger = log.New(&buf, "", 0)
	_, ch, _ := h.subscribe()
	h.emit(map[string]any{"secret": "sk-SUPERSECRET", "bad": make(chan int)})
	frame := string(<-ch)
	if !strings.Contains(frame, `"type":"error"`) || !strings.Contains(frame, "event dropped") {
		t.Fatalf("frame %s", frame)
	}
	if strings.Contains(frame, "sk-SUPERSECRET") || strings.Contains(buf.String(), "sk-SUPERSECRET") {
		t.Fatalf("payload leaked log=%q frame=%s", buf.String(), frame)
	}
	if !strings.Contains(buf.String(), "event dropped") {
		t.Fatalf("log %q", buf.String())
	}
}

func TestDeliverDropsNewest(t *testing.T) {
	sub := &subscriber{ch: make(chan []byte, 1)}
	frame := []byte("data: {}\n\n")
	sub.deliver(frame)
	sub.deliver(frame)
	if sub.pendingDrop != 1 {
		t.Fatalf("pendingDrop %d", sub.pendingDrop)
	}
}
