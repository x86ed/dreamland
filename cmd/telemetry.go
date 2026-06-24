package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"dreamland/internal/config"
	"dreamland/internal/scaffold"
	"dreamland/internal/telemetry"
	"dreamland/internal/telemetry/tools"
)

var telemetryCmd = &cobra.Command{
	Use:   "telemetry",
	Short: "Session telemetry commands",
}

func init() {
	rootCmd.AddCommand(telemetryCmd)
	registerCollectors()

	writeCmd := &cobra.Command{
		Use:   "write",
		Short: "Write a telemetry snapshot from the current tool hook payload (reads stdin)",
		RunE:  runTelemetryWrite,
	}
	writeCmd.Flags().String("tool", "", "tool name (claude-code, codex, cursor, kiro, antigravity, github-copilot)")
	writeCmd.Flags().String("phase", "", "kiro phase: start or stop")
	_ = writeCmd.MarkFlagRequired("tool")
	telemetryCmd.AddCommand(writeCmd)

	snapshotCmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Output the current session telemetry snapshot",
		RunE:  runTelemetrySnapshot,
	}
	snapshotCmd.Flags().String("format", "json", "output format: json or trailers")
	snapshotCmd.Flags().Duration("max-age", 4*time.Hour, "warn if snapshot is older than this")
	telemetryCmd.AddCommand(snapshotCmd)

	resetCmd := &cobra.Command{
		Use:   "reset",
		Short: "Delete the current session telemetry file",
		RunE:  runTelemetryReset,
	}
	telemetryCmd.AddCommand(resetCmd)

	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Install the commit-msg telemetry hook",
		RunE:  runTelemetryInstall,
	}
	telemetryCmd.AddCommand(installCmd)

	uninstallCmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove the dreamland block from the commit-msg hook",
		RunE:  runTelemetryUninstall,
	}
	telemetryCmd.AddCommand(uninstallCmd)
}

func registerCollectors() {
	telemetry.Register("claude-code", &tools.ClaudeCollector{})
	telemetry.Register("codex", &tools.CodexCollector{})
	telemetry.Register("cursor", &tools.CursorCollector{})
	telemetry.Register("antigravity", &tools.AntigravityCollector{})
	telemetry.Register("github-copilot", &tools.CopilotCollector{})
	// kiro uses a dynamic KiroCollector with Phase set at runtime
}

var validTools = []string{"claude-code", "codex", "cursor", "kiro", "antigravity", "github-copilot"}

func runTelemetryWrite(cmd *cobra.Command, _ []string) error {
	toolName, _ := cmd.Flags().GetString("tool")
	phase, _ := cmd.Flags().GetString("phase")

	cwd, err := osGetwd()
	if err != nil {
		return err
	}
	repoRoot, err := config.FindRepoRoot(cwd)
	if err != nil {
		return fmt.Errorf("not in a git repository: %w", err)
	}
	cfg, _ := config.Load(cwd)

	var collector telemetry.Collector
	if toolName == "kiro" {
		collector = &tools.KiroCollector{Phase: phase}
	} else {
		var ok bool
		collector, ok = telemetry.Registry[toolName]
		if !ok {
			return fmt.Errorf("unknown tool %q; valid: %v", toolName, validTools)
		}
	}

	result, err := collector.Collect(cmd.InOrStdin(), cfg)
	if err != nil {
		return fmt.Errorf("collect telemetry: %w", err)
	}
	if result == nil {
		return nil
	}
	return telemetry.Write(repoRoot, result)
}

func runTelemetrySnapshot(cmd *cobra.Command, _ []string) error {
	format, _ := cmd.Flags().GetString("format")
	maxAge, _ := cmd.Flags().GetDuration("max-age")

	cwd, err := osGetwd()
	if err != nil {
		return err
	}
	repoRoot, err := config.FindRepoRoot(cwd)
	if err != nil {
		return nil // not in a git repo — exit 0 quietly
	}

	snap, err := telemetry.Read(repoRoot)
	if err != nil {
		return err
	}
	if snap == nil {
		return nil
	}

	if snap.CapturedAt != "" && maxAge > 0 {
		if t, err := time.Parse(time.RFC3339, snap.CapturedAt); err == nil {
			if age := time.Since(t); age > maxAge {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: telemetry snapshot is %s old (captured_at: %s)\n",
					age.Round(time.Second), snap.CapturedAt)
			}
		}
	}

	switch format {
	case "trailers":
		out := formatTrailers(snap)
		if out != "" {
			fmt.Fprint(cmd.OutOrStdout(), out)
		}
	default:
		data, err := json.MarshalIndent(snap, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
	}
	return nil
}

func runTelemetryReset(_ *cobra.Command, _ []string) error {
	cwd, err := osGetwd()
	if err != nil {
		return err
	}
	repoRoot, err := config.FindRepoRoot(cwd)
	if err != nil {
		return nil
	}
	path := filepath.Join(repoRoot, ".dreamland-session.json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func runTelemetryInstall(cmd *cobra.Command, _ []string) error {
	cwd, err := osGetwd()
	if err != nil {
		return err
	}
	repoRoot, err := config.FindRepoRoot(cwd)
	if err != nil {
		return fmt.Errorf("not in a git repository: %w", err)
	}
	if err := scaffold.InstallCommitMsgHook(repoRoot); err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), "installed: .git/hooks/commit-msg")
	return nil
}

func runTelemetryUninstall(_ *cobra.Command, _ []string) error {
	cwd, err := osGetwd()
	if err != nil {
		return err
	}
	repoRoot, err := config.FindRepoRoot(cwd)
	if err != nil {
		return fmt.Errorf("not in a git repository: %w", err)
	}
	return scaffold.UninstallCommitMsgHook(repoRoot)
}

// formatTrailers renders snap as git trailer lines, omitting zero/empty fields.
func formatTrailers(snap *telemetry.SnapshotResult) string {
	var out string
	add := func(key, val string) {
		if val != "" {
			out += key + ": " + val + "\n"
		}
	}
	addInt := func(key string, val int64) {
		if val != 0 {
			out += fmt.Sprintf("%s: %d\n", key, val)
		}
	}
	add("AI-Tool", snap.Tool)
	add("AI-Model", snap.Model)
	add("AI-ThinkingEffort", snap.ThinkingEffort)
	addInt("AI-InputTokens", snap.InputTokens)
	addInt("AI-OutputTokens", snap.OutputTokens)
	addInt("AI-CachedTokens", snap.CachedTokens)
	addInt("AI-TotalTokens", snap.TotalTokens)
	add("AI-CapturedAt", snap.CapturedAt)
	return out
}
