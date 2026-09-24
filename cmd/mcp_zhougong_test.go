package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"dreamland/internal/agentissue"
	"dreamland/internal/zhougongdata"
)

func zhougongGit(t *testing.T, dir, author string, args ...string) {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Env = append(os.Environ(), "GIT_AUTHOR_NAME="+author, "GIT_COMMITTER_NAME="+author,
		"GIT_AUTHOR_EMAIL=a@b.c", "GIT_COMMITTER_EMAIL=a@b.c")
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// zhougongRepo makes a repo with main plus branch "feat" holding two morpheus commits.
func zhougongRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	zhougongGit(t, dir, "Adam", "init", "-q", "-b", "main")
	zhougongGit(t, dir, "Adam", "commit", "-q", "--allow-empty", "-m", "init")
	zhougongGit(t, dir, "Adam", "checkout", "-q", "-b", "feat")
	for i, tot := range []string{"1000", "1600"} {
		f := filepath.Join(dir, "f.go")
		if err := os.WriteFile(f, []byte(strings.Repeat("x\n", i+1)), 0o644); err != nil {
			t.Fatal(err)
		}
		zhougongGit(t, dir, "morpheus", "add", "-A")
		zhougongGit(t, dir, "morpheus", "commit", "-q", "-m", "w\n\nTokens: input=1 output=500 cached=1 total="+tot)
	}
	return dir
}

func zhougongClient(t *testing.T, root string) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	st, ct := mcp.NewInMemoryTransports()
	if _, err := newZhougongMCPServer(root).Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	s, err := mcp.NewClient(&mcp.Implementation{Name: "t", Version: "v0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func call(t *testing.T, s *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	res, err := s.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return res
}

func TestMCPZhougong_ListsExactlyFiveTools(t *testing.T) {
	s := zhougongClient(t, t.TempDir())
	res, err := s.ListTools(context.Background(), &mcp.ListToolsParams{})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, tool := range res.Tools {
		got = append(got, tool.Name)
	}
	sort.Strings(got)
	want := "zhougong_collect,zhougong_dashboard_start,zhougong_dashboard_stop,zhougong_new_agent_issue,zhougong_snapshot"
	if strings.Join(got, ",") != want {
		t.Errorf("tools=%v", got)
	}
}

func TestMCPZhougong_CollectUnknownBranchAndCap(t *testing.T) {
	root := zhougongRepo(t)
	s := zhougongClient(t, root)

	res := call(t, s, "zhougong_collect", map[string]any{"branches": []string{"feat", "ghost"}})
	if !res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "ghost") {
		t.Errorf("want IsError naming ghost: %+v", res)
	}
	res = call(t, s, "zhougong_collect", map[string]any{"branches": []string{}})
	if !res.IsError {
		t.Errorf("empty branches should error")
	}
	nine := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"}
	res = call(t, s, "zhougong_collect", map[string]any{"branches": nine})
	if !res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "8") {
		t.Errorf("cap not enforced: %+v", res)
	}
	if _, err := os.Stat(filepath.Join(root, ".dreamland")); err == nil {
		t.Errorf("failed collect must not write anything")
	}
}

func TestMCPZhougong_CollectAndDashboardIdempotence(t *testing.T) {
	root := zhougongRepo(t)
	s := zhougongClient(t, root)

	res := call(t, s, "zhougong_collect", map[string]any{"branches": []string{"feat", "main"}})
	if res.IsError {
		t.Fatalf("collect: %+v", res.Content)
	}
	b, _ := json.Marshal(res.StructuredContent)
	if !strings.Contains(string(b), `"total":600`) || !strings.Contains(string(b), "comparisonTable") {
		t.Errorf("collect output: %s", b)
	}
	// Cached second call and refresh both succeed.
	for _, refresh := range []bool{false, true} {
		if r := call(t, s, "zhougong_collect", map[string]any{"branches": []string{"feat"}, "refresh": refresh}); r.IsError {
			t.Errorf("refresh=%v: %+v", refresh, r.Content)
		}
	}

	url := func(r *mcp.CallToolResult) string {
		var o struct{ URL string }
		bb, _ := json.Marshal(r.StructuredContent)
		_ = json.Unmarshal(bb, &o)
		return o.URL
	}
	u1 := url(call(t, s, "zhougong_dashboard_start", map[string]any{}))
	u2 := url(call(t, s, "zhougong_dashboard_start", map[string]any{}))
	if u1 == "" || u1 != u2 || !strings.HasPrefix(u1, "http://127.0.0.1:") {
		t.Fatalf("start urls %q %q", u1, u2)
	}
	resp, err := http.Get(u1 + "/api/summary")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	for i := 0; i < 2; i++ {
		if r := call(t, s, "zhougong_dashboard_stop", map[string]any{}); r.IsError {
			t.Errorf("stop #%d errored", i)
		}
	}
	if _, err := http.Get(u1 + "/api/summary"); err == nil {
		t.Errorf("dashboard still reachable after stop")
	}
	if r := call(t, s, "zhougong_dashboard_start", map[string]any{"port": 1}); !r.IsError {
		t.Logf("port 1 unexpectedly bindable (running privileged?)")
		call(t, s, "zhougong_dashboard_stop", map[string]any{})
	}
}

func TestZhougongArchiveCommand(t *testing.T) {
	root := zhougongRepo(t)
	orig, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	if _, _, err := execCLI(t, "zhougong-archive", "--branch", "ghost"); err == nil {
		t.Errorf("unknown branch should fail")
	}
	if _, _, err := execCLI(t, "zhougong-archive", "--branch", "feat"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, ".dreamland", "runs", "feat.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"sourceBranch": "feat"`, `"schemaVersion": 1`, `"mergedAt"`, `"source": "archived"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("record missing %s: %s", want, b)
		}
	}
	if _, _, err := execCLI(t, "zhougong-archive", "--branch", "feat", "--slug", "custom"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".dreamland", "runs", "custom.json")); err != nil {
		t.Errorf("slug flag: %v", err)
	}
}

func structured(t *testing.T, r *mcp.CallToolResult, v any) {
	t.Helper()
	if r.IsError {
		t.Fatalf("tool error: %+v", r.Content)
	}
	b, _ := json.Marshal(r.StructuredContent)
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatal(err)
	}
}

func TestMCPZhougong_CollectWritesCacheAndSkipsFresh(t *testing.T) {
	root := zhougongRepo(t)
	s := zhougongClient(t, root)
	path := filepath.Join(root, ".dreamland", "cache", "zhougong", "feat.json")

	call(t, s, "zhougong_collect", map[string]any{"branches": []string{"feat"}})
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sha := strings.TrimSpace(gitOut(t, root, "rev-parse", "feat"))
	if !strings.Contains(string(first), sha) {
		t.Errorf("cache lacks head sha: %s", first)
	}
	time.Sleep(1100 * time.Millisecond)
	call(t, s, "zhougong_collect", map[string]any{"branches": []string{"feat"}})
	if again, _ := os.ReadFile(path); string(again) != string(first) {
		t.Errorf("fresh entry was recollected")
	}
	call(t, s, "zhougong_collect", map[string]any{"branches": []string{"feat"}, "refresh": true})
	if again, _ := os.ReadFile(path); string(again) == string(first) {
		t.Errorf("refresh=true should rewrite the entry")
	}
	zhougongGit(t, root, "morpheus", "commit", "-q", "--allow-empty", "-m", "w\n\nTokens: input=1 output=500 cached=1 total=2000")
	call(t, s, "zhougong_collect", map[string]any{"branches": []string{"feat"}})
	if b, _ := os.ReadFile(path); !strings.Contains(string(b), strings.TrimSpace(gitOut(t, root, "rev-parse", "feat"))) {
		t.Errorf("stale entry not recollected")
	}
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	out, err := c.Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

type snapshotOut struct {
	Branches []struct {
		Name, Source, HeadSha, CollectedAt string
		Missing, Stale                     bool
		Summary                            *struct{ Total int64 }
	}
	Comparison *struct {
		Baseline string
		Metrics  []struct {
			Metric string
			Cells  []struct{ Value, Delta *float64 }
		}
	}
}

func TestMCPZhougong_SnapshotStaleMissingAndComparison(t *testing.T) {
	root := zhougongRepo(t)
	zhougongGit(t, root, "Adam", "branch", "other", "feat")
	s := zhougongClient(t, root)
	call(t, s, "zhougong_collect", map[string]any{"branches": []string{"feat", "other"}})

	var out snapshotOut
	structured(t, call(t, s, "zhougong_snapshot", map[string]any{"branches": []string{"feat", "other", "ghost"}, "baseline": "feat"}), &out)
	if len(out.Branches) != 3 || out.Branches[0].Stale || out.Branches[0].CollectedAt == "" || out.Branches[0].HeadSha == "" {
		t.Fatalf("branches: %+v", out.Branches)
	}
	if !out.Branches[2].Missing || out.Branches[2].Summary != nil {
		t.Errorf("ghost should be missing: %+v", out.Branches[2])
	}
	if out.Comparison == nil || out.Comparison.Baseline != "feat" || len(out.Comparison.Metrics[0].Cells) != 2 {
		t.Errorf("comparison must cover only non-missing branches: %+v", out.Comparison)
	}

	zhougongGit(t, root, "morpheus", "commit", "-q", "--allow-empty", "-m", "more")
	out = snapshotOut{}
	structured(t, call(t, s, "zhougong_snapshot", map[string]any{"branches": []string{"feat", "other"}}), &out)
	if !out.Branches[0].Stale || out.Branches[1].Stale {
		t.Errorf("only feat should be stale: %+v", out.Branches)
	}

	out = snapshotOut{}
	structured(t, call(t, s, "zhougong_snapshot", map[string]any{}), &out)
	if len(out.Branches) != 2 {
		t.Errorf("empty list should return all cached, got %d", len(out.Branches))
	}
}

func TestMCPZhougong_SnapshotMatchesDashboardCompare(t *testing.T) {
	root := zhougongRepo(t)
	zhougongGit(t, root, "Adam", "branch", "other", "feat")
	s := zhougongClient(t, root)
	call(t, s, "zhougong_collect", map[string]any{"branches": []string{"feat", "other"}})

	var out struct{ Comparison json.RawMessage }
	structured(t, call(t, s, "zhougong_snapshot", map[string]any{"branches": []string{"feat", "other"}}), &out)

	var u struct{ URL string }
	structured(t, call(t, s, "zhougong_dashboard_start", map[string]any{}), &u)
	defer call(t, s, "zhougong_dashboard_stop", map[string]any{})
	resp, err := http.Get(u.URL + "/api/compare?branches=feat,other")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var dash json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&dash); err != nil {
		t.Fatal(err)
	}
	norm := func(b []byte) string {
		var v any
		_ = json.Unmarshal(b, &v)
		n, _ := json.Marshal(v)
		return string(n)
	}
	if norm(out.Comparison) != norm(dash) {
		t.Errorf("snapshot comparison differs from dashboard:\n%s\n%s", out.Comparison, dash)
	}
}

func TestMCPZhougong_DashboardReadsDiskCache(t *testing.T) {
	root := zhougongRepo(t)
	call(t, zhougongClient(t, root), "zhougong_collect", map[string]any{"branches": []string{"feat"}})

	s := zhougongClient(t, root) // fresh server, empty in-memory store
	var u struct{ URL string }
	structured(t, call(t, s, "zhougong_dashboard_start", map[string]any{}), &u)
	defer call(t, s, "zhougong_dashboard_stop", map[string]any{})
	resp, err := http.Get(u.URL + "/api/summary")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body struct {
		Datasets []struct{ Summary struct{ Name string } }
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Datasets) != 1 || body.Datasets[0].Summary.Name != "feat" {
		t.Errorf("dashboard should list cached feat: %+v", body)
	}
}

func TestMCPZhougong_SnapshotArchivedAndCap(t *testing.T) {
	root := zhougongRepo(t)
	if _, err := zhougongdata.WriteArchive(root, "foo", zhougongdata.Dataset{Branch: "feat", Runs: []zhougongdata.Run{{Agent: "morpheus", Commits: 1, Total: 5}}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	s := zhougongClient(t, root)
	var out snapshotOut
	structured(t, call(t, s, "zhougong_snapshot", map[string]any{"branches": []string{"foo"}}), &out)
	if len(out.Branches) != 1 || out.Branches[0].Source != "archived" || out.Branches[0].Stale || out.Branches[0].Missing {
		t.Errorf("archived: %+v", out.Branches)
	}
	nine := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"}
	res := call(t, s, "zhougong_snapshot", map[string]any{"branches": nine})
	if !res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "8") {
		t.Errorf("cap not enforced: %+v", res)
	}
}

func issueArgs(extra map[string]any) map[string]any {
	m := map[string]any{"name": "sandman", "role": "r", "rationale": "why", "tier": "full-edit", "routing": "a->b", "criteria": "ok"}
	for k, v := range extra {
		m[k] = v
	}
	return m
}

func TestMCPZhougong_NewAgentIssueTwoPhase(t *testing.T) {
	var calls [][]string
	t.Cleanup(agentissue.SetRunGh(func(args ...string) (string, error) {
		calls = append(calls, args)
		if args[1] == "list" {
			return "", nil
		}
		return "https://example.test/issues/9\n", nil
	}))
	s := zhougongClient(t, t.TempDir())

	res := call(t, s, "zhougong_new_agent_issue", issueArgs(nil))
	if res.IsError || len(calls) != 0 {
		t.Fatalf("preview must not call gh: %+v calls=%v", res, calls)
	}
	var prev struct{ Preview, PreviewID string }
	b, _ := json.Marshal(res.StructuredContent)
	_ = json.Unmarshal(b, &prev)
	if prev.PreviewID == "" || !strings.Contains(prev.Preview, "### Acceptance criteria") {
		t.Fatalf("preview = %+v", prev)
	}

	res = call(t, s, "zhougong_new_agent_issue", issueArgs(map[string]any{"confirm": true, "previewId": prev.PreviewID}))
	if res.IsError || len(calls) != 2 {
		t.Fatalf("confirm: %+v calls=%v", res, calls)
	}
	create := calls[1]
	want := agentissue.Fields{Name: "sandman", Role: "r", Rationale: "why", Tier: "full-edit", Routing: "a->b", Criteria: "ok"}
	if create[3] != want.Title() || create[5] != want.Body() || create[7] != agentissue.Label {
		t.Errorf("create args differ from command: %q", create)
	}
}

func TestMCPZhougong_NewAgentIssueConfirmWithoutPreview(t *testing.T) {
	var calls int
	t.Cleanup(agentissue.SetRunGh(func(...string) (string, error) { calls++; return "", nil }))
	s := zhougongClient(t, t.TempDir())
	for _, extra := range []map[string]any{{"confirm": true}, {"confirm": true, "previewId": "unknown"}} {
		if res := call(t, s, "zhougong_new_agent_issue", issueArgs(extra)); !res.IsError {
			t.Errorf("want IsError for %v", extra)
		}
	}
	if calls != 0 {
		t.Error("gh called")
	}
}
