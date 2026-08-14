package cmd

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"dreamland/internal/config"
)

func TestOtelReceiverAddr_Default(t *testing.T) {
	if got := otelReceiverAddr(""); got != "localhost:4318" {
		t.Errorf("got %q, want localhost:4318", got)
	}
}

func TestOtelReceiverAddr_TranslatesGRPCPort(t *testing.T) {
	if got := otelReceiverAddr("http://localhost:4317"); got != "localhost:4318" {
		t.Errorf("got %q, want localhost:4318", got)
	}
}

func TestOtelReceiverAddr_CustomPortPreserved(t *testing.T) {
	if got := otelReceiverAddr("http://localhost:9999"); got != "localhost:9999" {
		t.Errorf("got %q, want localhost:9999", got)
	}
}

func TestOtelReceiverAddr_InvalidURLFallsBack(t *testing.T) {
	if got := otelReceiverAddr("://not a url"); got != "localhost:4318" {
		t.Errorf("got %q, want localhost:4318 fallback", got)
	}
}

func TestRunOtelReceiver_GetwdError(t *testing.T) {
	orig := osGetwd
	osGetwd = func() (string, error) { return "", errors.New("getwd failed") }
	t.Cleanup(func() { osGetwd = orig })

	if err := runOtelReceiver(nil, nil); err == nil {
		t.Fatal("expected error when osGetwd fails")
	}
}

func TestRunOtelReceiver_NotInGitRepo(t *testing.T) {
	root := t.TempDir()
	orig := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = orig })

	if err := runOtelReceiver(nil, nil); err == nil {
		t.Fatal("expected error when cwd is not inside a git repository")
	}
}

func TestRunOtelReceiver_SpawnsDetachedChildWhenNotListening(t *testing.T) {
	root := makeCoauthorRepo(t, config.Config{
		CodingTool:   "GitHub Copilot",
		OtelEndpoint: "http://127.0.0.1:0",
	})
	_ = root

	origForeground := otelReceiverForeground
	otelReceiverForeground = false
	t.Cleanup(func() { otelReceiverForeground = origForeground })

	origExe := osExecutable
	osExecutable = func() (string, error) { return "/bin/echo", nil }
	t.Cleanup(func() { osExecutable = origExe })

	if err := runOtelReceiver(nil, nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunOtelReceiver_ExecutableLookupFailsFallsBackToDreamland(t *testing.T) {
	root := makeCoauthorRepo(t, config.Config{
		CodingTool:   "GitHub Copilot",
		OtelEndpoint: "http://127.0.0.1:0",
	})
	_ = root

	origForeground := otelReceiverForeground
	otelReceiverForeground = false
	t.Cleanup(func() { otelReceiverForeground = origForeground })

	origExe := osExecutable
	osExecutable = func() (string, error) { return "", errors.New("no executable") }
	t.Cleanup(func() { osExecutable = origExe })

	// falls back to exe = "dreamland", which won't resolve on PATH in the test
	// environment, so Start() fails — runOtelReceiver must swallow that error.
	if err := runOtelReceiver(nil, nil); err != nil {
		t.Errorf("expected best-effort nil error even when child fails to start, got: %v", err)
	}
}

func TestRunOtelReceiver_NilConfig(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	// No .dreamland.json — cfg will be nil, exercising the fallback to &config.Config{}.

	orig := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = orig })

	origForeground := otelReceiverForeground
	otelReceiverForeground = false
	t.Cleanup(func() { otelReceiverForeground = origForeground })

	origExe := osExecutable
	osExecutable = func() (string, error) { return "/bin/echo", nil }
	t.Cleanup(func() { osExecutable = origExe })

	if err := runOtelReceiver(nil, nil); err != nil {
		t.Errorf("unexpected error with nil config: %v", err)
	}
}

func TestRunOtelReceiver_ForegroundListenAndServeFails(t *testing.T) {
	// Occupy a real port ourselves first, so ListenAndServe hits a genuine "address
	// already in use" — deterministic on every platform/resolver. Earlier attempts
	// used a malformed address ("256.0.0.1", then an out-of-range port) hoping
	// net.Listen would reject it synchronously; both instead got silently accepted
	// by the CI runner's (Linux/cgo) resolver, leaving ListenAndServe's Accept()
	// loop blocked forever — a real hang that ran until go test's own timeout.
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { occupied.Close() })

	root := makeCoauthorRepo(t, config.Config{
		CodingTool:   "GitHub Copilot",
		OtelEndpoint: "http://" + occupied.Addr().String(),
	})

	// Without this, runOtelReceiver's config.Load reads whatever real config exists
	// at the actual process cwd (not root), so the OtelEndpoint override above is
	// silently ignored, ListenAndServe binds the real default address instead, and
	// — with nothing else listening there on a bare CI runner — blocks in Accept()
	// forever. This was the actual cause of the hang, independent of address format.
	origGetwd := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = origGetwd })

	origForeground := otelReceiverForeground
	otelReceiverForeground = true
	t.Cleanup(func() { otelReceiverForeground = origForeground })

	if err := runOtelReceiver(nil, nil); err == nil {
		t.Fatal("expected error from ListenAndServe on an unbindable address")
	}
}

func TestRunOtelReceiver_NoOpWhenAlreadyListening(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	root := makeCoauthorRepo(t, config.Config{
		CodingTool:   "GitHub Copilot",
		OtelEndpoint: "http://" + ln.Addr().String(),
	})
	_ = root

	origForeground := otelReceiverForeground
	otelReceiverForeground = false
	t.Cleanup(func() { otelReceiverForeground = origForeground })

	done := make(chan error, 1)
	go func() { done <- runOtelReceiver(nil, nil) }()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("runOtelReceiver did not return promptly when a receiver was already listening")
	}
}
