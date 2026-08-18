package workflowgraph

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatchPathsOnlyIncludesExistingDirs(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}

	paths := WatchPaths(root, "")
	if len(paths) != 1 {
		t.Fatalf("got %d watch paths, want 1 (only .claude/agents exists): %v", len(paths), paths)
	}
	if paths[0] != filepath.Join(root, ".claude", "agents") {
		t.Errorf("watch path = %q, want %q", paths[0], filepath.Join(root, ".claude", "agents"))
	}
}

func TestWatchPathsIncludesActiveChangeDirWhenPresent(t *testing.T) {
	root := t.TempDir()
	changeDir := filepath.Join(root, "openspec", "changes", "some-change")
	if err := os.MkdirAll(changeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	paths := WatchPaths(root, changeDir)
	found := false
	for _, p := range paths {
		if p == changeDir {
			found = true
		}
	}
	if !found {
		t.Errorf("expected changeDir %q in watch paths, got %v", changeDir, paths)
	}

	// A nonexistent change dir must not be included (already-archived change, etc).
	paths = WatchPaths(root, filepath.Join(root, "openspec", "changes", "gone"))
	for _, p := range paths {
		if p == filepath.Join(root, "openspec", "changes", "gone") {
			t.Error("expected a nonexistent change dir to be excluded")
		}
	}
}

func TestWatcherFiresOnChange(t *testing.T) {
	root := t.TempDir()
	watched := filepath.Join(root, "watched")
	if err := os.MkdirAll(watched, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(watched, "a.txt"), []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}

	fired := make(chan struct{}, 10)
	w := NewWatcher([]string{watched}, 20*time.Millisecond, func() { fired <- struct{}{} })
	w.Start()
	defer w.Stop()

	// Let one interval pass with no change — must not fire.
	select {
	case <-fired:
		t.Fatal("OnChange fired with no underlying change")
	case <-time.After(60 * time.Millisecond):
	}

	// Touch the file with a definitely-later mtime, then expect a fire.
	future := time.Now().Add(time.Second)
	if err := os.Chtimes(filepath.Join(watched, "a.txt"), future, future); err != nil {
		t.Fatal(err)
	}

	select {
	case <-fired:
	case <-time.After(2 * time.Second):
		t.Fatal("expected OnChange to fire after mtime advanced, timed out")
	}
}

func TestBroadcasterPublishNotifiesAllSubscribersWithoutBlocking(t *testing.T) {
	b := NewBroadcaster()
	ch1 := b.Subscribe()
	ch2 := b.Subscribe()

	b.Publish()

	for i, ch := range []chan struct{}{ch1, ch2} {
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d did not receive the publish", i)
		}
	}

	// A second Publish with no reader must coalesce (buffered by 1), not block.
	done := make(chan struct{})
	go func() {
		b.Publish()
		b.Publish() // must not block even though ch1/ch2 haven't been drained
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Publish blocked on an undrained subscriber")
	}

	b.Unsubscribe(ch1)
	drained := 0
	for range ch1 {
		drained++
		if drained > 10 {
			t.Fatal("ch1 never closed after Unsubscribe")
		}
	}
}
