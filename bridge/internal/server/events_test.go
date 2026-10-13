package server

import (
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

func TestDeliverDropsNewest(t *testing.T) {
	sub := &subscriber{ch: make(chan []byte, 1)}
	frame := []byte("data: {}\n\n")
	sub.deliver(frame)
	sub.deliver(frame)
	if sub.pendingDrop != 1 {
		t.Fatalf("pendingDrop %d", sub.pendingDrop)
	}
}
