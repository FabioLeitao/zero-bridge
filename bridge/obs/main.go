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
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

type otelObserver struct {
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
	meter          metric.Meter
}

func newOTLPObserver(endpoint, serviceName string, sampleRatio float64) (*otelObserver, error) {
	ctx := context.Background()

	// Trace exporter. WithInsecure is opt-in via OTEL_EXPORTER_OTLP_INSECURE.
	traceExp, err := otlptracehttp.New(ctx, traceOptions(endpoint)...)
	if err != nil {
		return nil, err
	}
	res, err := otelResource(ctx, serviceName)
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(sampleRatio)),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	// Metric exporter
	metricExp, err := otlpmetrichttp.New(ctx, metricOptions(endpoint)...)
	if err != nil {
		return nil, err
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp, sdkmetric.WithInterval(10*time.Second))),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	return &otelObserver{
		tracerProvider: tp,
		meterProvider:  mp,
		meter:          mp.Meter("zero-bridge"),
	}, nil
}

func traceOptions(endpoint string) []otlptracehttp.Option {
	opts := []otlptracehttp.Option{otlptracehttp.WithEndpoint(endpoint)}
	if observability.InsecureExportEnabled() {
		opts = append(opts, otlptracehttp.WithInsecure())
	}
	return opts
}

func metricOptions(endpoint string) []otlpmetrichttp.Option {
	opts := []otlpmetrichttp.Option{otlpmetrichttp.WithEndpoint(endpoint)}
	if observability.InsecureExportEnabled() {
		opts = append(opts, otlpmetrichttp.WithInsecure())
	}
	return opts
}

func otelResource(ctx context.Context, serviceName string) (*resource.Resource, error) {
	return resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", serviceName),
			attribute.String("service.version", "dev"),
		),
	)
}

func (o *otelObserver) StartSpan(ctx context.Context, name string, opts ...observability.SpanOption) (context.Context, func()) {
	cfg := &observability.SpanConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	var otelKind trace.SpanKind
	switch cfg.Kind {
	case observability.SpanKindServer:
		otelKind = trace.SpanKindServer
	case observability.SpanKindClient:
		otelKind = trace.SpanKindClient
	default:
		otelKind = trace.SpanKindInternal
	}

	attrs := make([]attribute.KeyValue, 0, len(cfg.Attributes))
	for k, v := range cfg.Attributes {
		attrs = append(attrs, attribute.String(k, v))
	}

	ctx, span := otel.Tracer("zero-bridge").Start(ctx, name,
		trace.WithSpanKind(otelKind),
		trace.WithAttributes(attrs...),
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

func (o *otelObserver) Log(context.Context, string, string, ...any) {}

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
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
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
