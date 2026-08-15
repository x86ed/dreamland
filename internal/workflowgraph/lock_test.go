package workflowgraph

import (
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestLockAcquireRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hypnos.lock")
	l := NewLock(path)
	if err := l.Acquire(); err != nil {
		t.Fatalf("Acquire: unexpected error %v", err)
	}
	if err := l.Release(); err != nil {
		t.Fatalf("Release: unexpected error %v", err)
	}
	// Release is safe to call again (no-op) and Acquire is safe to call again.
	if err := l.Release(); err != nil {
		t.Fatalf("second Release: unexpected error %v", err)
	}
	if err := l.Acquire(); err != nil {
		t.Fatalf("re-Acquire after Release: unexpected error %v", err)
	}
	l.Release()
}

// TestLockSerializesConcurrentWriters is the concurrency scenario this lock
// exists for: two "processes" (here, two goroutines each opening their own
// Lock on the same path — flock is per-open-file-description, so this
// genuinely exercises cross-handle exclusion, not just in-process mutex
// behavior) touching a shared counter under WithLock must never interleave.
func TestLockSerializesConcurrentWriters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hypnos.lock")

	var mu sync.Mutex // guards the plain (unsynchronized-by-the-lock) counter for the test's own bookkeeping
	counter := 0
	maxObservedConcurrency := 0
	inCriticalSection := 0

	var wg sync.WaitGroup
	const n = 20
	for range n {
		wg.Go(func() {
			err := WithLock(path, func() error {
				mu.Lock()
				inCriticalSection++
				if inCriticalSection > maxObservedConcurrency {
					maxObservedConcurrency = inCriticalSection
				}
				mu.Unlock()

				// Simulate real work: read, mutate, write, with enough time
				// for a race to manifest if the lock weren't exclusive.
				local := counter
				time.Sleep(2 * time.Millisecond)
				counter = local + 1

				mu.Lock()
				inCriticalSection--
				mu.Unlock()
				return nil
			})
			if err != nil {
				t.Errorf("WithLock: unexpected error %v", err)
			}
		})
	}
	wg.Wait()

	if counter != n {
		t.Errorf("counter = %d, want %d — lock did not serialize the read-modify-write", counter, n)
	}
	if maxObservedConcurrency > 1 {
		t.Errorf("observed %d goroutines inside the locked section simultaneously, want at most 1", maxObservedConcurrency)
	}
}

func TestLockBlocksUntilReleased(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hypnos.lock")

	first := NewLock(path)
	if err := first.Acquire(); err != nil {
		t.Fatalf("first Acquire: unexpected error %v", err)
	}

	acquired := make(chan struct{})
	go func() {
		second := NewLock(path)
		if err := second.Acquire(); err != nil {
			t.Errorf("second Acquire: unexpected error %v", err)
			return
		}
		close(acquired)
		second.Release()
	}()

	select {
	case <-acquired:
		t.Fatal("second Acquire returned before first Release — lock did not block")
	case <-time.After(100 * time.Millisecond):
	}

	if err := first.Release(); err != nil {
		t.Fatalf("first Release: unexpected error %v", err)
	}

	select {
	case <-acquired:
	case <-time.After(2 * time.Second):
		t.Fatal("second Acquire never returned after first Release")
	}
}
