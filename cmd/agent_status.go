package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"dreamland/internal/agentidentity"
	"dreamland/internal/config"
)

var agentStatusCmd = &cobra.Command{
	Use:   "agent-status",
	Short: "Record or clear the currently-dispatched agent for this session (Claude Code PreToolUse/PostToolUse hook binding)",
	RunE:  runAgentStatus,
}

var (
	agentStatusStart bool
	agentStatusStop  bool
)

func init() {
	rootCmd.AddCommand(agentStatusCmd)
	agentStatusCmd.Flags().BoolVar(&agentStatusStart, "start", false, "record a dispatch beginning (PreToolUse)")
	agentStatusCmd.Flags().BoolVar(&agentStatusStop, "stop", false, "clear a dispatch (PostToolUse)")
}

// agentStatusStaleAfter bounds how long an agent-status entry is trusted. dreamland
// statusline treats anything older as idle, and every agent-status write prunes
// expired entries — so a missed --stop (crashed session, missed hook) self-heals
// instead of leaving a permanently wrong indicator.
const agentStatusStaleAfter = 10 * time.Minute

// agentStatusEntry is one session's record in .dreamland/agent-status.json.
type agentStatusEntry struct {
	Agent     string `json:"agent"`
	ToolUseID string `json:"tool_use_id,omitempty"`
	StartedAt string `json:"started_at"` // RFC3339
}

// runAgentStatus is best-effort by design: it must never block the PreToolUse/
// PostToolUse hook it's bound to, so every failure path exits 0 rather than
// propagating an error (see the agent-dispatch-visibility spec).
func runAgentStatus(cmd *cobra.Command, args []string) error {
	payload := readHookPayload(os.Stdin)
	if payload == nil {
		return nil
	}
	sessionID, _ := payload["session_id"].(string)
	if sessionID == "" {
		return nil
	}

	cwd, err := osGetwd()
	if err != nil {
		return nil
	}
	repoRoot, err := config.FindRepoRoot(cwd)
	if err != nil {
		return nil
	}
	statusPath := agentStatusPath(repoRoot)

	switch {
	case agentStatusStart:
		agentName := agentidentity.FromPayload(payload)
		if agentName == "" {
			return nil
		}
		toolUseID, _ := payload["tool_use_id"].(string)
		updateAgentStatus(statusPath, func(m map[string]agentStatusEntry) {
			m[sessionID] = agentStatusEntry{
				Agent:     agentName,
				ToolUseID: toolUseID,
				StartedAt: time.Now().UTC().Format(time.RFC3339),
			}
		})
	case agentStatusStop:
		updateAgentStatus(statusPath, func(m map[string]agentStatusEntry) {
			delete(m, sessionID)
		})
	}
	return nil
}

func agentStatusPath(repoRoot string) string {
	return filepath.Join(repoRoot, ".dreamland", "agent-status.json")
}

// updateAgentStatus reads statusPath (a missing or malformed file is treated as an
// empty map, not an error), applies mutate, prunes any entry older than
// agentStatusStaleAfter, and writes the result back. Plain read-then-write with no
// file locking — an accepted trade-off for this cosmetic feature: two concurrent
// dreamland agent-status invocations racing on the same file could lose one write,
// worst case one dispatch's start/stop record doesn't land. See design.md Risks in
// the claude-code-agent-statusline change. Failures are swallowed (best-effort).
func updateAgentStatus(statusPath string, mutate func(map[string]agentStatusEntry)) {
	m := map[string]agentStatusEntry{}
	if data, err := os.ReadFile(statusPath); err == nil {
		_ = json.Unmarshal(data, &m) // malformed existing file: start fresh rather than fail
	}

	mutate(m)

	now := time.Now().UTC()
	for id, entry := range m {
		startedAt, err := time.Parse(time.RFC3339, entry.StartedAt)
		if err != nil || now.Sub(startedAt) > agentStatusStaleAfter {
			delete(m, id)
		}
	}

	if err := os.MkdirAll(filepath.Dir(statusPath), 0o755); err != nil {
		return
	}
	data, err := json.Marshal(m)
	if err != nil {
		return
	}
	_ = os.WriteFile(statusPath, data, 0o644)
}
