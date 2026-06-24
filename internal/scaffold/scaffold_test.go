package scaffold

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fakeGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestInstall_ClaudeCode(t *testing.T) {
	root := fakeGitRepo(t)
	results, err := Install(Config{RepoRoot: root, CodingTool: "Claude Code"})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	agentDir := filepath.Join(root, ".claude", "agents")
	agents := []string{"orchestrator.md", "spec-writer.md", "implementer.md", "tester.md", "pr-closer.md"}
	for _, a := range agents {
		if _, err := os.Stat(filepath.Join(agentDir, a)); err != nil {
			t.Errorf("missing agent file %s: %v", a, err)
		}
	}

	settingsPath := filepath.Join(root, ".claude", "settings.json")
	if _, err := os.Stat(settingsPath); err != nil {
		t.Errorf("settings.json not created: %v", err)
	}

	hasInstalled := false
	for _, r := range results {
		if r.Action == "installed" {
			hasInstalled = true
			break
		}
	}
	if !hasInstalled {
		t.Error("expected at least one 'installed' result")
	}
}

func TestInstall_Codex(t *testing.T) {
	root := fakeGitRepo(t)
	_, err := Install(Config{RepoRoot: root, CodingTool: "Codex CLI"})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	agentDir := filepath.Join(root, ".codex", "agents")
	for _, a := range []string{"orchestrator.toml", "spec-writer.toml", "implementer.toml", "tester.toml", "pr-closer.toml"} {
		if _, err := os.Stat(filepath.Join(agentDir, a)); err != nil {
			t.Errorf("missing agent file %s: %v", a, err)
		}
	}

	hooksPath := filepath.Join(root, ".codex", "hooks.json")
	if _, err := os.Stat(hooksPath); err != nil {
		t.Errorf("hooks.json not created: %v", err)
	}
}

func TestInstall_Cursor(t *testing.T) {
	root := fakeGitRepo(t)
	_, err := Install(Config{RepoRoot: root, CodingTool: "Cursor"})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	rulesDir := filepath.Join(root, ".cursor", "rules")
	for _, a := range []string{"orchestrator.mdc", "spec-writer.mdc", "implementer.mdc", "tester.mdc", "pr-closer.mdc"} {
		if _, err := os.Stat(filepath.Join(rulesDir, a)); err != nil {
			t.Errorf("missing agent file %s: %v", a, err)
		}
	}

	hooksPath := filepath.Join(root, ".cursor", "hooks.json")
	data, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("hooks.json not created: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("hooks.json invalid JSON: %v", err)
	}
	if _, ok := m["version"]; !ok {
		t.Error("hooks.json missing 'version' field")
	}
}

func TestInstall_Kiro(t *testing.T) {
	root := fakeGitRepo(t)
	_, err := Install(Config{RepoRoot: root, CodingTool: "Kiro"})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	steeringDir := filepath.Join(root, ".kiro", "steering")
	for _, a := range []string{"orchestrator.md", "spec-writer.md", "implementer.md", "tester.md", "pr-closer.md"} {
		data, err := os.ReadFile(filepath.Join(steeringDir, a))
		if err != nil {
			t.Errorf("missing steering file %s: %v", a, err)
			continue
		}
		if !strings.Contains(string(data), "inclusion: always") {
			t.Errorf("steering file %s missing 'inclusion: always' frontmatter", a)
		}
	}

	agentJSON := filepath.Join(root, ".kiro", "agent.json")
	data, err := os.ReadFile(agentJSON)
	if err != nil {
		t.Fatalf("agent.json not created: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("agent.json invalid JSON: %v", err)
	}
	hooks, ok := m["hooks"].(map[string]interface{})
	if !ok {
		t.Fatal("agent.json missing 'hooks' object")
	}
	if _, ok := hooks["agentSpawn"]; !ok {
		t.Error("agent.json missing 'agentSpawn' hook")
	}
}

func TestInstall_GitHubCopilot(t *testing.T) {
	root := fakeGitRepo(t)
	_, err := Install(Config{RepoRoot: root, CodingTool: "GitHub Copilot"})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	copilotDir := filepath.Join(root, ".github", "agents")
	for _, a := range []string{
		"orchestrator.agent.md",
		"spec-writer.agent.md",
		"implementer.agent.md",
		"tester.agent.md",
		"pr-closer.agent.md",
	} {
		if _, err := os.Stat(filepath.Join(copilotDir, a)); err != nil {
			t.Errorf("missing agent file %s: %v", a, err)
		}
	}

	tasksPath := filepath.Join(root, ".vscode", "tasks.json")
	data, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatalf(".vscode/tasks.json not created: %v", err)
	}
	if !strings.Contains(string(data), "folderOpen") {
		t.Error(".vscode/tasks.json missing folderOpen session-start task")
	}
	if !strings.Contains(string(data), "dreamland: end of turn") {
		t.Error(".vscode/tasks.json missing end-of-turn task")
	}
}

func TestInstall_Antigravity(t *testing.T) {
	root := fakeGitRepo(t)
	_, err := Install(Config{RepoRoot: root, CodingTool: "Antigravity"})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	skillsDir := filepath.Join(root, ".agents", "skills")
	for _, skill := range []string{"orchestrator", "spec-writer", "implementer", "tester", "pr-closer"} {
		skillFile := filepath.Join(skillsDir, skill, "SKILL.md")
		data, err := os.ReadFile(skillFile)
		if err != nil {
			t.Errorf("missing skill file %s: %v", skillFile, err)
			continue
		}
		if !strings.HasPrefix(string(data), "---\n") {
			t.Errorf("skill file %s missing YAML frontmatter", skill)
		}
		if !strings.Contains(string(data), "name: "+skill) {
			t.Errorf("skill file %s missing 'name: %s' in frontmatter", skill, skill)
		}
	}
}

func TestInstall_SkipsExisting(t *testing.T) {
	root := fakeGitRepo(t)

	// Pre-create one agent file.
	agentDir := filepath.Join(root, ".claude", "agents")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	existingPath := filepath.Join(agentDir, "orchestrator.md")
	if err := os.WriteFile(existingPath, []byte("existing content"), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := Install(Config{RepoRoot: root, CodingTool: "Claude Code"})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	for _, r := range results {
		if r.Path == existingPath && r.Action != "skipped (already exists)" {
			t.Errorf("expected skipped for existing file, got %q", r.Action)
		}
	}

	// File must be unchanged.
	data, _ := os.ReadFile(existingPath)
	if string(data) != "existing content" {
		t.Error("existing file was overwritten without --force")
	}
}

func TestInstall_ForceOverwrites(t *testing.T) {
	root := fakeGitRepo(t)

	agentDir := filepath.Join(root, ".claude", "agents")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	existingPath := filepath.Join(agentDir, "orchestrator.md")
	if err := os.WriteFile(existingPath, []byte("existing content"), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := Install(Config{RepoRoot: root, CodingTool: "Claude Code", Force: true})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	for _, r := range results {
		if r.Path == existingPath {
			if r.Action != "installed (forced)" {
				t.Errorf("expected 'installed (forced)', got %q", r.Action)
			}
		}
	}

	data, _ := os.ReadFile(existingPath)
	if string(data) == "existing content" {
		t.Error("existing file not overwritten with --force")
	}
}

func TestAtomicJSONMerge_AbsentFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "sub", "settings.json")
	patch := []byte(`{"hooks":{"SessionStart":[{"command":"dreamland version-bump"}]}}`)

	if err := atomicJSONMerge(target, patch); err != nil {
		t.Fatalf("atomicJSONMerge: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := m["hooks"]; !ok {
		t.Error("expected 'hooks' key in merged output")
	}
}

func TestAtomicJSONMerge_PreservesExistingKeys(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "settings.json")

	existing := []byte(`{"theme":"dark","hooks":{}}`)
	if err := os.WriteFile(target, existing, 0o644); err != nil {
		t.Fatal(err)
	}

	patch := []byte(`{"hooks":{"SessionStart":[{"command":"dreamland version-bump"}]}}`)
	if err := atomicJSONMerge(target, patch); err != nil {
		t.Fatalf("atomicJSONMerge: %v", err)
	}

	data, _ := os.ReadFile(target)
	var m map[string]interface{}
	json.Unmarshal(data, &m)

	if m["theme"] != "dark" {
		t.Errorf("'theme' key was lost, got: %v", m["theme"])
	}
}

func TestAtomicJSONMerge_NoDuplicateHooks(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "settings.json")

	patch := []byte(`{"hooks":{"SessionStart":[{"command":"dreamland version-bump"}]}}`)
	if err := atomicJSONMerge(target, patch); err != nil {
		t.Fatal(err)
	}
	// Merge again.
	if err := atomicJSONMerge(target, patch); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(target)
	// The merged JSON should contain only one "dreamland version-bump" in SessionStart.
	count := strings.Count(string(data), "dreamland version-bump")
	if count != 1 {
		t.Errorf("hook entry duplicated, count = %d", count)
	}
}

func TestInstall_UnknownTool(t *testing.T) {
	root := fakeGitRepo(t)
	_, err := Install(Config{RepoRoot: root, CodingTool: "UnknownTool"})
	if err == nil {
		t.Fatal("expected error for unknown coding tool")
	}
}

func TestBindCursor_EnsuresVersion1(t *testing.T) {
	root := fakeGitRepo(t)
	// Write a cursor hooks.json that is missing the version field.
	cursorDir := filepath.Join(root, ".cursor")
	if err := os.MkdirAll(cursorDir, 0o755); err != nil {
		t.Fatal(err)
	}
	existing := []byte(`{"hooks":{"sessionStart":[]}}`)
	if err := os.WriteFile(filepath.Join(cursorDir, "hooks.json"), existing, 0o644); err != nil {
		t.Fatal(err)
	}

	patch, err := fs.ReadFile(TemplateFS, "templates/hooks/bindings/cursor/hooks.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bindCursor(root, patch, false); err != nil {
		t.Fatalf("bindCursor: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(cursorDir, "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if v, ok := m["version"]; !ok || v != float64(1) {
		t.Errorf("expected version=1, got %v (ok=%v)", v, ok)
	}
}

func TestInstall_Antigravity_Force(t *testing.T) {
	root := fakeGitRepo(t)

	// First install — should write hooks.json.
	results, err := Install(Config{RepoRoot: root, CodingTool: "Antigravity"})
	if err != nil {
		t.Fatalf("first Install: %v", err)
	}
	hasInstalled := false
	for _, r := range results {
		if r.Action == "installed" {
			hasInstalled = true
		}
	}
	if !hasInstalled {
		t.Error("expected at least one 'installed' result on first run")
	}

	// Second install without force — hooks.json should be skipped.
	results2, err := Install(Config{RepoRoot: root, CodingTool: "Antigravity", Force: false})
	if err != nil {
		t.Fatalf("second Install: %v", err)
	}
	for _, r := range results2 {
		if strings.Contains(r.Path, "hooks.json") && r.Action != "skipped (already exists)" {
			t.Errorf("expected hooks.json skipped on re-run, got action=%q", r.Action)
		}
	}

	// Third install with force — should overwrite.
	results3, err := Install(Config{RepoRoot: root, CodingTool: "Antigravity", Force: true})
	if err != nil {
		t.Fatalf("forced Install: %v", err)
	}
	hasForced := false
	for _, r := range results3 {
		if strings.Contains(r.Path, "hooks.json") && r.Action == "installed (forced)" {
			hasForced = true
		}
	}
	if !hasForced {
		t.Error("expected 'installed (forced)' result on force re-run")
	}
}

func TestInstallSkills_ForceOverwrites(t *testing.T) {
	root := fakeGitRepo(t)

	// First install.
	if _, err := Install(Config{RepoRoot: root, CodingTool: "Antigravity"}); err != nil {
		t.Fatalf("first Install: %v", err)
	}

	orchestratorPath := filepath.Join(root, ".agents", "skills", "orchestrator", "SKILL.md")
	if err := os.WriteFile(orchestratorPath, []byte("custom content"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Re-install without force — custom content preserved.
	if _, err := Install(Config{RepoRoot: root, CodingTool: "Antigravity", Force: false}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(orchestratorPath)
	if string(data) != "custom content" {
		t.Error("expected custom content preserved without --force")
	}

	// Re-install with force — file overwritten.
	if _, err := Install(Config{RepoRoot: root, CodingTool: "Antigravity", Force: true}); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(orchestratorPath)
	if string(data) == "custom content" {
		t.Error("expected file overwritten with --force")
	}
}

func TestAtomicJSONMerge_InvalidJSONPatch(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "out.json")
	err := atomicJSONMerge(target, []byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON patch")
	}
}

func TestAtomicJSONMerge_CreatesParentDir(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "sub", "dir", "out.json")
	patch := []byte(`{"key":"value"}`)
	if err := atomicJSONMerge(target, patch); err != nil {
		t.Fatalf("atomicJSONMerge: %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if !strings.Contains(string(data), "value") {
		t.Errorf("unexpected content: %s", data)
	}
}

func TestInstall_GitHubCopilot_MergesVSCodeTasks(t *testing.T) {
	root := fakeGitRepo(t)

	// Pre-existing tasks.json with a user task.
	vscodeDir := filepath.Join(root, ".vscode")
	if err := os.MkdirAll(vscodeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	existing := []byte(`{"version":"2.0.0","tasks":[{"label":"my-build","type":"shell","command":"go build ./..."}]}`)
	if err := os.WriteFile(filepath.Join(vscodeDir, "tasks.json"), existing, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Install(Config{RepoRoot: root, CodingTool: "GitHub Copilot"}); err != nil {
		t.Fatalf("Install: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(vscodeDir, "tasks.json"))
	if err != nil {
		t.Fatal(err)
	}
	// Both existing and dreamland tasks should be present.
	if !strings.Contains(string(data), "my-build") {
		t.Error("pre-existing task was lost after merge")
	}
	if !strings.Contains(string(data), "dreamland: session start") {
		t.Error("dreamland session-start task missing after merge")
	}
}

// --- Additional tests to improve coverage ---

func TestInstallFlatAgents_WriteFileError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root; skip permission test")
	}
	root := fakeGitRepo(t)

	// Create the target agents dir then make it unwritable.
	agentDir := filepath.Join(root, ".claude", "agents")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(agentDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(agentDir, 0o755) })

	_, err := Install(Config{RepoRoot: root, CodingTool: "Claude Code", Force: true})
	if err == nil {
		t.Fatal("expected error when agent dir is unwritable")
	}
}

func TestInstallSkills_WriteFileError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root; skip permission test")
	}
	root := fakeGitRepo(t)

	// Create the skills dir and one skill subdir, then make it unwritable.
	skillsDir := filepath.Join(root, ".agents", "skills")
	orchestratorDir := filepath.Join(skillsDir, "orchestrator")
	if err := os.MkdirAll(orchestratorDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(orchestratorDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(orchestratorDir, 0o755) })

	_, err := Install(Config{RepoRoot: root, CodingTool: "Antigravity", Force: true})
	if err == nil {
		t.Fatal("expected error when skill dir is unwritable")
	}
}

func TestAtomicJSONMerge_MkdirAllError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root; skip permission test")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	target := filepath.Join(dir, "sub", "out.json")
	patch := []byte(`{"key":"value"}`)
	if err := atomicJSONMerge(target, patch); err == nil {
		t.Fatal("expected error when parent dir is unwritable")
	}
}

func TestAtomicJSONMerge_ReadFileNonNotExistError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root; skip permission test")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "out.json")

	// Create file then make it unreadable.
	if err := os.WriteFile(target, []byte(`{"key":"value"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(target, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(target, 0o644) })

	patch := []byte(`{"other":"value"}`)
	if err := atomicJSONMerge(target, patch); err == nil {
		t.Fatal("expected error when target file is unreadable")
	}
}

func TestAtomicJSONMerge_CorruptExistingJSON(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.json")

	// Write corrupt JSON — atomicJSONMerge should treat it as fresh map.
	if err := os.WriteFile(target, []byte("not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}

	patch := []byte(`{"key":"value"}`)
	if err := atomicJSONMerge(target, patch); err != nil {
		t.Fatalf("expected nil error for corrupt existing JSON, got: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "value") {
		t.Errorf("expected merged output to contain patch, got: %s", data)
	}
}

func TestAtomicJSONMerge_CreateTempError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root; skip permission test")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "out.json")

	// Write valid JSON so ReadFile succeeds.
	if err := os.WriteFile(target, []byte(`{"existing":"value"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Make the dir unwritable after file exists — CreateTemp will fail.
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	patch := []byte(`{"key":"value"}`)
	if err := atomicJSONMerge(target, patch); err == nil {
		t.Fatal("expected error when temp file cannot be created")
	}
}

func TestBindClaudeCode_MergeError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root; skip permission test")
	}
	root := t.TempDir()

	// Make .claude dir unwritable so atomicJSONMerge's MkdirAll fails.
	claudeDir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claudeDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(claudeDir, 0o755) })

	patch := []byte(`{"hooks":{}}`)
	_, err := bindClaudeCode(root, patch, false)
	if err == nil {
		t.Fatal("expected error from bindClaudeCode when dir is unwritable")
	}
}

func TestBindCodex_MergeError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root; skip permission test")
	}
	root := t.TempDir()

	// Make .codex dir unwritable.
	codexDir := filepath.Join(root, ".codex")
	if err := os.MkdirAll(codexDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(codexDir, 0o755) })

	patch := []byte(`{"hooks":{}}`)
	_, err := bindCodex(root, patch, false)
	if err == nil {
		t.Fatal("expected error from bindCodex when dir is unwritable")
	}
}

func TestBindCursor_MergeError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root; skip permission test")
	}
	root := t.TempDir()

	// Make .cursor dir unwritable.
	cursorDir := filepath.Join(root, ".cursor")
	if err := os.MkdirAll(cursorDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(cursorDir, 0o755) })

	patch := []byte(`{"hooks":{}}`)
	_, err := bindCursor(root, patch, false)
	if err == nil {
		t.Fatal("expected error from bindCursor when dir is unwritable")
	}
}

func TestBindCursor_VersionInjected_WhenAbsent(t *testing.T) {
	root := fakeGitRepo(t)
	// Patch without "version" field — bindCursor should inject version:1.
	patch := []byte(`{"hooks":{}}`)
	result, err := bindCursor(root, patch, false)
	if err != nil {
		t.Fatalf("bindCursor: %v", err)
	}
	_ = result

	data, err := os.ReadFile(filepath.Join(root, ".cursor", "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	v, ok := m["version"]
	if !ok || v != float64(1) {
		t.Errorf("expected version=1 injected, got %v (ok=%v)", v, ok)
	}
}

func TestBindKiro_MergeError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root; skip permission test")
	}
	root := t.TempDir()

	// Make .kiro dir unwritable.
	kiroDir := filepath.Join(root, ".kiro")
	if err := os.MkdirAll(kiroDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(kiroDir, 0o755) })

	patch := []byte(`{"hooks":{}}`)
	_, err := bindKiro(root, patch, false)
	if err == nil {
		t.Fatal("expected error from bindKiro when dir is unwritable")
	}
}

func TestBindGitHubCopilot_MergeError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root; skip permission test")
	}
	root := t.TempDir()

	// Make .vscode dir unwritable.
	vscodeDir := filepath.Join(root, ".vscode")
	if err := os.MkdirAll(vscodeDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(vscodeDir, 0o755) })

	patch := []byte(`{"tasks":[]}`)
	_, err := bindGitHubCopilot(root, patch, false)
	if err == nil {
		t.Fatal("expected error from bindGitHubCopilot when dir is unwritable")
	}
}
