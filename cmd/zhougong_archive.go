package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"dreamland/internal/config"
	"dreamland/internal/zhougongdata"
)

var (
	zhougongArchiveBranch string
	zhougongArchiveSlug   string
)

var zhougongArchiveCmd = &cobra.Command{
	Use:   "zhougong-archive",
	Short: "Write a durable run record for a branch to .dreamland/runs/",
	RunE:  runZhougongArchive,
}

func init() {
	zhougongArchiveCmd.Flags().StringVar(&zhougongArchiveBranch, "branch", "", "branch to archive")
	zhougongArchiveCmd.Flags().StringVar(&zhougongArchiveSlug, "slug", "", "record name (default: branch name)")
	_ = zhougongArchiveCmd.MarkFlagRequired("branch")
	rootCmd.AddCommand(zhougongArchiveCmd)
}

func runZhougongArchive(cmd *cobra.Command, args []string) error {
	cwd, err := osGetwd()
	if err != nil {
		return err
	}
	repoRoot, err := config.FindRepoRoot(cwd)
	if err != nil {
		return fmt.Errorf("not in a git repository: %w", err)
	}
	ds, err := zhougongdata.ParseBranch(repoRoot, zhougongArchiveBranch)
	if err != nil {
		return err
	}
	slug := zhougongArchiveSlug
	if slug == "" {
		slug = zhougongdata.SlugFor(zhougongArchiveBranch)
	}
	path, err := zhougongdata.WriteArchive(repoRoot, slug, ds, time.Now())
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), path)
	return nil
}
