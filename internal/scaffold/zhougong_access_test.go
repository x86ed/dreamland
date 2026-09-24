package scaffold

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const zhougongServer = "dreamland-zhougong"

// TestZhougongMCPServerScopedToZhougongOnly asserts the dreamland-zhougong server is
// declared only in zhougong's own agent template: no other embedded template
// (agents or commands, any platform) references it, and a scaffolded .mcp.json does not.
func TestZhougongMCPServerScopedToZhougongOnly(t *testing.T) {
	err := fs.WalkDir(TemplateFS, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(TemplateFS, path)
		if err != nil {
			return err
		}
		isZhougong := path == "templates/agents/claude-code/zhougong.md"
		if !isZhougong && strings.Contains(string(b), zhougongServer) {
			t.Errorf("%s references %s; only the zhougong claude-code agent template may", path, zhougongServer)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	root := fakeGitRepo(t)
	if _, err := Install(Config{RepoRoot: root, CodingTool: "Claude Code"}); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(root, ".mcp.json")); err == nil && strings.Contains(string(b), zhougongServer) {
		t.Errorf(".mcp.json must not register %s", zhougongServer)
	}
	agents, _ := filepath.Glob(filepath.Join(root, ".claude", "agents", "*.md"))
	for _, a := range agents {
		b, _ := os.ReadFile(a)
		has := strings.Contains(string(b), zhougongServer)
		if isZ := filepath.Base(a) == "zhougong.md"; has != isZ {
			t.Errorf("%s: references %s = %v, want %v", a, zhougongServer, has, isZ)
		}
	}
}

func TestZhougongTemplateGrantsMCPTools(t *testing.T) {
	b, err := fs.ReadFile(TemplateFS, "templates/agents/claude-code/zhougong.md")
	if err != nil {
		t.Fatal(err)
	}
	fm := strings.SplitN(string(b), "---", 3)[1]
	if !strings.Contains(fm, "mcpServers:") || !strings.Contains(fm, "mcp-zhougong") {
		t.Errorf("frontmatter must declare the inline mcpServers entry:\n%s", fm)
	}
	for _, tool := range []string{"zhougong_collect", "zhougong_dashboard_start", "zhougong_dashboard_stop"} {
		if !strings.Contains(fm, "mcp__dreamland-zhougong__"+tool) {
			t.Errorf("tools missing %s", tool)
		}
	}
}

// TestRepoZhougongAgentMatchesTemplate keeps the checked-in agent in sync with its template.
func TestRepoZhougongAgentMatchesTemplate(t *testing.T) {
	want, _ := fs.ReadFile(TemplateFS, "templates/agents/claude-code/zhougong.md")
	got, err := os.ReadFile("../../.claude/agents/zhougong.md")
	if err != nil {
		t.Skip("not running inside the dreamland repo")
	}
	if string(got) != string(want) {
		t.Errorf(".claude/agents/zhougong.md differs from its scaffold template")
	}
	if _, err := os.Stat("../../.mcp.json"); err == nil {
		b, _ := os.ReadFile("../../.mcp.json")
		if strings.Contains(string(b), zhougongServer) {
			t.Errorf("repo .mcp.json must not register %s", zhougongServer)
		}
	}
}
