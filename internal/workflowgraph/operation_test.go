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

func TestApplyOperationsCreateSkillNode(t *testing.T) {
	root := newClaudeRepo(t)
	g := New(root)

	ops := []Operation{
		{Type: OpCreateNode, Kind: NodeKindSkill, ID: "new-skill", Description: "Created via a plan."},
	}
	applied, err := ApplyOperations(root, g, ops)
	if err != nil {
		t.Fatalf("ApplyOperations: unexpected error %v", err)
	}
	if applied != 1 {
		t.Errorf("applied = %d, want 1", applied)
	}
	skill, ok := g.Skills["new-skill"]
	if !ok || skill.Owner != OwnerDreamland {
		t.Errorf("expected skill %+v with Owner=dreamland", skill)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "skills", "new-skill", "SKILL.md")); err != nil {
		t.Errorf("expected SKILL.md written: %v", err)
	}
}

func TestApplyOneCreateNodeUnsupportedKind(t *testing.T) {
	root := newClaudeRepo(t)
	g := New(root)
	err := applyOne(root, g, Operation{Type: OpCreateNode, Kind: NodeKindHook, ID: "x"})
	if err == nil {
		t.Error("expected an error creating a hook node directly")
	}
}

func TestApplyOneUpdateNodeUnsupportedKind(t *testing.T) {
	root := newClaudeRepo(t)
	g := New(root)
	err := applyOne(root, g, Operation{Type: OpUpdateNode, Kind: NodeKindSkill, ID: "x"})
	if err == nil {
		t.Error("expected an error updating a non-agent node kind")
	}
}

func TestApplyOneUpdateNodeUnknownAgent(t *testing.T) {
	root := newClaudeRepo(t)
	g := New(root)
	err := applyOne(root, g, Operation{Type: OpUpdateNode, Kind: NodeKindAgent, ID: "ghost"})
	if err == nil {
		t.Error("expected an error updating an unknown agent")
	}
}

func TestApplyOneUpdateNodePositionOnlyDoesNotSync(t *testing.T) {
	root := newClaudeRepo(t)
	g := New(root)
	if err := CreateAgent(root, g, "mover", "desc", TierFullEdit); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(filepath.Join(root, ".claude", "agents", "mover.md"))
	if err != nil {
		t.Fatal(err)
	}

	posX, posY := 42.0, 7.0
	if err := applyOne(root, g, Operation{Type: OpUpdateNode, Kind: NodeKindAgent, ID: "mover", PosX: &posX, PosY: &posY}); err != nil {
		t.Fatalf("applyOne: unexpected error %v", err)
	}
	if g.Agents["mover"].PosX != 42.0 || g.Agents["mover"].PosY != 7.0 {
		t.Errorf("position not updated: got (%v, %v)", g.Agents["mover"].PosX, g.Agents["mover"].PosY)
	}
	after, err := os.Stat(filepath.Join(root, ".claude", "agents", "mover.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !before.ModTime().Equal(after.ModTime()) {
		t.Error("expected a position-only update to leave the platform file untouched (no sync)")
	}
}

func TestApplyOneDeleteNodeUnsupportedKind(t *testing.T) {
	root := newClaudeRepo(t)
	g := New(root)
	err := applyOne(root, g, Operation{Type: OpDeleteNode, Kind: NodeKindHook, ID: "x"})
	if err == nil {
		t.Error("expected an error deleting a hook node directly")
	}
}

func TestApplyOneCreateEdgeAttachment(t *testing.T) {
	root := newClaudeRepo(t)
	g := New(root)
	if err := CreateAgent(root, g, "author", "desc", TierFullEdit); err != nil {
		t.Fatal(err)
	}
	if err := CreateSkill(root, g, "some-skill", "desc"); err != nil {
		t.Fatal(err)
	}
	if err := applyOne(root, g, Operation{Type: OpCreateEdge, EdgeKind: EdgeAttachment, From: "some-skill", To: "author"}); err != nil {
		t.Fatalf("applyOne: unexpected error %v", err)
	}
	found := false
	for _, e := range g.Edges {
		if e.Kind == EdgeAttachment && e.From == "some-skill" && e.To == "author" {
			found = true
		}
	}
	if !found {
		t.Error("expected an EdgeAttachment edge after applyOne")
	}
}

func TestApplyOneCreateEdgeUnknownKind(t *testing.T) {
	root := newClaudeRepo(t)
	g := New(root)
	err := applyOne(root, g, Operation{Type: OpCreateEdge, EdgeKind: EdgeKind("bogus")})
	if err == nil {
		t.Error("expected an error for an unknown create_edge edge kind")
	}
}

func TestApplyOneDeleteEdgeVariants(t *testing.T) {
	root := newClaudeRepo(t)
	g := New(root)
	if err := CreateAgent(root, g, "src", "desc", TierFullEdit); err != nil {
		t.Fatal(err)
	}
	if err := CreateAgent(root, g, "dst", "desc", TierFullEdit); err != nil {
		t.Fatal(err)
	}
	if err := CreateSkill(root, g, "a-skill", "desc"); err != nil {
		t.Fatal(err)
	}
	if err := AddRoutingEdge(root, g, "src", "dst"); err != nil {
		t.Fatal(err)
	}
	if err := AttachSkill(g, "dst", "a-skill"); err != nil {
		t.Fatal(err)
	}
	if err := AttachHookToProject(root, g, EventPreToolUse, "dreamland custom-check"); err != nil {
		t.Fatal(err)
	}
	if err := AttachHookToAgent(root, g, "dst", EventPreToolUse, "dreamland agent-check"); err != nil {
		t.Fatal(err)
	}

	if err := applyOne(root, g, Operation{Type: OpDeleteEdge, EdgeKind: EdgeRouting, From: "src", To: "dst"}); err != nil {
		t.Fatalf("delete routing edge: unexpected error %v", err)
	}
	if err := applyOne(root, g, Operation{Type: OpDeleteEdge, EdgeKind: EdgeAttachment, From: "a-skill", To: "dst"}); err != nil {
		t.Fatalf("delete attachment edge: unexpected error %v", err)
	}
	if err := applyOne(root, g, Operation{Type: OpDeleteEdge, EdgeKind: EdgeHookBinding, To: "project", Event: EventPreToolUse, Command: "dreamland custom-check"}); err != nil {
		t.Fatalf("delete project hookbinding edge: unexpected error %v", err)
	}
	if err := applyOne(root, g, Operation{Type: OpDeleteEdge, EdgeKind: EdgeHookBinding, To: "dst", Event: EventPreToolUse, Command: "dreamland agent-check"}); err != nil {
		t.Fatalf("delete agent hookbinding edge: unexpected error %v", err)
	}

	for _, e := range g.Edges {
		if e.Kind == EdgeRouting && e.From == "src" && e.To == "dst" {
			t.Error("expected the routing edge to be removed")
		}
		if e.Kind == EdgeAttachment {
			t.Error("expected the attachment edge to be removed")
		}
		if e.Kind == EdgeHookBinding {
			t.Error("expected both hookbinding edges to be removed")
		}
	}

	err := applyOne(root, g, Operation{Type: OpDeleteEdge, EdgeKind: EdgeKind("bogus")})
	if err == nil {
		t.Error("expected an error for an unknown delete_edge edge kind")
	}
}

func TestApplyOneUnknownOperationType(t *testing.T) {
	root := newClaudeRepo(t)
	g := New(root)
	err := applyOne(root, g, Operation{Type: OpType("bogus")})
	if err == nil {
		t.Error("expected an error for an unknown operation type")
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
