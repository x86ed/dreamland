package oneiroi

import (
	"fmt"
	"math/rand/v2"
	"strings"
)

// maxCollisionRetries bounds GenerateFamily/GenerateThirdWord's draw-and-retry loop
// (design.md decision 2: "up to 50 attempts, then fails loudly").
const maxCollisionRetries = 50

// GenerateFamily draws two distinct words uniformly at random from pool, retrying (both
// words, not a single-word swap) up to maxCollisionRetries times whenever the drawn pair
// already exists in existing (per Registry.HasFamily). Returns a descriptive error if
// the pool is exhausted for new 2-word combinations.
func GenerateFamily(pool []string, existing *Registry) (word1, word2 string, err error) {
	if len(pool) < 2 {
		return "", "", fmt.Errorf("oneiroi: word pool has fewer than 2 words, cannot draw a family name")
	}

	for attempt := 0; attempt < maxCollisionRetries; attempt++ {
		w1, w2 := drawTwoDistinct(pool)
		if !existing.HasFamily(w1, w2) {
			return w1, w2, nil
		}
	}

	return "", "", fmt.Errorf("oneiroi: word pool exhausted for new 2-word combinations after %d attempts", maxCollisionRetries)
}

// GenerateThirdWord draws one word from pool not present in familyWords or
// excludeWords (case-insensitive), retrying up to maxCollisionRetries times. Returns a
// descriptive error if the pool is exhausted.
func GenerateThirdWord(pool []string, familyWords []string, excludeWords []string) (string, error) {
	excluded := make(map[string]bool, len(familyWords)+len(excludeWords))
	for _, w := range familyWords {
		excluded[strings.ToLower(w)] = true
	}
	for _, w := range excludeWords {
		excluded[strings.ToLower(w)] = true
	}

	candidates := make([]string, 0, len(pool))
	for _, w := range pool {
		if !excluded[strings.ToLower(w)] {
			candidates = append(candidates, w)
		}
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("oneiroi: word pool exhausted for a new third word (every candidate is excluded)")
	}

	return candidates[rand.IntN(len(candidates))], nil
}

// drawTwoDistinct draws two distinct random words from pool (len(pool) >= 2 required).
func drawTwoDistinct(pool []string) (string, string) {
	i := rand.IntN(len(pool))
	j := rand.IntN(len(pool) - 1)
	if j >= i {
		j++
	}
	return pool[i], pool[j]
}
