package cmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"dreamland/internal/workflowgraph"
)

// syncBuffer is a mutex-guarded byte buffer, safe to write from a goroutine
// running serveGraph while the test goroutine polls its contents.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// withStubbedBrowser overrides execCommand so openBrowser's exec.Cmd never
// actually shells out to a real "open"/"xdg-open"/"rundll32" — it runs "true"
// instead, an always-succeeding no-op, so tests that exercise serveGraph or
// openBrowser directly never launch a real browser.
func withStubbedBrowser(t *testing.T) (gotName *string, gotArgs *[]string) {
	t.Helper()
	orig := execCommand
	var name string
	var args []string
	execCommand = func(n string, a ...string) *exec.Cmd {
		name = n
		args = a
		return exec.Command("true")
	}
	t.Cleanup(func() { execCommand = orig })
	return &name, &args
}

func resetHypnosServeFlags(t *testing.T) {
	t.Helper()
	origMode, origPlan := hypnosServeMode, hypnosServePlan
	t.Cleanup(func() {
		hypnosServeMode = origMode
		hypnosServePlan = origPlan
	})
}

func chdirTemp(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

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

// writeTestSkill installs a minimal skill directory at root, so importSkills
// (called by workflowgraph.Import/rebuildGraph) picks it up as a SkillNode.
func writeTestSkill(t *testing.T, root, skillID string) {
	t.Helper()
	skillDir := filepath.Join(root, ".claude", "skills", skillID)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: " + skillID + "\ndescription: A test skill.\n---\n\nBody.\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
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

func TestStaticAssetsServedThroughEmbeddedFS(t *testing.T) {
	root := newTestClaudeRepo(t)
	g, err := workflowgraph.Import(root)
	if err != nil {
		t.Fatal(err)
	}
	mux, err := newHypnosMux(root, false, &guardedGraph{g: g}, workflowgraph.NewBroadcaster())
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	for _, path := range []string{"/", "/app.js", "/nodes.js", "/growable.js", "/vendor/litegraph.min.js", "/vendor/litegraph.css"} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s: got %d, want 200", path, resp.StatusCode)
		}
		if len(body) == 0 {
			t.Errorf("%s: empty body", path)
		}
	}

	// litegraph.min.js should be a real, substantial vendored asset, not a stub.
	resp, err := http.Get(srv.URL + "/vendor/litegraph.min.js")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if len(body) < 100000 {
		t.Errorf("vendor/litegraph.min.js is only %d bytes — expected a real vendored library", len(body))
	}
	if !strings.Contains(string(body), "registerNodeType") {
		t.Error("vendor/litegraph.min.js does not contain \"registerNodeType\" — wrong asset or corrupted vendoring")
	}
}

func TestAPIStatusRoute(t *testing.T) {
	root := newTestClaudeRepo(t)
	if err := os.WriteFile(filepath.Join(root, ".dreamland-session.json"), []byte(`{"agent":"phobetor"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	g, err := workflowgraph.Import(root)
	if err != nil {
		t.Fatal(err)
	}
	mux, err := newHypnosMux(root, false, &guardedGraph{g: g}, workflowgraph.NewBroadcaster())
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/status")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/status: got %d, want 200", resp.StatusCode)
	}
	var got StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.CurrentAgent != "phobetor" {
		t.Errorf("CurrentAgent = %q, want %q", got.CurrentAgent, "phobetor")
	}
	if got.Changes == nil {
		t.Error("Changes should be an empty slice, not null, when marshaled")
	}
}

func TestAPIModeReflectsInteractiveFlag(t *testing.T) {
	root := newTestClaudeRepo(t)
	g, err := workflowgraph.Import(root)
	if err != nil {
		t.Fatal(err)
	}
	broadcaster := workflowgraph.NewBroadcaster()

	for _, interactive := range []bool{false, true} {
		mux, err := newHypnosMux(root, interactive, &guardedGraph{g: g}, broadcaster)
		if err != nil {
			t.Fatal(err)
		}
		srv := httptest.NewServer(mux)
		resp, err := http.Get(srv.URL + "/api/mode")
		if err != nil {
			t.Fatal(err)
		}
		var got map[string]bool
		json.NewDecoder(resp.Body).Decode(&got)
		resp.Body.Close()
		srv.Close()
		if got["interactive"] != interactive {
			t.Errorf("interactive=%v: got /api/mode = %v", interactive, got)
		}
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

	// The positions file is written after every mutation (SavePositions is
	// unconditional), but position is the only thing it persists — the real,
	// meaningful persistence check for a newly created agent is the actual
	// platform file, already asserted above. Confirm the new agent's key is
	// present here only to the extent that's true: every agent, including
	// this one, gets an entry (defaulting to position 0,0 since it was never
	// moved) — not a stand-in for "the agent's data was cached."
	positionsData, err := os.ReadFile(positionsPathFor(root))
	if err != nil {
		t.Fatalf("expected positions file written: %v", err)
	}
	if !strings.Contains(string(positionsData), "webagent") {
		t.Error("expected the positions file to include an entry for the new agent")
	}
}

// TestInteractiveModeAttachSkillSurvivesSubsequentMutation covers
// persist-skill-attachment-edges' "An attached skill survives a subsequent
// unrelated mutation" scenario (specs/litegraph-workflow-editor/spec.md):
// rebuildGraph runs on every /api/mutate call, and before this change had no
// way to carry an EdgeAttachment forward, so a second, unrelated save would
// silently drop a skill attached by the first.
func TestInteractiveModeAttachSkillSurvivesSubsequentMutation(t *testing.T) {
	root := newTestClaudeRepo(t)
	writeTestSkill(t, root, "openspec-propose")

	g, err := workflowgraph.Import(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := workflowgraph.CreateAgent(root, g, "author", "Authors things.", workflowgraph.TierFullEdit); err != nil {
		t.Fatal(err)
	}
	if err := workflowgraph.CreateAgent(root, g, "other", "Another agent.", workflowgraph.TierFullEdit); err != nil {
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

	// First save: attach the skill to "author".
	attachOps := []workflowgraph.Operation{
		{Type: workflowgraph.OpCreateEdge, EdgeKind: workflowgraph.EdgeAttachment, From: "openspec-propose", To: "author"},
	}
	body, _ := json.Marshal(attachOps)
	resp, err := http.Post(srv.URL+"/api/mutate", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("first POST /api/mutate (attach): got %d, want 200", resp.StatusCode)
	}

	// Second, unrelated save: move a different node.
	posX, posY := 100.0, 200.0
	moveOps := []workflowgraph.Operation{
		{Type: workflowgraph.OpUpdateNode, Kind: workflowgraph.NodeKindAgent, ID: "other", PosX: &posX, PosY: &posY},
	}
	body, _ = json.Marshal(moveOps)
	resp, err = http.Post(srv.URL+"/api/mutate", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("second POST /api/mutate (move): got %d, want 200", resp.StatusCode)
	}

	found := false
	for _, e := range guarded.Get().Edges {
		if e.Kind == workflowgraph.EdgeAttachment && e.From == "openspec-propose" && e.To == "author" {
			found = true
		}
	}
	if !found {
		t.Error("expected the skill attachment to survive the second, unrelated mutation")
	}

	data, err := os.ReadFile(skillAttachmentsPathFor(root))
	if err != nil {
		t.Fatalf("expected skill-attachments file written: %v", err)
	}
	if !strings.Contains(string(data), "openspec-propose") || !strings.Contains(string(data), "author") {
		t.Errorf("expected the skill-attachments file to contain the attached pair, got:\n%s", data)
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

// TestWatcherToSSEIntegration wires a real Watcher, Broadcaster, and the
// GET /api/events route together exactly as serveGraph does, then makes a
// real hand-edit to a watched file and confirms an SSE message arrives —
// closing the "hand-edit a platform file while /hypnos-view is open" loop
// (tasks.md §12.4) as far as it can go without a browser: the browser-side
// re-render on receiving that message is app.js's subscribeEvents, which
// cannot be executed outside one.
func TestWatcherToSSEIntegration(t *testing.T) {
	root := newTestClaudeRepo(t)
	if err := os.WriteFile(filepath.Join(root, ".claude", "agents", "existing.md"), []byte("---\nname: existing\ndescription: d\n---\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	guarded := &guardedGraph{}
	broadcaster := workflowgraph.NewBroadcaster()
	fired := make(chan struct{}, 4)
	watcher := workflowgraph.NewWatcher(workflowgraph.WatchPaths(root, ""), 30*time.Millisecond, func() {
		g, err := workflowgraph.Import(root)
		if err != nil {
			return
		}
		guarded.Set(g)
		broadcaster.Publish()
		fired <- struct{}{}
	})
	watcher.Start()
	defer watcher.Stop()

	mux, err := newHypnosMux(root, false, guarded, broadcaster)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	scanner := bufio.NewScanner(resp.Body)
	messages := make(chan string, 4)
	go func() {
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "data:") {
				messages <- line
			}
		}
	}()

	// Give the watcher a moment past its first poll baseline, then make a
	// real hand-edit — description change is enough to advance the mtime.
	time.Sleep(60 * time.Millisecond)
	future := time.Now().Add(time.Second)
	newContent := []byte("---\nname: existing\ndescription: hand-edited\n---\nbody\n")
	path := filepath.Join(root, ".claude", "agents", "existing.md")
	if err := os.WriteFile(path, newContent, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}

	select {
	case <-fired:
	case <-time.After(2 * time.Second):
		t.Fatal("watcher never fired after the hand-edit")
	}
	select {
	case <-messages:
	case <-time.After(2 * time.Second):
		t.Fatal("no SSE message received after the watcher fired")
	}

	g := guarded.Get()
	if g == nil || g.Agents["existing"] == nil || g.Agents["existing"].Description != "hand-edited" {
		t.Errorf("expected the served graph to reflect the hand-edit, got %+v", g)
	}
}

// TestWatcherPollCarriesSkillAttachmentForward covers
// persist-skill-attachment-edges' "An attached skill survives the
// filesystem-watcher's periodic rebuild" scenario, distinctly from the
// explicit-mutation and restart scenarios covered elsewhere: it wires a real
// Watcher calling rebuildGraph (exactly as serveGraph does) rather than
// workflowgraph.Import directly, saves an attachment out-of-band, then makes
// an unrelated hand-edit to trigger a poll-driven rebuild and asserts the
// attachment is still present in the graph the watcher publishes.
func TestWatcherPollCarriesSkillAttachmentForward(t *testing.T) {
	root := newTestClaudeRepo(t)
	writeTestSkill(t, root, "openspec-propose")
	if err := os.WriteFile(filepath.Join(root, ".claude", "agents", "author.md"), []byte("---\nname: author\ndescription: d\n---\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	g, err := rebuildGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := workflowgraph.AttachSkill(g, "author", "openspec-propose"); err != nil {
		t.Fatalf("AttachSkill: unexpected error %v", err)
	}
	if err := workflowgraph.SaveSkillAttachments(skillAttachmentsPathFor(root), g); err != nil {
		t.Fatal(err)
	}

	guarded := &guardedGraph{}
	broadcaster := workflowgraph.NewBroadcaster()
	fired := make(chan struct{}, 4)
	watcher := workflowgraph.NewWatcher(workflowgraph.WatchPaths(root, ""), 30*time.Millisecond, func() {
		rg, err := rebuildGraph(root) // the real production rebuild path, not a bare Import
		if err != nil {
			return
		}
		guarded.Set(rg)
		broadcaster.Publish()
		fired <- struct{}{}
	})
	watcher.Start()
	defer watcher.Stop()

	// Make an unrelated hand-edit to advance the watched mtime and trigger a
	// poll-driven rebuild, with no further mutation of the attachment itself.
	time.Sleep(60 * time.Millisecond)
	future := time.Now().Add(time.Second)
	path := filepath.Join(root, ".claude", "agents", "author.md")
	newContent := []byte("---\nname: author\ndescription: hand-edited\n---\nbody\n")
	if err := os.WriteFile(path, newContent, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}

	select {
	case <-fired:
	case <-time.After(2 * time.Second):
		t.Fatal("watcher never fired after the hand-edit")
	}

	found := false
	for _, e := range guarded.Get().Edges {
		if e.Kind == workflowgraph.EdgeAttachment && e.From == "openspec-propose" && e.To == "author" {
			found = true
		}
	}
	if !found {
		t.Error("expected the skill attachment to survive a watcher-poll-triggered rebuild")
	}
}

// TestRebuildGraphPreservesPositionAcrossRestart reproduces the real bug
// found via manual browser testing: Save Positions appeared to work (it
// persisted across a page reload, served from the same running process's
// in-memory graph) but was silently lost on an actual server restart,
// because every rebuild path called workflowgraph.Import directly — which
// never derives position from anything, since no platform file represents
// it — instead of loading the last-saved cache first. rebuildGraph is the
// fix; this simulates a restart by calling it fresh in a new "process"
// (nothing carried over except what's on disk, exactly like a real restart).
func TestRebuildGraphPreservesPositionAcrossRestart(t *testing.T) {
	root := newTestClaudeRepo(t)
	g, err := rebuildGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := workflowgraph.CreateAgent(root, g, "hypnos", "Author.", workflowgraph.TierFullEdit); err != nil {
		t.Fatal(err)
	}

	// Simulate a Save Positions call: mutate position, save the positions
	// file — the exact sequence the /api/mutate handler performs.
	agent := g.Agents["hypnos"]
	agent.PosX = 485
	agent.PosY = 509
	if err := workflowgraph.SavePositions(positionsPathFor(root), g); err != nil {
		t.Fatal(err)
	}

	// Simulate a server restart: a fresh rebuildGraph call with no in-memory
	// state carried over, only what's on disk (platform files + cache).
	restarted, err := rebuildGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	restartedAgent := restarted.Agents["hypnos"]
	if restartedAgent == nil {
		t.Fatal("expected \"hypnos\" to still exist after rebuild")
	}
	if restartedAgent.PosX != 485 || restartedAgent.PosY != 509 {
		t.Errorf("position lost across simulated restart: got (%v, %v), want (485, 509)", restartedAgent.PosX, restartedAgent.PosY)
	}
}

// TestRebuildGraphPreservesSkillAttachmentAcrossRestart directly mirrors
// TestRebuildGraphPreservesPositionAcrossRestart, for
// persist-skill-attachment-edges' "An attached skill survives a server
// restart" scenario. Simulates the /api/mutate handler's save sequence
// (AttachSkill then SaveSkillAttachments), then a fresh rebuildGraph call
// with no in-memory state carried over — exactly like a real process
// restart.
func TestRebuildGraphPreservesSkillAttachmentAcrossRestart(t *testing.T) {
	root := newTestClaudeRepo(t)
	writeTestSkill(t, root, "openspec-propose")

	g, err := rebuildGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := workflowgraph.CreateAgent(root, g, "author", "Authors things.", workflowgraph.TierFullEdit); err != nil {
		t.Fatal(err)
	}

	if err := workflowgraph.AttachSkill(g, "author", "openspec-propose"); err != nil {
		t.Fatalf("AttachSkill: unexpected error %v", err)
	}
	if err := workflowgraph.SaveSkillAttachments(skillAttachmentsPathFor(root), g); err != nil {
		t.Fatal(err)
	}

	// Simulate a server restart: a fresh rebuildGraph call.
	restarted, err := rebuildGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range restarted.Edges {
		if e.Kind == workflowgraph.EdgeAttachment && e.From == "openspec-propose" && e.To == "author" {
			found = true
		}
	}
	if !found {
		t.Error("expected the skill attachment to survive a simulated restart")
	}
}

// TestRebuildGraphDropsAttachmentForDeletedSkillOrAgent covers
// persist-skill-attachment-edges' "Deleting an attached skill or agent does
// not resurrect a dangling attachment edge on the next rebuild" scenario.
func TestRebuildGraphDropsAttachmentForDeletedSkillOrAgent(t *testing.T) {
	root := newTestClaudeRepo(t)
	writeTestSkill(t, root, "openspec-propose")

	g, err := rebuildGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := workflowgraph.CreateAgent(root, g, "author", "Authors things.", workflowgraph.TierFullEdit); err != nil {
		t.Fatal(err)
	}
	if err := workflowgraph.AttachSkill(g, "author", "openspec-propose"); err != nil {
		t.Fatalf("AttachSkill: unexpected error %v", err)
	}
	if err := workflowgraph.SaveSkillAttachments(skillAttachmentsPathFor(root), g); err != nil {
		t.Fatal(err)
	}

	// Delete the agent out from under the cache: remove its platform file
	// directly, so the next Import no longer finds "author" at all.
	if err := os.Remove(filepath.Join(root, ".claude", "agents", "author.md")); err != nil {
		t.Fatal(err)
	}

	rebuilt, err := rebuildGraph(root)
	if err != nil {
		t.Fatalf("rebuildGraph after deleting the attached agent: unexpected error %v", err)
	}
	for _, e := range rebuilt.Edges {
		if e.Kind == workflowgraph.EdgeAttachment {
			t.Errorf("expected no dangling EdgeAttachment edge after the target agent was deleted, found %+v", e)
		}
	}
}

func TestRebuildGraphHandlesNoCacheYet(t *testing.T) {
	root := newTestClaudeRepo(t)
	// No cache file exists yet — rebuildGraph must not error, and positions
	// default to zero (nothing to carry forward).
	g, err := rebuildGraph(root)
	if err != nil {
		t.Fatalf("rebuildGraph with no cache: unexpected error %v", err)
	}
	if g == nil {
		t.Fatal("expected a non-nil graph")
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
