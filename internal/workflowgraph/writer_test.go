package workflowgraph

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newClaudeRepo creates a temp repo with only Claude Code installed (an empty
// .claude/agents dir plus a minimal settings.json), matching this repo's own
// actual installed-platform shape.
func newClaudeRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".claude", "settings.json"), []byte(`{"hooks":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestCreateAgentWritesClaudeCodeFileWithHookBaseline(t *testing.T) {
	root := newClaudeRepo(t)
	g := New(root)

	if err := CreateAgent(root, g, "testagent", "Does test things.", TierFullEdit); err != nil {
		t.Fatalf("CreateAgent: unexpected error %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".claude", "agents", "testagent.md"))
	if err != nil {
		t.Fatalf("expected file to be written: %v", err)
	}
	content := string(data)

	for _, want := range []string{
		"name: testagent",
		"description: Does test things.",
		"tools: Read, Edit, Write, Bash",
		"dreamland coauthor --hook --agent-name testagent",
		"dreamland telemetry write --tool claude-code",
		"dreamland commit --reason handoff --agent-name testagent",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("rendered file missing %q\n---\n%s", want, content)
		}
	}
	// Regression: a real bug (fmt.Sprintf on a template with no %s verb
	// appends "%!(EXTRA string=...)" to the command) corrupted this exact
	// baseline in this repo's own live hypnos.md during manual testing.
	// Substring-only checks above wouldn't catch the appended garbage.
	if strings.Contains(content, "%!(EXTRA") {
		t.Errorf("rendered file has a malformed fmt.Sprintf command (%%!(EXTRA...)):\n%s", content)
	}
	for line := range strings.SplitSeq(content, "\n") {
		if strings.Contains(line, "command:") {
			cmd := strings.TrimSpace(strings.SplitN(line, "command:", 2)[1])
			switch {
			case strings.HasPrefix(cmd, "dreamland telemetry write"):
				if cmd != "dreamland telemetry write --tool claude-code" {
					t.Errorf("telemetry write command has unexpected trailing content: %q", cmd)
				}
			case strings.HasPrefix(cmd, "dreamland version-bump --patch"):
				if cmd != "dreamland version-bump --patch" {
					t.Errorf("version-bump --patch command has unexpected trailing content: %q", cmd)
				}
			case strings.HasPrefix(cmd, "dreamland version-bump --minor"):
				if cmd != "dreamland version-bump --minor --if-agent janus" {
					t.Errorf("version-bump --minor command has unexpected trailing content: %q", cmd)
				}
			}
		}
	}

	// Baseline hook nodes must exist in the graph too, agent-scoped.
	found := 0
	for _, h := range g.Hooks {
		if h.Scope == ScopeAgent {
			found++
		}
	}
	if found != len(claudeStopBaseline) {
		t.Errorf("got %d agent-scoped hook nodes, want %d", found, len(claudeStopBaseline))
	}
}

func TestCreateAgentDuplicateErrors(t *testing.T) {
	root := newClaudeRepo(t)
	g := New(root)
	if err := CreateAgent(root, g, "dup", "desc", TierFullEdit); err != nil {
		t.Fatal(err)
	}
	if err := CreateAgent(root, g, "dup", "desc again", TierFullEdit); err == nil {
		t.Error("expected an error creating a duplicate agent id, got nil")
	}
}

func TestRoutingEdgeRoundTripsThroughImport(t *testing.T) {
	root := newClaudeRepo(t)
	g := New(root)
	if err := CreateAgent(root, g, "alpha", "First agent.", TierFullEdit); err != nil {
		t.Fatal(err)
	}
	if err := CreateAgent(root, g, "beta", "Second agent.", TierFullEdit); err != nil {
		t.Fatal(err)
	}

	if err := AddRoutingEdge(root, g, "alpha", "beta"); err != nil {
		t.Fatalf("AddRoutingEdge: unexpected error %v", err)
	}

	content, err := os.ReadFile(filepath.Join(root, ".claude", "agents", "alpha.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "hand off directly to `beta`") {
		t.Errorf("expected rendered hand-off sentence, got:\n%s", content)
	}

	// Full round-trip: re-import from disk and confirm the edge reappears.
	reimported, err := Import(root)
	if err != nil {
		t.Fatalf("Import: unexpected error %v", err)
	}
	found := false
	for _, e := range reimported.Edges {
		if e.Kind == EdgeRouting && e.From == "alpha" && e.To == "beta" {
			found = true
		}
	}
	if !found {
		t.Error("expected routing edge alpha -> beta to round-trip through Import")
	}

	// Now remove it and confirm the sentence disappears.
	if err := RemoveRoutingEdge(root, g, "alpha", "beta"); err != nil {
		t.Fatalf("RemoveRoutingEdge: unexpected error %v", err)
	}
	content, err = os.ReadFile(filepath.Join(root, ".claude", "agents", "alpha.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "beta") {
		t.Errorf("expected hand-off sentence removed, got:\n%s", content)
	}
}

func TestDeleteAgentRemovesFileAndCleansUpEdges(t *testing.T) {
	root := newClaudeRepo(t)
	g := New(root)
	if err := CreateAgent(root, g, "source", "Source agent.", TierFullEdit); err != nil {
		t.Fatal(err)
	}
	if err := CreateAgent(root, g, "target", "Target agent.", TierFullEdit); err != nil {
		t.Fatal(err)
	}
	if err := AddRoutingEdge(root, g, "source", "target"); err != nil {
		t.Fatal(err)
	}

	if err := DeleteAgent(root, g, "target"); err != nil {
		t.Fatalf("DeleteAgent: unexpected error %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, ".claude", "agents", "target.md")); !os.IsNotExist(err) {
		t.Errorf("expected target.md removed, stat err = %v", err)
	}
	if _, ok := g.Agents["target"]; ok {
		t.Error("expected \"target\" removed from g.Agents")
	}

	content, err := os.ReadFile(filepath.Join(root, ".claude", "agents", "source.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "hand off directly to `target`") {
		t.Errorf("expected source's hand-off sentence to target regenerated away, got:\n%s", content)
	}
}

// TestDeleteAgentDropsDanglingSkillAttachmentEdges is part of
// persist-skill-attachment-edges (tasks.md 3.2): DeleteAgent already dropped
// edges sourced from the deleted agent, but an EdgeAttachment edge targeting
// it (From: skillID, To: agentID) was not covered by that filter. Left
// unfixed, deleting an agent that has an attached skill would leave a
// dangling attachment edge in g.Edges, which — once attachment edges are
// persisted to disk — could be written straight into the new cache file.
func TestDeleteAgentDropsDanglingSkillAttachmentEdges(t *testing.T) {
	root := newClaudeRepo(t)
	skillDir := filepath.Join(root, ".claude", "skills", "openspec-propose")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	skillContent := "---\nname: openspec-propose\ndescription: Propose a change.\n---\n\nBody.\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0o644); err != nil {
		t.Fatal(err)
	}

	g := New(root)
	if err := CreateAgent(root, g, "author", "Authors things.", TierFullEdit); err != nil {
		t.Fatal(err)
	}
	if err := importSkills(root, g); err != nil {
		t.Fatal(err)
	}
	if err := AttachSkill(g, "author", "openspec-propose"); err != nil {
		t.Fatalf("AttachSkill: unexpected error %v", err)
	}

	if err := DeleteAgent(root, g, "author"); err != nil {
		t.Fatalf("DeleteAgent: unexpected error %v", err)
	}

	for _, e := range g.Edges {
		if e.Kind == EdgeAttachment && e.To == "author" {
			t.Errorf("expected no EdgeAttachment edge referencing deleted agent %q to remain, found %+v", "author", e)
		}
	}
}

func TestAttachDetachHookToProject(t *testing.T) {
	root := newClaudeRepo(t)
	g := New(root)

	if err := AttachHookToProject(root, g, EventSessionStart, "dreamland version-bump"); err != nil {
		t.Fatalf("AttachHookToProject: unexpected error %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	hooks := doc["hooks"].(map[string]any)
	sessionStart := hooks["SessionStart"].([]any)
	if len(sessionStart) != 1 {
		t.Fatalf("expected one SessionStart binding, got %d", len(sessionStart))
	}

	// Attaching the same command again must not duplicate it.
	if err := AttachHookToProject(root, g, EventSessionStart, "dreamland version-bump"); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(filepath.Join(root, ".claude", "settings.json"))
	json.Unmarshal(data, &doc)
	hooks = doc["hooks"].(map[string]any)
	binding := hooks["SessionStart"].([]any)[0].(map[string]any)
	if len(binding["hooks"].([]any)) != 1 {
		t.Errorf("expected exactly one command after re-attaching, got %d", len(binding["hooks"].([]any)))
	}

	if projectHooks := countProjectHooks(g); projectHooks != 1 {
		t.Errorf("expected 1 project-scoped hook node in graph, got %d", projectHooks)
	}

	if err := DetachHookFromProject(root, g, EventSessionStart, "dreamland version-bump"); err != nil {
		t.Fatalf("DetachHookFromProject: unexpected error %v", err)
	}
	data, err = os.ReadFile(filepath.Join(root, ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	json.Unmarshal(data, &doc)
	hooks = doc["hooks"].(map[string]any)
	if v, ok := hooks["SessionStart"]; ok && len(v.([]any)) != 0 {
		t.Errorf("expected SessionStart binding removed entirely, got %+v", v)
	}
	if projectHooks := countProjectHooks(g); projectHooks != 0 {
		t.Errorf("expected 0 project-scoped hook nodes after detach, got %d", projectHooks)
	}
}

func TestAttachHookToProjectFallsBackPerAgentOnCopilot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".github", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}

	g := New(root)
	if err := CreateAgent(root, g, "one", "First.", TierFullEdit); err != nil {
		t.Fatal(err)
	}
	if err := CreateAgent(root, g, "two", "Second.", TierFullEdit); err != nil {
		t.Fatal(err)
	}

	if err := AttachHookToProject(root, g, EventSubagentStop, "dreamland custom-audit"); err != nil {
		t.Fatalf("AttachHookToProject: unexpected error %v", err)
	}

	for _, id := range []string{"one", "two"} {
		content, err := os.ReadFile(filepath.Join(root, ".github", "agents", id+".agent.md"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(content), "dreamland custom-audit") {
			t.Errorf("agent %q: expected the fanned-out hook command in its frontmatter, got:\n%s", id, content)
		}
	}

	if err := DetachHookFromProject(root, g, EventSubagentStop, "dreamland custom-audit"); err != nil {
		t.Fatalf("DetachHookFromProject: unexpected error %v", err)
	}
	for _, id := range []string{"one", "two"} {
		content, err := os.ReadFile(filepath.Join(root, ".github", "agents", id+".agent.md"))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(content), "dreamland custom-audit") {
			t.Errorf("agent %q: expected the fanned-out hook command removed, got:\n%s", id, content)
		}
	}
}

func countProjectHooks(g *Graph) int {
	n := 0
	for _, h := range g.Hooks {
		if h.Scope == ScopeProject {
			n++
		}
	}
	return n
}

func TestAttachDetachHookToAgent(t *testing.T) {
	root := newClaudeRepo(t)
	g := New(root)
	if err := CreateAgent(root, g, "solo", "Solo agent.", TierRouterExcluded); err != nil {
		t.Fatal(err)
	}

	if err := AttachHookToAgent(root, g, "solo", EventPreToolUse, "dreamland custom-check"); err != nil {
		t.Fatalf("AttachHookToAgent: unexpected error %v", err)
	}
	content, err := os.ReadFile(filepath.Join(root, ".claude", "agents", "solo.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "dreamland custom-check") {
		t.Errorf("expected custom hook command in rendered file, got:\n%s", content)
	}

	if err := DetachHookFromAgent(root, g, "solo", EventPreToolUse, "dreamland custom-check"); err != nil {
		t.Fatalf("DetachHookFromAgent: unexpected error %v", err)
	}
	content, err = os.ReadFile(filepath.Join(root, ".claude", "agents", "solo.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "dreamland custom-check") {
		t.Errorf("expected custom hook command removed, got:\n%s", content)
	}
}

func TestAttachDetachSkillNeverTouchesSkillFile(t *testing.T) {
	root := newClaudeRepo(t)
	skillDir := filepath.Join(root, ".claude", "skills", "openspec-propose")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	skillContent := "---\nname: openspec-propose\ndescription: Propose a change.\n---\n\nBody.\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0o644); err != nil {
		t.Fatal(err)
	}

	g := New(root)
	if err := CreateAgent(root, g, "author", "Authors things.", TierFullEdit); err != nil {
		t.Fatal(err)
	}
	if err := importSkills(root, g); err != nil {
		t.Fatal(err)
	}

	if err := AttachSkill(g, "author", "openspec-propose"); err != nil {
		t.Fatalf("AttachSkill: unexpected error %v", err)
	}
	found := false
	for _, e := range g.Edges {
		if e.Kind == EdgeAttachment && e.From == "openspec-propose" && e.To == "author" {
			found = true
		}
	}
	if !found {
		t.Error("expected an attachment edge after AttachSkill")
	}

	after, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != skillContent {
		t.Error("AttachSkill modified the skill's own file — it must not")
	}

	if err := DetachSkill(g, "author", "openspec-propose"); err != nil {
		t.Fatalf("DetachSkill: unexpected error %v", err)
	}
	for _, e := range g.Edges {
		if e.Kind == EdgeAttachment {
			t.Errorf("expected no attachment edges after DetachSkill, found %+v", e)
		}
	}
	after, err = os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != skillContent {
		t.Error("DetachSkill modified the skill's own file — it must not")
	}
}

func TestPlatformAgentFilename(t *testing.T) {
	cases := []struct {
		platform string
		want     string
	}{
		{"claude-code", "myagent.md"},
		{"kiro", "myagent.md"},
		{"cursor", "myagent.mdc"},
		{"codex", "myagent.toml"},
		{"antigravity", filepath.Join("myagent", "SKILL.md")},
		{"github-copilot", "myagent.agent.md"},
	}
	for _, tt := range cases {
		got, err := platformAgentFilename(tt.platform, "myagent")
		if err != nil {
			t.Errorf("%s: unexpected error %v", tt.platform, err)
		}
		if got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.platform, got, tt.want)
		}
	}
}

func TestPlatformAgentFilenameUnknownPlatform(t *testing.T) {
	if _, err := platformAgentFilename("bogus", "myagent"); err == nil {
		t.Error("expected an error for an unknown platform")
	}
}

func TestAttachSkillUnknownAgentOrSkill(t *testing.T) {
	root := newClaudeRepo(t)
	g := New(root)
	if err := CreateAgent(root, g, "author", "desc", TierFullEdit); err != nil {
		t.Fatal(err)
	}
	if err := CreateSkill(root, g, "real-skill", "desc"); err != nil {
		t.Fatal(err)
	}

	if err := AttachSkill(g, "ghost-agent", "real-skill"); err == nil {
		t.Error("expected an error attaching to an unknown agent")
	}
	if err := AttachSkill(g, "author", "ghost-skill"); err == nil {
		t.Error("expected an error attaching an unknown skill")
	}
}

func TestAttachSkillIdempotent(t *testing.T) {
	root := newClaudeRepo(t)
	g := New(root)
	if err := CreateAgent(root, g, "author", "desc", TierFullEdit); err != nil {
		t.Fatal(err)
	}
	if err := CreateSkill(root, g, "real-skill", "desc"); err != nil {
		t.Fatal(err)
	}
	if err := AttachSkill(g, "author", "real-skill"); err != nil {
		t.Fatal(err)
	}
	if err := AttachSkill(g, "author", "real-skill"); err != nil {
		t.Fatalf("second AttachSkill: unexpected error %v", err)
	}
	count := 0
	for _, e := range g.Edges {
		if e.Kind == EdgeAttachment && e.From == "real-skill" && e.To == "author" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly one attachment edge after attaching twice, got %d", count)
	}
}

func TestAddRoutingEdgeUnknownAgents(t *testing.T) {
	root := newClaudeRepo(t)
	g := New(root)
	if err := CreateAgent(root, g, "real", "desc", TierFullEdit); err != nil {
		t.Fatal(err)
	}
	if err := AddRoutingEdge(root, g, "ghost", "real"); err == nil {
		t.Error("expected an error for an unknown source agent")
	}
	if err := AddRoutingEdge(root, g, "real", "ghost"); err == nil {
		t.Error("expected an error for an unknown target agent")
	}
}

func TestAddRoutingEdgeIdempotent(t *testing.T) {
	root := newClaudeRepo(t)
	g := New(root)
	if err := CreateAgent(root, g, "src", "desc", TierFullEdit); err != nil {
		t.Fatal(err)
	}
	if err := CreateAgent(root, g, "dst", "desc", TierFullEdit); err != nil {
		t.Fatal(err)
	}
	if err := AddRoutingEdge(root, g, "src", "dst"); err != nil {
		t.Fatal(err)
	}
	if err := AddRoutingEdge(root, g, "src", "dst"); err != nil {
		t.Fatalf("second AddRoutingEdge: unexpected error %v", err)
	}
	count := 0
	for _, e := range g.Edges {
		if e.Kind == EdgeRouting && e.From == "src" && e.To == "dst" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly one routing edge after adding twice, got %d", count)
	}
}

func TestRemoveUnscopedCommand(t *testing.T) {
	arr := []any{
		"not-a-map", // non-map entry passed through unchanged
		map[string]any{
			"matcher": "SomeTool", // scoped binding, left untouched
			"hooks":   []any{map[string]any{"type": "command", "command": "keep-me"}},
		},
		map[string]any{
			"matcher": "",
			"hooks": []any{
				map[string]any{"type": "command", "command": "remove-me"},
				map[string]any{"type": "command", "command": "keep-unscoped"},
			},
		},
		map[string]any{
			"matcher": "",
			"hooks":   []any{map[string]any{"type": "command", "command": "remove-me"}}, // becomes empty, dropped entirely
		},
	}

	out := removeUnscopedCommand(arr, "remove-me")

	if len(out) != 3 {
		t.Fatalf("expected the fully-emptied unscoped binding dropped, got %d entries: %+v", len(out), out)
	}
	if out[0] != "not-a-map" {
		t.Errorf("expected the non-map entry preserved as-is, got %+v", out[0])
	}
	scoped := out[1].(map[string]any)
	if scoped["matcher"] != "SomeTool" {
		t.Errorf("expected the scoped binding untouched, got %+v", scoped)
	}
	remaining := out[2].(map[string]any)
	hooks := remaining["hooks"].([]any)
	if len(hooks) != 1 {
		t.Errorf("expected exactly one remaining hook after removal, got %+v", hooks)
	}
}
