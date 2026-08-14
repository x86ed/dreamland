package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"dreamland/internal/config"
)

var commitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Auto-commit pending changes on turn completion or agent handoff",
	RunE:  runCommit,
}

var commitReason string

func init() {
	rootCmd.AddCommand(commitCmd)
	commitCmd.Flags().StringVar(&commitReason, "reason", "", "turn-complete or handoff")
}

func runCommit(cmd *cobra.Command, args []string) error {
	if commitReason != "turn-complete" && commitReason != "handoff" {
		return Blocking(errors.New("--reason must be one of: turn-complete, handoff"))
	}

	cwd, err := osGetwd()
	if err != nil {
		return err
	}

	repoRoot, err := config.FindRepoRoot(cwd)
	if err != nil {
		if errors.Is(err, config.ErrNoGitRepo) {
			return nil // skip outside git repo
		}
		return Blocking(err)
	}

	cfg, err := config.Load(cwd)
	if err != nil {
		if errors.Is(err, config.ErrNoGitRepo) {
			return nil // skip outside git repo (already checked above, but be safe)
		}
		return Blocking(err)
	}
	if cfg == nil {
		cfg = &config.Config{}
	}

	// Gate on test results when committing at end-of-turn (not for handoff).
	if commitReason == "turn-complete" {
		if cfg.TestCommand != "" {
			testResult, err := readLastTestResult(repoRoot)
			if err != nil {
				return Blocking(fmt.Errorf("failed to read test result: %w", err))
			}

			// If a test command is configured but no result file exists, that's a broken invariant
			if testResult == nil {
				msg := fmt.Sprintf(
					"test command configured in .dreamland.json but no result recorded in .dreamland/last-test-result.json\n"+
						"this suggests `dreamland test` did not run before this commit attempt\n"+
						"route this to iktomi to investigate the hook wiring or test command configuration",
				)
				return Blocking(errors.New(msg))
			}

			// If the test result shows failure at the current HEAD, refuse to commit
			if testResult.Status == "fail" {
				currentHead, err := runCmd("git", "rev-parse", "HEAD")
				if err != nil {
					return Blocking(fmt.Errorf("git rev-parse HEAD: %w", err))
				}
				currentHead = strings.TrimSpace(currentHead)

				if testResult.HeadSHA == currentHead {
					return Blocking(fmt.Errorf(
						"tests failed at HEAD %s (recorded in .dreamland/last-test-result.json) — commit refused",
						currentHead[:8],
					))
				}
			}
			// If result exists but has stale HEAD SHA, fall through and allow commit (fail-open for staleness)
		}
		// If no test command configured, no gating applies
	}

	status, err := runCmd("git", "status", "--porcelain")
	if err != nil {
		return Blocking(fmt.Errorf("git status --porcelain: %w", err))
	}
	if strings.TrimSpace(status) == "" {
		return nil // clean tree, no-op
	}

	if _, err := gitExec("add", "-A"); err != nil {
		return Blocking(fmt.Errorf("git add -A: %w", err))
	}

	agentName := currentGitIdentityName(cfg)
	message := fmt.Sprintf("chore: %s checkpoint (%s)", commitReason, agentName)
	if out, err := gitExec("commit", "-m", message); err != nil {
		return Blocking(fmt.Errorf("git commit: %w\n%s", err, out))
	}
	return nil
}

// currentGitIdentityName reads the git-configured user.name (set by coauthor) and
// returns it if non-empty, otherwise falls back to resolveEnforcedAgentName.
// This ensures the commit subject always matches the actual git author, even when
// commit runs at a different lifecycle event than coauthor with a different payload shape.
func currentGitIdentityName(cfg *config.Config) string {
	name, err := runCmd("git", "config", "--local", "--get", "user.name")
	if err == nil {
		name = strings.TrimSpace(name)
		if name != "" {
			return name
		}
	}
	// Fallback: resolve fresh if not configured
	return resolveEnforcedAgentName(cfg)
}
