// Package cursor keeps per-session "last reported" token counts under
// <repo>/.dreamland/otel-cursors/ so cumulative sources can report only deltas.
package cursor

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Counts are the token totals last reported for a session.
type Counts struct{ Input, Output, Cached int64 }

type fileFormat struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
	CachedTokens int64 `json:"cached_tokens"`
}

func dirPath(repoRoot string) string {
	return filepath.Join(repoRoot, ".dreamland", "otel-cursors")
}

func path(repoRoot, id string) string {
	return filepath.Join(dirPath(repoRoot), id+".json")
}

// Load returns the stored cursor; found is false when no cursor file exists. A corrupt
// file is an error so callers can tell it from a missing one.
func Load(repoRoot, id string) (Counts, bool, error) {
	data, err := os.ReadFile(path(repoRoot, id))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Counts{}, false, nil
		}
		return Counts{}, false, err
	}
	var f fileFormat
	if err := json.Unmarshal(data, &f); err != nil {
		return Counts{}, true, err
	}
	return Counts{Input: f.InputTokens, Output: f.OutputTokens, Cached: f.CachedTokens}, true, nil
}

// Store atomically persists c as the cursor for id. A failed rename is retried up to 3
// times with 20 ms backoff (Windows sharing violations).
func Store(repoRoot, id string, c Counts) error {
	dir := dirPath(repoRoot)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(fileFormat{InputTokens: c.Input, OutputTokens: c.Output, CachedTokens: c.Cached})
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, id+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr != nil || cerr != nil {
		os.Remove(tmpName)
		if werr != nil {
			return werr
		}
		return cerr
	}
	dst := path(repoRoot, id)
	for attempt := 0; attempt < 3; attempt++ {
		if err = os.Rename(tmpName, dst); err == nil {
			return nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	os.Remove(tmpName)
	return err
}

// Prune deletes cursor files older than olderThan.
func Prune(repoRoot string, olderThan time.Duration) {
	entries, err := os.ReadDir(dirPath(repoRoot))
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-olderThan)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		info, err := e.Info()
		if err != nil || !info.ModTime().Before(cutoff) {
			continue
		}
		os.Remove(filepath.Join(dirPath(repoRoot), e.Name()))
	}
}
