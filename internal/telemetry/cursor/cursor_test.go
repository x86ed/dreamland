package cursor

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func cursorPath(repo, id string) string {
	return filepath.Join(repo, ".dreamland", "otel-cursors", id+".json")
}

func TestLoad_MissingFile(t *testing.T) {
	c, found, err := Load(t.TempDir(), "nope")
	if err != nil {
		t.Fatalf("Load of a missing cursor must not error, got %v", err)
	}
	if found {
		t.Error("found = true for a missing cursor")
	}
	if c != (Counts{}) {
		t.Errorf("Counts = %+v, want zero", c)
	}
}

func TestStoreLoad_RoundTrip(t *testing.T) {
	repo := t.TempDir()
	want := Counts{Input: 1000, Output: 200, Cached: 40}
	if err := Store(repo, "sess-1", want); err != nil {
		t.Fatal(err)
	}
	got, found, err := Load(repo, "sess-1")
	if err != nil || !found {
		t.Fatalf("Load = (%+v, %v, %v), want found without error", got, found, err)
	}
	if got != want {
		t.Errorf("Load = %+v, want %+v", got, want)
	}
	if _, err := os.Stat(cursorPath(repo, "sess-1")); err != nil {
		t.Errorf("cursor not stored at <repo>/.dreamland/otel-cursors/<id>.json: %v", err)
	}
}

func TestStore_OverwritesAndIsPerSession(t *testing.T) {
	repo := t.TempDir()
	if err := Store(repo, "A", Counts{Input: 1}); err != nil {
		t.Fatal(err)
	}
	if err := Store(repo, "B", Counts{Input: 2}); err != nil {
		t.Fatal(err)
	}
	if err := Store(repo, "A", Counts{Input: 3}); err != nil {
		t.Fatal(err)
	}
	a, _, _ := Load(repo, "A")
	b, _, _ := Load(repo, "B")
	if a.Input != 3 || b.Input != 2 {
		t.Errorf("A=%+v B=%+v, want A.Input=3 B.Input=2", a, b)
	}
}

func TestLoad_CorruptFileIsAnErrorNotZero(t *testing.T) {
	repo := t.TempDir()
	p := cursorPath(repo, "bad")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := Load(repo, "bad")
	if err == nil {
		t.Fatal("Load of a corrupt cursor must return an error so callers can tell it from a missing one")
	}
}

func TestPrune_ByMtime(t *testing.T) {
	repo := t.TempDir()
	for _, id := range []string{"old", "new"} {
		if err := Store(repo, id, Counts{Input: 1}); err != nil {
			t.Fatal(err)
		}
	}
	eightDays := time.Now().Add(-8 * 24 * time.Hour)
	if err := os.Chtimes(cursorPath(repo, "old"), eightDays, eightDays); err != nil {
		t.Fatal(err)
	}

	Prune(repo, 7*24*time.Hour)

	if _, err := os.Stat(cursorPath(repo, "old")); !os.IsNotExist(err) {
		t.Error("cursor older than 7 days was not pruned")
	}
	if _, err := os.Stat(cursorPath(repo, "new")); err != nil {
		t.Errorf("fresh cursor must be kept: %v", err)
	}
}

func TestPrune_MissingDirIsNoop(t *testing.T) {
	Prune(t.TempDir(), time.Hour) // must not panic
}

func TestStore_ConcurrentWritersLeaveAValidCursorAndNoTempFiles(t *testing.T) {
	repo := t.TempDir()
	const n = 40
	var wg sync.WaitGroup
	for i := 1; i <= n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := Store(repo, "same", Counts{Input: int64(i), Output: int64(i), Cached: int64(i)}); err != nil {
				t.Errorf("Store(%d): %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	got, found, err := Load(repo, "same")
	if err != nil || !found {
		t.Fatalf("Load after concurrent Stores = (%+v, %v, %v)", got, found, err)
	}
	if got.Input < 1 || got.Input > n || got.Input != got.Output || got.Input != got.Cached {
		t.Errorf("cursor is torn or out of range: %+v", got)
	}
	entries, _ := os.ReadDir(filepath.Dir(cursorPath(repo, "same")))
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") || e.Name() != "same.json" {
			t.Errorf("unexpected leftover file %q after concurrent Store", e.Name())
		}
	}
}
