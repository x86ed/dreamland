package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"dreamland/internal/config"
)

var statuslineCmd = &cobra.Command{
	Use:   "statusline",
	Short: "Print the currently-dispatched agent for this session (Claude Code statusLine binding)",
	RunE:  runStatusline,
}

func init() {
	rootCmd.AddCommand(statuslineCmd)
}

// statuslineIdleIndicator is printed when no agent is currently dispatched — no
// state file, no entry for this session, or the entry is stale.
const statuslineIdleIndicator = "[janus]"

func runStatusline(cmd *cobra.Command, args []string) error {
	fmt.Fprint(cmd.OutOrStdout(), renderStatusline(os.Stdin))
	return nil
}

// renderStatusline reads a statusLine JSON payload from r and returns the status
// segment to print. Read-only: it never writes or prunes .dreamland/agent-status.json
// — that's agent-status's job (see design.md). Any failure (bad payload, no repo,
// missing/stale/malformed state) falls back to the idle indicator rather than erroring
// — a blank or wrong statusline is a cosmetic issue, never worth failing the command.
func renderStatusline(r io.Reader) string {
	payload := readHookPayload(r)
	if payload == nil {
		return statuslineIdleIndicator
	}
	sessionID, _ := payload["session_id"].(string)
	if sessionID == "" {
		return statuslineIdleIndicator
	}

	cwd, err := osGetwd()
	if err != nil {
		return statuslineIdleIndicator
	}
	repoRoot, err := config.FindRepoRoot(cwd)
	if err != nil {
		return statuslineIdleIndicator
	}

	data, err := os.ReadFile(agentStatusPath(repoRoot))
	if err != nil {
		return statuslineIdleIndicator
	}
	var m map[string]agentStatusEntry
	if err := json.Unmarshal(data, &m); err != nil {
		return statuslineIdleIndicator
	}
	entry, ok := m[sessionID]
	if !ok {
		return statuslineIdleIndicator
	}
	startedAt, err := time.Parse(time.RFC3339, entry.StartedAt)
	if err != nil || time.Since(startedAt) > agentStatusStaleAfter {
		return statuslineIdleIndicator
	}
	return "[" + entry.Agent + "]"
}
