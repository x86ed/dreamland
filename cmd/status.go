package cmd

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
)

// ChangeStatus is one OpenSpec change's task-completion progress, as
// reported by `openspec list --json`.
type ChangeStatus struct {
	Name           string `json:"name"`
	CompletedTasks int    `json:"completedTasks"`
	TotalTasks     int    `json:"totalTasks"`
	Status         string `json:"status"`
}

// StatusResponse is GET /api/status's payload: the currently active agent
// (from .dreamland-session.json, the same file the coauthor/telemetry hooks
// already maintain every turn) and every in-progress OpenSpec change's
// task-completion progress.
//
// Real-data scope note: there is no per-task agent-assignment data anywhere
// in this system to know precisely "which agent is on task N of change X" —
// OpenSpec tracks task completion, not who's doing it, and that association
// only ever exists transiently inside a live agent session. CurrentAgent is
// the closest genuine signal available (the session's current agent
// identity), not a per-task-specific one; the UI highlights that one agent's
// node rather than attempting a task-to-agent mapping that isn't real data.
type StatusResponse struct {
	CurrentAgent string         `json:"currentAgent,omitempty"`
	Changes      []ChangeStatus `json:"changes"`
}

// readCurrentAgent reads the "agent" field from repoRoot's
// .dreamland-session.json. Returns "" (not an error) if the file is absent
// or unparseable — this is best-effort session telemetry, not a required
// data source.
func readCurrentAgent(repoRoot string) string {
	data, err := os.ReadFile(filepath.Join(repoRoot, ".dreamland-session.json"))
	if err != nil {
		return ""
	}
	var session struct {
		Agent string `json:"agent"`
	}
	if err := json.Unmarshal(data, &session); err != nil {
		return ""
	}
	return session.Agent
}

// listChangeStatuses shells out to `openspec list --json` in repoRoot and
// returns every change's progress. Returns an empty (not nil) slice, no
// error, if the openspec CLI isn't installed or the command fails — a
// missing dependency here shouldn't break the graph view, just omit the
// overlay.
func listChangeStatuses(repoRoot string) []ChangeStatus {
	cmd := exec.Command("openspec", "list", "--json")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return []ChangeStatus{}
	}
	return parseChangeStatuses(out)
}

// parseChangeStatuses parses `openspec list --json`'s output — split out
// from listChangeStatuses so it's testable without depending on the
// openspec CLI being installed in the test environment.
func parseChangeStatuses(data []byte) []ChangeStatus {
	var parsed struct {
		Changes []ChangeStatus `json:"changes"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return []ChangeStatus{}
	}
	if parsed.Changes == nil {
		return []ChangeStatus{}
	}
	return parsed.Changes
}

// currentStatus assembles the full StatusResponse for repoRoot.
func currentStatus(repoRoot string) StatusResponse {
	return StatusResponse{
		CurrentAgent: readCurrentAgent(repoRoot),
		Changes:      listChangeStatuses(repoRoot),
	}
}
