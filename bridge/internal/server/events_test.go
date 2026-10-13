package server

import "testing"

func TestDeliverDropsNewest(t *testing.T) {
	sub := &subscriber{ch: make(chan []byte, 1)}
	frame := []byte("data: {}\n\n")
	sub.deliver(frame)
	sub.deliver(frame)
	if sub.pendingDrop != 1 {
		t.Fatalf("pendingDrop %d", sub.pendingDrop)
	}
}
