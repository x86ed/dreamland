package cmd

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"
)

// TestExecuteHelp verifies Execute returns cleanly when --help is passed.
func TestExecuteHelp(t *testing.T) {
	old := os.Args
	defer func() { os.Args = old }()
	os.Args = []string{"dreamland", "--help"}
	Execute()
}

// TestExecuteUnknownFlag verifies Execute calls os.Exit(1) on a bad flag.
// Uses a subprocess so the os.Exit does not kill the test process.
func TestExecuteUnknownFlag(t *testing.T) {
	if os.Getenv("DREAMLAND_TEST_EXIT") == "1" {
		os.Args = []string{"dreamland", "--no-such-flag"}
		Execute()
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestExecuteUnknownFlag")
	cmd.Env = append(os.Environ(), "DREAMLAND_TEST_EXIT=1")
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit but got nil")
	}
}

// TestExecuteBlockingError verifies Execute calls os.Exit(2) when the command
// returns a Blocking error (e.g. `commit --reason` validation failure).
// Uses a subprocess so the os.Exit does not kill the test process.
func TestExecuteBlockingError(t *testing.T) {
	if os.Getenv("DREAMLAND_TEST_BLOCKING_EXIT") == "1" {
		os.Args = []string{"dreamland", "commit", "--reason=bogus"}
		Execute()
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestExecuteBlockingError")
	cmd.Env = append(os.Environ(), "DREAMLAND_TEST_BLOCKING_EXIT=1")
	err := cmd.Run()
	var exitErr *exec.ExitError
	if err == nil {
		t.Fatal("expected non-zero exit but got nil")
	}
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected *exec.ExitError, got %T: %v", err, err)
	}
	if exitErr.ExitCode() != 2 {
		t.Errorf("exit code = %d, want 2 for a Blocking error", exitErr.ExitCode())
	}
}

// TestHelloHandler verifies the hello tool returns the expected greeting.
func TestHelloHandler(t *testing.T) {
	result, out, err := helloHandler(context.Background(), nil, helloInput{Name: "World"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Message != "Hello, World!" {
		t.Errorf("got %q, want %q", out.Message, "Hello, World!")
	}
	if len(result.Content) == 0 {
		t.Error("expected at least one content item")
	}
}

// TestRunServeExitsOnStdinEOF verifies runServe exits when stdin is closed.
func TestRunServeExitsOnStdinEOF(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	w.Close() // immediate EOF

	oldStdin := os.Stdin
	os.Stdin = r
	defer func() {
		os.Stdin = oldStdin
		r.Close()
	}()

	done := make(chan error, 1)
	go func() { done <- runServe(serveCmd, nil) }()

	select {
	case <-done:
		// exited as expected
	case <-time.After(10 * time.Second):
		t.Error("runServe did not exit within 10s after stdin EOF")
	}
}
