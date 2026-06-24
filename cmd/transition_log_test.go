package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTransitionLog_CreatesDir(t *testing.T) {
	root := makeGitRepo(t)
	orig, _ := os.Getwd()
	_ = os.Chdir(root)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	if err := runTransitionLog(transitionLogCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	logPath := filepath.Join(root, ".dreamland", "transition.log")
	if _, err := os.Stat(logPath); err != nil {
		t.Errorf("log file not created: %v", err)
	}
}

func TestTransitionLog_LineFormat(t *testing.T) {
	root := makeGitRepo(t)
	orig, _ := os.Getwd()
	_ = os.Chdir(root)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	t.Setenv("CLAUDE_SESSION_ID", "test-session-123")

	if err := runTransitionLog(transitionLogCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".dreamland", "transition.log"))
	if err != nil {
		t.Fatal(err)
	}
	line := string(data)
	if !strings.Contains(line, "[test-session-123] turn complete") {
		t.Errorf("unexpected log line: %q", line)
	}
}

func TestTransitionLog_TwoLines(t *testing.T) {
	root := makeGitRepo(t)
	orig, _ := os.Getwd()
	_ = os.Chdir(root)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	_ = runTransitionLog(transitionLogCmd, nil)
	_ = runTransitionLog(transitionLogCmd, nil)

	data, err := os.ReadFile(filepath.Join(root, ".dreamland", "transition.log"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 log lines, got %d:\n%s", len(lines), string(data))
	}
}

func TestTransitionLog_RandomIDFallback(t *testing.T) {
	for _, env := range []string{"CLAUDE_SESSION_ID", "CODEX_SESSION_ID", "CURSOR_SESSION_ID", "KIRO_SESSION_ID"} {
		t.Setenv(env, "")
	}

	id1 := resolveSessionID()
	id2 := resolveSessionID()
	if len(id1) != 8 {
		t.Errorf("random ID length = %d, want 8", len(id1))
	}
	// Two random IDs should almost certainly differ.
	if id1 == id2 {
		t.Log("warning: two random IDs collided (extremely unlikely)")
	}
}

// --- Additional tests to improve coverage ---

func TestRunTransitionLog_OsGetwdError(t *testing.T) {
	orig := osGetwd
	osGetwd = func() (string, error) { return "", errors.New("getwd failed") }
	t.Cleanup(func() { osGetwd = orig })

	if err := runTransitionLog(nil, nil); err == nil || err.Error() != "getwd failed" {
		t.Fatalf("expected getwd error, got: %v", err)
	}
}

func TestRunTransitionLog_FindRepoRootError(t *testing.T) {
	// Use a temp dir with no .git so FindRepoRoot returns an error.
	root := t.TempDir()
	orig := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = orig })

	if err := runTransitionLog(nil, nil); err == nil {
		t.Fatal("expected error from FindRepoRoot when no .git dir")
	}
}

func TestRunTransitionLog_OpenFileError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root; skip permission test")
	}

	root := makeGitRepo(t)

	// Create .dreamland dir but make it unwritable so OpenFile fails.
	logDir := filepath.Join(root, ".dreamland")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(logDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(logDir, 0o755) })

	// Should return nil — OpenFile error is suppressed.
	if err := runTransitionLog(nil, nil); err != nil {
		t.Fatalf("expected nil (OpenFile error suppressed), got: %v", err)
	}
}
