package workflowgraph

import (
	"path/filepath"
	"testing"
)

func TestLoadMissingCacheReturnsNilNotError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workflow-graph.json")

	g, err := Load(path)
	if err != nil {
		t.Fatalf("Load on missing file: unexpected error %v", err)
	}
	if g != nil {
		t.Fatalf("Load on missing file: expected nil graph, got %+v", g)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workflow-graph.json")

	g := New("/repo")
	g.Agents["hypnos"] = &AgentNode{
		ID:              "hypnos",
		Description:     "Authors new agent definitions.",
		Tier:            TierFullEdit,
		InstructionBody: "You are the Hypnos agent...",
	}
	g.Hooks["hypnos-stop-telemetry"] = &HookNode{
		ID:      "hypnos-stop-telemetry",
		Command: "dreamland telemetry write --tool claude-code",
		Event:   EventTurnEnd,
		Scope:   ScopeAgent,
	}
	g.Skills["openspec-propose"] = &SkillNode{
		ID:          "openspec-propose",
		Description: "Propose a new change.",
		Owner:       OwnerExternal,
	}
	g.Edges = append(g.Edges, Edge{Kind: EdgeHookBinding, From: "hypnos-stop-telemetry", To: "hypnos"})

	if err := Save(path, g); err != nil {
		t.Fatalf("Save: unexpected error %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: unexpected error %v", err)
	}
	if loaded == nil {
		t.Fatal("Load: expected a graph, got nil")
	}

	if loaded.Project.RepoRoot != "/repo" {
		t.Errorf("RepoRoot = %q, want %q", loaded.Project.RepoRoot, "/repo")
	}
	agent, ok := loaded.Agents["hypnos"]
	if !ok {
		t.Fatal("expected agent \"hypnos\" to round-trip")
	}
	if agent.Tier != TierFullEdit {
		t.Errorf("agent.Tier = %q, want %q", agent.Tier, TierFullEdit)
	}
	hook, ok := loaded.Hooks["hypnos-stop-telemetry"]
	if !ok {
		t.Fatal("expected hook to round-trip")
	}
	if hook.Scope != ScopeAgent {
		t.Errorf("hook.Scope = %q, want %q", hook.Scope, ScopeAgent)
	}
	skill, ok := loaded.Skills["openspec-propose"]
	if !ok {
		t.Fatal("expected skill to round-trip")
	}
	if skill.Owner != OwnerExternal {
		t.Errorf("skill.Owner = %q, want %q", skill.Owner, OwnerExternal)
	}
	if len(loaded.Edges) != 1 || loaded.Edges[0].Kind != EdgeHookBinding {
		t.Errorf("edges did not round-trip: %+v", loaded.Edges)
	}
}
