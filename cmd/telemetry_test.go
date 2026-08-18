package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dreamland/internal/config"
	"dreamland/internal/telemetry"
)

// telemetryGitRepo creates a temp dir with .git, changes into it, and cleans up.
func telemetryGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git", "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	orig, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	return root
}

// execCLI executes the root cobra command with the given args, capturing stdout/stderr.
func execCLI(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})
	rootCmd.SetArgs(args)
	err = rootCmd.Execute()
	return outBuf.String(), errBuf.String(), err
}

func TestTelemetryWriteUnknownTool(t *testing.T) {
	telemetryGitRepo(t)
	_, _, err := execCLI(t, "telemetry", "write", "--tool", "unknown-tool")
	if err == nil {
		t.Error("expected error for unknown tool, got nil")
	}
}

func TestTelemetrySnapshotMissingFile(t *testing.T) {
	telemetryGitRepo(t)
	out, _, err := execCLI(t, "telemetry", "snapshot")
	if err != nil {
		t.Fatalf("snapshot with no file: %v", err)
	}
	if out != "" {
		t.Errorf("expected no output when session file absent, got: %q", out)
	}
}

func TestTelemetrySnapshotJSON(t *testing.T) {
	root := telemetryGitRepo(t)
	if err := telemetry.Write(root, &telemetry.SnapshotResult{
		Tool:         "claude-code",
		Model:        "claude-sonnet-4-6",
		InputTokens:  1000,
		OutputTokens: 200,
	}); err != nil {
		t.Fatal(err)
	}

	out, _, err := execCLI(t, "telemetry", "snapshot", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "claude-code") {
		t.Errorf("snapshot output missing tool: %q", out)
	}
	if !strings.Contains(out, "claude-sonnet-4-6") {
		t.Errorf("snapshot output missing model: %q", out)
	}
}

func TestTelemetrySnapshotTrailers(t *testing.T) {
	root := telemetryGitRepo(t)
	if err := telemetry.Write(root, &telemetry.SnapshotResult{
		Tool:         "codex",
		Model:        "o4-mini",
		InputTokens:  500,
		OutputTokens: 100,
	}); err != nil {
		t.Fatal(err)
	}

	out, _, err := execCLI(t, "telemetry", "snapshot", "--format", "trailers")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "AI-Tool: codex") {
		t.Errorf("trailers missing AI-Tool: %q", out)
	}
	if !strings.Contains(out, "AI-Model: o4-mini") {
		t.Errorf("trailers missing AI-Model: %q", out)
	}
	if !strings.Contains(out, "AI-InputTokens: 500") {
		t.Errorf("trailers missing AI-InputTokens: %q", out)
	}
}

func TestTelemetryReset(t *testing.T) {
	root := telemetryGitRepo(t)
	sessionPath := filepath.Join(root, ".dreamland-session.json")
	if err := os.WriteFile(sessionPath, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := execCLI(t, "telemetry", "reset"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sessionPath); !os.IsNotExist(err) {
		t.Error("session file should be deleted after reset")
	}
}

func TestTelemetryResetNoFile(t *testing.T) {
	telemetryGitRepo(t)
	if _, _, err := execCLI(t, "telemetry", "reset"); err != nil {
		t.Errorf("reset with no file should not error: %v", err)
	}
}

func TestTelemetryInstallAndUninstall(t *testing.T) {
	root := telemetryGitRepo(t)
	if _, _, err := execCLI(t, "telemetry", "install"); err != nil {
		t.Fatalf("install: %v", err)
	}
	hookPath := filepath.Join(root, ".git", "hooks", "commit-msg")
	if _, err := os.Stat(hookPath); err != nil {
		t.Fatalf("commit-msg hook not found: %v", err)
	}
	if _, _, err := execCLI(t, "telemetry", "uninstall"); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if _, err := os.Stat(hookPath); !os.IsNotExist(err) {
		t.Error("hook file should be removed after uninstall")
	}
}

func TestFormatTrailers_OmitsZeroFields(t *testing.T) {
	snap := &telemetry.SnapshotResult{
		Tool:  "cursor",
		Model: "gpt-4o",
	}
	out := formatTrailers(snap)
	if strings.Contains(out, "AI-InputTokens") {
		t.Error("AI-InputTokens should be omitted when zero")
	}
	if strings.Contains(out, "AI-ThinkingEffort") {
		t.Error("AI-ThinkingEffort should be omitted when empty")
	}
	if !strings.Contains(out, "AI-Tool: cursor") {
		t.Error("AI-Tool should be present")
	}
	if strings.Contains(out, "AI-Agent") {
		t.Error("AI-Agent should be omitted when empty")
	}
}

func TestFormatTrailers_IncludesAgent(t *testing.T) {
	snap := &telemetry.SnapshotResult{
		Tool:  "claude-code",
		Agent: "phobetor",
		Model: "claude-sonnet-5",
	}
	out := formatTrailers(snap)
	if !strings.Contains(out, "AI-Agent: phobetor") {
		t.Errorf("AI-Agent trailer not present, got:\n%s", out)
	}
}

func TestTelemetryWriteClaudeCode_AgentFromSubagentType(t *testing.T) {
	root := telemetryGitRepo(t)
	rootCmd.SetIn(strings.NewReader(`{"tool_input":{"subagent_type":"phobetor"}}`))
	t.Cleanup(func() { rootCmd.SetIn(nil) })
	if _, _, err := execCLI(t, "telemetry", "write", "--tool", "claude-code"); err != nil {
		t.Fatalf("expected success: %v", err)
	}
	snap, err := telemetry.Read(root)
	if err != nil || snap == nil {
		t.Fatalf("expected snapshot to be written, err=%v snap=%v", err, snap)
	}
	if snap.Agent != "phobetor" {
		t.Errorf("got Agent=%q, want phobetor", snap.Agent)
	}
}

func TestTelemetryWriteClaudeCode_AgentDefaultsToJanus(t *testing.T) {
	root := telemetryGitRepo(t)
	rootCmd.SetIn(strings.NewReader(`{}`))
	t.Cleanup(func() { rootCmd.SetIn(nil) })
	if _, _, err := execCLI(t, "telemetry", "write", "--tool", "claude-code"); err != nil {
		t.Fatalf("expected success: %v", err)
	}
	snap, err := telemetry.Read(root)
	if err != nil || snap == nil {
		t.Fatalf("expected snapshot to be written, err=%v snap=%v", err, snap)
	}
	if snap.Agent != "janus" {
		t.Errorf("got Agent=%q, want janus", snap.Agent)
	}
}

// TestTelemetryWriteClaudeCode_AgentNameFlagOverridesPayload is the regression test for
// the bug where `dreamland telemetry write` had no --agent-name override at all: a bare
// Stop hook payload never carries agent identity (only SubagentStop does, per Anthropic's
// hooks reference), so every subagent-internal turn-boundary telemetry write silently fell
// through to the "janus" default, independent of whatever coauthor had already correctly
// set in git config via its own --agent-name. --agent-name must win over the (here, empty)
// payload-derived value, mirroring commit/coauthor's existing precedence.
func TestTelemetryWriteClaudeCode_AgentNameFlagOverridesPayload(t *testing.T) {
	root := telemetryGitRepo(t)
	rootCmd.SetIn(strings.NewReader(`{"hook_event_name":"Stop","session_id":"s1"}`))
	t.Cleanup(func() { rootCmd.SetIn(nil) })
	// telemetryAgentName is a package-level var bound by StringVar; pflag only writes it
	// when --agent-name is actually passed, so it must be explicitly reset after this test
	// or its value leaks into any later test that omits the flag.
	t.Cleanup(func() { telemetryAgentName = "" })
	if _, _, err := execCLI(t, "telemetry", "write", "--tool", "claude-code", "--agent-name", "morpheus"); err != nil {
		t.Fatalf("expected success: %v", err)
	}
	snap, err := telemetry.Read(root)
	if err != nil || snap == nil {
		t.Fatalf("expected snapshot to be written, err=%v snap=%v", err, snap)
	}
	if snap.Agent != "morpheus" {
		t.Errorf("got Agent=%q, want morpheus (--agent-name override)", snap.Agent)
	}
}

// TestTelemetryWriteClaudeCode_AgentNameFlagAbsent_PayloadStillWorks is the regression
// half of the above: without --agent-name, payload-derived resolution is unaffected.
func TestTelemetryWriteClaudeCode_AgentNameFlagAbsent_PayloadStillWorks(t *testing.T) {
	root := telemetryGitRepo(t)
	rootCmd.SetIn(strings.NewReader(`{"tool_input":{"subagent_type":"nyx"}}`))
	t.Cleanup(func() { rootCmd.SetIn(nil) })
	if _, _, err := execCLI(t, "telemetry", "write", "--tool", "claude-code"); err != nil {
		t.Fatalf("expected success: %v", err)
	}
	snap, err := telemetry.Read(root)
	if err != nil || snap == nil {
		t.Fatalf("expected snapshot to be written, err=%v snap=%v", err, snap)
	}
	if snap.Agent != "nyx" {
		t.Errorf("got Agent=%q, want nyx (payload-derived, no override given)", snap.Agent)
	}
}

// TestTelemetryWriteClaudeCode_UnregisteredAgentNameFlagIgnored confirms an unrecognized
// --agent-name value does not bypass the allow-list protection from task 1.2 — it must not
// leak an unregistered identity into the AI-Agent trailer via the override path either.
func TestTelemetryWriteClaudeCode_UnregisteredAgentNameFlagIgnored(t *testing.T) {
	root := telemetryGitRepo(t)
	rootCmd.SetIn(strings.NewReader(`{}`))
	t.Cleanup(func() { rootCmd.SetIn(nil) })
	t.Cleanup(func() { telemetryAgentName = "" })
	if _, _, err := execCLI(t, "telemetry", "write", "--tool", "claude-code", "--agent-name", "not-a-real-agent"); err != nil {
		t.Fatalf("expected success: %v", err)
	}
	snap, err := telemetry.Read(root)
	if err != nil || snap == nil {
		t.Fatalf("expected snapshot to be written, err=%v snap=%v", err, snap)
	}
	if snap.Agent != "janus" {
		t.Errorf("got Agent=%q, want janus (unregistered --agent-name override must not leak through)", snap.Agent)
	}
}

func TestTelemetryWriteClaudeCode_UnrecognizedAgentFallsBackToJanus(t *testing.T) {
	root := telemetryGitRepo(t)
	rootCmd.SetIn(strings.NewReader(`{"tool_input":{"subagent_type":"not-a-real-agent"}}`))
	t.Cleanup(func() { rootCmd.SetIn(nil) })
	if _, _, err := execCLI(t, "telemetry", "write", "--tool", "claude-code"); err != nil {
		t.Fatalf("expected success: %v", err)
	}
	snap, err := telemetry.Read(root)
	if err != nil || snap == nil {
		t.Fatalf("expected snapshot to be written, err=%v snap=%v", err, snap)
	}
	if snap.Agent != "janus" {
		t.Errorf("got Agent=%q, want janus (unrecognized value must not leak through)", snap.Agent)
	}
}

func TestTelemetryWriteKnownToolCLI(t *testing.T) {
	// Exercise runTelemetryWrite success path via CLI with empty stdin.
	telemetryGitRepo(t)
	rootCmd.SetIn(strings.NewReader("{}"))
	t.Cleanup(func() { rootCmd.SetIn(nil) })
	_, _, err := execCLI(t, "telemetry", "write", "--tool", "claude-code")
	if err != nil {
		t.Fatalf("expected success with empty stdin: %v", err)
	}
}

func TestTelemetryWriteKiroPhaseCLI(t *testing.T) {
	// Exercise the kiro --phase path in runTelemetryWrite.
	telemetryGitRepo(t)
	rootCmd.SetIn(strings.NewReader("{}"))
	t.Cleanup(func() { rootCmd.SetIn(nil) })
	_, _, err := execCLI(t, "telemetry", "write", "--tool", "kiro", "--phase", "start")
	if err != nil {
		t.Fatalf("expected success for kiro start phase: %v", err)
	}
}

func TestTelemetryReset_OsGetwd_Error(t *testing.T) {
	telemetryGitRepo(t)
	orig := osGetwd
	osGetwd = func() (string, error) { return "", errors.New("getwd failed") }
	t.Cleanup(func() { osGetwd = orig })
	_, _, err := execCLI(t, "telemetry", "reset")
	if err == nil {
		t.Error("expected error when osGetwd fails")
	}
}

func TestTelemetryInstall_OsGetwd_Error(t *testing.T) {
	telemetryGitRepo(t)
	orig := osGetwd
	osGetwd = func() (string, error) { return "", errors.New("getwd failed") }
	t.Cleanup(func() { osGetwd = orig })
	_, _, err := execCLI(t, "telemetry", "install")
	if err == nil {
		t.Error("expected error when osGetwd fails")
	}
}

func TestTelemetryUninstall_OsGetwd_Error(t *testing.T) {
	telemetryGitRepo(t)
	orig := osGetwd
	osGetwd = func() (string, error) { return "", errors.New("getwd failed") }
	t.Cleanup(func() { osGetwd = orig })
	_, _, err := execCLI(t, "telemetry", "uninstall")
	if err == nil {
		t.Error("expected error when osGetwd fails")
	}
}

func TestTelemetryReset_NoGitRepo(t *testing.T) {
	root := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(root)
	t.Cleanup(func() { os.Chdir(orig) })
	_, _, err := execCLI(t, "telemetry", "reset")
	if err != nil {
		t.Errorf("expected nil outside git repo: %v", err)
	}
}

func TestTelemetrySnapshotMaxAge(t *testing.T) {
	// Write an old snapshot; snapshot --max-age 1ns should warn but not error.
	root := telemetryGitRepo(t)
	if err := telemetry.Write(root, &telemetry.SnapshotResult{
		Tool:  "cursor",
		Model: "gpt-4o",
	}); err != nil {
		t.Fatal(err)
	}
	_, stderr, err := execCLI(t, "telemetry", "snapshot", "--max-age", "1ns")
	if err != nil {
		t.Fatalf("expected no error: %v", err)
	}
	if !strings.Contains(stderr, "warning") {
		t.Errorf("expected max-age warning in stderr, got: %q", stderr)
	}
}

func TestRegisterCollectors(t *testing.T) {
	expected := []string{"claude-code", "codex", "cursor", "antigravity", "github-copilot"}
	for _, name := range expected {
		if _, ok := telemetry.Registry[name]; !ok {
			t.Errorf("tool %q not registered", name)
		}
	}
}

// errReader is an io.Reader that always returns an error, used to test stdin failure paths.
type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("forced read error") }

func TestTelemetryWrite_NoGitRepo(t *testing.T) {
	root := t.TempDir() // no .git
	orig, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	rootCmd.SetIn(strings.NewReader("{}"))
	t.Cleanup(func() { rootCmd.SetIn(nil) })
	_, _, err := execCLI(t, "telemetry", "write", "--tool", "claude-code")
	if err == nil {
		t.Error("expected error when not in a git repo")
	}
}

func TestTelemetryWrite_CollectError(t *testing.T) {
	telemetryGitRepo(t)
	rootCmd.SetIn(errReader{})
	t.Cleanup(func() { rootCmd.SetIn(nil) })
	_, _, err := execCLI(t, "telemetry", "write", "--tool", "claude-code")
	if err == nil {
		t.Error("expected error when stdin fails")
	}
}

func TestTelemetryInstall_NoGitRepo(t *testing.T) {
	root := t.TempDir() // no .git
	orig, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	_, _, err := execCLI(t, "telemetry", "install")
	if err == nil {
		t.Error("expected error outside git repo")
	}
}

func TestTelemetryInstall_HookError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root; skip permission test")
	}
	root := telemetryGitRepo(t)
	hooksDir := filepath.Join(root, ".git", "hooks")
	if err := os.Chmod(hooksDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(hooksDir, 0o755) })
	_, _, err := execCLI(t, "telemetry", "install")
	if err == nil {
		t.Error("expected error when hooks dir is read-only")
	}
}

func TestTelemetryUninstall_NoGitRepo(t *testing.T) {
	root := t.TempDir() // no .git
	orig, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	_, _, err := execCLI(t, "telemetry", "uninstall")
	if err == nil {
		t.Error("expected error outside git repo")
	}
}

func TestTelemetryReset_RemoveError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root; skip permission test")
	}
	root := telemetryGitRepo(t)
	sessionPath := filepath.Join(root, ".dreamland-session.json")
	if err := os.WriteFile(sessionPath, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o755) })
	_, _, err := execCLI(t, "telemetry", "reset")
	if err == nil {
		t.Error("expected error when session file cannot be removed")
	}
}

// Compile-time check that config package is importable.
var _ = config.Config{}
