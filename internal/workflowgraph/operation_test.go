package workflowgraph

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateOperationsRejectsUnknownReferenceBeforeAnyWrite(t *testing.T) {
	root := newClaudeRepo(t)
	g := New(root)

	ops := []Operation{
		{Type: OpCreateEdge, EdgeKind: EdgeRouting, From: "ghost", To: "also-ghost"},
	}
	applied, err := ApplyOperations(root, g, ops)
	if err == nil {
		t.Fatal("expected an error for a plan referencing unknown node ids")
	}
	if applied != 0 {
		t.Errorf("applied = %d, want 0 (plan must be rejected before any write)", applied)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "agents")); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Join(root, ".claude", "agents"))
	if len(entries) != 0 {
		t.Errorf("expected no files written, found %v", entries)
	}
}

func TestValidateOperationsAllowsForwardReferenceWithinSamePlan(t *testing.T) {
	root := newClaudeRepo(t)
	g := New(root)

	// alpha doesn't exist yet, but the plan creates it before the edge that
	// references it — this must validate successfully.
	ops := []Operation{
		{Type: OpCreateNode, Kind: NodeKindAgent, ID: "hypnos", Description: "Author.", Tier: TierFullEdit},
		{Type: OpCreateNode, Kind: NodeKindAgent, ID: "alpha", Description: "New agent.", Tier: TierFullEdit},
		{Type: OpCreateEdge, EdgeKind: EdgeRouting, From: "hypnos", To: "alpha"},
	}
	applied, err := ApplyOperations(root, g, ops)
	if err != nil {
		t.Fatalf("ApplyOperations: unexpected error %v", err)
	}
	if applied != len(ops) {
		t.Errorf("applied = %d, want %d", applied, len(ops))
	}

	content, err := os.ReadFile(filepath.Join(root, ".claude", "agents", "hypnos.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "hand off directly to `alpha`") {
		t.Errorf("expected the hand-off sentence in hypnos.md, got:\n%s", content)
	}
}

func TestApplyOperationsProducesSameResultAsManualEdits(t *testing.T) {
	// Two identically-configured repos: one built via ApplyOperations, one
	// via direct writer.go calls (what /hypnos-interactive would do click by
	// click). Their resulting files must be byte-identical — this is the
	// literal spec scenario "Applying a plan produces the same result as the
	// equivalent manual edits."
	planRoot := newClaudeRepo(t)
	manualRoot := newClaudeRepo(t)

	ops := []Operation{
		{Type: OpCreateNode, Kind: NodeKindAgent, ID: "hypnos", Description: "Author.", Tier: TierFullEdit},
		{Type: OpCreateNode, Kind: NodeKindAgent, ID: "mengpo", Description: "Archivist.", Tier: TierWriteOnlyNoEdit},
		{Type: OpCreateEdge, EdgeKind: EdgeRouting, From: "hypnos", To: "mengpo"},
	}
	planGraph := New(planRoot)
	if _, err := ApplyOperations(planRoot, planGraph, ops); err != nil {
		t.Fatalf("ApplyOperations: unexpected error %v", err)
	}

	manualGraph := New(manualRoot)
	if err := CreateAgent(manualRoot, manualGraph, "hypnos", "Author.", TierFullEdit); err != nil {
		t.Fatal(err)
	}
	if err := CreateAgent(manualRoot, manualGraph, "mengpo", "Archivist.", TierWriteOnlyNoEdit); err != nil {
		t.Fatal(err)
	}
	if err := AddRoutingEdge(manualRoot, manualGraph, "hypnos", "mengpo"); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"hypnos.md", "mengpo.md"} {
		planContent, err := os.ReadFile(filepath.Join(planRoot, ".claude", "agents", name))
		if err != nil {
			t.Fatal(err)
		}
		manualContent, err := os.ReadFile(filepath.Join(manualRoot, ".claude", "agents", name))
		if err != nil {
			t.Fatal(err)
		}
		if string(planContent) != string(manualContent) {
			t.Errorf("%s differs between plan-apply and manual edits:\n--- plan ---\n%s\n--- manual ---\n%s", name, planContent, manualContent)
		}
	}
}

func TestApplyOperationsStopsAtFirstRuntimeError(t *testing.T) {
	root := newClaudeRepo(t)
	g := New(root)
	if err := CreateAgent(root, g, "existing", "desc", TierFullEdit); err != nil {
		t.Fatal(err)
	}

	ops := []Operation{
		{Type: OpCreateNode, Kind: NodeKindAgent, ID: "new-one", Description: "desc", Tier: TierFullEdit},
		{Type: OpDeleteNode, Kind: NodeKindSkill, ID: "existing"}, // wrong kind for an agent id — runtime error
		{Type: OpCreateNode, Kind: NodeKindAgent, ID: "never-reached", Description: "desc", Tier: TierFullEdit},
	}
	applied, err := ApplyOperations(root, g, ops)
	if err == nil {
		t.Fatal("expected a runtime error from the mismatched-kind delete")
	}
	if applied != 1 {
		t.Errorf("applied = %d, want 1 (only the first operation should have run)", applied)
	}
	if _, ok := g.Agents["never-reached"]; ok {
		t.Error("expected the third operation to never run")
	}
}

func TestApplyOperationsHookBindingEdges(t *testing.T) {
	root := newClaudeRepo(t)
	g := New(root)
	if err := CreateAgent(root, g, "solo", "desc", TierRouterExcluded); err != nil {
		t.Fatal(err)
	}

	ops := []Operation{
		{Type: OpCreateEdge, EdgeKind: EdgeHookBinding, To: "solo", Event: EventPreToolUse, Command: "dreamland custom-check"},
	}
	if _, err := ApplyOperations(root, g, ops); err != nil {
		t.Fatalf("ApplyOperations: unexpected error %v", err)
	}
	content, err := os.ReadFile(filepath.Join(root, ".claude", "agents", "solo.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "dreamland custom-check") {
		t.Errorf("expected hook command in rendered file, got:\n%s", content)
	}
}
