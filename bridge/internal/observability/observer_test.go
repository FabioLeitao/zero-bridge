package observability

import (
	"context"
	"errors"
	"testing"
)

type recordingObserver struct {
	spans   int
	metrics int
	logs    int
}

func (o *recordingObserver) StartSpan(ctx context.Context, name string, opts ...SpanOption) (context.Context, func()) {
	o.spans++
	cfg := &SpanConfig{}
	for _, opt := range opts {
		opt(cfg)
	}
	return ctx, func() { o.spans++ }
}

func (o *recordingObserver) RecordMetric(name string, value float64, opts ...MetricOption) {
	o.metrics++
	cfg := &MetricConfig{}
	for _, opt := range opts {
		opt(cfg)
	}
}

func (o *recordingObserver) Log(ctx context.Context, level string, msg string, args ...any) {
	o.logs++
}

func (o *recordingObserver) Shutdown(context.Context) error { return nil }

func TestHelpersReachTheGlobalObserver(t *testing.T) {
	SetGlobal(nil)
	if _, ok := Global.(NoopObserver); !ok {
		t.Fatalf("nil observer %#v", Global)
	}
	rec := &recordingObserver{}
	SetGlobal(rec)
	ctx, end := StartHTTPSpan(context.Background(), "/health", "GET", WithKind(SpanKindServer))
	end()
	StartACPSpan(ctx, "session/prompt")
	RecordHTTPMetric("/health", "GET", 200, 1)
	RecordACPMetric("session/prompt", 1, nil)
	RecordACPMetric("session/prompt", 1, errors.New("down"))
	LogWithContext(ctx, "info", "up")
	if err := Global.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if rec.spans < 2 || rec.metrics < 4 || rec.logs != 1 {
		t.Fatalf("observer %+v", rec)
	}
}
