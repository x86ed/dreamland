// Package zhougongdata derives per-run agent metrics from git history: commit authors
// identify agents, cumulative "Tokens:" trailers give per-turn token deltas, and
// numstat gives code-line counts.
package zhougongdata

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// SchemaVersion is written into archived run records.
const SchemaVersion = 1

// MaxBranches is the upper limit on branches/records in any single analysis.
const MaxBranches = 8

// ExcludedCodeGlobs documents which paths are excluded from code-line counting:
// anything under openspec/ and any *.md file.
var ExcludedCodeGlobs = []string{"openspec/**", "*.md"}

// KnownAgents are the commit authors treated as agents; any other author is unattributed.
var KnownAgents = map[string]bool{
	"janus": true, "phantasos": true, "nyx": true, "morpheus": true, "phobetor": true,
	"baku": true, "iktomi": true, "zhougong": true, "hypnos": true, "mengpo": true,
}

// Run is one branch-scoped sequence of consecutive commits by the same agent.
type Run struct {
	Agent        string   `json:"agent"`
	Commits      int      `json:"commits"`
	Input        int64    `json:"input"`
	Output       int64    `json:"output"`
	Cached       int64    `json:"cached"`
	Total        int64    `json:"total"`
	LinesAdded   int      `json:"linesAdded"`
	LinesRemoved int      `json:"linesRemoved"`
	TokenToCode  *float64 `json:"tokenToCode"`
}

// UntrackedCommit is an agent commit that carried no parseable Tokens trailer.
type UntrackedCommit struct {
	Hash    string `json:"hash"`
	Agent   string `json:"agent"`
	Subject string `json:"subject"`
}

// Dataset is the parsed result for one branch or archived record.
type Dataset struct {
	Name          string            `json:"name"`
	Branch        string            `json:"branch"`
	Source        string            `json:"source"`
	SourceBranch  string            `json:"sourceBranch,omitempty"`
	MergedAt      string            `json:"mergedAt,omitempty"`
	SchemaVersion int               `json:"schemaVersion"`
	Runs          []Run             `json:"runs"`
	Unattributed  int               `json:"unattributed"`
	Untracked     []UntrackedCommit `json:"untracked"`
	Skipped       int               `json:"skipped"`
}

// CheckMaxBranches returns an error naming the limit when n exceeds MaxBranches.
func CheckMaxBranches(n int) error {
	if n > MaxBranches {
		return fmt.Errorf("at most %d branches may be analysed at once, got %d", MaxBranches, n)
	}
	return nil
}

var tokensLineRe = regexp.MustCompile(`(?m)^Tokens:`)

var trailerRe = regexp.MustCompile(`(?m)^Tokens: input=(\d+) output=(\d+) cached=(\d+) total=(\d+)\s*$`)

type tokens struct{ in, out, cached, total int64 }

func git(repoRoot string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", repoRoot}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(string(ee.Stderr)))
		}
		return "", err
	}
	return string(out), nil
}

func refExists(repoRoot, ref string) bool {
	_, err := git(repoRoot, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	return err == nil
}

// isCode reports whether a changed path counts toward code-line totals.
func isCode(path string) bool {
	return !strings.HasPrefix(path, "openspec/") && !strings.HasSuffix(path, ".md")
}

// ParseBranch parses the commits unique to branch (relative to main when main exists
// and differs from branch) into a Dataset. It returns an error naming the branch if
// it does not exist.
func ParseBranch(repoRoot, branch string) (Dataset, error) {
	ds := Dataset{Name: branch, Branch: branch, Source: "live", SchemaVersion: SchemaVersion, Runs: []Run{}, Untracked: []UntrackedCommit{}}
	if !refExists(repoRoot, branch) {
		return ds, fmt.Errorf("branch %q does not exist", branch)
	}
	rng := branch
	if branch != "main" && refExists(repoRoot, "main") {
		rng = "main.." + branch
	}
	out, err := git(repoRoot, "log", "--reverse", "--numstat", "--format=%x1e%H%x1f%an%x1f%s%x1f%B%x1f", rng)
	if err != nil {
		return ds, fmt.Errorf("branch %q: %w", branch, err)
	}

	var prev *tokens
	var cur *Run
	for _, rec := range strings.Split(out, "\x1e") {
		if strings.TrimSpace(rec) == "" {
			continue
		}
		parts := strings.SplitN(rec, "\x1f", 5)
		if len(parts) < 5 {
			ds.Skipped++
			continue
		}
		hash, author, subject, body, stat := parts[0], parts[1], parts[2], parts[3], parts[4]
		if !KnownAgents[author] {
			ds.Unattributed++
			continue
		}

		added, removed := countLines(stat)
		if cur == nil || cur.Agent != author {
			ds.Runs = append(ds.Runs, Run{Agent: author})
			cur = &ds.Runs[len(ds.Runs)-1]
		}
		cur.Commits++
		cur.LinesAdded += added
		cur.LinesRemoved += removed

		m := trailerRe.FindStringSubmatch(body)
		if m == nil {
			if tokensLineRe.MatchString(body) {
				ds.Skipped++
			}
			ds.Untracked = append(ds.Untracked, UntrackedCommit{Hash: hash, Agent: author, Subject: subject})
			continue
		}
		t := tokens{atoi(m[1]), atoi(m[2]), atoi(m[3]), atoi(m[4])}
		switch {
		case prev == nil:
			// First tracked commit has no baseline; attribute nothing.
		case t.total < prev.total:
			cur.Input += t.in
			cur.Output += t.out
			cur.Cached += t.cached
			cur.Total += t.total
		default:
			cur.Input += nonNeg(t.in - prev.in)
			cur.Output += nonNeg(t.out - prev.out)
			cur.Cached += nonNeg(t.cached - prev.cached)
			cur.Total += t.total - prev.total
		}
		prev = &t
	}
	for i := range ds.Runs {
		r := &ds.Runs[i]
		if lines := r.LinesAdded + r.LinesRemoved; lines > 0 {
			v := float64(r.Output) / float64(lines)
			r.TokenToCode = &v
		}
	}
	return ds, nil
}

func atoi(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

func nonNeg(n int64) int64 {
	if n < 0 {
		return 0
	}
	return n
}

func countLines(stat string) (added, removed int) {
	for _, line := range strings.Split(stat, "\n") {
		f := strings.SplitN(strings.TrimSpace(line), "\t", 3)
		if len(f) != 3 || f[0] == "-" || !isCode(f[2]) {
			continue
		}
		a, errA := strconv.Atoi(f[0])
		r, errR := strconv.Atoi(f[1])
		if errA != nil || errR != nil {
			continue
		}
		added += a
		removed += r
	}
	return
}
