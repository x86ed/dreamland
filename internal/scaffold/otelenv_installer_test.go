package scaffold

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dreamland/internal/config"
)

func testCfg(tool, endpoint string) *config.Config {
	return &config.Config{
		CodingTool:   tool,
		OtelEndpoint: endpoint,
		ModelID:      "test-model",
	}
}

func TestInstallOtelEnv_ClaudeCode(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := testCfg("Claude Code", "http://localhost:4317")
	if err := InstallOtelEnv(root, cfg); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(root, ".claude", "scripts", "dreamland-otel-env.sh")
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("script not found: %v", err)
	}
	if !strings.Contains(string(data), "http://localhost:4317") {
		t.Errorf("script missing endpoint: %s", data)
	}
	if !strings.Contains(string(data), "OTEL_EXPORTER_OTLP_ENDPOINT") {
		t.Errorf("script missing OTEL_EXPORTER_OTLP_ENDPOINT")
	}
}

func TestInstallOtelEnv_Cursor(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := testCfg("Cursor", "http://localhost:4317")
	if err := InstallOtelEnv(root, cfg); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(root, ".cursor", "hooks", "dreamland-otel-env.sh")
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("cursor script not found: %v", err)
	}
	// Cursor script should output JSON env
	if !strings.Contains(string(data), "env") {
		t.Errorf("cursor script should output JSON env object: %s", data)
	}
}

func TestInstallOtelEnv_Copilot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := testCfg("GitHub Copilot", "http://localhost:4317")
	if err := InstallOtelEnv(root, cfg); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(root, ".vscode", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("settings.json not found: %v", err)
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("invalid settings.json: %v", err)
	}
	if _, ok := settings["github.copilot.chat.otel.enabled"]; !ok {
		t.Error("settings.json missing github.copilot.chat.otel.enabled")
	}
	// Port should be translated to 4318 for HTTP
	if endpoint, _ := settings["github.copilot.chat.otel.otlpEndpoint"].(string); !strings.Contains(endpoint, "4318") {
		t.Errorf("Copilot endpoint should use port 4318, got: %q", endpoint)
	}
}

func TestInstallOtelEnv_Copilot_PreservesExistingSettings(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".vscode"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Write pre-existing setting.
	existing := []byte(`{"editor.fontSize": 14}`)
	if err := os.WriteFile(filepath.Join(root, ".vscode", "settings.json"), existing, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := testCfg("GitHub Copilot", "http://localhost:4317")
	if err := InstallOtelEnv(root, cfg); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(root, ".vscode", "settings.json"))
	var settings map[string]any
	json.Unmarshal(data, &settings)
	if _, ok := settings["editor.fontSize"]; !ok {
		t.Error("pre-existing editor.fontSize should be preserved")
	}
	if _, ok := settings["github.copilot.chat.otel.enabled"]; !ok {
		t.Error("OTEL setting should be added")
	}
}

func TestScaffoldTelemetry_Copilot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := ScaffoldTelemetry(root, "GitHub Copilot"); err != nil {
		t.Fatal(err)
	}
	hookFile := filepath.Join(root, ".github", "hooks", "dreamland-telemetry.json")
	data, err := os.ReadFile(hookFile)
	if err != nil {
		t.Fatalf("hook file not found: %v", err)
	}
	if !strings.Contains(string(data), "agentStop") {
		t.Errorf("hook file missing agentStop: %s", data)
	}
	if !strings.Contains(string(data), "dreamland telemetry write --tool github-copilot") {
		t.Errorf("hook file missing telemetry write command: %s", data)
	}
}

func TestScaffoldTelemetry_Idempotent(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := ScaffoldTelemetry(root, "GitHub Copilot"); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(root, ".github", "hooks", "dreamland-telemetry.json"))
	if err := ScaffoldTelemetry(root, "GitHub Copilot"); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(root, ".github", "hooks", "dreamland-telemetry.json"))
	if string(before) != string(after) {
		t.Error("second ScaffoldTelemetry should not modify existing file")
	}
}

func TestInstallOtelEnv_Kiro(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := testCfg("Kiro", "http://localhost:4317")
	if err := InstallOtelEnv(root, cfg); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(root, ".kiro", "hooks", "dreamland-otel-env.sh")
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("kiro script not found: %v", err)
	}
	if !strings.Contains(string(data), "export") {
		t.Errorf("kiro script should use export statements: %s", data)
	}
	if !strings.Contains(string(data), "http://localhost:4317") {
		t.Errorf("kiro script missing endpoint: %s", data)
	}
}

func TestInstallOtelEnv_Antigravity(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := testCfg("Antigravity", "http://localhost:4317")
	if err := InstallOtelEnv(root, cfg); err != nil {
		t.Fatal(err)
	}
	hooksPath := filepath.Join(root, ".agents", "hooks.json")
	data, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf(".agents/hooks.json not found: %v", err)
	}
	if !strings.Contains(string(data), "SessionStart") {
		t.Errorf("hooks.json missing SessionStart: %s", data)
	}
	if !strings.Contains(string(data), "IDE_OTEL_IDE_NAME") {
		t.Errorf("hooks.json missing IDE_OTEL_IDE_NAME: %s", data)
	}
	if !strings.Contains(string(data), "http://localhost:4317") {
		t.Errorf("hooks.json missing endpoint: %s", data)
	}
}

func TestInstallOtelEnv_DefaultEndpoint(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Empty OtelEndpoint should default to localhost:4317
	cfg := &config.Config{CodingTool: "Claude Code", OtelEndpoint: ""}
	if err := InstallOtelEnv(root, cfg); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".claude", "scripts", "dreamland-otel-env.sh"))
	if err != nil {
		t.Fatalf("script not found: %v", err)
	}
	if !strings.Contains(string(data), "localhost:4317") {
		t.Errorf("script should use default endpoint: %s", data)
	}
}

func TestScaffoldTelemetry_NonCopilot(t *testing.T) {
	// For tools other than Copilot, ScaffoldTelemetry should return nil immediately.
	if err := ScaffoldTelemetry(t.TempDir(), "Claude Code"); err != nil {
		t.Errorf("expected nil for non-Copilot tool, got: %v", err)
	}
}

func TestInstallOtelEnv_ClaudeCode_MkdirError(t *testing.T) {
	root := t.TempDir()
	// Block .claude/scripts creation by making .claude a regular file.
	if err := os.WriteFile(filepath.Join(root, ".claude"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := testCfg("Claude Code", "http://localhost:4317")
	// Errors are logged to stderr, not returned.
	if err := InstallOtelEnv(root, cfg); err != nil {
		t.Fatal(err)
	}
}

func TestInstallOtelEnv_Cursor_MkdirError(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".cursor"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := testCfg("Cursor", "http://localhost:4317")
	if err := InstallOtelEnv(root, cfg); err != nil {
		t.Fatal(err)
	}
}

func TestInstallOtelEnv_Kiro_MkdirError(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".kiro"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := testCfg("Kiro", "http://localhost:4317")
	if err := InstallOtelEnv(root, cfg); err != nil {
		t.Fatal(err)
	}
}

func TestInstallOtelEnv_Antigravity_MkdirError(t *testing.T) {
	root := t.TempDir()
	// Block .agents dir creation.
	if err := os.WriteFile(filepath.Join(root, ".agents"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := testCfg("Antigravity", "http://localhost:4317")
	if err := InstallOtelEnv(root, cfg); err != nil {
		t.Fatal(err)
	}
}

func TestMergeVscodeSettings_MkdirError(t *testing.T) {
	root := t.TempDir()
	// Block .vscode dir creation.
	if err := os.WriteFile(filepath.Join(root, ".vscode"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := MergeVscodeSettings(root, map[string]any{"key": "val"})
	if err == nil {
		t.Error("expected error when .vscode is a file")
	}
}

func TestMergeVscodeSettings_CorruptJSON(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".vscode"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, ".vscode", "settings.json"), []byte("{bad"), 0o644)
	// Corrupt JSON resets to {} and proceeds — should not error.
	if err := MergeVscodeSettings(root, map[string]any{"new": "val"}); err != nil {
		t.Errorf("corrupt JSON should reset to {}, got error: %v", err)
	}
}

func TestInstallCopilotHookBinding_MkdirError(t *testing.T) {
	root := t.TempDir()
	// Block .github dir creation.
	if err := os.WriteFile(filepath.Join(root, ".github"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := ScaffoldTelemetry(root, "GitHub Copilot")
	if err == nil {
		t.Error("expected error when .github is a file")
	}
}

func TestInstallOtelEnv_Copilot_Error(t *testing.T) {
	root := t.TempDir()
	// Make .vscode a file so MergeVscodeSettings errors.
	if err := os.WriteFile(filepath.Join(root, ".vscode"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := testCfg("GitHub Copilot", "http://localhost:4317")
	// Error is logged to stderr, not returned.
	if err := InstallOtelEnv(root, cfg); err != nil {
		t.Fatal(err)
	}
}

func TestInstallOtelEnv_ClaudeCode_AtomicWriteError(t *testing.T) {
	root := t.TempDir()
	// Pre-create scripts dir as read-only so atomicWrite fails.
	scriptsDir := filepath.Join(root, ".claude", "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(scriptsDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(scriptsDir, 0o755) })
	cfg := testCfg("Claude Code", "http://localhost:4317")
	if err := InstallOtelEnv(root, cfg); err != nil {
		t.Fatal(err)
	}
}

func TestInstallOtelEnv_Cursor_AtomicWriteError(t *testing.T) {
	root := t.TempDir()
	hooksDir := filepath.Join(root, ".cursor", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(hooksDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(hooksDir, 0o755) })
	cfg := testCfg("Cursor", "http://localhost:4317")
	if err := InstallOtelEnv(root, cfg); err != nil {
		t.Fatal(err)
	}
}

func TestInstallOtelEnv_Kiro_AtomicWriteError(t *testing.T) {
	root := t.TempDir()
	hooksDir := filepath.Join(root, ".kiro", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(hooksDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(hooksDir, 0o755) })
	cfg := testCfg("Kiro", "http://localhost:4317")
	if err := InstallOtelEnv(root, cfg); err != nil {
		t.Fatal(err)
	}
}

func TestInstallOtelEnv_ClaudeCode_AtomicJSONMergeError(t *testing.T) {
	root := t.TempDir()
	// Let script write succeed, then block settings.json read with mode 0000.
	settingsDir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	settingsFile := filepath.Join(settingsDir, "settings.json")
	if err := os.WriteFile(settingsFile, []byte("{}"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(settingsFile, 0o644) })

	cfg := testCfg("Claude Code", "http://localhost:4317")
	if err := InstallOtelEnv(root, cfg); err != nil {
		t.Fatal(err)
	}
	// Script should exist; settings.json was blocked.
	if _, err := os.Stat(filepath.Join(settingsDir, "scripts", "dreamland-otel-env.sh")); err != nil {
		t.Errorf("script should exist: %v", err)
	}
}

func TestInstallOtelEnv_Cursor_AtomicJSONMergeError(t *testing.T) {
	root := t.TempDir()
	cursorDir := filepath.Join(root, ".cursor")
	if err := os.MkdirAll(cursorDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hooksJSON := filepath.Join(cursorDir, "hooks.json")
	if err := os.WriteFile(hooksJSON, []byte("{}"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(hooksJSON, 0o644) })

	cfg := testCfg("Cursor", "http://localhost:4317")
	if err := InstallOtelEnv(root, cfg); err != nil {
		t.Fatal(err)
	}
}

func TestRenderOtelEnvScript_UnknownPlatform(t *testing.T) {
	_, err := RenderOtelEnvScript("Unknown Platform", "http://localhost:4317")
	if err == nil {
		t.Error("expected error for unknown platform")
	}
}

func TestInstallOtelEnv_Antigravity_PreservesExisting(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := []byte(`{"version":1,"existing_key":"val"}`)
	if err := os.WriteFile(filepath.Join(root, ".agents", "hooks.json"), existing, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := testCfg("Antigravity", "http://localhost:4317")
	if err := InstallOtelEnv(root, cfg); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(root, ".agents", "hooks.json"))
	if !strings.Contains(string(data), "existing_key") {
		t.Error("pre-existing key should be preserved in hooks.json")
	}
	if !strings.Contains(string(data), "SessionStart") {
		t.Error("SessionStart should be added")
	}
}

func TestEnsureGitignoreEntry_ReadError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root; skip permission test")
	}
	root := t.TempDir()
	gitignore := filepath.Join(root, ".gitignore")
	if err := os.WriteFile(gitignore, []byte("existing\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(gitignore, 0o644) })
	err := EnsureGitignoreEntry(root, ".dreamland-session.json")
	if err == nil {
		t.Error("expected error when .gitignore is unreadable")
	}
}

func TestEnsureGitignoreEntry_OpenFileError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root; skip permission test")
	}
	root := t.TempDir()
	// Make root read-only so OpenFile(O_CREATE) fails.
	if err := os.Chmod(root, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(root, 0o755) })
	err := EnsureGitignoreEntry(root, ".dreamland-session.json")
	if err == nil {
		t.Error("expected error when parent dir is read-only")
	}
}

func TestEnsureGitignoreEntry(t *testing.T) {
	root := t.TempDir()
	entry := ".dreamland-session.json"

	// First call creates the file.
	if err := EnsureGitignoreEntry(root, entry); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), entry) {
		t.Errorf(".gitignore missing entry: %s", data)
	}

	// Second call is idempotent.
	if err := EnsureGitignoreEntry(root, entry); err != nil {
		t.Fatal(err)
	}
	data2, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	if strings.Count(string(data2), entry) != 1 {
		t.Errorf("entry should appear exactly once, got: %s", data2)
	}
}
