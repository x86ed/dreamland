package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestAppendCodingToolTrailer_NotTruncatedAtSpace(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "commit-msg")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("feat: something\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()

	if err := appendCodingToolTrailer(f.Name(), "GitHub Copilot", "@github.com"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(f.Name())
	if !strings.Contains(string(data), "Co-authored-by: GitHub Copilot <github-copilot@github.com>") {
		t.Errorf("coding-tool trailer missing or truncated, got:\n%s", string(data))
	}
}

func TestAppendCodingToolTrailer_NotDuplicated(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "commit-msg")
	if err != nil {
		t.Fatal(err)
	}
	content := "feat: something\nCo-authored-by: GitHub Copilot <github-copilot@github.com>\n"
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	f.Close()

	if err := appendCodingToolTrailer(f.Name(), "GitHub Copilot", "@github.com"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(f.Name())
	count := strings.Count(string(data), "Co-authored-by: GitHub Copilot")
	if count != 1 {
		t.Errorf("coding-tool trailer duplicated, count = %d", count)
	}
}

func TestAppendCodingToolTrailer_EmptyCodingTool(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "commit-msg")
	if err != nil {
		t.Fatal(err)
	}
	original := "feat: something\n"
	if _, err := f.WriteString(original); err != nil {
		t.Fatal(err)
	}
	f.Close()

	if err := appendCodingToolTrailer(f.Name(), "", "@github.com"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(f.Name())
	if string(data) != original {
		t.Errorf("file modified when coding tool empty, got:\n%s", string(data))
	}
}

func TestAppendBothTrailers_OrderAndIndependentIdempotency(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "commit-msg")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("feat: something\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()

	if err := appendCoauthorTrailer(f.Name(), "claude-sonnet-4-6", "@github.com"); err != nil {
		t.Fatalf("appendCoauthorTrailer: %v", err)
	}
	if err := appendCodingToolTrailer(f.Name(), "Claude Code", "@github.com"); err != nil {
		t.Fatalf("appendCodingToolTrailer: %v", err)
	}

	data, _ := os.ReadFile(f.Name())
	content := string(data)
	modelIdx := strings.Index(content, "Co-authored-by: claude-sonnet-4-6")
	toolIdx := strings.Index(content, "Co-authored-by: Claude Code")
	if modelIdx == -1 || toolIdx == -1 {
		t.Fatalf("expected both trailers present, got:\n%s", content)
	}
	if modelIdx >= toolIdx {
		t.Errorf("expected model trailer before coding-tool trailer, got:\n%s", content)
	}

	// Re-run both — only the model trailer is "already present" from a hypothetical
	// prior run; the coding-tool one should append independently without duplicating
	// the model line, and vice versa is exercised by the dedicated dedup tests above.
	if err := appendCoauthorTrailer(f.Name(), "claude-sonnet-4-6", "@github.com"); err != nil {
		t.Fatalf("appendCoauthorTrailer (rerun): %v", err)
	}
	if err := appendCodingToolTrailer(f.Name(), "Claude Code", "@github.com"); err != nil {
		t.Fatalf("appendCodingToolTrailer (rerun): %v", err)
	}
	data, _ = os.ReadFile(f.Name())
	content = string(data)
	if strings.Count(content, "Co-authored-by: claude-sonnet-4-6") != 1 {
		t.Errorf("model trailer duplicated after rerun, got:\n%s", content)
	}
	if strings.Count(content, "Co-authored-by: Claude Code") != 1 {
		t.Errorf("coding-tool trailer duplicated after rerun, got:\n%s", content)
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

// TestAgentNameFromHookPayloadFrom_ClaudeCodeSubagentStopAgentType tests that Claude
// Code's real SubagentStop payload shape — a top-level "agent_type" field alongside
// "agent_id"/"agent_transcript_path"/"last_assistant_message"/"stop_hook_active"
// (confirmed against Anthropic's published hooks reference) — resolves to the finishing
// sub-agent's name rather than falling through to "". This is the shape the
// .claude/settings.json SubagentStop hook chain (`dreamland coauthor --hook` then
// `dreamland commit --reason handoff`) actually receives on every sub-agent hand-off.
func TestAgentNameFromHookPayloadFrom_ClaudeCodeSubagentStopAgentType(t *testing.T) {
	payload := `{
		"session_id": "abc123",
		"transcript_path": "~/.claude/projects/.../abc123.jsonl",
		"cwd": "/Users/example",
		"permission_mode": "default",
		"hook_event_name": "SubagentStop",
		"stop_hook_active": false,
		"agent_id": "def456",
		"agent_type": "phantasos",
		"agent_transcript_path": "~/.claude/projects/.../abc123/subagents/agent-def456.jsonl",
		"last_assistant_message": "Analysis complete.",
		"background_tasks": [],
		"session_crons": []
	}`
	reader := bytes.NewReader([]byte(payload))
	got := agentNameFromHookPayloadFrom(reader)
	if got != "phantasos" {
		t.Errorf("got %q, want phantasos", got)
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
	got := resolveEnforcedAgentName(cfg, "")
	if got != "janus" {
		t.Errorf("got %q, want janus for Claude Code fallback", got)
	}
}

func TestRunCoauthor_AgentNameFlagSkipsStdinRead(t *testing.T) {
	agentNames := []string{"janus", "phantasos", "nyx", "morpheus", "phobetor", "baku", "iktomi", "zhougong", "hypnos", "mengpo"}

	for _, name := range agentNames {
		t.Run(name, func(t *testing.T) {
			makeCoauthorRepo(t, config.Config{CodingTool: "Claude Code"})

			// A pipe with nothing written and never closed: if the code attempted the
			// stdin read despite --agent-name being set, this call would block forever —
			// agentNameFromHookPayloadFrom's io.ReadAll has no timeout of its own (that's
			// intentional; --hook gates whether it's ever called at all — see coauthor.go).
			// --agent-name must win before that gate is even checked.
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { w.Close(); r.Close() })
			origStdin := os.Stdin
			os.Stdin = r
			t.Cleanup(func() { os.Stdin = origStdin })

			var gitCalls []string
			stubRunCmd(t, func(_ string, args ...string) (string, error) {
				gitCalls = append(gitCalls, strings.Join(args, " "))
				return "", nil
			})

			origTrailer, origHook, origAgentName := coauthorTrailer, coauthorHook, coauthorAgentName
			coauthorTrailer = ""
			coauthorHook = true
			coauthorAgentName = name
			t.Cleanup(func() { coauthorTrailer = origTrailer; coauthorHook = origHook; coauthorAgentName = origAgentName })

			done := make(chan error, 1)
			go func() { done <- runCoauthor(nil, nil) }()
			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("runCoauthor: %v", err)
				}
			case <-time.After(2 * time.Second):
				t.Fatalf("runCoauthor blocked on stdin despite --agent-name=%s", name)
			}

			found := false
			for _, c := range gitCalls {
				if c == "config --local user.name "+name {
					found = true
				}
			}
			if !found {
				t.Errorf("expected git config user.name %s, got calls: %v", name, gitCalls)
			}
		})
	}
}

func TestRunCoauthor_AgentNameFlagAbsent_FallsBackToExistingChain(t *testing.T) {
	makeCoauthorRepo(t, config.Config{CodingTool: "GitHub Copilot"})
	withPipedStdin(t, `{"hook_event_name":"SubagentStop","agent_type":"iktomi"}`)

	var gitCalls []string
	stubRunCmd(t, func(_ string, args ...string) (string, error) {
		gitCalls = append(gitCalls, strings.Join(args, " "))
		return "", nil
	})

	origTrailer, origHook, origAgentName := coauthorTrailer, coauthorHook, coauthorAgentName
	coauthorTrailer = ""
	coauthorHook = true
	coauthorAgentName = ""
	t.Cleanup(func() { coauthorTrailer = origTrailer; coauthorHook = origHook; coauthorAgentName = origAgentName })

	if err := runCoauthor(nil, nil); err != nil {
		t.Fatalf("runCoauthor: %v", err)
	}

	found := false
	for _, c := range gitCalls {
		if c == "config --local user.name iktomi" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected fallback chain to resolve iktomi from hook payload, got calls: %v", gitCalls)
	}
}

// TestRunCoauthor_ClaudeCodeSubagentStopResolvesRealAgent is the regression test for the
// bug where .claude/settings.json's SubagentStop hook chain (`dreamland coauthor --hook`
// then `dreamland commit --reason handoff`) silently fell back to "janus" on every
// sub-agent hand-off instead of the sub-agent that actually finished. It failed under the
// stale (pre-fix) assumption that Claude Code's SubagentStop payload carries no sub-agent
// identifier at all; the real payload carries a top-level "agent_type" field, which
// resolveEnforcedAgentName/agentNameFromHookPayloadFrom must resolve to git config
// user.name, not the janus default.
func TestRunCoauthor_ClaudeCodeSubagentStopResolvesRealAgent(t *testing.T) {
	makeCoauthorRepo(t, config.Config{CodingTool: "Claude Code"})
	withPipedStdin(t, `{
		"session_id": "abc123",
		"transcript_path": "~/.claude/projects/.../abc123.jsonl",
		"cwd": "/Users/example",
		"permission_mode": "default",
		"hook_event_name": "SubagentStop",
		"stop_hook_active": false,
		"agent_id": "def456",
		"agent_type": "phantasos",
		"agent_transcript_path": "~/.claude/projects/.../abc123/subagents/agent-def456.jsonl",
		"last_assistant_message": "Analysis complete.",
		"background_tasks": [],
		"session_crons": []
	}`)

	var gitCalls []string
	stubRunCmd(t, func(_ string, args ...string) (string, error) {
		gitCalls = append(gitCalls, strings.Join(args, " "))
		return "", nil
	})

	origTrailer, origHook, origAgentName := coauthorTrailer, coauthorHook, coauthorAgentName
	coauthorTrailer = ""
	coauthorHook = true
	coauthorAgentName = ""
	t.Cleanup(func() { coauthorTrailer = origTrailer; coauthorHook = origHook; coauthorAgentName = origAgentName })

	if err := runCoauthor(nil, nil); err != nil {
		t.Fatalf("runCoauthor: %v", err)
	}

	found := false
	for _, c := range gitCalls {
		if c == "config --local user.name phantasos" {
			found = true
		}
		if c == "config --local user.name janus" {
			t.Errorf("resolved to janus fallback instead of the real finishing sub-agent phantasos, calls: %v", gitCalls)
		}
	}
	if !found {
		t.Errorf("expected git config user.name phantasos, got calls: %v", gitCalls)
	}
}

func TestResolveEnforcedAgentName_OtherPlatformFallback(t *testing.T) {
	// Other platforms should fall back to the tool name
	cfg := &config.Config{CodingTool: "GitHub Copilot"}
	got := resolveEnforcedAgentName(cfg, "")
	if got != "GitHub Copilot" {
		t.Errorf("got %q, want GitHub Copilot fallback", got)
	}
}

func TestAppendTokensReport_AllZeroOmitted(t *testing.T) {
	root := t.TempDir()
	if err := telemetry.Write(root, &telemetry.SnapshotResult{
		Tool: "github-copilot", Model: "gpt-4o", InputTokens: 0, OutputTokens: 0, CachedTokens: 0, TotalTokens: 0,
	}); err != nil {
		t.Fatal(err)
	}

	f, err := os.CreateTemp(t.TempDir(), "commit-msg")
	if err != nil {
		t.Fatal(err)
	}
	original := "feat: add something\nCo-authored-by: gpt-4o <gpt-4o@github.com>\n"
	f.WriteString(original)
	f.Close()

	if err := appendTokensReport(f.Name(), root); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(f.Name())
	if string(data) != original {
		t.Errorf("expected no Tokens: line for all-zero snapshot, got:\n%s", string(data))
	}
}

func TestAppendTokensReport_Available(t *testing.T) {
	root := t.TempDir()
	if err := telemetry.Write(root, &telemetry.SnapshotResult{
		InputTokens: 100, OutputTokens: 50, CachedTokens: 10, TotalTokens: 150,
	}); err != nil {
		t.Fatal(err)
	}

	f, err := os.CreateTemp(t.TempDir(), "commit-msg")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("feat: add something\nCo-authored-by: claude-sonnet-4-6 <claude-sonnet-4-6@github.com>\n")
	f.Close()

	if err := appendTokensReport(f.Name(), root); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(f.Name())
	if !strings.Contains(string(data), "Tokens: input=100 output=50 cached=10 total=150") {
		t.Errorf("Tokens line not appended, got:\n%s", string(data))
	}
}

func TestAppendTokensReport_Unavailable(t *testing.T) {
	root := t.TempDir() // no telemetry snapshot written

	f, err := os.CreateTemp(t.TempDir(), "commit-msg")
	if err != nil {
		t.Fatal(err)
	}
	original := "feat: add something\n"
	f.WriteString(original)
	f.Close()

	if err := appendTokensReport(f.Name(), root); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(f.Name())
	if string(data) != original {
		t.Errorf("expected file unchanged when telemetry unavailable, got:\n%s", string(data))
	}
}

func TestAppendTokensReport_NotDuplicated(t *testing.T) {
	root := t.TempDir()
	if err := telemetry.Write(root, &telemetry.SnapshotResult{InputTokens: 1, TotalTokens: 1}); err != nil {
		t.Fatal(err)
	}

	f, err := os.CreateTemp(t.TempDir(), "commit-msg")
	if err != nil {
		t.Fatal(err)
	}
	content := "feat: x\nTokens: input=1 output=0 cached=0 total=1\n"
	f.WriteString(content)
	f.Close()

	if err := appendTokensReport(f.Name(), root); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(f.Name())
	if strings.Count(string(data), "Tokens: ") != 1 {
		t.Errorf("Tokens line duplicated, got:\n%s", string(data))
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

// --- Generated-By trailer short-circuit (task 6.3/6.4, oneiroi-seed-script) ---

// TestRunCoauthor_TrailerMode_GeneratedByTrailer_LeavesFileByteForByteUnchanged covers
// the "Commit message declares zero tokens and no coauthor" / self-authored-commit
// posture: a commit message already carrying a `Generated-By: dreamland-oneiroi-seed`
// trailer (written by internal/oneiroi's CommitScaffold, task 6.1) must be treated as
// complete — `dreamland coauthor --trailer` must skip both the Co-authored-by append and
// the Tokens: append entirely, even when a non-zero telemetry snapshot is available that
// would otherwise cause appendTokensReport to write a line.
func TestRunCoauthor_TrailerMode_GeneratedByTrailer_LeavesFileByteForByteUnchanged(t *testing.T) {
	withCoauthorFlags(t, "", false)
	root := makeCoauthorRepo(t, config.Config{ModelID: "claude-sonnet-4-6"})
	orig := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = orig })

	// Fake non-zero telemetry snapshot — must be ignored because Generated-By is present.
	if err := telemetry.Write(root, &telemetry.SnapshotResult{
		InputTokens: 999, OutputTokens: 999, CachedTokens: 999, TotalTokens: 2997,
	}); err != nil {
		t.Fatal(err)
	}

	original := "oneiroi: seed amber-falcon (full-edit tier)\n\n" +
		"Tokens: input=0 output=0 cached=0 total=0\n" +
		"Generated-By: dreamland-oneiroi-seed\n"
	msgFile := filepath.Join(root, "COMMIT_EDITMSG")
	if err := os.WriteFile(msgFile, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	coauthorTrailer = msgFile
	t.Cleanup(func() { coauthorTrailer = "" })

	if err := runCoauthor(nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(msgFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != original {
		t.Errorf("Generated-By commit message was modified, byte-for-byte diff:\nwant:\n%q\ngot:\n%q", original, string(data))
	}
}

// TestRunCoauthor_TrailerMode_NoGeneratedByTrailer_StillAppendsCoauthorAndTokens is the
// explicit regression check that a normal (non-oneiroi-generated) commit message is
// unaffected by the new Generated-By short-circuit — it still gets the existing
// Co-authored-by:/Tokens: treatment.
func TestRunCoauthor_TrailerMode_NoGeneratedByTrailer_StillAppendsCoauthorAndTokens(t *testing.T) {
	withCoauthorFlags(t, "", false)
	root := makeCoauthorRepo(t, config.Config{ModelID: "claude-sonnet-4-6"})
	orig := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = orig })

	if err := telemetry.Write(root, &telemetry.SnapshotResult{
		InputTokens: 100, OutputTokens: 50, CachedTokens: 10, TotalTokens: 160,
	}); err != nil {
		t.Fatal(err)
	}

	msgFile := filepath.Join(root, "COMMIT_EDITMSG")
	if err := os.WriteFile(msgFile, []byte("feat: something\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	coauthorTrailer = msgFile
	t.Cleanup(func() { coauthorTrailer = "" })

	if err := runCoauthor(nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(msgFile)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "Co-authored-by: claude-sonnet-4-6") {
		t.Errorf("expected Co-authored-by trailer appended for a non-Generated-By message, got:\n%s", content)
	}
	if !strings.Contains(content, "Tokens: input=100 output=50 cached=10 total=160") {
		t.Errorf("expected Tokens line appended for a non-Generated-By message, got:\n%s", content)
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
