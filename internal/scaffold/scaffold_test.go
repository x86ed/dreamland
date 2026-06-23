package scaffold

import (
	"encoding/json"
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
