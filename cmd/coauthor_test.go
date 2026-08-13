package cmd

import (
	"encoding/json"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dreamland/internal/config"
	"dreamland/internal/telemetry"
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

// TestAgentNameFromHookPayloadFrom_CopilotAgentType tests that the function
// correctly parses GitHub Copilot's agent_type field.
func TestAgentNameFromHookPayloadFrom_CopilotAgentType(t *testing.T) {
	payload := `{"hook_event_name":"SubagentStop","session_id":"s1","agent_type":"morpheus"}`
	reader := bytes.NewReader([]byte(payload))
	got := agentNameFromHookPayloadFrom(reader)
	if got != "morpheus" {
		t.Errorf("got %q, want morpheus", got)
	}
}

// TestAgentNameFromHookPayloadFrom_ClaudeCodeSubagentType tests that the function
// correctly parses Claude Code's tool_input.subagent_type field.
func TestAgentNameFromHookPayloadFrom_ClaudeCodeSubagentType(t *testing.T) {
	payload := `{"hook_event_name":"PreToolUse","tool_input":{"subagent_type":"phobetor"}}`
	reader := bytes.NewReader([]byte(payload))
	got := agentNameFromHookPayloadFrom(reader)
	if got != "phobetor" {
		t.Errorf("got %q, want phobetor", got)
	}
}

// TestAgentNameFromHookPayloadFrom_AgentTypeTakesPriority tests that agent_type
// takes priority over tool_input.subagent_type.
func TestAgentNameFromHookPayloadFrom_AgentTypeTakesPriority(t *testing.T) {
	payload := `{"agent_type":"morpheus","tool_input":{"subagent_type":"phobetor"}}`
	reader := bytes.NewReader([]byte(payload))
	got := agentNameFromHookPayloadFrom(reader)
	if got != "morpheus" {
		t.Errorf("got %q, want morpheus", got)
	}
}

// TestAgentNameFromHookPayloadFrom_NoAgentTypeField tests that the function
// returns "" when no agent field is present.
func TestAgentNameFromHookPayloadFrom_NoAgentTypeField(t *testing.T) {
	payload := `{"hook_event_name":"Stop","session_id":"s1"}`
	reader := bytes.NewReader([]byte(payload))
	got := agentNameFromHookPayloadFrom(reader)
	if got != "" {
		t.Errorf("got %q, want empty string when no agent_type field present", got)
	}
}

// TestAgentNameFromHookPayloadFrom_EmptyStdin tests that the function handles empty input.
func TestAgentNameFromHookPayloadFrom_EmptyStdin(t *testing.T) {
	reader := bytes.NewReader([]byte(""))
	got := agentNameFromHookPayloadFrom(reader)
	if got != "" {
		t.Errorf("got %q, want empty string for empty stdin", got)
	}
}

// TestAgentNameFromHookPayloadFrom_InvalidJSON tests that the function handles invalid JSON.
func TestAgentNameFromHookPayloadFrom_InvalidJSON(t *testing.T) {
	reader := bytes.NewReader([]byte("not json"))
	got := agentNameFromHookPayloadFrom(reader)
	if got != "" {
		t.Errorf("got %q, want empty string for invalid JSON", got)
	}
}

func TestResolveEnforcedAgentName_ClaudeCodeFallback(t *testing.T) {
	// Claude Code without a hook payload should fall back to janus, not the tool name
	cfg := &config.Config{CodingTool: "Claude Code"}
	got := resolveEnforcedAgentName(cfg)
	if got != "janus" {
		t.Errorf("got %q, want janus for Claude Code fallback", got)
	}
}

func TestResolveEnforcedAgentName_OtherPlatformFallback(t *testing.T) {
	// Other platforms should fall back to the tool name
	cfg := &config.Config{CodingTool: "GitHub Copilot"}
	got := resolveEnforcedAgentName(cfg)
	if got != "GitHub Copilot" {
		t.Errorf("got %q, want GitHub Copilot fallback", got)
	}
}

// makeCoauthorRepo creates a temporary git repo with .dreamland.json for testing.
func makeCoauthorRepo(t *testing.T, cfg config.Config) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".dreamland.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// --- runCoauthor tests ---

// withCoauthorFlags saves/restores the coauthor command's package-level flags.
func withCoauthorFlags(t *testing.T, trailer string, hook bool) {
	t.Helper()
	origTrailer, origHook := coauthorTrailer, coauthorHook
	coauthorTrailer, coauthorHook = trailer, hook
	t.Cleanup(func() { coauthorTrailer, coauthorHook = origTrailer, origHook })
}

func TestRunCoauthor_GetwdError(t *testing.T) {
	withCoauthorFlags(t, "", false)
	orig := osGetwd
	osGetwd = func() (string, error) { return "", errors.New("getwd failed") }
	t.Cleanup(func() { osGetwd = orig })

	if err := runCoauthor(nil, nil); err == nil {
		t.Fatal("expected error when osGetwd fails")
	}
}

func TestRunCoauthor_NoOpOutsideGitRepo(t *testing.T) {
	withCoauthorFlags(t, "", false)
	root := t.TempDir() // not a git repo
	orig := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = orig })

	if err := runCoauthor(nil, nil); err != nil {
		t.Fatalf("expected nil (skip) outside git repo, got: %v", err)
	}
}

func TestRunCoauthor_ConfigLoadError(t *testing.T) {
	withCoauthorFlags(t, "", false)
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Make .dreamland.json a directory so config.Load fails with a non-ErrNoGitRepo error.
	if err := os.Mkdir(filepath.Join(root, ".dreamland.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	orig := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = orig })

	if err := runCoauthor(nil, nil); err == nil {
		t.Fatal("expected error from config.Load")
	}
}

func TestRunCoauthor_NilConfig_DefaultMode(t *testing.T) {
	withCoauthorFlags(t, "", false)
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	// No .dreamland.json written — cfg will be nil, exercising the fallback.
	orig := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = orig })

	for _, env := range []string{"CLAUDE_AGENT_ID", "CODEX_AGENT_ID", "CURSOR_AGENT_ID", "KIRO_AGENT_ID"} {
		t.Setenv(env, "")
	}

	stubRunCmd(t, func(_ string, _ ...string) (string, error) { return "", nil })

	if err := runCoauthor(nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hookPath := filepath.Join(root, ".git", "hooks", "prepare-commit-msg")
	if _, err := os.Stat(hookPath); err != nil {
		t.Errorf("expected hook to be installed: %v", err)
	}
}

func TestRunCoauthor_DefaultMode_GitConfigNameError(t *testing.T) {
	withCoauthorFlags(t, "", false)
	root := makeCoauthorRepo(t, config.Config{CodingTool: "GitHub Copilot"})
	orig := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = orig })

	stubRunCmd(t, func(_ string, args ...string) (string, error) {
		if len(args) > 1 && args[0] == "config" && len(args) > 2 && args[2] == "user.name" {
			return "", errors.New("git config user.name failed")
		}
		return "", nil
	})

	if err := runCoauthor(nil, nil); err == nil {
		t.Fatal("expected error when git config user.name fails")
	}
}

func TestRunCoauthor_DefaultMode_GitConfigEmailError(t *testing.T) {
	withCoauthorFlags(t, "", false)
	root := makeCoauthorRepo(t, config.Config{CodingTool: "GitHub Copilot"})
	orig := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = orig })

	stubRunCmd(t, func(_ string, args ...string) (string, error) {
		if len(args) > 2 && args[0] == "config" && args[2] == "user.email" {
			return "", errors.New("git config user.email failed")
		}
		return "", nil
	})

	if err := runCoauthor(nil, nil); err == nil {
		t.Fatal("expected error when git config user.email fails")
	}
}

func TestRunCoauthor_DefaultMode_InstallHookError(t *testing.T) {
	withCoauthorFlags(t, "", false)
	root := t.TempDir()
	// Make .git a *file*, not a directory: config.Load/FindRepoRoot still find it (os.Stat
	// succeeds on a file too), but installPrepareCommitMsgHook's MkdirAll(.git/hooks) will
	// fail because ".git" is not a directory.
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: elsewhere"), 0o644); err != nil {
		t.Fatal(err)
	}
	orig := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = orig })

	stubRunCmd(t, func(_ string, _ ...string) (string, error) { return "", nil })

	if err := runCoauthor(nil, nil); err == nil {
		t.Fatal("expected error when installPrepareCommitMsgHook fails")
	}
}

func TestRunCoauthor_DefaultMode_HookFlagResolvesAgent(t *testing.T) {
	withCoauthorFlags(t, "", true)
	root := makeCoauthorRepo(t, config.Config{CodingTool: "Claude Code"})
	orig := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = orig })

	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteString(`{"agent_type":"morpheus"}`); err != nil {
		t.Fatal(err)
	}
	w.Close()
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = oldStdin; r.Close() })

	var configuredName string
	stubRunCmd(t, func(_ string, args ...string) (string, error) {
		if len(args) > 2 && args[0] == "config" && args[2] == "user.name" {
			configuredName = args[3]
		}
		return "", nil
	})

	if err := runCoauthor(nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if configuredName != "morpheus" {
		t.Errorf("expected agent identity morpheus from hook payload, got %q", configuredName)
	}
}

func TestRunCoauthor_TrailerMode_Success(t *testing.T) {
	withCoauthorFlags(t, "", false) // trailer path set below with actual file
	root := makeCoauthorRepo(t, config.Config{ModelID: "claude-sonnet-4-6"})
	orig := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = orig })

	msgFile := filepath.Join(root, "COMMIT_EDITMSG")
	if err := os.WriteFile(msgFile, []byte("feat: something\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	coauthorTrailer = msgFile
	t.Cleanup(func() { coauthorTrailer = "" })

	if err := runCoauthor(nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(msgFile)
	if !strings.Contains(string(data), "Co-authored-by: claude-sonnet-4-6") {
		t.Errorf("expected trailer appended, got:\n%s", data)
	}
}

func TestRunCoauthor_TrailerMode_AppendError(t *testing.T) {
	withCoauthorFlags(t, "", false)
	root := makeCoauthorRepo(t, config.Config{ModelID: "claude-sonnet-4-6"})
	orig := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = orig })

	coauthorTrailer = filepath.Join(root, "does-not-exist.txt")
	t.Cleanup(func() { coauthorTrailer = "" })

	if err := runCoauthor(nil, nil); err == nil {
		t.Fatal("expected error when the commit-msg file doesn't exist")
	}
}

// --- installPrepareCommitMsgHook additional branches ---

func TestInstallPrepareCommitMsgHook_FindRepoRootError(t *testing.T) {
	root := t.TempDir() // not a git repo
	if err := installPrepareCommitMsgHook(root); err == nil {
		t.Fatal("expected error when repoDir is not a git repository")
	}
}

func TestInstallPrepareCommitMsgHook_MkdirAllError(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Block .git/hooks with a regular file so MkdirAll(.git/hooks) fails.
	if err := os.WriteFile(filepath.Join(root, ".git", "hooks"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := installPrepareCommitMsgHook(root); err == nil {
		t.Fatal("expected error when .git/hooks is blocked by a file")
	}
}

// --- appendCoauthorTrailer additional branches ---

func TestAppendCoauthorTrailer_ReadFileError(t *testing.T) {
	err := appendCoauthorTrailer(filepath.Join(t.TempDir(), "missing.txt"), "claude-sonnet-4-6", "@github.com")
	if err == nil {
		t.Fatal("expected error for missing commit-msg file")
	}
}

func TestAppendCoauthorTrailer_NoTrailingNewline(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "commit-msg")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("feat: no trailing newline"); err != nil {
		t.Fatal(err)
	}
	f.Close()

	if err := appendCoauthorTrailer(f.Name(), "claude-sonnet-4-6", "@github.com"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(f.Name())
	if !strings.Contains(string(data), "feat: no trailing newline\nCo-authored-by:") {
		t.Errorf("expected newline inserted before trailer, got:\n%q", data)
	}
}

// --- appendTokensReport tests ---

func TestAppendTokensReport_NoSnapshot(t *testing.T) {
	root := t.TempDir()
	msgFile := filepath.Join(root, "COMMIT_EDITMSG")
	if err := os.WriteFile(msgFile, []byte("feat: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := appendTokensReport(msgFile, root); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(msgFile)
	if strings.Contains(string(data), "Tokens:") {
		t.Errorf("expected no Tokens line when no snapshot exists, got:\n%s", data)
	}
}

func TestAppendTokensReport_AllZero(t *testing.T) {
	root := t.TempDir()
	if err := telemetry.Write(root, &telemetry.SnapshotResult{Tool: "claude-code"}); err != nil {
		t.Fatal(err)
	}
	msgFile := filepath.Join(root, "COMMIT_EDITMSG")
	if err := os.WriteFile(msgFile, []byte("feat: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := appendTokensReport(msgFile, root); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(msgFile)
	if strings.Contains(string(data), "Tokens:") {
		t.Errorf("expected no Tokens line for all-zero snapshot, got:\n%s", data)
	}
}

func TestAppendTokensReport_ReadFileError(t *testing.T) {
	root := t.TempDir()
	if err := telemetry.Write(root, &telemetry.SnapshotResult{Tool: "claude-code", InputTokens: 100}); err != nil {
		t.Fatal(err)
	}

	err := appendTokensReport(filepath.Join(root, "missing.txt"), root)
	if err == nil {
		t.Fatal("expected error when commit-msg file doesn't exist")
	}
}

func TestAppendTokensReport_Idempotent(t *testing.T) {
	root := t.TempDir()
	if err := telemetry.Write(root, &telemetry.SnapshotResult{Tool: "claude-code", InputTokens: 100}); err != nil {
		t.Fatal(err)
	}
	msgFile := filepath.Join(root, "COMMIT_EDITMSG")
	original := "feat: x\nTokens: input=999 output=0 cached=0 total=999\n"
	if err := os.WriteFile(msgFile, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := appendTokensReport(msgFile, root); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(msgFile)
	if string(data) != original {
		t.Errorf("expected file unchanged (idempotent), got:\n%s", data)
	}
}

func TestAppendTokensReport_AppendsWithTrailingNewline(t *testing.T) {
	root := t.TempDir()
	if err := telemetry.Write(root, &telemetry.SnapshotResult{Tool: "claude-code", InputTokens: 100, OutputTokens: 20}); err != nil {
		t.Fatal(err)
	}
	msgFile := filepath.Join(root, "COMMIT_EDITMSG")
	if err := os.WriteFile(msgFile, []byte("feat: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := appendTokensReport(msgFile, root); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(msgFile)
	if !strings.Contains(string(data), "Tokens: input=100 output=20") {
		t.Errorf("expected Tokens line appended, got:\n%s", data)
	}
}

func TestAppendTokensReport_AppendsWithoutTrailingNewline(t *testing.T) {
	root := t.TempDir()
	if err := telemetry.Write(root, &telemetry.SnapshotResult{Tool: "claude-code", InputTokens: 5, OutputTokens: 1}); err != nil {
		t.Fatal(err)
	}
	msgFile := filepath.Join(root, "COMMIT_EDITMSG")
	if err := os.WriteFile(msgFile, []byte("feat: no newline at end"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := appendTokensReport(msgFile, root); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(msgFile)
	if !strings.Contains(string(data), "feat: no newline at end\nTokens: input=5 output=1") {
		t.Errorf("expected newline inserted before Tokens line, got:\n%q", data)
	}
}
