package handoff

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestStateRoot(t *testing.T) {
	t.Setenv("DREAMLAND_STATE_DIR", "/x/state")
	if got := StateRoot(); got != "/x/state" {
		t.Errorf("StateRoot = %q", got)
	}
	t.Setenv("DREAMLAND_STATE_DIR", "")
	if got := StateRoot(); filepath.Base(got) != "dreamland" {
		t.Errorf("StateRoot fallback = %q", got)
	}
}

func TestRepoID(t *testing.T) {
	a := RepoID("/tmp/repo-a")
	if len(a) != 16 {
		t.Fatalf("len = %d", len(a))
	}
	if a == RepoID("/tmp/repo-b") {
		t.Error("different repos share an id")
	}
	if a != RepoID("/tmp/repo-a/") {
		t.Error("trailing slash changes id")
	}
	if runtime.GOOS == "windows" && RepoID(`C:\Repo`) != RepoID(`c:\repo`) {
		t.Error("windows ids are case sensitive")
	}
}

func TestResolveChange(t *testing.T) {
	one := func() ([]string, error) { return []string{"only"}, nil }
	two := func() ([]string, error) { return []string{"a", "b"}, nil }
	bad := func() ([]string, error) { return nil, errors.New("no openspec") }
	cases := []struct {
		name, tag string
		active    func() ([]string, error)
		want      string
	}{
		{"tag wins", "c1", one, "c1"},
		{"sole active change", "", one, "only"},
		{"ambiguous falls back", "", two, "_session-s1"},
		{"error falls back", "", bad, "_session-s1"},
		{"invalid tag falls back", "Bad Slug", nil, "_session-s1"},
	}
	for _, c := range cases {
		if got := ResolveChange(c.tag, "s1", c.active); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

func TestValidators(t *testing.T) {
	for _, s := range []string{"", ".", "..", "a/b", "a b"} {
		if ValidSessionID(s) {
			t.Errorf("session %q accepted", s)
		}
	}
	if !ValidSessionID("abc-1.2_x") {
		t.Error("good session rejected")
	}
	for _, s := range []string{"", "-a", "A", "a_b", "../x"} {
		if ValidChange(s) {
			t.Errorf("change %q accepted", s)
		}
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	return NewStoreAt(t.TempDir(), t.TempDir())
}

func inc(c Counter) (Counter, bool) {
	c.PhobetorFailures++
	return c, false
}

func TestCounterRoundTripAndDelete(t *testing.T) {
	s := newTestStore(t)
	if err := s.UpdateCounter("c1", inc); err != nil {
		t.Fatal(err)
	}
	c, err := s.ReadCounter("c1")
	if err != nil || c.PhobetorFailures != 1 {
		t.Fatalf("c=%+v err=%v", c, err)
	}
	if err := s.UpdateCounter("c1", func(c Counter) (Counter, bool) { return Counter{}, true }); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(s.Dir, "c1.json")); !os.IsNotExist(err) {
		t.Errorf("counter file still exists: %v", err)
	}
	if err := s.UpdateCounter("../evil", inc); err == nil {
		t.Error("invalid key accepted")
	}
}

func TestCounterConcurrent(t *testing.T) {
	s := newTestStore(t)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.UpdateCounter("c1", inc); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	c, _ := s.ReadCounter("c1")
	if c.PhobetorFailures != 2 {
		t.Errorf("counter = %d, want 2", c.PhobetorFailures)
	}
}

func TestLockTimeoutAndStale(t *testing.T) {
	s := newTestStore(t)
	old := LockTimeout
	LockTimeout = 50 * time.Millisecond
	defer func() { LockTimeout = old }()

	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	lock := filepath.Join(s.Dir, "c1.json.lock")
	if err := os.WriteFile(lock, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateCounter("c1", inc); !errors.Is(err, ErrLockTimeout) {
		t.Fatalf("err = %v, want ErrLockTimeout", err)
	}
	past := time.Now().Add(-time.Minute)
	if err := os.Chtimes(lock, past, past); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateCounter("c1", inc); err != nil {
		t.Fatalf("stale lock not broken: %v", err)
	}
	if _, err := os.Stat(lock); !os.IsNotExist(err) {
		t.Error("lock not removed after use")
	}
}

func TestPruneOldFiles(t *testing.T) {
	s := newTestStore(t)
	if err := s.UpdateCounter("fresh", inc); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateCounter("stale", inc); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-15 * 24 * time.Hour)
	if err := os.Chtimes(filepath.Join(s.Dir, "stale.json"), old, old); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateCounter("fresh", inc); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(s.Dir, "stale.json")); !os.IsNotExist(err) {
		t.Error("stale counter not pruned")
	}
	if _, err := os.Stat(filepath.Join(s.Dir, "fresh.json")); err != nil {
		t.Error("fresh counter pruned")
	}
}

func TestPendingRoundTripAndCorrupt(t *testing.T) {
	s := newTestStore(t)
	err := s.UpdatePending("s1", func(es []Entry) []Entry {
		return append(es, Entry{ID: "x", State: StatePending, Directive: Directive{Kind: KindDispatch, Target: "phobetor"}})
	})
	if err != nil {
		t.Fatal(err)
	}
	es, err := s.ReadPending("s1")
	if err != nil || len(es) != 1 || !es[0].Blocking() {
		t.Fatalf("es=%+v err=%v", es, err)
	}
	if got := s.Sessions(); len(got) != 1 || got[0] != "s1" {
		t.Errorf("Sessions = %v", got)
	}
	if err := os.WriteFile(filepath.Join(s.Dir, "pending", "s1.json"), []byte("{nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadPending("s1"); err == nil {
		t.Error("corrupt pending file did not error")
	}
	if _, err := s.ReadPending("../x"); err == nil {
		t.Error("invalid session accepted")
	}
}

func TestDirectiveIDStable(t *testing.T) {
	if DirectiveID("s", "a", "r") != DirectiveID("s", "a", "r") || DirectiveID("s", "a", "r") == DirectiveID("s", "a", "r2") {
		t.Error("DirectiveID not deterministic/distinguishing")
	}
}

func TestRenameReplacesExistingFile(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 3; i++ {
		if err := s.UpdateCounter("c1", inc); err != nil {
			t.Fatalf("update %d (rename over existing): %v", i, err)
		}
	}
	c, _ := s.ReadCounter("c1")
	if c.PhobetorFailures != 3 {
		t.Errorf("counter = %d", c.PhobetorFailures)
	}
	leftovers, _ := filepath.Glob(filepath.Join(s.Dir, "*.tmp*"))
	if len(leftovers) != 0 {
		t.Errorf("temp files left behind: %v", leftovers)
	}
}
