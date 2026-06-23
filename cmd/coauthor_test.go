package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveAgentName_EnvVar(t *testing.T) {
	t.Setenv("CLAUDE_AGENT_ID", "orchestrator")

	got := resolveAgentName("Claude Code")
	if got != "orchestrator" {
		t.Errorf("got %q, want orchestrator", got)
	}
}

func TestResolveAgentName_FallbackTool(t *testing.T) {
	// Make sure no platform env vars are set.
	for _, env := range []string{"CLAUDE_AGENT_ID", "CODEX_AGENT_ID", "CURSOR_AGENT_ID", "KIRO_AGENT_ID"} {
		t.Setenv(env, "")
	}

	got := resolveAgentName("GitHub Copilot")
	if got != "GitHub Copilot" {
		t.Errorf("got %q, want %q", got, "GitHub Copilot")
	}
}

func TestResolveAgentName_Priority(t *testing.T) {
	// CLAUDE_AGENT_ID takes priority.
	t.Setenv("CLAUDE_AGENT_ID", "spec-writer")
	t.Setenv("CODEX_AGENT_ID", "other")

	got := resolveAgentName("Codex CLI")
	if got != "spec-writer" {
		t.Errorf("got %q, want spec-writer", got)
	}
}

func TestInstallPrepareCommitMsgHook(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git", "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := installPrepareCommitMsgHook(root); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hookPath := filepath.Join(root, ".git", "hooks", "prepare-commit-msg")
	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("hook file not created: %v", err)
	}
	if !strings.Contains(string(data), "dreamland coauthor --trailer") {
		t.Errorf("hook missing delegation line, got: %q", string(data))
	}

	info, err := os.Stat(hookPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o111 == 0 {
		t.Errorf("hook not executable, mode = %v", info.Mode())
	}
}

func TestInstallPrepareCommitMsgHook_Idempotent(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git", "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Install twice.
	_ = installPrepareCommitMsgHook(root)
	_ = installPrepareCommitMsgHook(root)

	hookPath := filepath.Join(root, ".git", "hooks", "prepare-commit-msg")
	data, _ := os.ReadFile(hookPath)
	// Count occurrences of the delegation line.
	count := strings.Count(string(data), "dreamland coauthor --trailer")
	if count != 1 {
		t.Errorf("delegation line appears %d times, want 1", count)
	}
}

func TestAppendCoauthorTrailer(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "commit-msg")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("feat: add something\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()

	if err := appendCoauthorTrailer(f.Name(), "claude-sonnet-4-6", "@github.com"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(f.Name())
	if !strings.Contains(string(data), "Co-authored-by: claude-sonnet-4-6 <claude-sonnet-4-6@github.com>") {
		t.Errorf("trailer not appended, got:\n%s", string(data))
	}
}

func TestAppendCoauthorTrailer_NotDuplicated(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "commit-msg")
	if err != nil {
		t.Fatal(err)
	}
	content := "feat: add something\nCo-authored-by: claude-sonnet-4-6 <claude-sonnet-4-6@github.com>\n"
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	f.Close()

	if err := appendCoauthorTrailer(f.Name(), "claude-sonnet-4-6", "@github.com"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(f.Name())
	count := strings.Count(string(data), "Co-authored-by: claude-sonnet-4-6")
	if count != 1 {
		t.Errorf("trailer duplicated, count = %d", count)
	}
}

func TestAppendCoauthorTrailer_ModelNameExtracted(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "commit-msg")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("feat: something\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()

	// model_id with settings string
	if err := appendCoauthorTrailer(f.Name(), "claude-sonnet-4-6 temperature=1.0", "@github.com"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(f.Name())
	if !strings.Contains(string(data), "Co-authored-by: claude-sonnet-4-6 <claude-sonnet-4-6@github.com>") {
		t.Errorf("model name not correctly extracted, got:\n%s", string(data))
	}
	if strings.Contains(string(data), "temperature") {
		t.Errorf("settings leaked into trailer, got:\n%s", string(data))
	}
}

func TestAppendCoauthorTrailer_EmptyModelID(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "commit-msg")
	if err != nil {
		t.Fatal(err)
	}
	original := "feat: something\n"
	if _, err := f.WriteString(original); err != nil {
		t.Fatal(err)
	}
	f.Close()

	if err := appendCoauthorTrailer(f.Name(), "", "@github.com"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(f.Name())
	if string(data) != original {
		t.Errorf("file modified when model_id empty, got:\n%s", string(data))
	}
}
