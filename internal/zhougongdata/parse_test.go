package zhougongdata

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

type tc struct {
	author string
	files  map[string]string
	total  int64 // 0 = no trailer
	body   string
}

func run(t *testing.T, dir string, env []string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run(t, dir, nil, "init", "-q", "-b", "main")
	commit(t, dir, tc{author: "Adam", files: map[string]string{"README.txt": "x\n"}})
	return dir
}

func commit(t *testing.T, dir string, c tc) {
	t.Helper()
	for p, content := range c.files {
		full := filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	msg := "work"
	if c.body != "" {
		msg += "\n\n" + c.body
	}
	if c.total > 0 {
		msg += "\n\nTokens: input=" + itoa(c.total/10) + " output=" + itoa(c.total/2) + " cached=" + itoa(c.total/5) + " total=" + itoa(c.total)
	}
	env := []string{"GIT_AUTHOR_NAME=" + c.author, "GIT_COMMITTER_NAME=" + c.author, "GIT_AUTHOR_EMAIL=a@b.c", "GIT_COMMITTER_EMAIL=a@b.c"}
	run(t, dir, env, "add", "-A")
	run(t, dir, env, "commit", "-q", "--allow-empty", "-m", msg)
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func branchWith(t *testing.T, dir, name string, cs ...tc) {
	t.Helper()
	run(t, dir, nil, "checkout", "-q", "-b", name, "main")
	for _, c := range cs {
		commit(t, dir, c)
	}
}

func TestParseBranch_UnknownBranch(t *testing.T) {
	dir := newRepo(t)
	_, err := ParseBranch(dir, "nope")
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("want error naming branch, got %v", err)
	}
}

func TestParseBranch_TokenDeltas(t *testing.T) {
	cases := []struct {
		name      string
		totals    []int64
		wantTotal []int64 // per run (agents alternate so each commit is its own run)
	}{
		{"monotonic", []int64{1000, 1600, 2000}, []int64{0, 600, 400}},
		{"reset", []int64{1000, 1600, 300}, []int64{0, 600, 300}},
		{"missing trailer", []int64{1000, 0, 1600}, []int64{0, 0, 600}},
	}
	agents := []string{"morpheus", "phobetor", "morpheus"}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := newRepo(t)
			var cs []tc
			for i, tot := range c.totals {
				cs = append(cs, tc{author: agents[i], total: tot, files: map[string]string{"a.go": strings.Repeat("l\n", i+1)}})
			}
			branchWith(t, dir, "feat", cs...)
			ds, err := ParseBranch(dir, "feat")
			if err != nil {
				t.Fatal(err)
			}
			if len(ds.Runs) != len(c.wantTotal) {
				t.Fatalf("runs=%d want %d", len(ds.Runs), len(c.wantTotal))
			}
			for i, w := range c.wantTotal {
				if ds.Runs[i].Total != w {
					t.Errorf("run %d total=%d want %d", i, ds.Runs[i].Total, w)
				}
				if ds.Runs[i].Total < 0 {
					t.Errorf("negative delta")
				}
			}
			if c.name == "missing trailer" && (len(ds.Untracked) != 1 || ds.Untracked[0].Agent != "phobetor") {
				t.Errorf("untracked=%+v", ds.Untracked)
			}
			if c.name != "missing trailer" && len(ds.Untracked) != 0 {
				t.Errorf("unexpected untracked %+v", ds.Untracked)
			}
		})
	}
}

func TestParseBranch_CodeLinesAndRatio(t *testing.T) {
	dir := newRepo(t)
	branchWith(t, dir, "feat",
		tc{author: "morpheus", total: 1000, files: map[string]string{"a.go": "1\n2\n"}},
		tc{author: "morpheus", total: 1600, files: map[string]string{"b.go": "1\n2\n3\n", "openspec/x.txt": "z\n", "notes.md": "y\n"}},
		tc{author: "phantasos", total: 2000, files: map[string]string{"doc.md": "only markdown\n"}},
	)
	ds, err := ParseBranch(dir, "feat")
	if err != nil {
		t.Fatal(err)
	}
	if len(ds.Runs) != 2 {
		t.Fatalf("runs=%+v", ds.Runs)
	}
	m := ds.Runs[0]
	if m.Commits != 2 || m.LinesAdded != 5 || m.LinesRemoved != 0 {
		t.Errorf("morpheus run=%+v", m)
	}
	if m.Output != 300 || m.TokenToCode == nil || *m.TokenToCode != 60 {
		t.Errorf("ratio: output=%d ratio=%v", m.Output, m.TokenToCode)
	}
	if ds.Runs[1].TokenToCode != nil {
		t.Errorf("markdown-only run ratio should be nil")
	}
}

func TestParseBranch_UnattributedAndBranchScope(t *testing.T) {
	dir := newRepo(t)
	branchWith(t, dir, "feat",
		tc{author: "Some Human", files: map[string]string{"h.go": "h\n"}},
		tc{author: "morpheus", total: 1000, files: map[string]string{"a.go": "a\n"}},
		tc{author: "morpheus", body: "Tokens: garbage", files: map[string]string{"c.go": "a\n"}},
		tc{author: "morpheus", body: "AI-InputTokens: 5", files: map[string]string{"d.go": "a\n"}},
	)
	ds, err := ParseBranch(dir, "feat")
	if err != nil {
		t.Fatal(err)
	}
	if ds.Unattributed != 1 {
		t.Errorf("unattributed=%d", ds.Unattributed)
	}
	if len(ds.Runs) != 1 || ds.Runs[0].Commits != 3 {
		t.Errorf("runs=%+v (main's commits must not be included)", ds.Runs)
	}
	if ds.Skipped != 1 || len(ds.Untracked) != 2 {
		t.Errorf("skipped=%d untracked=%v", ds.Skipped, ds.Untracked)
	}
}

func TestFlowAndTransitions(t *testing.T) {
	runs := []Run{{Agent: "nyx"}, {Agent: "morpheus"}, {Agent: "phobetor"}, {Agent: "morpheus"}, {Agent: "phobetor"}}
	if got := strings.Join(FlowPath(runs), ">"); got != "nyx>morpheus>phobetor>morpheus>phobetor" {
		t.Errorf("flow=%s", got)
	}
	tr := Transitions(runs)
	if tr[0] != (Transition{From: "morpheus", To: "phobetor", Count: 2}) || len(tr) != 3 {
		t.Errorf("transitions=%+v", tr)
	}
	a := Dataset{Runs: []Run{{Agent: "a"}, {Agent: "b"}}}
	b := Dataset{Runs: []Run{{Agent: "c"}}}
	c := Dataset{Runs: []Run{{Agent: "c"}}}
	if got := strings.Join(TypicalFlow([]Dataset{a, b, c}), ">"); got != "c" {
		t.Errorf("typical=%s", got)
	}
	if got := strings.Join(TypicalFlow([]Dataset{a, b}), ">"); got != "a>b" {
		t.Errorf("tie should pick earliest, got %s", got)
	}
	if len(TypicalFlow(nil)) != 0 {
		t.Errorf("empty typical")
	}
	stats := AgentStats([]Run{{Agent: "x", Commits: 2, Total: 5}, {Agent: "y"}, {Agent: "x", Commits: 1, Total: 1}})
	if len(stats) != 2 || stats[0].Commits != 3 || stats[0].Total != 6 || stats[0].Runs != 2 {
		t.Errorf("stats=%+v", stats)
	}
}

func mk(name string, n int) Dataset {
	ds := Dataset{Name: name, Source: "live"}
	for i := 0; i < n; i++ {
		ds.Runs = append(ds.Runs, Run{Agent: "a", Commits: 1, Total: 100, Output: 50, LinesAdded: 10})
	}
	return ds
}

func TestCompare_ThreeBranchDeltas(t *testing.T) {
	res := Compare([]Dataset{mk("A", 10), mk("B", 15), mk("C", 5), NoData("ghost")}, "A")
	if res.Baseline != "A" {
		t.Fatalf("baseline=%s", res.Baseline)
	}
	runs := res.Metrics[0]
	if runs.Metric != "runs" || *runs.Cells[0].Value != 10 || *runs.Cells[1].Value != 15 || *runs.Cells[2].Value != 5 {
		t.Fatalf("runs row=%+v", runs)
	}
	if *runs.Cells[1].Delta != 5 || *runs.Cells[1].DeltaPct != 50 || *runs.Cells[2].Delta != -5 || *runs.Cells[2].DeltaPct != -50 {
		t.Errorf("deltas wrong: %+v", runs.Cells)
	}
	if runs.Cells[0].Delta != nil {
		t.Errorf("baseline has no delta")
	}
	if runs.Cells[3].Value != nil || !res.Branches[3].NoData {
		t.Errorf("no-data column should be empty")
	}
	// default baseline is first; unknown baseline falls back too.
	if Compare([]Dataset{mk("X", 1), mk("Y", 2)}, "zzz").Baseline != "X" {
		t.Errorf("fallback baseline")
	}
	// zero baseline: no percent.
	z := Compare([]Dataset{mk("Z", 0), mk("Y", 2)}, "Z")
	if z.Metrics[0].Cells[1].DeltaPct != nil || *z.Metrics[0].Cells[1].Delta != 2 {
		t.Errorf("zero baseline: %+v", z.Metrics[0].Cells[1])
	}
	if got := Compare(nil, ""); got.Baseline != "" {
		t.Errorf("empty compare")
	}
}

func TestCheckMaxBranches(t *testing.T) {
	if CheckMaxBranches(8) != nil {
		t.Errorf("8 allowed")
	}
	if err := CheckMaxBranches(9); err == nil || !strings.Contains(err.Error(), "8") {
		t.Errorf("9 rejected naming limit, got %v", err)
	}
}

func TestArchiveRoundTrip(t *testing.T) {
	root := t.TempDir()
	if got, err := LoadArchived(root); err != nil || len(got) != 0 {
		t.Fatalf("empty: %v %v", got, err)
	}
	ds := mk("feat/x", 2)
	ds.Branch = "feat/x"
	slug := SlugFor(ds.Branch)
	if slug != "feat-x" {
		t.Fatalf("slug=%s", slug)
	}
	if _, err := WriteArchive(root, "", ds, time.Now()); err == nil {
		t.Errorf("empty slug should fail")
	}
	if _, err := WriteArchive(root, slug, ds, time.Unix(0, 0)); err != nil {
		t.Fatal(err)
	}
	got, err := LoadArchived(root)
	if err != nil || len(got) != 1 {
		t.Fatalf("load: %v %v", got, err)
	}
	g := got[0]
	if g.Name != "feat-x" || g.Source != "archived" || g.SourceBranch != "feat/x" || g.MergedAt == "" || g.SchemaVersion != SchemaVersion || len(g.Runs) != 2 {
		t.Errorf("record=%+v", g)
	}
	if err := os.WriteFile(filepath.Join(root, RunsDir, "bad.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadArchived(root); err == nil {
		t.Errorf("bad json should error")
	}
}
