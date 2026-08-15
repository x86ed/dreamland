package cmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"dreamland/internal/workflowgraph"
)

func newTestCommand() *cobra.Command {
	c := &cobra.Command{}
	c.SetOut(&bytes.Buffer{})
	c.SetErr(&bytes.Buffer{})
	return c
}

func writePlanFile(t *testing.T, dir string, ops []workflowgraph.Operation) string {
	t.Helper()
	data, err := json.Marshal(ops)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "plan.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func newTestClaudeRepo(t *testing.T) string {
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

func TestViewModeHasNoMutateRoute(t *testing.T) {
	root := newTestClaudeRepo(t)
	g, err := workflowgraph.Import(root)
	if err != nil {
		t.Fatal(err)
	}
	guarded := &guardedGraph{g: g}
	broadcaster := workflowgraph.NewBroadcaster()

	mux, err := newHypnosMux(root, false, guarded, broadcaster)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/api/mutate", "application/json", strings.NewReader("[]"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("view-mode POST /api/mutate: got %d, want %d (404, not a permission error)", resp.StatusCode, http.StatusNotFound)
	}
}

func TestViewModeGraphRouteReturnsCurrentGraph(t *testing.T) {
	root := newTestClaudeRepo(t)
	graph := workflowgraph.New(root)
	graph.Agents["hypnos"] = &workflowgraph.AgentNode{ID: "hypnos", Description: "test"}
	guarded := &guardedGraph{g: graph}
	broadcaster := workflowgraph.NewBroadcaster()

	mux, err := newHypnosMux(root, false, guarded, broadcaster)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/graph")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/graph: got %d, want 200", resp.StatusCode)
	}
	var got workflowgraph.Graph
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if _, ok := got.Agents["hypnos"]; !ok {
		t.Errorf("expected agent \"hypnos\" in the returned graph, got %+v", got.Agents)
	}
}

func TestInteractiveModeMutateRouteAppliesAndPersists(t *testing.T) {
	root := newTestClaudeRepo(t)
	g, err := workflowgraph.Import(root)
	if err != nil {
		t.Fatal(err)
	}
	guarded := &guardedGraph{g: g}
	broadcaster := workflowgraph.NewBroadcaster()

	mux, err := newHypnosMux(root, true, guarded, broadcaster)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ops := []workflowgraph.Operation{
		{Type: workflowgraph.OpCreateNode, Kind: workflowgraph.NodeKindAgent, ID: "webagent", Description: "Created via HTTP.", Tier: workflowgraph.TierFullEdit},
	}
	body, _ := json.Marshal(ops)
	resp, err := http.Post(srv.URL+"/api/mutate", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/mutate: got %d, want 200", resp.StatusCode)
	}
	var result map[string]int
	json.NewDecoder(resp.Body).Decode(&result)
	if result["applied"] != 1 {
		t.Errorf("applied = %d, want 1", result["applied"])
	}

	if _, err := os.Stat(filepath.Join(root, ".claude", "agents", "webagent.md")); err != nil {
		t.Errorf("expected webagent.md written to disk: %v", err)
	}
	if _, ok := guarded.Get().Agents["webagent"]; !ok {
		t.Error("expected guardedGraph to reflect the new agent after mutate")
	}

	cacheData, err := os.ReadFile(cachePathFor(root))
	if err != nil {
		t.Fatalf("expected graph cache written: %v", err)
	}
	if !strings.Contains(string(cacheData), "webagent") {
		t.Error("expected the graph cache to include the new agent")
	}
}

func TestInteractiveModeMutateRouteInvalidOpReturns400(t *testing.T) {
	root := newTestClaudeRepo(t)
	g, err := workflowgraph.Import(root)
	if err != nil {
		t.Fatal(err)
	}
	guarded := &guardedGraph{g: g}
	broadcaster := workflowgraph.NewBroadcaster()

	mux, err := newHypnosMux(root, true, guarded, broadcaster)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ops := []workflowgraph.Operation{
		{Type: workflowgraph.OpCreateEdge, EdgeKind: workflowgraph.EdgeRouting, From: "ghost", To: "also-ghost"},
	}
	body, _ := json.Marshal(ops)
	resp, err := http.Post(srv.URL+"/api/mutate", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("got %d, want 400 for a plan referencing unknown node ids", resp.StatusCode)
	}
}

func TestSSEEventsRouteStreamsOnPublish(t *testing.T) {
	root := newTestClaudeRepo(t)
	g, err := workflowgraph.Import(root)
	if err != nil {
		t.Fatal(err)
	}
	guarded := &guardedGraph{g: g}
	broadcaster := workflowgraph.NewBroadcaster()

	mux, err := newHypnosMux(root, false, guarded, broadcaster)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/events: got %d, want 200", resp.StatusCode)
	}

	// Give the handler a moment to subscribe before publishing.
	time.Sleep(20 * time.Millisecond)
	broadcaster.Publish()

	scanner := bufio.NewScanner(resp.Body)
	found := false
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "data: refresh") {
			found = true
			break
		}
	}
	if err := scanner.Err(); err != nil && !found {
		t.Logf("scanner ended with error (expected on context timeout): %v", err)
	}
	if !found {
		t.Error("expected a \"data: refresh\" line after Publish")
	}
}

func TestRunApplyPlanAppliesFromRealFile(t *testing.T) {
	root := newTestClaudeRepo(t)
	ops := []workflowgraph.Operation{
		{Type: workflowgraph.OpCreateNode, Kind: workflowgraph.NodeKindAgent, ID: "planned", Description: "Created headlessly.", Tier: workflowgraph.TierFullEdit},
	}
	planPath := writePlanFile(t, t.TempDir(), ops)

	c := newTestCommand()
	if err := runApplyPlan(c, root, planPath); err != nil {
		t.Fatalf("runApplyPlan: unexpected error %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "agents", "planned.md")); err != nil {
		t.Errorf("expected planned.md written: %v", err)
	}
	out := c.OutOrStdout().(*bytes.Buffer).String()
	if !strings.Contains(out, "applied 1/1") {
		t.Errorf("expected output to report applied 1/1, got %q", out)
	}
}

func TestRunApplyPlanRejectsInvalidReferenceBeforeAnyWrite(t *testing.T) {
	root := newTestClaudeRepo(t)
	ops := []workflowgraph.Operation{
		{Type: workflowgraph.OpCreateEdge, EdgeKind: workflowgraph.EdgeRouting, From: "ghost", To: "also-ghost"},
	}
	planPath := writePlanFile(t, t.TempDir(), ops)

	c := newTestCommand()
	if err := runApplyPlan(c, root, planPath); err == nil {
		t.Fatal("expected an error for a plan referencing unknown node ids")
	}
	entries, _ := os.ReadDir(filepath.Join(root, ".claude", "agents"))
	if len(entries) != 0 {
		t.Errorf("expected no files written, found %v", entries)
	}
}

func TestRunApplyPlanRequiresPlanFlag(t *testing.T) {
	c := newTestCommand()
	if err := runApplyPlan(c, t.TempDir(), ""); err == nil {
		t.Error("expected an error when --plan is empty")
	}
}

func TestRunApplyPlanStopsAtFirstRuntimeErrorAndReportsPartialCount(t *testing.T) {
	root := newTestClaudeRepo(t)
	g, err := workflowgraph.Import(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := workflowgraph.CreateAgent(root, g, "existing", "desc", workflowgraph.TierFullEdit); err != nil {
		t.Fatal(err)
	}

	ops := []workflowgraph.Operation{
		{Type: workflowgraph.OpCreateNode, Kind: workflowgraph.NodeKindAgent, ID: "first", Description: "d", Tier: workflowgraph.TierFullEdit},
		{Type: workflowgraph.OpDeleteNode, Kind: workflowgraph.NodeKindSkill, ID: "existing"}, // wrong kind — runtime error
	}
	planPath := writePlanFile(t, t.TempDir(), ops)

	c := newTestCommand()
	err = runApplyPlan(c, root, planPath)
	if err == nil {
		t.Fatal("expected a runtime error from the mismatched-kind delete")
	}
	out := c.OutOrStdout().(*bytes.Buffer).String()
	if !strings.Contains(out, "applied 1/2") {
		t.Errorf("expected output to report applied 1/2, got %q", out)
	}
	if _, statErr := os.Stat(filepath.Join(root, ".claude", "agents", "first.md")); statErr != nil {
		t.Errorf("expected the first (successful) operation's file to exist: %v", statErr)
	}
}
