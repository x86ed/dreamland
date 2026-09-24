package zhougongdata

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// CacheDir is the gitignored per-branch cache directory, relative to the repo root.
const CacheDir = ".dreamland/cache/zhougong"

// Entry is one cached branch dataset with the HEAD sha it was collected at.
type Entry struct {
	Dataset
	HeadSha     string `json:"headSha"`
	CollectedAt string `json:"collectedAt"`
}

// BranchSlug replaces "/" with "-" so a branch name is a single path element.
func BranchSlug(branch string) string {
	return strings.NewReplacer("/", "-", "\\", "-").Replace(branch)
}

func hashedSlug(branch string) string {
	return fmt.Sprintf("%s-%x", BranchSlug(branch), sha1.Sum([]byte(branch)))[:len(BranchSlug(branch))+9]
}

func readFile(path string) (Entry, error) {
	var e Entry
	b, err := os.ReadFile(path)
	if err != nil {
		return e, err
	}
	if err := json.Unmarshal(b, &e); err != nil {
		return e, fmt.Errorf("%s: %w", path, err)
	}
	return e, nil
}

// pathFor picks the file for branch: the plain slug unless another branch owns it.
func pathFor(repoRoot, branch string) string {
	dir := filepath.Join(repoRoot, CacheDir)
	plain := filepath.Join(dir, BranchSlug(branch)+".json")
	if e, err := readFile(plain); err == nil && e.Branch != branch {
		return filepath.Join(dir, hashedSlug(branch)+".json")
	}
	return plain
}

// Write stores e atomically (temp file plus rename). CollectedAt defaults to now.
func Write(repoRoot string, e Entry) error {
	if e.Branch == "" {
		return fmt.Errorf("cache entry has no branch")
	}
	e.SchemaVersion = SchemaVersion
	if e.CollectedAt == "" {
		e.CollectedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if e.Runs == nil {
		e.Runs = []Run{}
	}
	if e.Untracked == nil {
		e.Untracked = []UntrackedCommit{}
	}
	b, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return err
	}
	path := pathFor(repoRoot, e.Branch)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}

// Read returns the cache entry for branch; ok is false when none exists.
func Read(repoRoot, branch string) (Entry, bool, error) {
	dir := filepath.Join(repoRoot, CacheDir)
	for _, p := range []string{BranchSlug(branch) + ".json", hashedSlug(branch) + ".json"} {
		e, err := readFile(filepath.Join(dir, p))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return Entry{}, false, err
		}
		if e.Branch == branch {
			return e, true, nil
		}
	}
	return Entry{}, false, nil
}

// ReadAll returns every readable cache entry, sorted by branch. Unreadable files are skipped.
func ReadAll(repoRoot string) []Entry {
	files, _ := filepath.Glob(filepath.Join(repoRoot, CacheDir, "*.json"))
	out := []Entry{}
	for _, f := range files {
		if e, err := readFile(f); err == nil && e.Branch != "" {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Branch < out[j].Branch })
	return out
}

// IsStale reports whether e no longer matches currentSha or the current schema version.
func IsStale(e Entry, currentSha string) bool {
	return e.HeadSha != currentSha || e.SchemaVersion != SchemaVersion
}

// HeadSha returns the commit sha branch currently points to.
func HeadSha(repoRoot, branch string) (string, error) {
	out, err := git(repoRoot, "rev-parse", "--verify", "--quiet", branch+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("branch %q does not exist", branch)
	}
	return strings.TrimSpace(out), nil
}

// Collect returns the dataset for branch, reusing a fresh cache entry unless refresh is set.
// When it had to parse, the returned Entry is non-nil and the caller should Write it.
func Collect(repoRoot, branch string, refresh bool) (Dataset, *Entry, error) {
	sha, err := HeadSha(repoRoot, branch)
	if err != nil {
		return Dataset{}, nil, err
	}
	if !refresh {
		if e, ok, err := Read(repoRoot, branch); err == nil && ok && !IsStale(e, sha) {
			ds := e.Dataset
			ds.Name, ds.Source = e.Branch, "live"
			return ds, nil, nil
		}
	}
	ds, err := ParseBranch(repoRoot, branch)
	if err != nil {
		return Dataset{}, nil, err
	}
	return ds, &Entry{Dataset: ds, HeadSha: sha}, nil
}
