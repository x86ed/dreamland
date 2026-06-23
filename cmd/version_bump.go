package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"dreamland/internal/config"
)

var versionBumpCmd = &cobra.Command{
	Use:   "version-bump",
	Short: "Bump the project version (session-start: minor/major; end-of-turn: --patch)",
	RunE:  runVersionBump,
}

var (
	vbMajor    bool
	vbMinor    bool
	vbPatch    bool
	vbBreaking bool
	vbVersion  string
)

func init() {
	rootCmd.AddCommand(versionBumpCmd)
	versionBumpCmd.Flags().BoolVar(&vbMajor, "major", false, "bump major version")
	versionBumpCmd.Flags().BoolVar(&vbMinor, "minor", false, "bump minor version")
	versionBumpCmd.Flags().BoolVar(&vbPatch, "patch", false, "bump patch version (end-of-turn mode)")
	versionBumpCmd.Flags().BoolVar(&vbBreaking, "breaking", false, "breaking change: bump major instead of minor")
	versionBumpCmd.Flags().StringVar(&vbVersion, "version", "", "set explicit version (e.g. v1.2.3)")
}

// branchBumpEntry is one entry in the .dreamland/branch-bumps JSON object.
type branchBumpEntry struct {
	Version       string `json:"version"`
	InitializedAt string `json:"initialized_at"`
}

func runVersionBump(cmd *cobra.Command, _ []string) error {
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

	// Push branch if it had no upstream.
	if noUpstream {
		if out, err := gitExec("push", "--set-upstream", "origin", branch); err != nil {
			return fmt.Errorf("git push --set-upstream: %w\n%s", err, out)
		}
	}

	return nil
}

func performBump(_ *cobra.Command, cfg *config.Config, _ string, lastTag, level, explicit string) error {
	if cfg.VersionBumpCommand == "" {
		// Go path: manage git tags directly.
		newVer, err := bumpSemver(lastTag, level, explicit)
		if err != nil {
			return err
		}
		if _, err := gitExec("tag", "-a", newVer, "-m", newVer); err != nil {
			return fmt.Errorf("git tag %s: %w", newVer, err)
		}
		return nil
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

func gitExec(args ...string) (string, error) {
	return runCmd("git", args...)
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
