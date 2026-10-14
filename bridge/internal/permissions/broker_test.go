package permissions

import (
	"encoding/json"
	"sync"
	"testing"
	"time"
)

type fakeClock struct {
	mu      sync.Mutex
	now     time.Time
	waiters []chan time.Time
}

func newClock() *fakeClock { return &fakeClock{now: time.Unix(0, 0)} }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) After(time.Duration) <-chan time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	ch := make(chan time.Time, 1)
	c.waiters = append(c.waiters, ch)
	return ch
}

func (c *fakeClock) Advance() {
	c.mu.Lock()
	c.now = c.now.Add(time.Minute)
	ws := c.waiters
	c.waiters = nil
	c.mu.Unlock()
	for _, ch := range ws {
		ch <- c.now
	}
}

func params(options string) json.RawMessage {
	return json.RawMessage(`{"toolCall":{"title":"write","kind":"edit","status":"pending","rawInput":{"path":"a"}},"options":` + options + `}`)
}

const both = `[{"optionId":"allow","kind":"allow_once"},{"optionId":"deny","kind":"reject_once"},{"optionId":"always_allow","kind":"allow_always"}]`
const alwaysOnly = `[{"optionId":"always_allow","kind":"allow_always"},{"optionId":"deny","kind":"reject_once"}]`

func optionID(v any) string {
	b, _ := json.Marshal(v)
	var out struct {
		Outcome struct {
			Outcome  string `json:"outcome"`
			OptionID string `json:"optionId"`
		} `json:"outcome"`
	}
	_ = json.Unmarshal(b, &out)
	if out.Outcome.Outcome == "cancelled" {
		return "cancelled"
	}
	return out.Outcome.OptionID
}

func TestAllowNeverWidens(t *testing.T) {
	b := New(time.Second)
	b.Clock = newClock()
	done := make(chan any, 1)
	go func() { done <- b.Wait("s", params(both)) }()
	time.Sleep(20 * time.Millisecond)
	p := b.Pending("s")
	if b.Resolve("s", p.ReqID, "allow") != "ok" {
		t.Fatal("resolve")
	}
	if got := optionID(<-done); got != "allow" {
		t.Fatalf("option %s", got)
	}
	b2 := New(time.Second)
	b2.Clock = newClock()
	done2 := make(chan any, 1)
	go func() { done2 <- b2.Wait("s", params(alwaysOnly)) }()
	time.Sleep(20 * time.Millisecond)
	p = b2.Pending("s")
	if b2.Resolve("s", p.ReqID, "allow") != "ok" {
		t.Fatal("resolve2")
	}
	if got := optionID(<-done2); got != "deny" {
		t.Fatalf("widened to %s", got)
	}
}

func TestDoubleResolve(t *testing.T) {
	b := New(time.Second)
	b.Clock = newClock()
	go func() { _ = b.Wait("s", params(both)) }()
	time.Sleep(20 * time.Millisecond)
	id := b.Pending("s").ReqID
	if b.Resolve("s", id, "deny") != "ok" {
		t.Fatal("first")
	}
	if got := b.Resolve("s", id, "deny"); got != "resolved:deny" {
		t.Fatalf("second %s", got)
	}
	if b.Resolve("s", "nope", "deny") != "unknown" {
		t.Fatal("unknown")
	}
}

func TestTimeout(t *testing.T) {
	clk := newClock()
	b := New(time.Second)
	b.Clock = clk
	done := make(chan any, 1)
	go func() { done <- b.Wait("s", params(both)) }()
	time.Sleep(20 * time.Millisecond)
	clk.Advance()
	if got := optionID(<-done); got != "deny" {
		t.Fatalf("option %s", got)
	}
	id := b.sessions["s"].hist[0].id
	if b.Resolve("s", id, "allow") != "resolved:expired" {
		t.Fatal("late resolve")
	}
}

func TestQueueOneExposed(t *testing.T) {
	b := New(time.Second)
	b.Clock = newClock()
	go func() { _ = b.Wait("s", params(both)) }()
	time.Sleep(20 * time.Millisecond)
	first := b.Pending("s").ReqID
	secondDone := make(chan any, 1)
	go func() { secondDone <- b.Wait("s", params(both)) }()
	time.Sleep(20 * time.Millisecond)
	if b.Pending("s").ReqID != first || b.Pending("s").Queued != 1 {
		t.Fatalf("pending %+v", b.Pending("s"))
	}
	if b.Resolve("other", first, "deny") != "unknown" {
		t.Fatal("other session")
	}
	_ = b.Resolve("s", first, "deny")
	time.Sleep(20 * time.Millisecond)
	second := b.Pending("s").ReqID
	if second == "" || second == first {
		t.Fatal("not promoted")
	}
	_ = b.Resolve("s", second, "deny")
	<-secondDone
}

func TestQueueOverflow(t *testing.T) {
	b := New(time.Second)
	b.Clock = newClock()
	go func() { _ = b.Wait("s", params(both)) }()
	time.Sleep(10 * time.Millisecond)
	var extra []chan any
	for i := 0; i < queueLimit; i++ {
		ch := make(chan any, 1)
		extra = append(extra, ch)
		go func() { ch <- b.Wait("s", params(both)) }()
	}
	time.Sleep(30 * time.Millisecond)
	overflow := make(chan any, 1)
	go func() { overflow <- b.Wait("s", params(both)) }()
	select {
	case got := <-overflow:
		if optionID(got) != "deny" {
			t.Fatalf("overflow %s", optionID(got))
		}
	case <-time.After(time.Second):
		t.Fatal("overflow blocked")
	}
	_ = extra
}

func TestStopAborts(t *testing.T) {
	b := New(time.Second)
	b.Clock = newClock()
	done := make(chan any, 1)
	go func() { done <- b.Wait("s", params(both)) }()
	time.Sleep(20 * time.Millisecond)
	b.Stop()
	if got := optionID(<-done); got != "deny" {
		t.Fatalf("option %s", got)
	}
	if b.sessions["s"].hist[0].decision != "aborted" {
		t.Fatal(b.sessions["s"].hist[0].decision)
	}
}

func TestResolveRace(t *testing.T) {
	for n := 0; n < 100; n++ {
		b := New(time.Second)
		b.Clock = newClock()
		go func() { _ = b.Wait("s", params(both)) }()
		var id string
		for id == "" {
			id = b.Pending("s").ReqID
		}
		var wg sync.WaitGroup
		var oks atomicInt
		wg.Add(8)
		for i := 0; i < 8; i++ {
			go func() {
				defer wg.Done()
				if b.Resolve("s", id, "allow") == "ok" {
					oks.add()
				}
			}()
		}
		wg.Wait()
		if oks.n != 1 {
			t.Fatalf("winners %d", oks.n)
		}
	}
}

type atomicInt struct {
	mu sync.Mutex
	n  int
}

func (a *atomicInt) add() {
	a.mu.Lock()
	a.n++
	a.mu.Unlock()
}
