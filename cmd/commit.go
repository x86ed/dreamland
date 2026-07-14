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
		return errors.New("--reason must be one of: turn-complete, handoff")
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

	status, err := runCmd("git", "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("git status --porcelain: %w", err)
	}
	if strings.TrimSpace(status) == "" {
		return nil // clean tree, no-op
	}

	if _, err := gitExec("add", "-A"); err != nil {
		return fmt.Errorf("git add -A: %w", err)
	}

	agentName := resolveAgentName(cfg.CodingTool)
	message := fmt.Sprintf("chore: %s checkpoint (%s)", commitReason, agentName)
	if out, err := gitExec("commit", "-m", message); err != nil {
		return fmt.Errorf("git commit: %w\n%s", err, out)
	}
	return nil
}
