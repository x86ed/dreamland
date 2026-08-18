package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"dreamland/internal/config"
)

var versionBumpCmd = &cobra.Command{
	Use:   "version-bump",
	Short: "Bump the project version (session-start: minor/major; end-of-turn: --patch; per-turn agent-scoped: --minor --if-agent)",
	RunE:  runVersionBump,
}

var (
	vbMajor    bool
	vbMinor    bool
	vbPatch    bool
	vbBreaking bool
	vbVersion  string
	vbChange   string
	vbIfAgent  string

	vbChangeFromCommand bool
)

func init() {
	rootCmd.AddCommand(versionBumpCmd)
	versionBumpCmd.Flags().BoolVar(&vbMajor, "major", false, "bump major version")
	versionBumpCmd.Flags().BoolVar(&vbMinor, "minor", false, "bump minor version")
	versionBumpCmd.Flags().BoolVar(&vbPatch, "patch", false, "bump patch version (end-of-turn mode)")
	versionBumpCmd.Flags().BoolVar(&vbBreaking, "breaking", false, "breaking change: bump major instead of minor")
	versionBumpCmd.Flags().StringVar(&vbVersion, "version", "", "set explicit version (e.g. v1.2.3)")
	versionBumpCmd.Flags().StringVar(&vbChange, "change", "", "change slug: bump minor once per OpenSpec change (independent of the branch marker)")
	versionBumpCmd.Flags().StringVar(&vbIfAgent, "if-agent", "", "only run if the hook payload's agent_type matches this name (silent no-op otherwise)")
	versionBumpCmd.Flags().BoolVar(&vbChangeFromCommand, "change-from-command", false, "detect an openspec change slug from the hook payload's Bash tool_input.command on stdin and bump minor for it (silent no-op if no match) — for PostToolUse hook binding")
}

// branchBumpEntry is one entry in the .dreamland/branch-bumps JSON object.
type branchBumpEntry struct {
	Version       string `json:"version"`
	InitializedAt string `json:"initialized_at"`
}

func runVersionBump(cmd *cobra.Command, _ []string) error {
	if vbChangeFromCommand {
		slug, ok := changeSlugFromBashHookStdin(cmd.InOrStdin())
		if !ok {
			return nil // hook fired for a Bash call unrelated to `openspec new change` — silent no-op
		}
		vbChange = slug
	}

	// Validate: at most one of major/minor/patch/version.
	explicit := 0
	for _, b := range []bool{vbMajor, vbMinor, vbPatch} {
		if b {
			explicit++
		}
	}
	if vbVersion != "" {
		explicit++
	}
	if explicit > 1 {
		return errors.New("at most one of --major, --minor, --patch, --version may be specified")
	}

	if vbIfAgent != "" && agentNameFromHookPayloadFrom(os.Stdin) != vbIfAgent {
		return nil // hook fired for a different agent — silent no-op
	}

	cwd, err := osGetwd()
	if err != nil {
		return err
	}
	cfg, err := config.Load(cwd)
	if err != nil {
		return err
	}
	if cfg == nil {
		cfg = &config.Config{}
	}

	repoRoot, err := config.FindRepoRoot(cwd)
	if err != nil {
		return err
	}

	// Resolve last semver tag (or baseline v0.0.0).
	lastTag, err := gitLastTag()
	if err != nil {
		return err
	}

	// Check for commits since last tag.
	if hasNoChanges(lastTag) {
		return nil // exit 0 silently
	}

	if vbPatch {
		// End-of-turn patch mode: skip branch marker.
		return performBump(cmd, cfg, repoRoot, lastTag, "patch", vbVersion)
	}

	if vbMinor && vbIfAgent != "" {
		// Per-turn agent-scoped minor bump (e.g. Janus on every routing decision):
		// unconditional like --patch, no branch-marker dedup.
		return performBump(cmd, cfg, repoRoot, lastTag, "minor", vbVersion)
	}

	if vbChange != "" {
		// Change-scoped minor mode: independent of, and skips, the branch marker.
		return runChangeBump(cfg, repoRoot, lastTag)
	}

	// Session-start minor/major mode: check branch marker.
	branch, err := gitCurrentBranch()
	if err != nil {
		return err
	}

	bumpsFile := filepath.Join(repoRoot, ".dreamland", "branch-bumps")
	bumps, err := readBranchBumps(bumpsFile)
	if err != nil {
		return err
	}

	// If this branch already has an entry and no explicit override, skip.
	if _, exists := bumps[branch]; exists && vbVersion == "" {
		return nil // exit 0 silently
	}

	// Check upstream; if absent treat branch as new.
	noUpstream := !hasUpstream()

	// Determine bump level.
	level := "minor"
	if vbMajor || vbBreaking {
		level = "major"
	}
	if vbMinor {
		level = "minor"
	}
	if vbVersion != "" {
		level = ""
	}

	if err := performBump(cmd, cfg, repoRoot, lastTag, level, vbVersion); err != nil {
		return err
	}

	// Record bump in branch-bumps.
	newTag, err := gitLastTag()
	if err != nil {
		newTag = lastTag
	}
	bumps[branch] = branchBumpEntry{
		Version:       newTag,
		InitializedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := writeBranchBumps(bumpsFile, bumps); err != nil {
		return err
	}

	// Push branch if it had no upstream. Best-effort: the tag and branch-bumps record
	// above already succeeded by this point, and this command runs from a SessionStart
	// hook — a repo with no "origin" remote (or no network, or no push permission) is a
	// legitimate, common case (local-only or throwaway repos) that must not fail the
	// whole session-start hook chain over a non-essential publish step.
	if noUpstream {
		if out, err := gitExec("push", "--set-upstream", "origin", branch); err != nil {
			fmt.Fprintf(os.Stderr, "dreamland: version-bump warning: git push --set-upstream failed (tag %s was still created): %v\n%s\n", newTag, err, out)
		}
	}

	return nil
}

// openspecNewChangeRe matches the two `openspec` CLI shapes that create a new change,
// capturing the change slug (optionally double-quoted).
var openspecNewChangeRe = regexp.MustCompile(`openspec\s+(?:new\s+change|change\s+create)\s+"?([A-Za-z0-9][A-Za-z0-9._-]*)"?`)

// changeSlugFromBashHookStdin reads a Claude Code PostToolUse hook payload (matcher:
// Bash) from r and extracts the change slug if the completed command matches
// `openspec new change <slug>` / `openspec change create <slug>`. Returns ok=false if
// the payload isn't valid JSON, has no tool_input.command, or the command doesn't
// match — all silent-no-op cases, since this hook fires for every Bash call, not just
// the one that creates a new change.
func changeSlugFromBashHookStdin(r io.Reader) (string, bool) {
	data, err := io.ReadAll(io.LimitReader(r, 1<<16))
	if err != nil || len(data) == 0 {
		return "", false
	}

	var payload struct {
		ToolInput struct {
			Command string `json:"command"`
		} `json:"tool_input"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", false
	}

	m := openspecNewChangeRe.FindStringSubmatch(payload.ToolInput.Command)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// runChangeBump bumps minor once per OpenSpec change slug, tracked in
// .dreamland/change-bumps (same append-and-check-membership shape as branch-bumps).
func runChangeBump(cfg *config.Config, repoRoot, lastTag string) error {
	bumpsFile := filepath.Join(repoRoot, ".dreamland", "change-bumps")
	bumps, err := readBranchBumps(bumpsFile)
	if err != nil {
		return err
	}
	if _, exists := bumps[vbChange]; exists {
		return nil // already bumped for this change
	}

	if err := performBump(nil, cfg, repoRoot, lastTag, "minor", ""); err != nil {
		return err
	}

	newTag, err := gitLastTag()
	if err != nil {
		newTag = lastTag
	}
	bumps[vbChange] = branchBumpEntry{
		Version:       newTag,
		InitializedAt: time.Now().UTC().Format(time.RFC3339),
	}
	return writeBranchBumps(bumpsFile, bumps)
}

// maxTagCollisionRetries bounds the tag-name collision retry loop in performBump.
// This working copy can be shared across many concurrent agent sessions/branches
// that each run version-bump locally without pushing tags upstream, so local-only
// tags can pile up and collide with a freshly computed "next" version even though
// they belong to unrelated work. 1000 is comfortably above any collision run seen
// in practice and keeps the loop bounded.
const maxTagCollisionRetries = 1000

func performBump(_ *cobra.Command, cfg *config.Config, _ string, lastTag, level, explicit string) error {
	if cfg.VersionBumpCommand == "" {
		// Go path: manage git tags directly. Skip past any tag name that already
		// exists locally (e.g. an orphaned tag left behind by another concurrent
		// session sharing this working copy) instead of failing on `git tag`'s
		// "already exists" — that would otherwise abort the bump (and, for
		// --change bumps, leave the change's slug unrecorded in change-bumps,
		// forcing a re-run every time).
		base := lastTag
		for attempts := 0; attempts < maxTagCollisionRetries; attempts++ {
			newVer, err := bumpSemver(base, level, explicit)
			if err != nil {
				return err
			}
			if explicit == "" && tagExists(newVer) {
				base = newVer
				continue
			}
			if _, err := gitExec("tag", "-a", newVer, "-m", newVer); err != nil {
				return fmt.Errorf("git tag %s: %w", newVer, err)
			}
			return nil
		}
		return fmt.Errorf("git tag: exhausted %d attempts avoiding local tag-name collisions starting from %s", maxTagCollisionRetries, lastTag)
	}

	// Delegated path.
	arg := level
	if explicit != "" {
		arg = explicit
	}
	parts := strings.Fields(cfg.VersionBumpCommand)
	parts = append(parts, arg)
	out, err := runCmd(parts[0], parts[1:]...)
	if err != nil {
		if isNotFound(err) {
			return fmt.Errorf("%s not found — install it first (e.g. `npm install -g %s`)", parts[0], parts[0])
		}
		return fmt.Errorf("%s %s: %w\n%s", parts[0], arg, err, out)
	}
	return nil
}

func bumpSemver(base, level, explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	// Strip leading 'v'.
	v := strings.TrimPrefix(base, "v")
	parts := strings.SplitN(v, ".", 3)
	for len(parts) < 3 {
		parts = append(parts, "0")
	}
	major, minor, patch := parseIntOrZero(parts[0]), parseIntOrZero(parts[1]), parseIntOrZero(parts[2])
	switch level {
	case "major":
		major++
		minor, patch = 0, 0
	case "minor":
		minor++
		patch = 0
	case "patch":
		patch++
	default:
		return "", fmt.Errorf("unknown bump level %q", level)
	}
	return fmt.Sprintf("v%d.%d.%d", major, minor, patch), nil
}

func parseIntOrZero(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func gitLastTag() (string, error) {
	out, err := runCmd("git", "describe", "--tags", "--abbrev=0", "--match", "v[0-9]*")
	if err != nil {
		return "v0.0.0", nil // no tags → baseline
	}
	return strings.TrimSpace(out), nil
}

func hasNoChanges(lastTag string) bool {
	out, err := runCmd("git", "diff", lastTag+"..HEAD")
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) == ""
}

func gitCurrentBranch() (string, error) {
	out, err := runCmd("git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD: %w", err)
	}
	return strings.TrimSpace(out), nil
}

func hasUpstream() bool {
	_, err := runCmd("git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	return err == nil
}

func readBranchBumps(path string) (map[string]branchBumpEntry, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]branchBumpEntry{}, nil
	}
	if err != nil {
		return nil, err
	}
	var m map[string]branchBumpEntry
	if err := json.Unmarshal(data, &m); err != nil {
		return map[string]branchBumpEntry{}, nil // corrupt file → start fresh
	}
	return m, nil
}

func writeBranchBumps(path string, bumps map[string]branchBumpEntry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(bumps, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// gitLockRetryAttempts/gitLockRetryDelay are exposed as vars so tests can shrink the
// delay rather than waiting on real sleeps.
var (
	gitLockRetryAttempts = 3
	gitLockRetryDelay    = 150 * time.Millisecond
)

// gitExec runs git with a short bounded retry on lock contention: with multiple
// dreamland-driven agent sessions concurrently committing to the same working tree
// (coauthor/commit/version-bump all shell out through here), two writers can race on
// creating .git/index.lock or .git/config.lock at the same instant. That's expected,
// transient, and clears within milliseconds once the other writer finishes — surfacing
// it as a hard failure on the very first collision (observed live, repeatedly, in a
// heavily concurrent dogfood session) is needless churn. A real, persistent failure
// still surfaces exactly as before once retries are exhausted.
func gitExec(args ...string) (string, error) {
	var out string
	var err error
	for attempt := 1; attempt <= gitLockRetryAttempts; attempt++ {
		out, err = runCmd("git", args...)
		if err == nil || !isGitLockContention(err) || attempt == gitLockRetryAttempts {
			return out, err
		}
		time.Sleep(gitLockRetryDelay)
	}
	return out, err
}

// isGitLockContention reports whether err is git's transient "Unable to create
// '.git/index.lock' (or config.lock): File exists" failure, as opposed to any other
// git error (invalid arguments, detached HEAD, permission issues, etc.), which must
// still fail immediately and not be masked by a retry loop.
func isGitLockContention(err error) bool {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}
	stderr := string(exitErr.Stderr)
	return strings.Contains(stderr, "Unable to create") && strings.Contains(stderr, ".lock")
}

// tagExists reports whether a git tag with the given name already exists locally.
func tagExists(name string) bool {
	out, err := runCmd("git", "rev-parse", "-q", "--verify", "refs/tags/"+name)
	return err == nil && strings.TrimSpace(out) != ""
}

// runCmd executes a command and returns combined stdout output.
// Exposed as a variable so tests can stub it.
var runCmd = func(name string, args ...string) (string, error) {
	c := exec.Command(name, args...)
	out, err := c.Output()
	return string(out), err
}

func isNotFound(err error) bool {
	var e *exec.Error
	return errors.As(err, &e) && errors.Is(e.Err, exec.ErrNotFound)
}
