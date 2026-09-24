package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"dreamland/internal/config"
)

var zhougongDashboardPort int

var zhougongDashboardCmd = &cobra.Command{
	Use:   "zhougong-dashboard",
	Short: "Start, stop, or inspect the zhougong metrics dashboard",
}

var zhougongDashboardStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the dashboard (or reuse the running one) and print its URL",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		root, err := zhougongDashRepoRoot()
		if err != nil {
			return err
		}
		url, err := startZhougongDashboard(root, zhougongDashboardPort)
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), url)
		return nil
	},
}

var zhougongDashboardStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the dashboard and release its port",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		root, err := zhougongDashRepoRoot()
		if err != nil {
			return err
		}
		_, running := zhougongDashRunning(root)
		if err := stopZhougongDashboard(root); err != nil {
			return err
		}
		if running {
			fmt.Fprintln(cmd.OutOrStdout(), "zhougong dashboard stopped")
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "zhougong dashboard not running")
		}
		return nil
	},
}

var zhougongDashboardStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Print the dashboard URL, or \"not running\"",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		root, err := zhougongDashRepoRoot()
		if err != nil {
			return err
		}
		if url, ok := zhougongDashRunning(root); ok {
			fmt.Fprintln(cmd.OutOrStdout(), url)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "not running")
		}
		return nil
	},
}

func init() {
	zhougongDashboardStartCmd.Flags().IntVar(&zhougongDashboardPort, "port", 0, "port to bind on 127.0.0.1; 0 picks a free port")
	zhougongDashboardCmd.AddCommand(zhougongDashboardStartCmd, zhougongDashboardStopCmd, zhougongDashboardStatusCmd)
	rootCmd.AddCommand(zhougongDashboardCmd)
}

func zhougongDashRepoRoot() (string, error) {
	cwd, err := osGetwd()
	if err != nil {
		return "", err
	}
	root, err := config.FindRepoRoot(cwd)
	if err != nil {
		return "", fmt.Errorf("not in a git repository: %w", err)
	}
	return root, nil
}

func zhougongDashRunning(repoRoot string) (string, bool) {
	st, ok := readZhougongDashState(repoRoot)
	if !ok || !processAlive(st.PID) {
		return "", false
	}
	return st.URL, true
}
