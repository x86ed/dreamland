// Package oneiroi implements the NICKA-style name generator, the up-to-3-word
// versioning scheme, and the open agent registry backing `dreamland oneiroi
// seed`/`revise`/`fork` (see design.md and the oneiroi-seed-naming capability).
package oneiroi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// registryPath is where the open agent registry lives, relative to a repo root.
const registryPath = ".dreamland/oneiroi/registry.json"

// Revision records a prior third word replaced by a later `dreamland oneiroi revise`.
type Revision struct {
	Word   string    `json:"word"`
	Reason string    `json:"reason"`
	At     time.Time `json:"at"`
}

// Entry is one registry-tracked oneiroi identity.
type Entry struct {
	Name      string     `json:"name"`
	Words     []string   `json:"words"`
	Role      string     `json:"role"`
	ToolTier  string     `json:"tool_tier"`
	Parent    *string    `json:"parent"`
	Created   time.Time  `json:"created"`
	Revisions []Revision `json:"revisions"`
}

// Registry is the parsed contents of .dreamland/oneiroi/registry.json.
type Registry struct {
	Agents []Entry `json:"agents"`
}

// Load reads .dreamland/oneiroi/registry.json from repoRoot. A missing file is not an
// error — it returns an empty Registry, matching the "no error when the registry file
// does not exist" requirement (a repo that has never seeded an oneiroi behaves exactly
// as it does today).
func Load(repoRoot string) (*Registry, error) {
	data, err := os.ReadFile(filepath.Join(repoRoot, registryPath))
	if err != nil {
		if os.IsNotExist(err) {
			return &Registry{}, nil
		}
		return nil, err
	}
	var reg Registry
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, err
	}
	return &reg, nil
}

// Save atomically (temp-file + rename) writes reg to
// <repoRoot>/.dreamland/oneiroi/registry.json, matching atomicJSONMerge's pattern in
// internal/scaffold/scaffold.go.
func (r *Registry) Save(repoRoot string) error {
	target := filepath.Join(repoRoot, registryPath)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(target), ".dreamland-tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, target)
}

// Names returns every live agent name currently in the registry.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.Agents))
	for _, e := range r.Agents {
		names = append(names, e.Name)
	}
	return names
}

// HasFamily reports whether word1/word2 (case-insensitive, order-sensitive — callers
// that need to check both orderings do so explicitly) already exists as a registered
// family or name in the registry.
func (r *Registry) HasFamily(word1, word2 string) bool {
	candidateName := strings.ToLower(word1 + "-" + word2)
	for _, e := range r.Agents {
		if strings.ToLower(e.Name) == candidateName {
			return true
		}
		if len(e.Words) >= 2 &&
			strings.EqualFold(e.Words[0], word1) &&
			strings.EqualFold(e.Words[1], word2) {
			return true
		}
	}
	return false
}
