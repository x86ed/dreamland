package zhougongdata

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// RunsDir is the committed directory of archived run records, relative to the repo root.
const RunsDir = ".dreamland/runs"

// SlugFor derives a filesystem-safe change slug from a branch name.
func SlugFor(branch string) string {
	return strings.NewReplacer("/", "-", "\\", "-", " ", "-").Replace(branch)
}

// WriteArchive writes ds as .dreamland/runs/<slug>.json with source "archived".
func WriteArchive(repoRoot, slug string, ds Dataset, mergedAt time.Time) (string, error) {
	if slug == "" {
		return "", fmt.Errorf("archive slug must not be empty")
	}
	ds.Name = slug
	ds.Source = "archived"
	ds.SourceBranch = ds.Branch
	ds.MergedAt = mergedAt.UTC().Format(time.RFC3339)
	ds.SchemaVersion = SchemaVersion
	dir := filepath.Join(repoRoot, RunsDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	b, err := json.MarshalIndent(ds, "", "  ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, slug+".json")
	return path, os.WriteFile(path, append(b, '\n'), 0o644)
}

// LoadArchived reads every .dreamland/runs/*.json record, marked source "archived".
// A missing directory yields no records.
func LoadArchived(repoRoot string) ([]Dataset, error) {
	files, err := filepath.Glob(filepath.Join(repoRoot, RunsDir, "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	out := []Dataset{}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var ds Dataset
		if err := json.Unmarshal(b, &ds); err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		ds.Source = "archived"
		if ds.Name == "" {
			ds.Name = strings.TrimSuffix(filepath.Base(f), ".json")
		}
		if ds.Runs == nil {
			ds.Runs = []Run{}
		}
		if ds.Untracked == nil {
			ds.Untracked = []UntrackedCommit{}
		}
		out = append(out, ds)
	}
	return out, nil
}
