// Package seedwords exposes the checked-in DoD/partner code-name word pool used by
// internal/oneiroi's name generator, embedded into the dreamland binary at compile time
// so `dreamland oneiroi seed`/`revise`/`fork` never reads from the filesystem at install
// time (matching the agent-scaffolding capability's "Agent templates are embedded in the
// binary" precedent) and never performs a network request.
package seedwords

import (
	"embed"
	"encoding/json"
)

//go:embed words.json
var wordsFS embed.FS

// wordList mirrors words.json's shape.
type wordList struct {
	Words []string `json:"words"`
}

// Load returns the embedded word pool.
func Load() ([]string, error) {
	data, err := wordsFS.ReadFile("words.json")
	if err != nil {
		return nil, err
	}
	var wl wordList
	if err := json.Unmarshal(data, &wl); err != nil {
		return nil, err
	}
	return wl.Words, nil
}
