package scaffold

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// withTemplateFS temporarily swaps the package-level TemplateFS seam (already
// exported as an fs.FS interface var for this purpose) so tests can simulate
// missing/unreadable template files without touching the real embedded
// templates. Restored automatically via t.Cleanup.
func withTemplateFS(t *testing.T, fsys fs.FS) {
	t.Helper()
	orig := TemplateFS
	TemplateFS = fsys
	t.Cleanup(func() { TemplateFS = orig })
}

// failReadFS wraps an fstest.MapFS so that fs.ReadDir still lists files
// normally (via the promoted MapFS.ReadDir), but fs.ReadFile on failPath
// returns an error — simulating a directory listing that succeeds while a
// specific file read fails.
type failReadFS struct {
	fstest.MapFS
	failPath string
}

func (f failReadFS) ReadFile(name string) ([]byte, error) {
	if name == f.failPath {
		return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrPermission}
	}
	return f.MapFS.ReadFile(name)
}

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
	agents := []string{"janus.md", "phantasos.md", "nyx.md", "morpheus.md", "phobetor.md", "baku.md", "iktomi.md", "zhougong.md", "hypnos.md", "mengpo.md"}
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

	settingsData, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("reading settings.json: %v", err)
	}
	if !strings.Contains(string(settingsData), "PostToolUse") || !strings.Contains(string(settingsData), "version-bump --change-from-command") {
		t.Errorf("settings.json missing PostToolUse change-scoped version-bump hook, got:\n%s", settingsData)
	}
}

func TestInstall_ClaudeCode_BareCommandsAlongsideDrmlnd(t *testing.T) {
	root := fakeGitRepo(t)
	if _, err := Install(Config{RepoRoot: root, CodingTool: "Claude Code"}); err != nil {
		t.Fatalf("Install: %v", err)
	}

	commandsDir := filepath.Join(root, ".claude", "commands")

	// drmlnd-prefixed set is unchanged.
	if _, err := os.Stat(filepath.Join(commandsDir, "drmlnd", "nyx.md")); err != nil {
		t.Errorf("drmlnd-prefixed nyx command missing: %v", err)
	}

	// Bare per-agent alias.
	if _, err := os.Stat(filepath.Join(commandsDir, "nyx.md")); err != nil {
		t.Errorf("bare nyx command missing: %v", err)
	}

	// Bare generic entry points, both derived from route.md.
	for _, name := range []string{"dreamland.md", "janus.md"} {
		p := filepath.Join(commandsDir, name)
		data, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("bare %s missing: %v", name, err)
			continue
		}
		if !strings.Contains(string(data), "Delegate this request to the `janus` agent") {
			t.Errorf("%s does not carry the generic routing instructions, got:\n%s", name, data)
		}
	}
}

func TestInstall_ClaudeCode_BareCommandFrontmatterStaysValid(t *testing.T) {
	root := fakeGitRepo(t)
	if _, err := Install(Config{RepoRoot: root, CodingTool: "Claude Code"}); err != nil {
		t.Fatalf("Install: %v", err)
	}

	for _, name := range []string{"nyx.md", "dreamland.md", "janus.md"} {
		data, err := os.ReadFile(filepath.Join(root, ".claude", "commands", name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		if !strings.HasPrefix(string(data), "---\n") {
			t.Errorf("%s frontmatter broken — marker must be appended, not prepended, got start:\n%.60s", name, data)
		}
		if !strings.Contains(string(data), bareCommandMarker) {
			t.Errorf("%s missing dreamland-managed marker", name)
		}
	}
}

func TestInstall_ClaudeCode_BareCommandUnmanagedFileNotOverwritten(t *testing.T) {
	root := fakeGitRepo(t)
	commandsDir := filepath.Join(root, ".claude", "commands")
	if err := os.MkdirAll(commandsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	userContent := "# my own janus notes, not a dreamland file\n"
	if err := os.WriteFile(filepath.Join(commandsDir, "janus.md"), []byte(userContent), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Install(Config{RepoRoot: root, CodingTool: "Claude Code"}); err != nil {
		t.Fatalf("Install: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(commandsDir, "janus.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != userContent {
		t.Errorf("user's own janus.md was overwritten, got:\n%s", data)
	}
}

func TestInstall_ClaudeCode_BareCommandManagedFileOverwrittenOnReinit(t *testing.T) {
	root := fakeGitRepo(t)
	if _, err := Install(Config{RepoRoot: root, CodingTool: "Claude Code"}); err != nil {
		t.Fatalf("first Install: %v", err)
	}

	nyxPath := filepath.Join(root, ".claude", "commands", "nyx.md")
	// Simulate an older template body under the same marker.
	if err := os.WriteFile(nyxPath, []byte(bareCommandMarker+"\nstale content\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Install(Config{RepoRoot: root, CodingTool: "Claude Code"}); err != nil {
		t.Fatalf("second Install: %v", err)
	}

	data, err := os.ReadFile(nyxPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "stale content") {
		t.Error("dreamland-managed bare command was not refreshed on re-init")
	}
}

func TestInstall_Codex(t *testing.T) {
	root := fakeGitRepo(t)
	_, err := Install(Config{RepoRoot: root, CodingTool: "Codex CLI"})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	agentDir := filepath.Join(root, ".codex", "agents")
	for _, a := range []string{"janus.toml", "phantasos.toml", "nyx.toml", "morpheus.toml", "phobetor.toml", "baku.toml", "iktomi.toml", "zhougong.toml", "hypnos.toml", "mengpo.toml"} {
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
	for _, a := range []string{"janus.mdc", "phantasos.mdc", "nyx.mdc", "morpheus.mdc", "phobetor.mdc", "baku.mdc", "iktomi.mdc", "zhougong.mdc", "hypnos.mdc", "mengpo.mdc"} {
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
	for _, a := range []string{"janus.md", "phantasos.md", "nyx.md", "morpheus.md", "phobetor.md", "baku.md", "iktomi.md", "zhougong.md", "hypnos.md", "mengpo.md"} {
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
		"janus.agent.md",
		"phantasos.agent.md",
		"nyx.agent.md",
		"morpheus.agent.md",
		"phobetor.agent.md",
		"baku.agent.md",
		"iktomi.agent.md",
		"zhougong.agent.md",
		"hypnos.agent.md",
		"mengpo.agent.md",
	} {
		if _, err := os.Stat(filepath.Join(copilotDir, a)); err != nil {
			t.Errorf("missing agent file %s: %v", a, err)
		}
	}

	hooksPath := filepath.Join(root, ".github", "hooks", "dreamland-hooks.json")
	data, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf(".github/hooks/dreamland-hooks.json not created: %v", err)
	}
	if !strings.Contains(string(data), "SessionStart") {
		t.Error("dreamland-hooks.json missing SessionStart event")
	}
	if !strings.Contains(string(data), "SubagentStop") {
		t.Error("dreamland-hooks.json missing SubagentStop event")
	}
}

func TestInstall_Antigravity(t *testing.T) {
	root := fakeGitRepo(t)
	_, err := Install(Config{RepoRoot: root, CodingTool: "Antigravity"})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	skillsDir := filepath.Join(root, ".agents", "skills")
	for _, skill := range []string{"janus", "phantasos", "nyx", "morpheus", "phobetor", "baku", "iktomi", "zhougong", "hypnos", "mengpo"} {
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

func TestInstall_EveryPlatformCoauthorHookIncludesHookFlag(t *testing.T) {
	cases := []struct {
		codingTool string
		hookPath   []string
	}{
		{"Claude Code", []string{".claude", "settings.json"}},
		{"GitHub Copilot", []string{".github", "hooks", "dreamland-hooks.json"}},
		{"Cursor", []string{".cursor", "hooks.json"}},
		{"Codex CLI", []string{".codex", "hooks.json"}},
		{"Kiro", []string{".kiro", "agent.json"}},
	}
	for _, c := range cases {
		t.Run(c.codingTool, func(t *testing.T) {
			root := fakeGitRepo(t)
			if _, err := Install(Config{RepoRoot: root, CodingTool: c.codingTool}); err != nil {
				t.Fatalf("Install: %v", err)
			}
			data, err := os.ReadFile(filepath.Join(append([]string{root}, c.hookPath...)...))
			if err != nil {
				t.Fatalf("reading installed hook file: %v", err)
			}
			content := string(data)
			count := strings.Count(content, `"dreamland coauthor`)
			if count == 0 {
				t.Fatalf("%s: expected at least one dreamland coauthor hook entry, found none in:\n%s", c.codingTool, content)
			}
			hookedCount := strings.Count(content, `"dreamland coauthor --hook`)
			if hookedCount != count {
				t.Errorf("%s: expected all %d dreamland coauthor hook-binding entries to include --hook, only %d did:\n%s", c.codingTool, count, hookedCount, content)
			}
		})
	}
}

func TestInstall_GitHubCopilot_AgentScopedHooks(t *testing.T) {
	root := fakeGitRepo(t)
	_, err := Install(Config{RepoRoot: root, CodingTool: "GitHub Copilot"})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	for _, a := range []string{
		"janus.agent.md", "phantasos.agent.md", "nyx.agent.md", "morpheus.agent.md",
		"phobetor.agent.md", "baku.agent.md", "iktomi.agent.md", "zhougong.agent.md",
		"hypnos.agent.md", "mengpo.agent.md",
	} {
		data, err := os.ReadFile(filepath.Join(root, ".github", "agents", a))
		if err != nil {
			t.Fatalf("missing %s: %v", a, err)
		}
		content := string(data)
		if !strings.Contains(content, "SubagentStart:") || !strings.Contains(content, "SubagentStop:") {
			t.Errorf("%s missing agent-scoped hooks: block (SubagentStart/SubagentStop)", a)
		}
		if !strings.Contains(content, "dreamland telemetry write --tool github-copilot") {
			t.Errorf("%s agent-scoped hooks missing telemetry write command", a)
		}
	}
}

func TestInstall_Cursor_Commands(t *testing.T) {
	root := fakeGitRepo(t)
	_, err := Install(Config{RepoRoot: root, CodingTool: "Cursor"})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	commandsDir := filepath.Join(root, ".cursor", "commands")
	names := map[string]string{
		"route.md":     "drmlnd-route",
		"phantasos.md": "drmlnd-phantasos",
		"nyx.md":       "drmlnd-nyx",
		"morpheus.md":  "drmlnd-morpheus",
		"phobetor.md":  "drmlnd-phobetor",
		"baku.md":      "drmlnd-baku",
		"iktomi.md":    "drmlnd-iktomi",
		"zhougong.md":  "drmlnd-zhougong",
		"hypnos.md":    "drmlnd-hypnos",
		"mengpo.md":    "drmlnd-mengpo",
	}
	for file, name := range names {
		data, err := os.ReadFile(filepath.Join(commandsDir, file))
		if err != nil {
			t.Errorf("missing command file %s: %v", file, err)
			continue
		}
		if want := "name: " + name; !strings.Contains(string(data), want) {
			t.Errorf("%s: expected frontmatter %q, got:\n%s", file, want, data)
		}
	}
}

func TestInstall_Cursor_Commands_ReplacesStaleUnprefixedFile(t *testing.T) {
	root := fakeGitRepo(t)
	commandsDir := filepath.Join(root, ".cursor", "commands")
	if err := os.MkdirAll(commandsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Simulate a pre-rename install: no frontmatter at all, just the old body.
	stale := "# Phantasos\n\nDelegate this request to the `janus` agent...\n"
	if err := os.WriteFile(filepath.Join(commandsDir, "phantasos.md"), []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Install(Config{RepoRoot: root, CodingTool: "Cursor"}); err != nil {
		t.Fatalf("Install: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(commandsDir, "phantasos.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "name: drmlnd-phantasos") {
		t.Errorf("expected stale file to be overwritten with drmlnd-prefixed frontmatter, got:\n%s", data)
	}
}

func TestInstall_Cursor_Commands_SkipsUpToDateFile(t *testing.T) {
	root := fakeGitRepo(t)
	commandsDir := filepath.Join(root, ".cursor", "commands")
	if err := os.MkdirAll(commandsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	current := "---\nname: drmlnd-phantasos\ndescription: custom\n---\n\ncustom body the user edited\n"
	if err := os.WriteFile(filepath.Join(commandsDir, "phantasos.md"), []byte(current), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := Install(Config{RepoRoot: root, CodingTool: "Cursor"})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(commandsDir, "phantasos.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != current {
		t.Errorf("expected up-to-date file to be left alone, got:\n%s", data)
	}

	var found bool
	for _, r := range results {
		if strings.HasSuffix(r.Path, filepath.Join("commands", "phantasos.md")) {
			found = true
			if r.Action != "skipped (already exists)" {
				t.Errorf("expected skip action, got %q", r.Action)
			}
		}
	}
	if !found {
		t.Fatal("expected a result entry for phantasos.md")
	}
}

func TestInstall_ClaudeCode_NoCursorDirCreated(t *testing.T) {
	root := fakeGitRepo(t)
	_, err := Install(Config{RepoRoot: root, CodingTool: "Claude Code"})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, ".cursor")); err == nil {
		t.Error("expected no .cursor directory created for Claude Code install")
	}
}

func TestInstall_ClaudeCode_Commands(t *testing.T) {
	root := fakeGitRepo(t)
	_, err := Install(Config{RepoRoot: root, CodingTool: "Claude Code"})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	commandsDir := filepath.Join(root, ".claude", "commands", "drmlnd")
	for _, c := range []string{"route.md", "phantasos.md", "nyx.md", "morpheus.md", "phobetor.md", "baku.md", "iktomi.md", "zhougong.md", "hypnos.md", "mengpo.md"} {
		if _, err := os.Stat(filepath.Join(commandsDir, c)); err != nil {
			t.Errorf("missing command file %s: %v", c, err)
		}
	}
}

func TestInstall_GitHubCopilot_Commands(t *testing.T) {
	root := fakeGitRepo(t)
	_, err := Install(Config{RepoRoot: root, CodingTool: "GitHub Copilot"})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	promptsDir := filepath.Join(root, ".github", "prompts")
	for _, agent := range []string{"route", "phantasos", "nyx", "morpheus", "phobetor", "baku", "iktomi", "zhougong", "hypnos", "mengpo"} {
		path := filepath.Join(promptsDir, agent+".prompt.md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("missing prompt file %s: %v", path, err)
			continue
		}
		if want := "name: drmlnd-" + agent; !strings.Contains(string(data), want) {
			t.Errorf("%s: expected frontmatter %q, got:\n%s", path, want, data)
		}
		if !strings.Contains(string(data), "agent: janus") {
			t.Errorf("%s: expected frontmatter \"agent: janus\", got:\n%s", path, data)
		}
	}
}

func TestInstall_Kiro_Commands(t *testing.T) {
	root := fakeGitRepo(t)
	_, err := Install(Config{RepoRoot: root, CodingTool: "Kiro"})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	steeringDir := filepath.Join(root, ".kiro", "steering")
	for _, agent := range []string{"route", "phantasos", "nyx", "morpheus", "phobetor", "baku", "iktomi", "zhougong", "hypnos", "mengpo"} {
		path := filepath.Join(steeringDir, "drmlnd-"+agent+".md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("missing steering command file %s: %v", path, err)
			continue
		}
		if want := "name: drmlnd-" + agent; !strings.Contains(string(data), want) {
			t.Errorf("%s: expected frontmatter %q, got:\n%s", path, want, data)
		}
		if !strings.Contains(string(data), "inclusion: manual") {
			t.Errorf("%s: expected frontmatter \"inclusion: manual\", got:\n%s", path, data)
		}
	}
}

func TestInstall_Antigravity_Commands(t *testing.T) {
	root := fakeGitRepo(t)
	_, err := Install(Config{RepoRoot: root, CodingTool: "Antigravity"})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	skillsDir := filepath.Join(root, ".agents", "skills")
	for _, agent := range []string{"route", "phantasos", "nyx", "morpheus", "phobetor", "baku", "iktomi", "zhougong", "hypnos", "mengpo"} {
		path := filepath.Join(skillsDir, "drmlnd-"+agent+".md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("missing flat command skill file %s: %v", path, err)
			continue
		}
		if want := "name: drmlnd-" + agent; !strings.Contains(string(data), want) {
			t.Errorf("%s: expected frontmatter %q, got:\n%s", path, want, data)
		}
	}

	// The agent personas themselves still install as directory-per-skill (e.g. phantasos/SKILL.md),
	// alongside — not overwritten by — the flat command files sharing the same parent directory.
	if _, err := os.Stat(filepath.Join(skillsDir, "phantasos", "SKILL.md")); err != nil {
		t.Errorf("expected agent persona skill directory to still exist: %v", err)
	}
}

func TestInstall_Codex_Commands(t *testing.T) {
	root := fakeGitRepo(t)
	_, err := Install(Config{RepoRoot: root, CodingTool: "Codex CLI"})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	skillsDir := filepath.Join(root, ".codex", "skills")
	for _, agent := range []string{"route", "phantasos", "nyx", "morpheus", "phobetor", "baku", "iktomi", "zhougong", "hypnos", "mengpo"} {
		path := filepath.Join(skillsDir, "drmlnd-"+agent, "SKILL.md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("missing codex skill file %s: %v", path, err)
			continue
		}
		if want := "name: drmlnd-" + agent; !strings.Contains(string(data), want) {
			t.Errorf("%s: expected frontmatter %q, got:\n%s", path, want, data)
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
	existingPath := filepath.Join(agentDir, "janus.md")
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
	existingPath := filepath.Join(agentDir, "janus.md")
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

	janusPath := filepath.Join(root, ".agents", "skills", "janus", "SKILL.md")
	if err := os.WriteFile(janusPath, []byte("custom content"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Re-install without force — custom content preserved.
	if _, err := Install(Config{RepoRoot: root, CodingTool: "Antigravity", Force: false}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(janusPath)
	if string(data) != "custom content" {
		t.Error("expected custom content preserved without --force")
	}

	// Re-install with force — file overwritten.
	if _, err := Install(Config{RepoRoot: root, CodingTool: "Antigravity", Force: true}); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(janusPath)
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

func TestInstall_GitHubCopilot_MergesHooksFile(t *testing.T) {
	root := fakeGitRepo(t)

	// Pre-existing dreamland-hooks.json with a user-added hook.
	hooksDir := filepath.Join(root, ".github", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	existing := []byte(`{"hooks":{"PostToolUse":[{"type":"command","command":"echo my-hook"}]}}`)
	if err := os.WriteFile(filepath.Join(hooksDir, "dreamland-hooks.json"), existing, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Install(Config{RepoRoot: root, CodingTool: "GitHub Copilot"}); err != nil {
		t.Fatalf("Install: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(hooksDir, "dreamland-hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	// Both existing and dreamland hooks should be present.
	if !strings.Contains(string(data), "my-hook") {
		t.Error("pre-existing hook was lost after merge")
	}
	if !strings.Contains(string(data), "SubagentStart") {
		t.Error("dreamland SubagentStart hook missing after merge")
	}
}

// --- Additional tests to improve coverage ---

func TestInstallFlatCommands_WriteFileError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root; skip permission test")
	}
	root := fakeGitRepo(t)

	commandsDir := filepath.Join(root, ".cursor", "commands")
	if err := os.MkdirAll(commandsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(commandsDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(commandsDir, 0o755) })

	_, err := Install(Config{RepoRoot: root, CodingTool: "Cursor", Force: true})
	if err == nil {
		t.Fatal("expected error when commands dir is unwritable")
	}
}

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
	janusSkillDir := filepath.Join(skillsDir, "janus")
	if err := os.MkdirAll(janusSkillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(janusSkillDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(janusSkillDir, 0o755) })

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

	// Make .github dir unwritable.
	githubDir := filepath.Join(root, ".github")
	if err := os.MkdirAll(githubDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(githubDir, 0o755) })

	patch := []byte(`{"hooks":{}}`)
	_, err := bindGitHubCopilot(root, patch, false)
	if err == nil {
		t.Fatal("expected error from bindGitHubCopilot when dir is unwritable")
	}
}

func TestBindAntigravity_AlreadyExists(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	pluginDir := filepath.Join(home, ".gemini", "antigravity-cli", "plugins", "dreamland")
	os.MkdirAll(pluginDir, 0o755)
	os.WriteFile(filepath.Join(pluginDir, "hooks.json"), []byte("{}"), 0o644)
	result, err := bindAntigravity("", []byte(`{"key":"val"}`), false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != "skipped (already exists)" {
		t.Errorf("expected skipped, got: %q", result.Action)
	}
}

func TestBindAntigravity_Force(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	result, err := bindAntigravity("", []byte(`{"key":"val"}`), true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != "installed (forced)" {
		t.Errorf("expected 'installed (forced)', got: %q", result.Action)
	}
}

// --- Further coverage: error branches reachable via the TemplateFS seam,
// direct calls to unexported helpers, and filesystem permission tricks. ---

func TestInstall_BindHooksError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root; skip permission test")
	}
	root := fakeGitRepo(t)
	// Pre-create settings.json as a directory so bindClaudeCode's
	// atomicJSONMerge fails reading it as a file.
	if err := os.MkdirAll(filepath.Join(root, ".claude", "settings.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := Install(Config{RepoRoot: root, CodingTool: "Claude Code"})
	if err == nil {
		t.Fatal("expected error when bindHooks fails")
	}
}

func TestInstallCommands_UnknownTool(t *testing.T) {
	results, err := installCommands(Config{CodingTool: "Bogus"})
	if err != nil {
		t.Errorf("expected nil error for unknown tool, got %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results for unknown tool, got %v", results)
	}
}

func TestInstall_ClaudeCode_BareCommandsWriteError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root; skip permission test")
	}
	root := fakeGitRepo(t)
	commandsDir := filepath.Join(root, ".claude", "commands")
	drmlndDir := filepath.Join(commandsDir, "drmlnd")
	if err := os.MkdirAll(drmlndDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(commandsDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(commandsDir, 0o755) })

	_, err := Install(Config{RepoRoot: root, CodingTool: "Claude Code"})
	if err == nil {
		t.Fatal("expected error when bare command file cannot be written")
	}
}

func TestInstallBareCommands_MkdirError(t *testing.T) {
	root := t.TempDir()
	blocker := filepath.Join(root, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	bareTargetDir := filepath.Join(blocker, "commands")
	_, err := installBareCommands(Config{}, platformSpec{}, bareTargetDir)
	if err == nil {
		t.Fatal("expected mkdir error")
	}
}

func TestInstallBareCommands_ReadDirError(t *testing.T) {
	withTemplateFS(t, fstest.MapFS{})
	root := t.TempDir()
	_, err := installBareCommands(Config{}, platformSpec{templateDir: "commands/nonexistent"}, filepath.Join(root, "commands"))
	if err == nil {
		t.Fatal("expected read dir error")
	}
}

func TestInstallBareCommands_SkipsDirectories(t *testing.T) {
	withTemplateFS(t, fstest.MapFS{
		"templates/commands/fake/a.md":        &fstest.MapFile{Data: []byte("---\nname: a\n---\nbody")},
		"templates/commands/fake/subdir/b.md": &fstest.MapFile{Data: []byte("nested")},
	})
	root := t.TempDir()
	bareTargetDir := filepath.Join(root, "commands")
	results, err := installBareCommands(Config{}, platformSpec{templateDir: "commands/fake"}, bareTargetDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result (subdir entry skipped), got %d: %+v", len(results), results)
	}
}

func TestInstallBareCommands_ReadFileError(t *testing.T) {
	mapFS := fstest.MapFS{"templates/commands/fake/a.md": &fstest.MapFile{Data: []byte("content")}}
	withTemplateFS(t, failReadFS{MapFS: mapFS, failPath: "templates/commands/fake/a.md"})
	root := t.TempDir()
	_, err := installBareCommands(Config{}, platformSpec{templateDir: "commands/fake"}, filepath.Join(root, "commands"))
	if err == nil {
		t.Fatal("expected read file error")
	}
}

func TestInstallFlatAgents_MkdirError(t *testing.T) {
	root := t.TempDir()
	blocker := filepath.Join(root, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	spec := platformSpec{templateDir: "agents/fake", targetDir: filepath.Join(blocker, "agents")}
	_, err := installFlatAgents(Config{}, spec)
	if err == nil {
		t.Fatal("expected mkdir error")
	}
}

func TestInstallFlatAgents_ReadDirError(t *testing.T) {
	withTemplateFS(t, fstest.MapFS{})
	root := t.TempDir()
	spec := platformSpec{templateDir: "agents/nonexistent", targetDir: filepath.Join(root, "agents")}
	_, err := installFlatAgents(Config{}, spec)
	if err == nil {
		t.Fatal("expected read dir error")
	}
}

func TestInstallFlatAgents_SkipsDirectories(t *testing.T) {
	withTemplateFS(t, fstest.MapFS{
		"templates/agents/fake/a.md":        &fstest.MapFile{Data: []byte("content")},
		"templates/agents/fake/subdir/b.md": &fstest.MapFile{Data: []byte("nested")},
	})
	root := t.TempDir()
	spec := platformSpec{templateDir: "agents/fake", targetDir: filepath.Join(root, "agents")}
	results, err := installFlatAgents(Config{RepoRoot: root}, spec)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result (subdir entry skipped), got %d: %+v", len(results), results)
	}
	if _, statErr := os.Stat(filepath.Join(root, "agents", "subdir")); !os.IsNotExist(statErr) {
		t.Error("subdir entry should not have been installed as a file")
	}
}

func TestInstallFlatAgents_ReadFileError(t *testing.T) {
	mapFS := fstest.MapFS{"templates/agents/fake/a.md": &fstest.MapFile{Data: []byte("content")}}
	withTemplateFS(t, failReadFS{MapFS: mapFS, failPath: "templates/agents/fake/a.md"})
	root := t.TempDir()
	spec := platformSpec{templateDir: "agents/fake", targetDir: filepath.Join(root, "agents")}
	_, err := installFlatAgents(Config{RepoRoot: root}, spec)
	if err == nil {
		t.Fatal("expected read file error")
	}
}

func TestInstallFlatCommands_MkdirError(t *testing.T) {
	root := t.TempDir()
	blocker := filepath.Join(root, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	spec := platformSpec{templateDir: "commands/fake", targetDir: filepath.Join(blocker, "commands")}
	_, err := installFlatCommands(Config{}, spec)
	if err == nil {
		t.Fatal("expected mkdir error")
	}
}

func TestInstallFlatCommands_ReadDirError(t *testing.T) {
	withTemplateFS(t, fstest.MapFS{})
	root := t.TempDir()
	spec := platformSpec{templateDir: "commands/nonexistent", targetDir: filepath.Join(root, "commands")}
	_, err := installFlatCommands(Config{}, spec)
	if err == nil {
		t.Fatal("expected read dir error")
	}
}

func TestInstallFlatCommands_SkipsDirectories(t *testing.T) {
	withTemplateFS(t, fstest.MapFS{
		"templates/commands/fake/a.md":        &fstest.MapFile{Data: []byte("---\nname: a\n---\nbody")},
		"templates/commands/fake/subdir/b.md": &fstest.MapFile{Data: []byte("nested")},
	})
	root := t.TempDir()
	spec := platformSpec{templateDir: "commands/fake", targetDir: filepath.Join(root, "commands")}
	results, err := installFlatCommands(Config{RepoRoot: root}, spec)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result (subdir entry skipped), got %d: %+v", len(results), results)
	}
}

func TestInstallFlatCommands_ReadFileError(t *testing.T) {
	mapFS := fstest.MapFS{"templates/commands/fake/a.md": &fstest.MapFile{Data: []byte("content")}}
	withTemplateFS(t, failReadFS{MapFS: mapFS, failPath: "templates/commands/fake/a.md"})
	root := t.TempDir()
	spec := platformSpec{templateDir: "commands/fake", targetDir: filepath.Join(root, "commands")}
	_, err := installFlatCommands(Config{RepoRoot: root}, spec)
	if err == nil {
		t.Fatal("expected read file error")
	}
}

func TestInstallSkills_TargetDirMkdirError(t *testing.T) {
	root := t.TempDir()
	blocker := filepath.Join(root, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	spec := platformSpec{templateDir: "agents/fake", targetDir: filepath.Join(blocker, "skills"), skillFile: "SKILL.md"}
	_, err := installSkills(Config{}, spec)
	if err == nil {
		t.Fatal("expected mkdir error")
	}
}

func TestInstallSkills_ReadDirError(t *testing.T) {
	withTemplateFS(t, fstest.MapFS{})
	root := t.TempDir()
	spec := platformSpec{templateDir: "agents/nonexistent", targetDir: filepath.Join(root, "skills"), skillFile: "SKILL.md"}
	_, err := installSkills(Config{}, spec)
	if err == nil {
		t.Fatal("expected read dir error")
	}
}

func TestInstallSkills_SkipsStrayFiles(t *testing.T) {
	withTemplateFS(t, fstest.MapFS{
		"templates/agents/fakeskill/skillA/SKILL.md": &fstest.MapFile{Data: []byte("content")},
		"templates/agents/fakeskill/strayfile.md":    &fstest.MapFile{Data: []byte("stray")},
	})
	root := t.TempDir()
	spec := platformSpec{templateDir: "agents/fakeskill", targetDir: filepath.Join(root, "skills"), skillFile: "SKILL.md"}
	results, err := installSkills(Config{RepoRoot: root}, spec)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result (stray file skipped), got %d: %+v", len(results), results)
	}
}

func TestInstallSkills_SkillDirMkdirError(t *testing.T) {
	withTemplateFS(t, fstest.MapFS{
		"templates/agents/fakeskill/skillA/SKILL.md": &fstest.MapFile{Data: []byte("content")},
	})
	root := t.TempDir()
	targetDir := filepath.Join(root, "skills")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A regular file named "skillA" blocks MkdirAll from creating the skill directory.
	if err := os.WriteFile(filepath.Join(targetDir, "skillA"), []byte("blocker"), 0o644); err != nil {
		t.Fatal(err)
	}
	spec := platformSpec{templateDir: "agents/fakeskill", targetDir: targetDir, skillFile: "SKILL.md"}
	_, err := installSkills(Config{RepoRoot: root}, spec)
	if err == nil {
		t.Fatal("expected mkdir error for skill dir")
	}
}

func TestInstallSkills_ReadFileError(t *testing.T) {
	mapFS := fstest.MapFS{"templates/agents/fakeskill/skillA/SKILL.md": &fstest.MapFile{Data: []byte("content")}}
	withTemplateFS(t, failReadFS{MapFS: mapFS, failPath: "templates/agents/fakeskill/skillA/SKILL.md"})
	root := t.TempDir()
	spec := platformSpec{templateDir: "agents/fakeskill", targetDir: filepath.Join(root, "skills"), skillFile: "SKILL.md"}
	_, err := installSkills(Config{RepoRoot: root}, spec)
	if err == nil {
		t.Fatal("expected read file error")
	}
}

func TestBindHooks_UnknownTool(t *testing.T) {
	_, err := bindHooks(Config{CodingTool: "Bogus"})
	if err == nil {
		t.Fatal("expected error for unknown coding tool")
	}
}

func TestBindHooks_TemplateReadError(t *testing.T) {
	withTemplateFS(t, fstest.MapFS{})
	root := t.TempDir()
	_, err := bindHooks(Config{RepoRoot: root, CodingTool: "Claude Code"})
	if err == nil {
		t.Fatal("expected error reading hook binding template")
	}
}

func TestDeepMerge_ScalarOverwrite(t *testing.T) {
	dst := map[string]interface{}{"theme": "dark", "count": float64(1)}
	src := map[string]interface{}{"theme": "light"}
	deepMerge(dst, src)
	if dst["theme"] != "light" {
		t.Errorf("expected scalar value overwritten to %q, got %v", "light", dst["theme"])
	}
	if dst["count"] != float64(1) {
		t.Errorf("unrelated key should be untouched, got %v", dst["count"])
	}
}

func TestBindAntigravity_MkdirError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// A regular file at ~/.gemini blocks MkdirAll from creating the plugin dir.
	if err := os.WriteFile(filepath.Join(home, ".gemini"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := bindAntigravity("", []byte(`{"key":"val"}`), false)
	if err == nil {
		t.Fatal("expected mkdir error")
	}
}

func TestBindAntigravity_WriteFileError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root; skip permission test")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	pluginDir := filepath.Join(home, ".gemini", "antigravity-cli", "plugins", "dreamland")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(pluginDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(pluginDir, 0o755) })

	_, err := bindAntigravity("", []byte(`{"key":"val"}`), false)
	if err == nil {
		t.Fatal("expected write file error")
	}
}
