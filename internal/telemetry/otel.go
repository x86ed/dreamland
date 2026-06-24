package telemetry

import (
	"context"
	"os"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// NewTracerProvider initializes an OTLP gRPC exporter when OTEL_EXPORTER_OTLP_ENDPOINT
// is set, otherwise falls back to a stdout JSON exporter.
func NewTracerProvider(ctx context.Context) (*sdktrace.TracerProvider, error) {
	var exporter sdktrace.SpanExporter
	var err error

	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" {
		exporter, err = otlptracegrpc.New(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		exporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, err
		}
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
	)
	return tp, nil
}
