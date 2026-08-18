package seedwords

import (
	"strings"
	"testing"
)

// TestLoad_ParsesWithoutError confirms the embedded words.json parses cleanly via
// Load() (task 1.3) — the checked-in file produced by gen/main.go (task 1.1/1.2)
// must be valid, embeddable JSON with no filesystem read required at install time.
func TestLoad_ParsesWithoutError(t *testing.T) {
	words, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if words == nil {
		t.Fatal("Load returned a nil word slice")
	}
}

// TestLoad_MoreThanOneHundredEntries confirms the DoD/partner code-name word pool
// (oneiroi-seed-naming's "checked-in word list" requirement) is large enough that
// GenerateFamily's 50-retry collision ceiling has a realistic chance of succeeding
// at this repo's roster scale.
func TestLoad_MoreThanOneHundredEntries(t *testing.T) {
	words, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(words) <= 100 {
		t.Errorf("got %d words, want more than 100", len(words))
	}
}

// TestLoad_NoDuplicates confirms words.json was deduplicated at generation time
// (decision 1 in design.md: "a deduplicated, lowercased ... set of every individual
// token"), so GenerateFamily never draws the same literal word twice from a
// duplicate entry.
func TestLoad_NoDuplicates(t *testing.T) {
	words, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	seen := make(map[string]bool, len(words))
	for _, w := range words {
		if seen[w] {
			t.Errorf("duplicate word entry %q", w)
		}
		seen[w] = true
	}
}

// TestLoad_AllLowercase confirms every token was lowercased at generation time, so
// case-insensitive collision checking in the registry (HasFamily) is unnecessary
// for the pool itself — the source of truth is already normalized.
func TestLoad_AllLowercase(t *testing.T) {
	words, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, w := range words {
		if w != strings.ToLower(w) {
			t.Errorf("word %q is not lowercase", w)
		}
	}
}

// TestLoad_NoEmptyOrWhitespaceEntries guards against a malformed split (e.g. multiple
// consecutive spaces in a source code name) sneaking an empty or whitespace-only
// token into the pool, which would make GenerateFamily draw an unusable "word".
func TestLoad_NoEmptyOrWhitespaceEntries(t *testing.T) {
	words, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, w := range words {
		if strings.TrimSpace(w) == "" {
			t.Error("word list contains an empty or whitespace-only entry")
		}
		if strings.ContainsAny(w, " \t\n") {
			t.Errorf("word %q contains whitespace — splitting on whitespace should have produced separate tokens", w)
		}
	}
}
