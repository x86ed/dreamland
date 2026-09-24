package scaffold

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type agentLayout struct {
	tool, dir, file string
	template        string
}

var agentLayouts = []agentLayout{
	{"Claude Code", ".claude/agents", "morpheus.md", "templates/agents/claude-code/morpheus.md"},
	{"Cursor", ".cursor/rules", "morpheus.mdc", "templates/agents/cursor/morpheus.mdc"},
	{"Codex CLI", ".codex/agents", "morpheus.toml", "templates/agents/codex/morpheus.toml"},
	{"Kiro", ".kiro/steering", "morpheus.md", "templates/agents/kiro/morpheus.md"},
	{"GitHub Copilot", ".github/agents", "morpheus.agent.md", "templates/agents/github-copilot/morpheus.agent.md"},
	{"Antigravity", ".agents/skills", "morpheus/SKILL.md", "templates/agents/antigravity/morpheus/SKILL.md"},
}

func actionFor(results []Result, path string) string {
	for _, r := range results {
		if r.Path == path {
			return r.Action
		}
	}
	return ""
}

func TestAgentSync_AllPlatformLayouts(t *testing.T) {
	for _, l := range agentLayouts {
		t.Run(l.tool, func(t *testing.T) {
			root := fakeGitRepo(t)
			path := filepath.Join(root, filepath.FromSlash(l.dir), filepath.FromSlash(l.file))
			tmpl, err := fs.ReadFile(TemplateFS, l.template)
			if err != nil {
				t.Fatal(err)
			}

			if _, err := Install(Config{RepoRoot: root, CodingTool: l.tool}); err != nil {
				t.Fatal(err)
			}
			got, _ := os.ReadFile(path)
			if !strings.HasPrefix(string(got), string(tmpl)) || !hasManagedMarker(got) {
				t.Fatalf("fresh install lacks template body or marker")
			}
			if strings.HasSuffix(l.file, ".toml") && !strings.Contains(string(got), "\n# "+DreamlandManagedMarker) {
				t.Error("toml marker must be a # comment")
			}

			// A marked file that differs is refreshed by plain init.
			if err := os.WriteFile(path, append([]byte("stale body\n"), []byte(managedMarkerLine(path)+"\n")...), 0o644); err != nil {
				t.Fatal(err)
			}
			results, err := Install(Config{RepoRoot: root, CodingTool: l.tool})
			if err != nil {
				t.Fatal(err)
			}
			if a := actionFor(results, path); a != "updated" {
				t.Errorf("marked stale file action = %q, want updated", a)
			}
			if got, _ := os.ReadFile(path); string(got) != string(withManagedMarker(path, tmpl)) {
				t.Error("marked stale file not refreshed")
			}

			// An unmarked differing file is kept, listed, and adopted by --force.
			if err := os.WriteFile(path, []byte("user owned\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := Install(Config{RepoRoot: root, CodingTool: l.tool}); err != nil {
				t.Fatal(err)
			}
			if got, _ := os.ReadFile(path); string(got) != "user owned\n" {
				t.Error("unmarked file overwritten without --force")
			}
			var kinds []string
			for _, i := range CheckAgentFiles(root) {
				if i.Path == path {
					kinds = append(kinds, i.Kind)
				}
			}
			if len(kinds) != 1 || kinds[0] != "unmarked-different" {
				t.Errorf("CheckAgentFiles = %v", kinds)
			}
			if _, err := Install(Config{RepoRoot: root, CodingTool: l.tool, Force: true}); err != nil {
				t.Fatal(err)
			}
			if got, _ := os.ReadFile(path); string(got) != string(withManagedMarker(path, tmpl)) {
				t.Error("--force did not adopt the file")
			}
			for _, i := range CheckAgentFiles(root) {
				if i.Path == path {
					t.Errorf("adopted file still reported: %+v", i)
				}
			}
		})
	}
}

func TestCheckAgentFiles_OutOfDateMarked(t *testing.T) {
	root := fakeGitRepo(t)
	path := filepath.Join(root, ".claude", "agents", "morpheus.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("old\n"+managedMarkerLine(path)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	issues := CheckAgentFiles(root)
	if len(issues) != 1 || issues[0].Kind != "out-of-date" {
		t.Fatalf("issues = %+v", issues)
	}
}

func TestAgentSync_UnmarkedIdenticalGainsMarker(t *testing.T) {
	root := fakeGitRepo(t)
	path := filepath.Join(root, ".claude", "agents", "morpheus.md")
	tmpl, _ := fs.ReadFile(TemplateFS, "templates/agents/claude-code/morpheus.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, tmpl, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(Config{RepoRoot: root, CodingTool: "Claude Code"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); !hasManagedMarker(got) {
		t.Error("identical unmarked file was not marked")
	}
}
