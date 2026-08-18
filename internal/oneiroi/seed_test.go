package oneiroi

import (
	"strings"
	"testing"
)

// TestGenerateFamily_FreshNoCollision covers the "Fresh family name generated with no
// collision" scenario (oneiroi-seed-naming spec): with an empty registry, GenerateFamily
// draws two distinct words from the pool and never errors.
func TestGenerateFamily_FreshNoCollision(t *testing.T) {
	pool := []string{"amber", "falcon", "onyx", "cobalt", "harbor"}
	existing := &Registry{}

	for i := 0; i < 20; i++ {
		word1, word2, err := GenerateFamily(pool, existing)
		if err != nil {
			t.Fatalf("GenerateFamily: %v", err)
		}
		if word1 == "" || word2 == "" {
			t.Fatalf("expected two non-empty words, got %q, %q", word1, word2)
		}
		if strings.EqualFold(word1, word2) {
			t.Fatalf("expected two distinct words, got %q and %q", word1, word2)
		}
		if !inPool(pool, word1) || !inPool(pool, word2) {
			t.Fatalf("drawn words %q/%q are not both members of the supplied pool", word1, word2)
		}
	}
}

// TestGenerateFamily_CollisionTriggersFullRedraw covers "Collision triggers a full
// two-word redraw, not a single-word swap": with a 3-word pool and one 2-word
// combination already registered, every successful draw must avoid the registered pair
// entirely (proving retries redraw both words, not just replace one).
func TestGenerateFamily_CollisionTriggersFullRedraw(t *testing.T) {
	pool := []string{"alpha", "bravo", "charlie"}
	existing := &Registry{Agents: []Entry{
		{Name: "alpha-bravo", Words: []string{"alpha", "bravo"}},
	}}

	for i := 0; i < 50; i++ {
		word1, word2, err := GenerateFamily(pool, existing)
		if err != nil {
			t.Fatalf("GenerateFamily: %v", err)
		}
		if collidesCaseInsensitive(word1, word2, "alpha", "bravo") {
			t.Fatalf("GenerateFamily returned the already-registered pair alpha/bravo: %q, %q", word1, word2)
		}
		if strings.EqualFold(word1, word2) {
			t.Fatalf("expected two distinct words, got %q and %q", word1, word2)
		}
	}
}

// TestGenerateFamily_ExhaustionReturnsError covers "Exhausted pool fails loudly": a
// 2-word pool whose only possible combination is already registered (in both possible
// orderings, since HasFamily is documented case-insensitive but not necessarily
// order-insensitive from the caller's perspective) must exhaust all 50 retries and
// return a descriptive error, never panic and never silently return a colliding pair.
func TestGenerateFamily_ExhaustionReturnsError(t *testing.T) {
	pool := []string{"alpha", "bravo"}
	existing := &Registry{Agents: []Entry{
		{Name: "alpha-bravo", Words: []string{"alpha", "bravo"}},
		{Name: "bravo-alpha", Words: []string{"bravo", "alpha"}},
	}}

	word1, word2, err := GenerateFamily(pool, existing)
	if err == nil {
		t.Fatalf("expected an exhaustion error, got word1=%q word2=%q, nil error", word1, word2)
	}
}

// TestGenerateThirdWord_FreshNoExclusion covers "Revision adds a third word to an
// existing 2-word family": a fresh draw excluding only the family's own two words
// succeeds and returns a word outside the family.
func TestGenerateThirdWord_FreshNoExclusion(t *testing.T) {
	pool := []string{"amber", "falcon", "onyx", "cobalt"}
	familyWords := []string{"amber", "falcon"}

	for i := 0; i < 20; i++ {
		word3, err := GenerateThirdWord(pool, familyWords, nil)
		if err != nil {
			t.Fatalf("GenerateThirdWord: %v", err)
		}
		if word3 == "" {
			t.Fatal("expected a non-empty third word")
		}
		if strings.EqualFold(word3, "amber") || strings.EqualFold(word3, "falcon") {
			t.Fatalf("third word %q collides with the family's own words", word3)
		}
	}
}

// TestGenerateThirdWord_ExcludesPriorThirdWordAndSiblings covers "Re-revision replaces,
// not appends" and "Fork's third word never collides with a sibling fork's third word":
// the exclude list (prior third word / sibling forks' third words) must never be drawn.
func TestGenerateThirdWord_ExcludesPriorThirdWordAndSiblings(t *testing.T) {
	pool := []string{"amber", "falcon", "onyx", "cobalt"}
	familyWords := []string{"amber", "falcon"}
	exclude := []string{"onyx"}

	for i := 0; i < 30; i++ {
		word3, err := GenerateThirdWord(pool, familyWords, exclude)
		if err != nil {
			t.Fatalf("GenerateThirdWord: %v", err)
		}
		if strings.EqualFold(word3, "onyx") {
			t.Fatal("GenerateThirdWord drew an excluded word")
		}
	}
}

// TestGenerateThirdWord_ExhaustionReturnsError mirrors GenerateFamily's exhaustion
// behavior: when every candidate word is excluded (family words + explicit excludes
// cover the entire pool), GenerateThirdWord must return an error, not panic or loop
// forever.
func TestGenerateThirdWord_ExhaustionReturnsError(t *testing.T) {
	pool := []string{"amber", "falcon", "onyx"}
	familyWords := []string{"amber", "falcon"}
	exclude := []string{"onyx"} // covers the only remaining pool word

	word3, err := GenerateThirdWord(pool, familyWords, exclude)
	if err == nil {
		t.Fatalf("expected an exhaustion error, got word3=%q, nil error", word3)
	}
}

func inPool(pool []string, word string) bool {
	for _, w := range pool {
		if strings.EqualFold(w, word) {
			return true
		}
	}
	return false
}

func collidesCaseInsensitive(word1, word2, want1, want2 string) bool {
	return strings.EqualFold(word1, want1) && strings.EqualFold(word2, want2)
}
