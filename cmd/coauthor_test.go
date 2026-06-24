package cmd

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dreamland/internal/config"
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

func makeCoauthorRepo(t *testing.T, cfg config.Config) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git", "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(cfg)
	if err := os.WriteFile(filepath.Join(root, ".dreamland.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	orig := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = orig })
	return root
}

func TestRunCoauthor_DefaultMode(t *testing.T) {
	root := makeCoauthorRepo(t, config.Config{CodingTool: "Claude Code", ModelID: "claude-sonnet-4-6"})

	var gitCalls []string
	origRunCmd := runCmd
	runCmd = func(_ string, args ...string) (string, error) {
		gitCalls = append(gitCalls, strings.Join(args, " "))
		return "", nil
	}
	t.Cleanup(func() { runCmd = origRunCmd })

	origTrailer := coauthorTrailer
	coauthorTrailer = ""
	t.Cleanup(func() { coauthorTrailer = origTrailer })

	if err := runCoauthor(nil, nil); err != nil {
		t.Fatalf("runCoauthor: %v", err)
	}

	hookPath := filepath.Join(root, ".git", "hooks", "prepare-commit-msg")
	hookData, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("hook not installed: %v", err)
	}
	if !strings.Contains(string(hookData), "dreamland coauthor --trailer") {
		t.Error("hook missing delegation line")
	}

	hasName := false
	for _, c := range gitCalls {
		if strings.Contains(c, "user.name") {
			hasName = true
		}
	}
	if !hasName {
		t.Error("expected git config user.name call")
	}
}

func TestRunCoauthor_TrailerMode(t *testing.T) {
	root := makeCoauthorRepo(t, config.Config{ModelID: "claude-sonnet-4-6"})
	_ = root

	msgFile, err := os.CreateTemp(t.TempDir(), "commit-msg")
	if err != nil {
		t.Fatal(err)
	}
	msgFile.WriteString("fix: something\n")
	msgFile.Close()

	origTrailer := coauthorTrailer
	coauthorTrailer = msgFile.Name()
	t.Cleanup(func() { coauthorTrailer = origTrailer })

	if err := runCoauthor(nil, nil); err != nil {
		t.Fatalf("runCoauthor trailer mode: %v", err)
	}

	data, _ := os.ReadFile(msgFile.Name())
	if !strings.Contains(string(data), "Co-authored-by: claude-sonnet-4-6") {
		t.Errorf("trailer not appended, got:\n%s", string(data))
	}
}

// --- Additional tests to improve coverage ---

func TestRunCoauthor_OsGetwdError(t *testing.T) {
	orig := osGetwd
	osGetwd = func() (string, error) { return "", errors.New("getwd failed") }
	t.Cleanup(func() { osGetwd = orig })

	if err := runCoauthor(nil, nil); err == nil || err.Error() != "getwd failed" {
		t.Fatalf("expected getwd error, got: %v", err)
	}
}

func TestRunCoauthor_ConfigLoadError(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git", "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Write invalid JSON so config.Load fails.
	if err := os.WriteFile(filepath.Join(root, ".dreamland.json"), []byte("not-json"), 0o644); err != nil {
		t.Fatal(err)
	}
	orig := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = orig })

	if err := runCoauthor(nil, nil); err == nil {
		t.Fatal("expected error from config.Load with invalid JSON")
	}
}

func TestRunCoauthor_NilConfig(t *testing.T) {
	// .git dir exists but no .dreamland.json → config.Load returns nil, nil
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git", "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	orig := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = orig })

	origRunCmd := runCmd
	runCmd = func(_ string, _ ...string) (string, error) { return "", nil }
	t.Cleanup(func() { runCmd = origRunCmd })

	origTrailer := coauthorTrailer
	coauthorTrailer = ""
	t.Cleanup(func() { coauthorTrailer = origTrailer })

	// Should succeed (cfg==nil gets replaced by &config.Config{})
	if err := runCoauthor(nil, nil); err != nil {
		t.Fatalf("expected nil error for missing config, got: %v", err)
	}
}

func TestRunCoauthor_GitConfigUserNameError(t *testing.T) {
	root := makeCoauthorRepo(t, config.Config{CodingTool: "Claude Code"})
	_ = root

	origRunCmd := runCmd
	runCmd = func(_ string, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "config" && strings.Contains(strings.Join(args, " "), "user.name") {
			return "", errors.New("git config user.name failed")
		}
		return "", nil
	}
	t.Cleanup(func() { runCmd = origRunCmd })

	origTrailer := coauthorTrailer
	coauthorTrailer = ""
	t.Cleanup(func() { coauthorTrailer = origTrailer })

	if err := runCoauthor(nil, nil); err == nil {
		t.Fatal("expected error from git config user.name failure")
	}
}

func TestRunCoauthor_GitConfigUserEmailError(t *testing.T) {
	root := makeCoauthorRepo(t, config.Config{CodingTool: "Claude Code"})
	_ = root

	origRunCmd := runCmd
	runCmd = func(_ string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		if len(args) > 0 && args[0] == "config" && strings.Contains(joined, "user.name") {
			return "", nil
		}
		if len(args) > 0 && args[0] == "config" && strings.Contains(joined, "user.email") {
			return "", errors.New("git config user.email failed")
		}
		return "", nil
	}
	t.Cleanup(func() { runCmd = origRunCmd })

	origTrailer := coauthorTrailer
	coauthorTrailer = ""
	t.Cleanup(func() { coauthorTrailer = origTrailer })

	if err := runCoauthor(nil, nil); err == nil {
		t.Fatal("expected error from git config user.email failure")
	}
}

func TestResolveAgentName_EmptyCodingTool(t *testing.T) {
	// Clear all env vars.
	for _, env := range []string{"CLAUDE_AGENT_ID", "CODEX_AGENT_ID", "CURSOR_AGENT_ID", "KIRO_AGENT_ID"} {
		t.Setenv(env, "")
	}
	got := resolveAgentName("")
	if got != "dreamland" {
		t.Errorf("expected 'dreamland' for empty codingTool, got %q", got)
	}
}

func TestInstallPrepareCommitMsgHook_FindRepoRootError(t *testing.T) {
	// Pass a directory without .git so FindRepoRoot fails.
	root := t.TempDir()
	if err := installPrepareCommitMsgHook(root); err == nil {
		t.Fatal("expected error when no .git dir")
	}
}

func TestInstallPrepareCommitMsgHook_MkdirAllError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root; skip permission test")
	}
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Make .git unwritable so MkdirAll(.git/hooks) fails.
	if err := os.Chmod(gitDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(gitDir, 0o755) })

	if err := installPrepareCommitMsgHook(root); err == nil {
		t.Fatal("expected error when .git is unwritable")
	}
}

func TestAppendCoauthorTrailer_ReadFileError(t *testing.T) {
	// Non-existent file → ReadFile fails.
	err := appendCoauthorTrailer("/nonexistent/commit-msg-file", "claude-sonnet-4-6", "@github.com")
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

func TestAppendCoauthorTrailer_NoTrailingNewline(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "commit-msg")
	if err != nil {
		t.Fatal(err)
	}
	// Write content WITHOUT trailing newline.
	if _, err := f.WriteString("feat: something"); err != nil {
		t.Fatal(err)
	}
	f.Close()

	if err := appendCoauthorTrailer(f.Name(), "claude-sonnet-4-6", "@github.com"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(f.Name())
	content := string(data)
	// Newline should be added before trailer.
	if !strings.Contains(content, "\nCo-authored-by: claude-sonnet-4-6") {
		t.Errorf("expected newline before trailer when original has no trailing newline, got:\n%s", content)
	}
}
