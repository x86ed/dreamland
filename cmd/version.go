package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"dreamland/internal/config"
)

// buildCommit is embedded at build time via ldflags: -X dreamland/cmd.buildCommit=<sha>
var buildCommit = "unknown"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print dreamland build information",
	RunE:  runVersion,
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

func runVersion(cmd *cobra.Command, args []string) error {
	fmt.Fprintf(cmd.OutOrStdout(), "dreamland build %s\n", buildCommit)
	return nil
}

// checkBinaryFreshness checks whether the running binary is stale relative to
// the source repository's current HEAD. This is advisory only (never blocks);
// it prints a warning to stderr if a mismatch is detected and only when running
// inside dreamland's own source repository (not a consumer project).
func checkBinaryFreshness(cwd string) {
	// Only run this check inside dreamland's own source repo
	repoRoot, err := config.FindRepoRoot(cwd)
	if err != nil {
		return // not in a git repo
	}

	if !isDreamlandSourceRepo(repoRoot) {
		return // not dreamland's own source repo
	}

	if buildCommit == "unknown" {
		return // binary wasn't built with ldflags, no check possible
	}

	// Get current HEAD SHA in the repo
	headSHA, err := runCmd("git", "-C", repoRoot, "rev-parse", "HEAD")
	if err != nil {
		return // can't get HEAD, silently skip
	}
	headSHA = strings.TrimSpace(headSHA)

	if buildCommit != headSHA {
		fmt.Fprintf(os.Stderr,
			"dreamland: binary build SHA %s does not match source HEAD %s — rebuild and reinstall before trusting hook output\n",
			buildCommit[:8], headSHA[:8])
	}
}

// isDreamlandSourceRepo checks if the repository at repoRoot is dreamland's own
// source repository by examining go.mod for "module dreamland".
func isDreamlandSourceRepo(repoRoot string) bool {
	goModPath := filepath.Join(repoRoot, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return false
	}
	return bytes.Contains(data, []byte("module dreamland"))
}
