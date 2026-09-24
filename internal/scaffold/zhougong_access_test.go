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
	for _, tool := range []string{"zhougong_collect", "zhougong_dashboard_start", "zhougong_dashboard_stop", "zhougong_new_agent_issue"} {
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

// TestZhougongTemplateRequiresSnapshotFirst asserts the scaffolded zhougong agent
// carries the snapshot-first interpretation requirement and can call the tool.
func TestZhougongTemplateRequiresSnapshotFirst(t *testing.T) {
	root := fakeGitRepo(t)
	if _, err := Install(Config{RepoRoot: root, CodingTool: "Claude Code"}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, ".claude", "agents", "zhougong.md"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(b)
	for _, want := range []string{
		"mcp__dreamland-zhougong__zhougong_snapshot",
		"Before answering the first question about a report or a branch/feature diff, call `zhougong_snapshot`",
		"`collectedAt`",
		"`stale`",
		"`unattributed` or `untracked`",
		"commit-author based",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("zhougong.md missing %q", want)
		}
	}
}

func TestZhougongInstructionsRequireApprovalBeforeConfirm(t *testing.T) {
	b, _ := fs.ReadFile(TemplateFS, "templates/agents/claude-code/zhougong.md")
	for _, want := range []string{"confirm=false", "explicit approval", "confirm=true", "previewId"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("zhougong instructions missing %q", want)
		}
	}
}

func TestPhantasosTemplateHasIssueIntake(t *testing.T) {
	b, _ := fs.ReadFile(TemplateFS, "templates/agents/claude-code/phantasos.md")
	if !strings.Contains(string(b), "gh issue view <n> --json title,body,labels") {
		t.Error("phantasos template missing issue-intake instruction")
	}
	// Only a dreamland-managed live copy is held to its template; an unmarked
	// legacy copy is re-synced by `dreamland init --force` (design Decision 12).
	if got, err := os.ReadFile("../../.claude/agents/phantasos.md"); err == nil && hasManagedMarker(got) &&
		string(got) != string(withManagedMarker("phantasos.md", b)) {
		t.Error(".claude/agents/phantasos.md differs from its template; run `dreamland init`")
	}
}
