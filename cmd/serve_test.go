package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"

	"dreamland/internal/config"
)

func init() {
	// Initialize mcpTracer to a no-op tracer so handler tests don't panic.
	mcpTracer = otel.Tracer("test")
}

// makeServeRepo creates a temp git repo with an optional .dreamland.json and stubs osGetwd.
func makeServeRepo(t *testing.T, cfg config.Config) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git", "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(cfg)
	if err := os.WriteFile(filepath.Join(root, ".dreamland.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	orig := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = orig })
	return root
}

func TestTransitionLogHandler_OK(t *testing.T) {
	makeServeRepo(t, config.Config{})

	_, out, err := transitionLogHandler(context.Background(), nil, transitionLogInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.OK {
		t.Error("OK should be true on success")
	}
}

func TestTransitionLogHandler_WritesLog(t *testing.T) {
	root := makeServeRepo(t, config.Config{})

	if _, _, err := transitionLogHandler(context.Background(), nil, transitionLogInput{}); err != nil {
		t.Fatalf("handler error: %v", err)
	}

	logPath := filepath.Join(root, ".dreamland", "transition.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("log not written: %v", err)
	}
	if !strings.Contains(string(data), "turn complete") {
		t.Errorf("unexpected log content: %s", data)
	}
}

func TestVersionBumpHandler_MultipleFlags(t *testing.T) {
	makeServeRepo(t, config.Config{})

	// major+minor together → validation error returned as MCP result
	_, out, err := versionBumpHandler(context.Background(), nil, versionBumpInput{Major: true, Minor: true})
	if err == nil {
		t.Fatal("expected error for conflicting flags")
	}
	if out.OK {
		t.Error("OK should be false on error")
	}
}

func TestVersionBumpHandler_PatchNoChanges(t *testing.T) {
	makeServeRepo(t, config.Config{})

	stubRunCmd(t, func(_ string, args ...string) (string, error) {
		if strings.Contains(strings.Join(args, " "), "describe") {
			return "v1.0.0\n", nil
		}
		if strings.Contains(strings.Join(args, " "), "diff") {
			return "", nil // no changes → silent no-op
		}
		return "", nil
	})

	_, out, err := versionBumpHandler(context.Background(), nil, versionBumpInput{Patch: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.OK {
		t.Error("OK should be true for no-op patch")
	}
}

func TestRunTestsHandler_NoConfig(t *testing.T) {
	// No .dreamland.json → config is nil → execTest returns nil silently
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	orig := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = orig })

	_, out, err := runTestsHandler(context.Background(), nil, runTestsInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.OK {
		t.Error("OK should be true when no config")
	}
}

func TestRunTestsHandler_SourceChanged(t *testing.T) {
	makeServeRepo(t, config.Config{Language: "Go", TestCommand: "true"})

	stubRunCmd(t, func(name string, args ...string) (string, error) {
		if name == "git" && strings.Contains(strings.Join(args, " "), "status") {
			return "M  main.go\n", nil
		}
		return "", nil
	})

	_, out, err := runTestsHandler(context.Background(), nil, runTestsInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.OK {
		t.Error("OK should be true when test command succeeds")
	}
}

func TestCoauthorMCPHandler_DefaultMode(t *testing.T) {
	makeServeRepo(t, config.Config{CodingTool: "Claude Code", ModelID: "claude-sonnet-4-6"})

	origRunCmd := runCmd
	runCmd = func(_ string, _ ...string) (string, error) { return "", nil }
	t.Cleanup(func() { runCmd = origRunCmd })

	_, out, err := coauthorMCPHandler(context.Background(), nil, coauthorInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.OK {
		t.Error("OK should be true on success")
	}
}

func TestCoauthorMCPHandler_TrailerMode(t *testing.T) {
	makeServeRepo(t, config.Config{ModelID: "claude-sonnet-4-6"})

	msgFile, err := os.CreateTemp(t.TempDir(), "commit-msg")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := msgFile.WriteString("fix: something\n"); err != nil {
		t.Fatal(err)
	}
	msgFile.Close()

	_, out, err := coauthorMCPHandler(context.Background(), nil, coauthorInput{Trailer: msgFile.Name()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.OK {
		t.Error("OK should be true in trailer mode")
	}

	data, _ := os.ReadFile(msgFile.Name())
	if !strings.Contains(string(data), "Co-authored-by: claude-sonnet-4-6") {
		t.Errorf("trailer not appended, got:\n%s", data)
	}
}

func TestCoauthorMCPHandler_Error(t *testing.T) {
	// No .git dir → execCoauthor (default mode) fails on gitExec
	root := t.TempDir()
	orig := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = orig })

	origRunCmd := runCmd
	runCmd = func(_ string, _ ...string) (string, error) {
		return "", errors.New("git not available")
	}
	t.Cleanup(func() { runCmd = origRunCmd })

	_, out, err := coauthorMCPHandler(context.Background(), nil, coauthorInput{})
	if err == nil {
		t.Fatal("expected error when git fails")
	}
	if out.OK {
		t.Error("OK should be false on error")
	}
}

func TestMakeSpanHandler_ExecutesInner(t *testing.T) {
	makeServeRepo(t, config.Config{})

	wrapped := makeSpanHandler("transition_log", transitionLogHandler)
	_, out, err := wrapped(context.Background(), nil, transitionLogInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.OK {
		t.Error("OK should be true")
	}
}

func TestMakeSpanHandler_WithNonNilConfig(t *testing.T) {
	orig := currentConfig
	t.Cleanup(func() { currentConfig = orig })
	currentConfig = &config.Config{ModelID: "test-model", CodingTool: "claude-code"}

	makeServeRepo(t, config.Config{})

	wrapped := makeSpanHandler("transition_log", transitionLogHandler)
	_, out, err := wrapped(context.Background(), nil, transitionLogInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.OK {
		t.Error("OK should be true")
	}
}
