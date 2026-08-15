package workflowgraph

import (
	"fmt"
	"os"
	"path/filepath"
)

// Lock is a cross-process advisory lock on a single file, used to serialize
// writes between concurrent `dreamland hypnos-serve` processes touching the
// same repository — an open /hypnos-interactive session and a `hypnos`
// --mode=apply-plan run, for instance (see design.md's "least-overhead"
// concurrency decision). Acquire blocks until the lock is held; Release must
// be called exactly once per successful Acquire.
type Lock struct {
	path string
	file *os.File
}

// NewLock returns a Lock for path (typically <repoRoot>/.dreamland/hypnos.lock).
func NewLock(path string) *Lock {
	return &Lock{path: path}
}

// Acquire blocks until the advisory lock is held, creating the lock file
// (and its parent directory) if it doesn't exist yet.
func (l *Lock) Acquire() error {
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return fmt.Errorf("create lock dir: %w", err)
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("open lock file: %w", err)
	}
	if err := platformLock(f); err != nil {
		f.Close()
		return fmt.Errorf("acquire lock: %w", err)
	}
	l.file = f
	return nil
}

// Release releases the lock and closes the underlying file handle. Safe to
// call only after a successful Acquire; a no-op otherwise.
func (l *Lock) Release() error {
	if l.file == nil {
		return nil
	}
	unlockErr := platformUnlock(l.file)
	closeErr := l.file.Close()
	l.file = nil
	if unlockErr != nil {
		return unlockErr
	}
	return closeErr
}

// WithLock acquires path's lock, runs fn, and releases the lock afterward
// regardless of fn's outcome — the "acquire / rebuild-from-disk / apply /
// release" shape every write entrypoint (§7's HTTP handlers, §8's apply-plan
// runner) wraps its mutation in.
func WithLock(path string, fn func() error) error {
	l := NewLock(path)
	if err := l.Acquire(); err != nil {
		return err
	}
	defer l.Release()
	return fn()
}
