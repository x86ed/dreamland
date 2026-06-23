package cmd

import (
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
