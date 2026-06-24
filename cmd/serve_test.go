package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"go.opentelemetry.io/otel"

	"dreamland/internal/config"
	"dreamland/internal/telemetry"
)

func init() {
	// Initialize mcpTracer to a no-op tracer so handler tests don't panic.
	mcpTracer = otel.Tracer("test")
}

func TestTelemetryWriteHandler_UnknownTool(t *testing.T) {
	telemetryGitRepo(t)
	_, out, err := telemetryWriteHandler(context.Background(), nil, telemetryWriteInput{
		Tool:    "unknown-tool",
		Payload: "{}",
	})
	if err != nil {
		t.Fatalf("handler should not error: %v", err)
	}
	if out.Written {
		t.Error("Written should be false for unknown tool")
	}
}

func TestTelemetryWriteHandler_KnownTool(t *testing.T) {
	telemetryGitRepo(t)
	_, out, err := telemetryWriteHandler(context.Background(), nil, telemetryWriteInput{
		Tool:    "claude-code",
		Payload: `{}`,
	})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !out.Written {
		t.Errorf("Written should be true for known tool, message: %q", out.Message)
	}
}

func TestTelemetryWriteHandler_NilCollectorResult(t *testing.T) {
	telemetryGitRepo(t)
	const nilTool = "nil-collector-test"
	orig := telemetry.Registry[nilTool]
	telemetry.Registry[nilTool] = &nilCollector{}
	t.Cleanup(func() {
		if orig == nil {
			delete(telemetry.Registry, nilTool)
		} else {
			telemetry.Registry[nilTool] = orig
		}
	})

	_, out, err := telemetryWriteHandler(context.Background(), nil, telemetryWriteInput{
		Tool:    nilTool,
		Payload: "{}",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Written {
		t.Error("Written should be false when collector returns nil")
	}
}

func TestHelloHandler_Direct(t *testing.T) {
	_, out, err := helloHandler(context.Background(), nil, helloInput{Name: "Dreamland"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Message != "Hello, Dreamland!" {
		t.Errorf("Message = %q", out.Message)
	}
}

func TestMakeSpanHandler_ExecutesInner(t *testing.T) {
	// Create a wrapped handler directly and invoke it to exercise the span logic.
	wrapped := makeSpanHandler("hello-test", helloHandler)
	_, out, err := wrapped(context.Background(), nil, helloInput{Name: "Span"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Message != "Hello, Span!" {
		t.Errorf("Message = %q", out.Message)
	}
}

func TestMakeSpanHandler_WithNonNilConfig(t *testing.T) {
	orig := currentConfig
	t.Cleanup(func() { currentConfig = orig })
	currentConfig = &config.Config{ModelID: "test-model", CodingTool: "claude-code"}

	wrapped := makeSpanHandler("hello-cfg", helloHandler)
	_, out, err := wrapped(context.Background(), nil, helloInput{Name: "Config"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Message != "Hello, Config!" {
		t.Errorf("Message = %q", out.Message)
	}
}

func TestTelemetryWriteHandler_CollectError(t *testing.T) {
	telemetryGitRepo(t)
	const errTool = "err-collector-test"
	orig := telemetry.Registry[errTool]
	telemetry.Registry[errTool] = &errCollector{}
	t.Cleanup(func() {
		if orig == nil {
			delete(telemetry.Registry, errTool)
		} else {
			telemetry.Registry[errTool] = orig
		}
	})

	_, out, err := telemetryWriteHandler(context.Background(), nil, telemetryWriteInput{
		Tool:    errTool,
		Payload: "{}",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Written {
		t.Error("Written should be false when collector errors")
	}
	if out.Message == "" {
		t.Error("Message should contain error text")
	}
}

func TestTelemetryWriteHandler_WithRepoRoot(t *testing.T) {
	root := telemetryGitRepo(t)
	orig := currentConfig
	t.Cleanup(func() { currentConfig = orig })
	currentConfig = &config.Config{ModelID: "test-model", RepoRoot: root}

	_, out, err := telemetryWriteHandler(context.Background(), nil, telemetryWriteInput{
		Tool:    "claude-code",
		Payload: `{}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Written {
		t.Errorf("Written should be true when cfg.RepoRoot is set, message: %q", out.Message)
	}
}

func TestTelemetryWriteHandler_WriteError(t *testing.T) {
	tmpDir := t.TempDir()
	fileAsDir := filepath.Join(tmpDir, "notadir")
	if err := os.WriteFile(fileAsDir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	orig := currentConfig
	t.Cleanup(func() { currentConfig = orig })
	currentConfig = &config.Config{RepoRoot: fileAsDir}

	_, out, err := telemetryWriteHandler(context.Background(), nil, telemetryWriteInput{
		Tool:    "claude-code",
		Payload: `{}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Written {
		t.Error("Written should be false when Write fails")
	}
}

// nilCollector implements telemetry.Collector and always returns nil, nil.
type nilCollector struct{}

func (n *nilCollector) Collect(_ io.Reader, _ *config.Config) (*telemetry.SnapshotResult, error) {
	return nil, nil
}

// errCollector implements telemetry.Collector and always returns an error.
type errCollector struct{}

func (e *errCollector) Collect(_ io.Reader, _ *config.Config) (*telemetry.SnapshotResult, error) {
	return nil, fmt.Errorf("forced collect error")
}
