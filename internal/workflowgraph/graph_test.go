package workflowgraph

import (
	"os"
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

// TestSaveCreatesParentDir is a regression test: Save used to call
// os.WriteFile directly with no os.MkdirAll first, so it errored on a fresh
// repo where .dreamland/ doesn't exist yet — caught via a real browser test
// against a brand-new temp repo (existing repos happened to already have
// .dreamland/ from other files, masking the gap).
func TestSaveCreatesParentDir(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".dreamland", "workflow-graph.json")

	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Fatalf("test setup: .dreamland should not exist yet, stat err = %v", err)
	}

	if err := Save(path, New(root)); err != nil {
		t.Fatalf("Save: unexpected error %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected the cache file to exist: %v", err)
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
		Event:   EventStop,
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
