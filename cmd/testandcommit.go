package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// testAndCommitCmd runs `test` then `commit` in a single process, in that
// order, guaranteed. Claude Code's Stop hook array does not document whether
// sibling commands run sequentially or concurrently — dogfooding this repo's
// own hooks showed `commit` observing no `.dreamland/last-test-result.json`
// moments before `test` (a separate process) finished writing one, because
// the two were dispatched together rather than one after the other. Binding
// this single command instead of the separate `test` + `commit
// --reason turn-complete` pair removes the race structurally: there is only
// one process, so there is nothing left for the hook runner to parallelize
// against.
var testAndCommitCmd = &cobra.Command{
	Use:   "test-and-commit",
	Short: "Run tests then auto-commit in one process, avoiding a race between separate hook-bound commands",
	RunE:  runTestAndCommit,
}

var tacReason string

func init() {
	rootCmd.AddCommand(testAndCommitCmd)
	testAndCommitCmd.Flags().StringVar(&tacReason, "reason", "", "turn-complete or handoff")
}

func runTestAndCommit(cmd *cobra.Command, args []string) error {
	if err := runTest(cmd, args); err != nil {
		fmt.Fprintf(os.Stderr, "dreamland test: %v\n", err)
	}
	commitReason = tacReason
	return runCommit(cmd, args)
}
