package permissions

import (
	"encoding/json"
	"sync"
	"testing"
	"time"
)

type waiter struct {
	ch chan time.Time
	at time.Time
}

type fakeClock struct {
	mu      sync.Mutex
	now     time.Time
	waiters []waiter
}

func newClock() *fakeClock { return &fakeClock{now: time.Unix(0, 0)} }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) After(d time.Duration) <-chan time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	ch := make(chan time.Time, 1)
	c.waiters = append(c.waiters, waiter{ch: ch, at: c.now.Add(d)})
	return ch
}

func (c *fakeClock) Add(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
}

func (c *fakeClock) FireDue() {
	c.mu.Lock()
	now := c.now
	var due []chan time.Time
	keep := make([]waiter, 0, len(c.waiters))
	for _, w := range c.waiters {
		if !w.at.After(now) {
			due = append(due, w.ch)
			continue
		}
		keep = append(keep, w)
	}
	c.waiters = keep
	c.mu.Unlock()
	for _, ch := range due {
		ch <- now
	}
}

func (c *fakeClock) Advance() {
	c.Add(time.Minute)
	c.FireDue()
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
	if got := b2.Resolve("s", p.ReqID, "allow"); got != "ok:deny" {
		t.Fatalf("resolve2 %s", got)
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
	// The first Wait has to be exposed before the queue can fill.
	// Pending is the signal; a fixed sleep races the goroutines.
	waitReq(t, b)
	var extra []chan any
	for i := 0; i < queueLimit; i++ {
		ch := make(chan any, 1)
		extra = append(extra, ch)
		go func() { ch <- b.Wait("s", params(both)) }()
	}
	waitQueued(t, b, queueLimit)
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

func TestEffectiveDecisionIsDeny(t *testing.T) {
	clk := newClock()
	b := New(time.Second)
	b.Clock = clk
	var mu sync.Mutex
	var evs []Event
	b.OnEvent = func(ev Event) {
		mu.Lock()
		evs = append(evs, ev)
		mu.Unlock()
	}
	done := make(chan any, 1)
	go func() { done <- b.Wait("s", params(alwaysOnly)) }()
	id := waitReq(t, b)
	if got := b.Resolve("s", id, "allow"); got != "ok:deny" {
		t.Fatalf("resolve %s", got)
	}
	if optionID(<-done) != "deny" {
		t.Fatal("agent did not get deny")
	}
	deadline := time.Now().Add(time.Second)
	for {
		mu.Lock()
		var decision string
		for _, ev := range evs {
			if ev.Type == "permission_resolved" && ev.ReqID == id {
				decision = ev.Decision
			}
		}
		mu.Unlock()
		if decision == "deny" {
			break
		}
		if decision == "allow" || time.Now().After(deadline) {
			t.Fatalf("event decision %q", decision)
		}
		time.Sleep(time.Millisecond)
	}
	if got := b.Resolve("s", id, "allow"); got != "resolved:deny" {
		t.Fatalf("history %s", got)
	}
}

func TestPromotedDeadlineStartsAtExposure(t *testing.T) {
	clk := newClock()
	timeout := 300 * time.Millisecond
	b := New(timeout)
	b.Clock = clk
	go func() { _ = b.Wait("s", params(both)) }()
	a := waitReq(t, b)
	clk.Add(20 * time.Millisecond)
	second := make(chan any, 1)
	go func() { second <- b.Wait("s", params(both)) }()
	waitQueued(t, b, 1)
	clk.Add(130 * time.Millisecond)
	if b.Resolve("s", a, "allow") != "ok" {
		t.Fatal("resolve A")
	}
	p := waitReqNot(t, b, a)
	if !p.ExpiresAt.Equal(clk.Now().Add(timeout)) {
		t.Fatalf("expiresAt %s want %s", p.ExpiresAt, clk.Now().Add(timeout))
	}
	clk.Add(170 * time.Millisecond)
	clk.FireDue()
	time.Sleep(30 * time.Millisecond)
	if got := b.Pending("s").ReqID; got != p.ReqID {
		t.Fatalf("promoted expired on the enqueue clock: %s", got)
	}
	clk.Add(130 * time.Millisecond)
	clk.FireDue()
	select {
	case got := <-second:
		if optionID(got) != "deny" {
			t.Fatalf("option %s", optionID(got))
		}
	case <-time.After(time.Second):
		t.Fatal("promoted did not expire")
	}
	if b.Pending("s").ReqID != "" {
		t.Fatal("resolved item still exposed")
	}
}

func TestQueueExpiresInChain(t *testing.T) {
	clk := newClock()
	timeout := 300 * time.Millisecond
	b := New(timeout)
	b.Clock = clk
	const n = 3
	done := make(chan string, n)
	for i := 0; i < n; i++ {
		go func() { done <- optionID(b.Wait("s", params(both))) }()
	}
	seen := map[string]bool{}
	deadline := time.Now().Add(2 * time.Second)
	for len(seen) < n {
		if time.Now().After(deadline) {
			t.Fatal("chain stalled")
		}
		p := b.Pending("s")
		if p.ReqID == "" || seen[p.ReqID] {
			time.Sleep(time.Millisecond)
			continue
		}
		if !p.ExpiresAt.Equal(clk.Now().Add(timeout)) {
			t.Fatalf("expiresAt %s", p.ExpiresAt)
		}
		seen[p.ReqID] = true
		prev := p.ReqID
		clk.Add(timeout)
		clk.FireDue()
		for b.Pending("s").ReqID == prev {
			if time.Now().After(deadline) {
				t.Fatal("resolved item stayed exposed")
			}
			time.Sleep(time.Millisecond)
		}
	}
	if b.Pending("s").ReqID != "" || b.Pending("s").Queued != 0 {
		t.Fatalf("left %+v", b.Pending("s"))
	}
	for i := 0; i < n; i++ {
		select {
		case got := <-done:
			if got != "deny" {
				t.Fatalf("option %s", got)
			}
		case <-time.After(time.Second):
			t.Fatal("wait blocked")
		}
	}
}

func waitReq(t *testing.T, b *Broker) string {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if id := b.Pending("s").ReqID; id != "" {
			return id
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("no exposed permission")
	return ""
}

func waitReqNot(t *testing.T, b *Broker, prev string) Pending {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		p := b.Pending("s")
		if p.ReqID != "" && p.ReqID != prev {
			return p
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("promotion stalled")
	return Pending{}
}

func waitQueued(t *testing.T, b *Broker, n int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if b.Pending("s").Queued == n {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("queued %d", b.Pending("s").Queued)
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
