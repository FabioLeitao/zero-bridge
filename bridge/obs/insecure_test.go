package main

import "testing"

func TestOTLPInsecureIsOptIn(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "")
	if len(traceOptions("127.0.0.1:4318")) != 1 || len(metricOptions("127.0.0.1:4318")) != 1 {
		t.Fatal("plaintext OTLP is on without the opt-in")
	}
	t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "true")
	if len(traceOptions("127.0.0.1:4318")) != 2 || len(metricOptions("127.0.0.1:4318")) != 2 {
		t.Fatal("opt-in did not add the insecure option")
	}
}
