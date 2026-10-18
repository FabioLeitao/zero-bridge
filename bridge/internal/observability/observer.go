package observability

// Observer is the minimal interface the bridge core uses for telemetry.
// The concrete implementation lives in bridge/obs (separate go.mod with OTel SDK).
// The core only depends on this interface; the zero implementation is a no-op.

import "context"

type Observer interface {
	// StartSpan starts a span for the given operation.
	// Returns a context with the span and an end function.
	StartSpan(ctx context.Context, name string, opts ...SpanOption) (context.Context, func())

	// RecordMetric records a metric value.
	RecordMetric(name string, value float64, opts ...MetricOption)

	// Log logs a structured message at the given level.
	// The observer must attach trace_id/span_id from context if present.
	Log(ctx context.Context, level string, msg string, args ...any)

	// Shutdown flushes and shuts down the observer.
	Shutdown(context.Context) error
}

// SpanOption configures a span.
type SpanOption func(*SpanConfig)

type SpanConfig struct {
	Attributes map[string]string
	Kind       SpanKind
}

type SpanKind int

const (
	SpanKindInternal SpanKind = iota
	SpanKindServer
	SpanKindClient
)

// WithAttributes adds attributes to the span.
func WithAttributes(attrs map[string]string) SpanOption {
	return func(c *SpanConfig) {
		if c.Attributes == nil {
			c.Attributes = make(map[string]string)
		}
		for k, v := range attrs {
			c.Attributes[k] = v
		}
	}
}

// WithKind sets the span kind.
func WithKind(kind SpanKind) SpanOption {
	return func(c *SpanConfig) {
		c.Kind = kind
	}
}

// MetricOption configures a metric.
type MetricOption func(*MetricConfig)

type MetricConfig struct {
	Attributes map[string]string
}

// WithMetricAttributes adds attributes to the metric.
func WithMetricAttributes(attrs map[string]string) MetricOption {
	return func(c *MetricConfig) {
		if c.Attributes == nil {
			c.Attributes = make(map[string]string)
		}
		for k, v := range attrs {
			c.Attributes[k] = v
		}
	}
}

// NoopObserver is the default no-op implementation used when telemetry is disabled.
type NoopObserver struct{}

func (NoopObserver) StartSpan(ctx context.Context, name string, opts ...SpanOption) (context.Context, func()) {
	return ctx, func() {}
}

func (NoopObserver) RecordMetric(name string, value float64, opts ...MetricOption) {}

func (NoopObserver) Log(ctx context.Context, level string, msg string, args ...any) {}

func (NoopObserver) Shutdown(context.Context) error { return nil }

// Global observer instance. Set by main() when telemetry is configured.
var Global Observer = NoopObserver{}

// SetGlobal sets the global observer. Not thread-safe; call once at startup.
func SetGlobal(o Observer) {
	if o != nil {
		Global = o
	} else {
		Global = NoopObserver{}
	}
}

// Helper functions for common operations.

// StartHTTPSpan starts a span for an HTTP request.
func StartHTTPSpan(ctx context.Context, route, method string, opts ...SpanOption) (context.Context, func()) {
	allOpts := append([]SpanOption{WithAttributes(map[string]string{
		"http.route":  route,
		"http.method": method,
	})}, opts...)
	return Global.StartSpan(ctx, "HTTP "+method+" "+route, allOpts...)
}

// StartACPSpan starts a span for an ACP call.
func StartACPSpan(ctx context.Context, method string, opts ...SpanOption) (context.Context, func()) {
	return Global.StartSpan(ctx, "ACP "+method, append([]SpanOption{
		WithKind(SpanKindClient),
		WithAttributes(map[string]string{"acp.method": method}),
	}, opts...)...)
}

// RecordHTTPMetric records an HTTP request metric.
func RecordHTTPMetric(route, method string, status int, duration float64) {
	Global.RecordMetric("http.request.duration_ms", duration,
		WithMetricAttributes(map[string]string{
			"http.route":  route,
			"http.method": method,
			"http.status": string(rune(status)),
		}))
	Global.RecordMetric("http.request.count", 1,
		WithMetricAttributes(map[string]string{
			"http.route":  route,
			"http.method": method,
			"http.status": string(rune(status)),
		}))
}

// RecordACPMetric records an ACP call metric.
func RecordACPMetric(method string, duration float64, err error) {
	attrs := map[string]string{"acp.method": method}
	if err != nil {
		attrs["error"] = "true"
	}
	Global.RecordMetric("acp.call.duration_ms", duration, WithMetricAttributes(attrs))
	Global.RecordMetric("acp.call.count", 1, WithMetricAttributes(attrs))
}

// LogWithContext logs with trace context.
func LogWithContext(ctx context.Context, level, msg string, args ...any) {
	Global.Log(ctx, level, msg, args...)
}
