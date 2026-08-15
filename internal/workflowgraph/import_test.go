package workflowgraph

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// repoRootForTest resolves this package's containing repository root
// (internal/workflowgraph -> repo root is two levels up).
func repoRootForTest(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("expected %s to be the dreamland repo root (go.mod not found): %v", root, err)
	}
	return root
}

func TestImportAgainstThisRepo(t *testing.T) {
	repoRoot := repoRootForTest(t)

	g, err := Import(repoRoot)
	if err != nil {
		t.Fatalf("Import: unexpected error %v", err)
	}

	// This repo has only Claude Code installed (confirmed: no .cursor/.codex/
	// .kiro/.agents/.github/agents directories exist here) — the importer
	// must not error on the five missing platforms, and every agent found
	// must carry a claude-code platform file.
	wantAgents := []string{"janus", "phantasos", "nyx", "morpheus", "phobetor", "baku", "iktomi", "zhougong", "hypnos", "mengpo"}
	if len(g.Agents) != len(wantAgents) {
		t.Errorf("got %d agents, want %d: %v", len(g.Agents), len(wantAgents), agentIDs(g))
	}
	for _, id := range wantAgents {
		agent, ok := g.Agents[id]
		if !ok {
			t.Errorf("expected agent %q to be imported", id)
			continue
		}
		if agent.PlatformFiles["claude-code"] == "" {
			t.Errorf("agent %q: expected a claude-code platform file", id)
		}
		if agent.Description == "" {
			t.Errorf("agent %q: expected a non-empty description", id)
		}
		for _, platform := range []string{"cursor", "codex", "kiro", "antigravity", "github-copilot"} {
			if agent.PlatformFiles[platform] != "" {
				t.Errorf("agent %q: unexpected %s platform file — this repo has no %s installed", id, platform, platform)
			}
		}
	}

	// hypnos has an explicit tools: Read, Edit, Write, Bash frontmatter line
	// (full-edit tier) — confirmed by reading the live file directly.
	if hypnos := g.Agents["hypnos"]; hypnos != nil && hypnos.Tier != TierFullEdit {
		t.Errorf("hypnos.Tier = %q, want %q", hypnos.Tier, TierFullEdit)
	}
	// janus is router-only: Read, Bash — no Edit/Write.
	if janus := g.Agents["janus"]; janus != nil && janus.Tier != TierRouterExcluded {
		t.Errorf("janus.Tier = %q, want %q", janus.Tier, TierRouterExcluded)
	}

	// The four openspec-* skills exist under .claude/skills/ and carry no
	// dreamland-managed marker — they must import as owner: external.
	wantSkills := []string{"openspec-propose", "openspec-explore", "openspec-apply-change", "openspec-archive-change"}
	for _, id := range wantSkills {
		skill, ok := g.Skills[id]
		if !ok {
			t.Errorf("expected skill %q to be imported", id)
			continue
		}
		if skill.Owner != OwnerExternal {
			t.Errorf("skill %q: Owner = %q, want %q", id, skill.Owner, OwnerExternal)
		}
	}

	// .claude/settings.json has real workspace-level hooks (SessionStart,
	// PreToolUse, PostToolUse, Stop, SubagentStop) — at least one
	// project-scoped hook node must exist for each recognized event.
	projectEvents := map[HookEvent]bool{}
	agentScopedCount := 0
	for _, h := range g.Hooks {
		if h.Scope == ScopeProject {
			projectEvents[h.Event] = true
		} else {
			agentScopedCount++
		}
	}
	for _, event := range []HookEvent{EventSessionStart, EventPreToolUse, EventPostToolUse, EventStop, EventSubagentStop} {
		if !projectEvents[event] {
			t.Errorf("expected a project-scoped hook for event %q (from .claude/settings.json)", event)
		}
	}
	// This repo's live .claude/agents/*.md files carry no `hooks:` frontmatter
	// block at all (checked all ten directly — a real, pre-existing drift from
	// the current template, which does have one; out of scope to fix here).
	// The importer must reflect that accurately: zero agent-scoped hooks, not
	// fabricate any from the template's shape.
	if agentScopedCount != 0 {
		t.Errorf("got %d agent-scoped hooks, want 0 — this repo's live agent files have no hooks: block today", agentScopedCount)
	}

	// Every project-scoped hook must have a hookbinding edge to "project", and
	// every agent-scoped hook must bind to a known agent id.
	for _, e := range g.Edges {
		if e.Kind != EdgeHookBinding {
			continue
		}
		hook, ok := g.Hooks[e.From]
		if !ok {
			t.Errorf("hookbinding edge %+v: source hook node not found", e)
			continue
		}
		if hook.Scope == ScopeProject && e.To != "project" {
			t.Errorf("project-scoped hook %q has edge to %q, want \"project\"", hook.ID, e.To)
		}
		if hook.Scope == ScopeAgent {
			if _, ok := g.Agents[e.To]; !ok {
				t.Errorf("agent-scoped hook %q has edge to unknown agent %q", hook.ID, e.To)
			}
		}
	}

	// Routing: hypnos's real instruction body hands off to phobetor and
	// mengpo — both should resolve as edges, and the stored instruction body
	// should no longer contain the raw "hand off directly to `phobetor`"
	// sentence once stripped.
	if hypnos := g.Agents["hypnos"]; hypnos != nil {
		var toPhobetor bool
		for _, e := range g.Edges {
			if e.Kind == EdgeRouting && e.From == "hypnos" && e.To == "phobetor" {
				toPhobetor = true
			}
		}
		if !toPhobetor {
			t.Error("expected a routing edge hypnos -> phobetor (present in the real instruction body)")
		}
		if handOffPattern.MatchString(hypnos.InstructionBody) {
			t.Error("hypnos.InstructionBody still contains a raw hand-off sentence after import — expected it stripped")
		}
	}
}

func agentIDs(g *Graph) []string {
	ids := make([]string, 0, len(g.Agents))
	for id := range g.Agents {
		ids = append(ids, id)
	}
	return ids
}

func TestExtractRoutesTo(t *testing.T) {
	agents := map[string]*AgentNode{
		"phobetor": {ID: "phobetor"},
		"mengpo":   {ID: "mengpo"},
	}

	t.Run("canonical match resolves and strips the sentence", func(t *testing.T) {
		body := "Do the work. Once done, hand off directly to `phobetor` for validation. That's it."
		cleaned, targets, unresolved := extractRoutesTo(body, agents)
		if len(targets) != 1 || targets[0] != "phobetor" {
			t.Errorf("targets = %v, want [phobetor]", targets)
		}
		if unresolved {
			t.Error("unresolved = true, want false (canonical match found)")
		}
		if handOffPattern.MatchString(cleaned) {
			t.Errorf("cleaned body still contains the hand-off sentence: %q", cleaned)
		}
		if !containsAll(cleaned, "Do the work.", "That's it.") {
			t.Errorf("cleaned body lost unrelated content: %q", cleaned)
		}
	})

	t.Run("target not a known agent id is left as free text, no edge", func(t *testing.T) {
		body := "Once done, hand off directly to `nonexistent-agent` for review."
		cleaned, targets, unresolved := extractRoutesTo(body, agents)
		if len(targets) != 0 {
			t.Errorf("targets = %v, want none", targets)
		}
		if cleaned != body {
			t.Errorf("cleaned = %q, want unchanged %q", cleaned, body)
		}
		_ = unresolved
	})

	t.Run("near-miss phrasing flags unresolved without fabricating an edge", func(t *testing.T) {
		body := "When finished, report to the validation agent for review."
		_, targets, unresolved := extractRoutesTo(body, agents)
		if len(targets) != 0 {
			t.Errorf("targets = %v, want none", targets)
		}
		if !unresolved {
			t.Error("unresolved = false, want true (near-miss hand-off phrasing present)")
		}
	})

	t.Run("no hand-off mention at all is not flagged", func(t *testing.T) {
		_, targets, unresolved := extractRoutesTo("Just do the work and report completion.", agents)
		if len(targets) != 0 || unresolved {
			t.Errorf("targets=%v unresolved=%v, want none/false", targets, unresolved)
		}
	})
}

func containsAll(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

func TestDeriveTier(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want Tier
	}{
		{"empty", "", ""},
		{"claude full edit", "Read, Edit, Write, Bash", TierFullEdit},
		{"claude router excluded", "Read, Bash", TierRouterExcluded},
		{"claude write only", "Write, Read, Bash", TierWriteOnlyNoEdit},
		{"copilot bracketed full edit", "[Read, Edit, Write, Bash, agent]", TierFullEdit},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := deriveTier(tt.in); got != tt.want {
				t.Errorf("deriveTier(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestImportAgentHooksFixture exercises the per-agent frontmatter hooks-block
// parsing path directly, since this repo's own live agent files currently
// have no hooks: block to import (see TestImportAgainstThisRepo) — using the
// exact block shape from internal/scaffold/templates/agents/claude-code/hypnos.md.
func TestImportAgentHooksFixture(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".claude", "agents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `---
name: hypnos
description: Authors new agent definitions.
tools: Read, Edit, Write, Bash
hooks:
  Stop:
    - hooks:
        - type: command
          command: dreamland coauthor --hook --agent-name hypnos
        - type: command
          command: dreamland telemetry write --tool claude-code
        - type: command
          command: dreamland version-bump --patch
        - type: command
          command: dreamland version-bump --minor --if-agent janus
        - type: command
          command: dreamland commit --reason handoff --agent-name hypnos
---

Body text.
`
	if err := os.WriteFile(filepath.Join(dir, "hypnos.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	g := New(root)
	g.Agents["hypnos"] = &AgentNode{ID: "hypnos", PlatformFiles: map[string]string{}}
	if err := importAgentHooks(root, g); err != nil {
		t.Fatalf("importAgentHooks: unexpected error %v", err)
	}

	wantCommands := []string{
		"dreamland coauthor --hook --agent-name hypnos",
		"dreamland telemetry write --tool claude-code",
		"dreamland version-bump --patch",
		"dreamland version-bump --minor --if-agent janus",
		"dreamland commit --reason handoff --agent-name hypnos",
	}
	if len(g.Hooks) != len(wantCommands) {
		t.Fatalf("got %d hook nodes, want %d: %+v", len(g.Hooks), len(wantCommands), g.Hooks)
	}
	for _, h := range g.Hooks {
		if h.Scope != ScopeAgent {
			t.Errorf("hook %q: Scope = %q, want %q", h.ID, h.Scope, ScopeAgent)
		}
		if h.Event != EventStop {
			t.Errorf("hook %q: Event = %q, want %q", h.ID, h.Event, EventStop)
		}
	}
	edgeTargets := map[string]bool{}
	for _, e := range g.Edges {
		if e.Kind == EdgeHookBinding {
			edgeTargets[e.To] = true
		}
	}
	if !edgeTargets["hypnos"] || len(edgeTargets) != 1 {
		t.Errorf("expected all hookbinding edges to target \"hypnos\", got %+v", edgeTargets)
	}
}

func TestImportSkillsOwner(t *testing.T) {
	root := t.TempDir()
	writeSkill := func(id, extra string) {
		dir := filepath.Join(root, ".claude", "skills", id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		content := "---\nname: " + id + "\ndescription: test skill\n---\n\nBody.\n" + extra
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeSkill("external-skill", "")
	writeSkill("dreamland-skill", "\n"+DreamlandManagedMarker+"\n")

	g := New(root)
	if err := importSkills(root, g); err != nil {
		t.Fatalf("importSkills: unexpected error %v", err)
	}

	if s := g.Skills["external-skill"]; s == nil || s.Owner != OwnerExternal {
		t.Errorf("external-skill: got %+v, want Owner=external", s)
	}
	if s := g.Skills["dreamland-skill"]; s == nil || s.Owner != OwnerDreamland {
		t.Errorf("dreamland-skill: got %+v, want Owner=dreamland", s)
	}
}
