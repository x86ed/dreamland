package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestJanusRoutesWorkflowGraphStructuralTasksToHypnos asserts every
// platform's installed janus.* file states the widened agent-roster dispatch
// rule (openspec/changes/hypnos-litegraph-editor's modified janus-router-agent
// requirement): hypnos is the /opsx:apply target not just for agent creation,
// but for any workflow-graph-structural task (a routing-edge change, or a
// hook/skill attach/detach).
func TestJanusRoutesWorkflowGraphStructuralTasksToHypnos(t *testing.T) {
	tests := []struct {
		tool       string
		janusPath  string
		hypnosPath string
	}{
		{"Claude Code", filepath.Join(".claude", "agents", "janus.md"), filepath.Join(".claude", "agents", "hypnos.md")},
		{"Codex CLI", filepath.Join(".codex", "agents", "janus.toml"), filepath.Join(".codex", "agents", "hypnos.toml")},
		{"Cursor", filepath.Join(".cursor", "rules", "janus.mdc"), filepath.Join(".cursor", "rules", "hypnos.mdc")},
		{"Kiro", filepath.Join(".kiro", "steering", "janus.md"), filepath.Join(".kiro", "steering", "hypnos.md")},
		{"Antigravity", filepath.Join(".agents", "skills", "janus", "SKILL.md"), filepath.Join(".agents", "skills", "hypnos", "SKILL.md")},
		{"GitHub Copilot", filepath.Join(".github", "agents", "janus.agent.md"), filepath.Join(".github", "agents", "hypnos.agent.md")},
	}

	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			root := fakeGitRepo(t)
			if _, err := Install(Config{RepoRoot: root, CodingTool: tt.tool}); err != nil {
				t.Fatalf("Install: %v", err)
			}

			janusData, err := os.ReadFile(filepath.Join(root, tt.janusPath))
			if err != nil {
				t.Fatalf("reading %s: %v", tt.janusPath, err)
			}
			if !strings.Contains(string(janusData), "workflow-graph-structural") {
				t.Errorf("%s does not mention routing workflow-graph-structural tasks to hypnos", tt.janusPath)
			}
			if !strings.Contains(string(janusData), "hypnos") {
				t.Errorf("%s does not mention hypnos at all", tt.janusPath)
			}

			hypnosData, err := os.ReadFile(filepath.Join(root, tt.hypnosPath))
			if err != nil {
				t.Fatalf("reading %s: %v", tt.hypnosPath, err)
			}
			if !strings.Contains(string(hypnosData), "apply-plan") {
				t.Errorf("%s does not mention the --mode=apply-plan responsibility", tt.hypnosPath)
			}
			if !strings.Contains(string(hypnosData), "workflow-graph-structural") {
				t.Errorf("%s does not mention the workflow-graph-structural dispatch condition", tt.hypnosPath)
			}
		})
	}
}

// TestHypnosServeCommandsInstalled asserts /hypnos-view and /hypnos-interactive
// land at the expected, unprefixed path on every platform and reference the
// right --mode flag — following router-slash-commands' per-platform
// conventions, but as a third, standalone command category (like /opsx:*),
// not the drmlnd-prefixed per-agent-routing set.
func TestHypnosServeCommandsInstalled(t *testing.T) {
	tests := []struct {
		tool        string
		viewPath    string
		interactive string
	}{
		{"Claude Code", filepath.Join(".claude", "commands", "hypnos-view.md"), filepath.Join(".claude", "commands", "hypnos-interactive.md")},
		{"Cursor", filepath.Join(".cursor", "commands", "hypnos-view.md"), filepath.Join(".cursor", "commands", "hypnos-interactive.md")},
		{"GitHub Copilot", filepath.Join(".github", "prompts", "hypnos-view.prompt.md"), filepath.Join(".github", "prompts", "hypnos-interactive.prompt.md")},
		{"Kiro", filepath.Join(".kiro", "steering", "hypnos-view.md"), filepath.Join(".kiro", "steering", "hypnos-interactive.md")},
		{"Antigravity", filepath.Join(".agents", "skills", "hypnos-view.md"), filepath.Join(".agents", "skills", "hypnos-interactive.md")},
		{"Codex CLI", filepath.Join(".codex", "skills", "hypnos-view", "SKILL.md"), filepath.Join(".codex", "skills", "hypnos-interactive", "SKILL.md")},
	}

	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			root := fakeGitRepo(t)
			if _, err := Install(Config{RepoRoot: root, CodingTool: tt.tool}); err != nil {
				t.Fatalf("Install: %v", err)
			}

			viewData, err := os.ReadFile(filepath.Join(root, tt.viewPath))
			if err != nil {
				t.Fatalf("reading %s: %v", tt.viewPath, err)
			}
			if !strings.Contains(string(viewData), "--mode=view") {
				t.Errorf("%s does not invoke --mode=view", tt.viewPath)
			}
			if strings.Contains(string(viewData), "drmlnd-hypnos-view") {
				t.Errorf("%s should not be drmlnd-prefixed", tt.viewPath)
			}

			interactiveData, err := os.ReadFile(filepath.Join(root, tt.interactive))
			if err != nil {
				t.Fatalf("reading %s: %v", tt.interactive, err)
			}
			if !strings.Contains(string(interactiveData), "--mode=interactive") {
				t.Errorf("%s does not invoke --mode=interactive", tt.interactive)
			}
		})
	}
}

// TestHypnosServeCommandsAlsoReachableViaDrmlndNamespaceOnClaudeCode documents
// a deliberate, accepted side effect: on Claude Code, any file dropped into
// commands/claude-code/ is installed both under the bare (unprefixed) path via
// installBareCommands and under .claude/commands/drmlnd/ via installFlatCommands
// — there was no existing mechanism to opt a single template file out of the
// drmlnd-prefixed pass, and writing one just for these two files wasn't worth
// the new code for a harmless redundant alias. Both are the intended behavior,
// not a bug — this test locks that in.
func TestHypnosServeCommandsAlsoReachableViaDrmlndNamespaceOnClaudeCode(t *testing.T) {
	root := fakeGitRepo(t)
	if _, err := Install(Config{RepoRoot: root, CodingTool: "Claude Code"}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	for _, name := range []string{"hypnos-view.md", "hypnos-interactive.md"} {
		if _, err := os.Stat(filepath.Join(root, ".claude", "commands", "drmlnd", name)); err != nil {
			t.Errorf("expected the redundant drmlnd-namespaced alias for %s: %v", name, err)
		}
	}
}
