package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"dreamland/internal/config"
)

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Run tests if source files changed since last commit",
	RunE:  runTest,
}

func init() {
	rootCmd.AddCommand(testCmd)
}

var sourceExtensions = map[string][]string{
	"Go":              {".go"},
	"Node/TypeScript": {".ts", ".tsx", ".js", ".jsx", ".mts", ".cts"},
	"Rust":            {".rs"},
	"Python":          {".py"},
}

func runTest(_ *cobra.Command, _ []string) error {
	cwd, err := osGetwd()
	if err != nil {
		return err
	}
	cfg, err := config.Load(cwd)
	if err != nil {
		if errors.Is(err, config.ErrNoGitRepo) {
			return nil // skip outside git repo
		}
		return err
	}
	if cfg == nil || cfg.TestCommand == "" {
		return nil
	}

	exts, ok := sourceExtensions[cfg.Language]
	if !ok {
		return nil
	}

	// --untracked-files=all is required here: without it, git status --porcelain
	// collapses a brand-new untracked directory into a single opaque entry
	// (e.g. "?? newpkg/") instead of listing the files inside it. That entry's
	// path has no source-file extension, so hasMatchingFiles below would never
	// see e.g. "newpkg/foo.go" and would wrongly conclude nothing changed,
	// silently skipping tests for entirely-new, never-committed packages.
	out, err := runCmd("git", "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return nil // git unavailable → skip
	}

	repoRoot, err := config.FindRepoRoot(cwd)
	if err != nil {
		return fmt.Errorf("failed to find repo root: %w", err)
	}

	if !hasMatchingFiles(out, exts) {
		// No tracked source changes since the last commit, so there is
		// nothing to test. Record that explicitly as "skipped" (rather than
		// leaving no result file at all) so `commit`/`test-and-commit` can
		// tell "test correctly determined no run was needed" apart from
		// "test never ran before this commit attempt" — the latter is the
		// broken-invariant case they should still refuse to commit through.
		if writeErr := writeLastTestResult(repoRoot, "skipped"); writeErr != nil {
			return fmt.Errorf("no source changes but could not record skip result: %w", writeErr)
		}
		return nil
	}

	parts := strings.Fields(cfg.TestCommand)
	if len(parts) == 0 {
		return nil
	}
	c := exec.Command(parts[0], parts[1:]...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	err = c.Run()
	if err != nil {
		// Test failed: record failure and return blocking error
		if writeErr := writeLastTestResult(repoRoot, "fail"); writeErr != nil {
			return Blocking(fmt.Errorf("test failed and could not record result: %w (test error: %v)", writeErr, err))
		}
		return Blocking(err)
	}

	// Test passed: record success
	if writeErr := writeLastTestResult(repoRoot, "pass"); writeErr != nil {
		return Blocking(fmt.Errorf("test passed but could not record result: %w", writeErr))
	}
	return nil
}

func hasMatchingFiles(gitStatus string, exts []string) bool {
	for _, line := range strings.Split(gitStatus, "\n") {
		if len(line) < 3 {
			continue
		}
		// git status --porcelain: "XY filename" (first 2 chars = status, then space, then path)
		path := strings.TrimSpace(line[2:])
		for _, ext := range exts {
			if strings.HasSuffix(path, ext) {
				return true
			}
		}
	}
	return false
}
