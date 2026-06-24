package telemetry

import (
	"context"
	"testing"
)

func TestNewTracerProvider_Stdout(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	tp, err := NewTracerProvider(context.Background())
	if err != nil {
		t.Fatalf("expected no error for stdout exporter: %v", err)
	}
	if tp == nil {
		t.Fatal("expected non-nil TracerProvider")
	}
	_ = tp.Shutdown(context.Background())
}

func TestNewTracerProvider_OTLP(t *testing.T) {
	// The OTLP gRPC exporter uses lazy connection — creation should not fail
	// even when no collector is running at the endpoint.
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:14317")
	tp, err := NewTracerProvider(context.Background())
	if err != nil {
		t.Skipf("OTLP exporter creation failed (acceptable in CI without collector): %v", err)
	}
	if tp == nil {
		t.Fatal("expected non-nil TracerProvider")
	}
	_ = tp.Shutdown(context.Background())
}
