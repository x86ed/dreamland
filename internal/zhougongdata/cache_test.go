package zhougongdata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCache_RoundTripAndMissing(t *testing.T) {
	root := t.TempDir()
	if _, ok, err := Read(root, "feat/x"); ok || err != nil {
		t.Fatalf("missing: ok=%v err=%v", ok, err)
	}
	ds := Dataset{Name: "feat/x", Branch: "feat/x", Source: "live", Runs: []Run{{Agent: "morpheus", Commits: 2, Total: 9}}}
	if err := Write(root, Entry{Dataset: ds, HeadSha: "abc"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, CacheDir, "feat-x.json")); err != nil {
		t.Fatal(err)
	}
	e, ok, err := Read(root, "feat/x")
	if !ok || err != nil {
		t.Fatalf("read: ok=%v err=%v", ok, err)
	}
	if e.HeadSha != "abc" || e.CollectedAt == "" || e.SchemaVersion != SchemaVersion || len(e.Runs) != 1 || e.Runs[0].Total != 9 {
		t.Errorf("round trip: %+v", e)
	}
	if left, _ := filepath.Glob(filepath.Join(root, CacheDir, ".tmp-*")); len(left) != 0 {
		t.Errorf("temp files left: %v", left)
	}
}

func TestCache_Staleness(t *testing.T) {
	e := Entry{Dataset: Dataset{SchemaVersion: SchemaVersion}, HeadSha: "abc"}
	if IsStale(e, "abc") {
		t.Errorf("same sha and schema must be fresh")
	}
	if !IsStale(e, "def") {
		t.Errorf("sha change must be stale")
	}
	e.SchemaVersion = SchemaVersion + 1
	if !IsStale(e, "abc") {
		t.Errorf("schema change must be stale")
	}
}

func TestCache_SlugCollision(t *testing.T) {
	root := t.TempDir()
	a := Entry{Dataset: Dataset{Name: "a/b", Branch: "a/b"}, HeadSha: "1"}
	b := Entry{Dataset: Dataset{Name: "a-b", Branch: "a-b"}, HeadSha: "2"}
	if err := Write(root, a); err != nil {
		t.Fatal(err)
	}
	if err := Write(root, b); err != nil {
		t.Fatal(err)
	}
	ea, ok, _ := Read(root, "a/b")
	eb, ok2, _ := Read(root, "a-b")
	if !ok || !ok2 || ea.HeadSha != "1" || eb.HeadSha != "2" {
		t.Fatalf("collision: %+v %+v", ea, eb)
	}
	if n := len(ReadAll(root)); n != 2 {
		t.Errorf("ReadAll=%d", n)
	}
	if err := Write(root, Entry{Dataset: a.Dataset, HeadSha: "3"}); err != nil {
		t.Fatal(err)
	}
	ea, _, _ = Read(root, "a/b")
	if ea.HeadSha != "3" || len(ReadAll(root)) != 2 {
		t.Errorf("rewrite after collision: %+v", ea)
	}
}

func TestBuildAgentMatrix_OrderAndCells(t *testing.T) {
	a := Dataset{Name: "a", Runs: []Run{
		{Agent: "nyx", Commits: 1, Total: 10, Output: 5},
		{Agent: "morpheus", Commits: 2, Total: 100, Output: 40},
		{Agent: "nyx", Commits: 1, Total: 10, Output: 5},
	}}
	b := Dataset{Name: "b", Runs: []Run{{Agent: "baku", Commits: 1, Total: 20, Output: 8}}}
	m := BuildAgentMatrix([]Dataset{a, b, NoData("c")})
	if got := strings.Join(m.Agents, ","); got != "morpheus,baku,nyx" {
		t.Fatalf("agent order = %s", got)
	}
	if len(m.Branches) != 3 || len(m.Cells) != 3 || len(m.Cells[0]) != 3 {
		t.Fatalf("shape: %+v", m)
	}
	if c := m.Cells[2][0]; c.Commits != 2 || c.Total != 20 || c.Output != 10 {
		t.Errorf("nyx/a = %+v", c)
	}
	if c := m.Cells[0][1]; c != (AgentCell{}) {
		t.Errorf("morpheus/b should be zero, got %+v", c)
	}
}

func TestBuildAgentMatrix_Empty(t *testing.T) {
	m := BuildAgentMatrix(nil)
	if m.Agents == nil || m.Cells == nil || m.Branches == nil {
		t.Errorf("nil slices: %+v", m)
	}
}
