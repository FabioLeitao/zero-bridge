package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"zero-bridge/bridge/internal/observability"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/metric"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/semconv/v1.26.0"
)

type otelObserver struct {
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
	loggerProvider *sdklog.LoggerProvider
	meter          metric.Meter
}

func newOTLPObserver(endpoint, serviceName string, sampleRatio float64) (*otelObserver, error) {
	ctx := context.Background()

	// Trace exporter
	traceExp, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(sampleRatio)),
		sdktrace.WithResource(resource(serviceName)),
	)
	otel.SetTracerProvider(tp)

	// Metric exporter
	metricExp, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithEndpoint(endpoint),
		otlpmetrichttp.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp, sdkmetric.WithInterval(10*time.Second))),
		sdkmetric.WithResource(resource(serviceName)),
	)
	otel.SetMeterProvider(mp)

	// Log exporter
	logExp, err := otlploghttp.New(ctx,
		otlploghttp.WithEndpoint(endpoint),
		otlploghttp.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}
	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExp)),
		sdklog.WithResource(resource(serviceName)),
	)
	global.SetLoggerProvider(lp)

	return &otelObserver{
		tracerProvider: tp,
		meterProvider:  mp,
		loggerProvider: lp,
		meter:          mp.Meter("zero-bridge"),
	}, nil
}

func resource(serviceName string) *sdktrace.Resource {
	return sdktrace.NewResource(
		attribute.String("service.name", serviceName),
		attribute.String("service.version", "dev"),
	)
}

func (o *otelObserver) StartSpan(ctx context.Context, name string, opts ...observability.SpanOption) (context.Context, func()) {
	cfg := &observability.SpanConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	var otelKind sdktrace.SpanKind
	switch cfg.Kind {
	case observability.SpanKindServer:
		otelKind = sdktrace.SpanKindServer
	case observability.SpanKindClient:
		otelKind = sdktrace.SpanKindClient
	default:
		otelKind = sdktrace.SpanKindInternal
	}

	attrs := make([]attribute.KeyValue, 0, len(cfg.Attributes))
	for k, v := range cfg.Attributes {
		attrs = append(attrs, attribute.String(k, v))
	}

	ctx, span := otel.Tracer("zero-bridge").Start(ctx, name,
		sdktrace.WithSpanKind(otelKind),
		sdktrace.WithAttributes(attrs...),
	)
	return ctx, func() { span.End() }
}

func (o *otelObserver) RecordMetric(name string, value float64, opts ...observability.MetricOption) {
	cfg := &observability.MetricConfig{}
	for _, opt := range opts {
		opt(cfg)
	}
	attrs := make([]attribute.KeyValue, 0, len(cfg.Attributes))
	for k, v := range cfg.Attributes {
		attrs = append(attrs, attribute.String(k, v))
	}
	// Use a simple counter/histogram based on name
	counter, err := o.meter.Int64Counter(name + ".count")
	if err == nil {
		counter.Add(context.Background(), 1, metric.WithAttributes(attrs...))
	}
}

func (o *otelObserver) Log(ctx context.Context, level, msg string, args ...any) {
	logger := global.Logger("zero-bridge")
	var r sdklog.Record
	r.SetTimestamp(time.Now())
	r.SetSeverity(parseSeverity(level))
	r.SetBody(sdklog.StringValue(msg))
	for i := 0; i < len(args); i += 2 {
		if i+1 < len(args) {
			if key, ok := args[i].(string); ok {
				r.AddAttributes(attribute.String(key, toString(args[i+1])))
			}
		}
	}
	logger.Emit(ctx, r)
}

func (o *otelObserver) Shutdown(ctx context.Context) error {
	var errs []error
	if o.tracerProvider != nil {
		if err := o.tracerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if o.meterProvider != nil {
		if err := o.meterProvider.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if o.loggerProvider != nil {
		if err := o.loggerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

func parseSeverity(level string) sdklog.Severity {
	switch level {
	case "debug":
		return sdklog.SeverityDebug
	case "info":
		return sdklog.SeverityInfo
	case "warn":
		return sdklog.SeverityWarn
	case "error":
		return sdklog.SeverityError
	default:
		return sdklog.SeverityInfo
	}
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func main() {
	endpoint := os.Getenv("OTLP_ENDPOINT")
	if endpoint == "" {
		log.Println("OTLP_ENDPOINT not set; telemetry disabled")
		observability.SetGlobal(observability.NoopObserver{})
	} else {
		serviceName := os.Getenv("OTEL_SERVICE_NAME")
		if serviceName == "" {
			serviceName = "zero-bridge"
		}
		sampleRatio := 0.1
		obs, err := newOTLPObserver(endpoint, serviceName, sampleRatio)
		if err != nil {
			log.Fatalf("failed to init OTel: %v", err)
		}
		observability.SetGlobal(obs)
		defer obs.Shutdown(context.Background())
		log.Printf("OTel initialized: endpoint=%s service=%s sample=%.2f", endpoint, serviceName, sampleRatio)
	}

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("Shutting down telemetry...")
}
