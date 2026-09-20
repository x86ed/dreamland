// Package cursor keeps per-session "last reported" token counts under
// <repo>/.dreamland/otel-cursors/ so cumulative sources can report only deltas.
package cursor

import (
	"errors"
	"time"
)

// STUB (nyx, TDD red phase): morpheus replaces this file with the real implementation
// per openspec/changes/parallel-session-otel-receiver tasks 1.2.

// Counts are the token totals last reported for a session.
type Counts struct{ Input, Output, Cached int64 }

// Load returns the stored cursor; found is false when no cursor file exists.
func Load(repoRoot, id string) (Counts, bool, error) { return Counts{}, false, nil }

// Store atomically persists c as the cursor for id.
func Store(repoRoot, id string, c Counts) error { return errors.New("cursor.Store: not implemented") }

// Prune deletes cursor files older than olderThan.
func Prune(repoRoot string, olderThan time.Duration) {}
