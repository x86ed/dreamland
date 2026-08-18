package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stubAgentPaths mirrors platformAgentSpec's per-platform path convention (see
// scaffold.go), substituting the externally supplied name for the fixed ten's.
func stubAgentPaths(root, name string) map[string]string {
	return map[string]string{
		"Claude Code":    filepath.Join(root, ".claude", "agents", name+".md"),
		"Codex CLI":      filepath.Join(root, ".codex", "agents", name+".toml"),
		"Cursor":         filepath.Join(root, ".cursor", "rules", name+".mdc"),
		"Kiro":           filepath.Join(root, ".kiro", "steering", name+".md"),
		"Antigravity":    filepath.Join(root, ".agents", "skills", name, "SKILL.md"),
		"GitHub Copilot": filepath.Join(root, ".github", "agents", name+".agent.md"),
	}
}

// TestInstallAgentStub_AllSixPlatforms covers the "Externally supplied name renders a
// stub on every platform" scenario (agent-scaffolding capability): given
// name="amber-falcon", role="example role", toolTier="full-edit", a stub file is
// written at the standard per-platform path for all six supported platforms.
func TestInstallAgentStub_AllSixPlatforms(t *testing.T) {
	root := fakeGitRepo(t)

	_, err := InstallAgentStub(Config{RepoRoot: root}, "amber-falcon", "example role", "full-edit")
	if err != nil {
		t.Fatalf("InstallAgentStub: %v", err)
	}

	for platform, path := range stubAgentPaths(root, "amber-falcon") {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("%s: missing stub file at %s: %v", platform, path, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("%s: stub file at %s is empty", platform, path)
		}
	}
}

// TestInstallAgentStub_PlaceholderBodyNamesHypnos covers "Stub instruction body is a
// placeholder, not authored prose": the instruction body must carry a placeholder
// comment naming hypnos as the next editor, on every platform, and must not contain
// role-specific persona prose the stub-rendering path never generated.
func TestInstallAgentStub_PlaceholderBodyNamesHypnos(t *testing.T) {
	root := fakeGitRepo(t)

	_, err := InstallAgentStub(Config{RepoRoot: root}, "amber-falcon", "example role", "full-edit")
	if err != nil {
		t.Fatalf("InstallAgentStub: %v", err)
	}

	for platform, path := range stubAgentPaths(root, "amber-falcon") {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: missing stub file: %v", platform, err)
		}
		content := string(data)
		if !strings.Contains(content, "hypnos") {
			t.Errorf("%s: stub body missing placeholder naming hypnos as next editor, got:\n%s", platform, content)
		}
		if !strings.Contains(content, "TODO") {
			t.Errorf("%s: stub body missing a TODO-style placeholder marker, got:\n%s", platform, content)
		}
	}
}

// TestInstallAgentStub_FullEditTier_ClaudeCodeToolGrant covers the tool-binding matrix's
// full-edit row on Claude Code, whose frontmatter `tools:` field is machine-checkable
// (unlike the platforms with no structured capability field).
func TestInstallAgentStub_FullEditTier_ClaudeCodeToolGrant(t *testing.T) {
	root := fakeGitRepo(t)

	_, err := InstallAgentStub(Config{RepoRoot: root}, "amber-falcon", "example role", "full-edit")
	if err != nil {
		t.Fatalf("InstallAgentStub: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".claude", "agents", "amber-falcon.md"))
	if err != nil {
		t.Fatalf("missing amber-falcon.md: %v", err)
	}
	content := string(data)
	for _, tool := range []string{"Read", "Edit", "Write", "Bash"} {
		if !strings.Contains(content, tool) {
			t.Errorf("full-edit tier stub missing %q tool grant, got:\n%s", tool, content)
		}
	}
}

// TestInstallAgentStub_FullEditTier_GitHubCopilotToolGrant mirrors the Claude Code
// check for the other platform with a structured `tools:` frontmatter list.
func TestInstallAgentStub_FullEditTier_GitHubCopilotToolGrant(t *testing.T) {
	root := fakeGitRepo(t)

	_, err := InstallAgentStub(Config{RepoRoot: root}, "amber-falcon", "example role", "full-edit")
	if err != nil {
		t.Fatalf("InstallAgentStub: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".github", "agents", "amber-falcon.agent.md"))
	if err != nil {
		t.Fatalf("missing amber-falcon.agent.md: %v", err)
	}
	content := string(data)
	for _, tool := range []string{"Read", "Edit", "Write", "Bash"} {
		if !strings.Contains(content, tool) {
			t.Errorf("full-edit tier stub missing %q tool grant, got:\n%s", tool, content)
		}
	}
}

// TestInstallAgentStub_HookBlockSubstitutesGeneratedName covers "Stub carries the same
// hook block shape as an existing agent of the same tier": the Claude Code Stop hook
// block must reference amber-falcon, not a fixed-ten name, in every one of its
// per-name-substituted commands.
func TestInstallAgentStub_HookBlockSubstitutesGeneratedName(t *testing.T) {
	root := fakeGitRepo(t)

	_, err := InstallAgentStub(Config{RepoRoot: root}, "amber-falcon", "example role", "full-edit")
	if err != nil {
		t.Fatalf("InstallAgentStub: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".claude", "agents", "amber-falcon.md"))
	if err != nil {
		t.Fatalf("missing amber-falcon.md: %v", err)
	}
	content := string(data)

	for _, want := range []string{
		"dreamland coauthor --hook --agent-name amber-falcon",
		"dreamland telemetry write --tool claude-code",
		"dreamland version-bump --patch",
		"dreamland commit --reason handoff --agent-name amber-falcon",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("stub hooks.Stop block missing %q, got:\n%s", want, content)
		}
	}
}

// TestInstallAgentStub_UnknownToolTier covers the "unknown toolTier value returns an
// error" requirement.
func TestInstallAgentStub_UnknownToolTier(t *testing.T) {
	root := fakeGitRepo(t)

	results, err := InstallAgentStub(Config{RepoRoot: root}, "amber-falcon", "example role", "not-a-real-tier")
	if err == nil {
		t.Fatal("expected error for unknown tool tier")
	}
	if len(results) != 0 {
		t.Errorf("expected no results written for an unknown tool tier, got: %v", results)
	}
	if _, statErr := os.Stat(filepath.Join(root, ".claude", "agents", "amber-falcon.md")); statErr == nil {
		t.Error("expected no stub file written when tool tier is invalid")
	}
}

// stubCommandPaths mirrors platformCommandSpec's per-platform per-agent slash-command
// path convention (see scaffold.go / the router-slash-commands capability).
func stubCommandPaths(root, name string) map[string]string {
	return map[string]string{
		"Claude Code":    filepath.Join(root, ".claude", "commands", "drmlnd", name+".md"),
		"Cursor":         filepath.Join(root, ".cursor", "commands", name+".md"),
		"GitHub Copilot": filepath.Join(root, ".github", "prompts", name+".prompt.md"),
		"Kiro":           filepath.Join(root, ".kiro", "steering", "drmlnd-"+name+".md"),
		"Antigravity":    filepath.Join(root, ".agents", "skills", "drmlnd-"+name+".md"),
		"Codex CLI":      filepath.Join(root, ".codex", "skills", "drmlnd-"+name, "SKILL.md"),
	}
}

// TestInstallAgentCommand_AllSixPlatforms covers the "Slash command installed for the
// generated name" scenario: `.claude/commands/drmlnd/amber-falcon.md` (and the
// equivalent per-platform path) exists after InstallAgentCommand, following the same
// per-agent direct-invoke convention the existing nine agents already use.
func TestInstallAgentCommand_AllSixPlatforms(t *testing.T) {
	root := fakeGitRepo(t)

	_, err := InstallAgentCommand(Config{RepoRoot: root}, "amber-falcon")
	if err != nil {
		t.Fatalf("InstallAgentCommand: %v", err)
	}

	for platform, path := range stubCommandPaths(root, "amber-falcon") {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("%s: missing command file at %s: %v", platform, path, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("%s: command file at %s is empty", platform, path)
		}
	}
}

// TestInstallAgentCommand_ClaudeCodeExactPath is the literal acceptance-scenario
// assertion from the oneiroi-seed-naming spec: ".claude/commands/drmlnd/amber-falcon.md"
// exists.
func TestInstallAgentCommand_ClaudeCodeExactPath(t *testing.T) {
	root := fakeGitRepo(t)

	if _, err := InstallAgentCommand(Config{RepoRoot: root}, "amber-falcon"); err != nil {
		t.Fatalf("InstallAgentCommand: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, ".claude", "commands", "drmlnd", "amber-falcon.md")); err != nil {
		t.Errorf(".claude/commands/drmlnd/amber-falcon.md missing: %v", err)
	}
}

// TestInstallAgentStub_FixedTenInstallUnaffected covers "Fixed-ten install path is
// unchanged": calling Install for the built-in ten (unrelated to InstallAgentStub) must
// still render from the hand-authored embedded templates, not the generalized stub path
// — a stub call for a new name must not perturb a subsequent fixed-ten Install in the
// same repo.
func TestInstallAgentStub_FixedTenInstallUnaffected(t *testing.T) {
	root := fakeGitRepo(t)

	if _, err := InstallAgentStub(Config{RepoRoot: root}, "amber-falcon", "example role", "full-edit"); err != nil {
		t.Fatalf("InstallAgentStub: %v", err)
	}
	if _, err := Install(Config{RepoRoot: root, CodingTool: "Claude Code"}); err != nil {
		t.Fatalf("Install: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".claude", "agents", "hypnos.md"))
	if err != nil {
		t.Fatalf("missing hypnos.md: %v", err)
	}
	if strings.Contains(string(data), "TODO(hypnos)") {
		t.Error("fixed-ten hypnos.md was rendered from the stub path instead of its hand-authored template")
	}
}
