package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"dreamland/internal/config"
)

func withAgentStatusFlags(t *testing.T, start, stop bool) {
	t.Helper()
	origStart, origStop := agentStatusStart, agentStatusStop
	agentStatusStart, agentStatusStop = start, stop
	t.Cleanup(func() { agentStatusStart, agentStatusStop = origStart, origStop })
}

func readAgentStatusFile(t *testing.T, root string) map[string]agentStatusEntry {
	t.Helper()
	data, err := os.ReadFile(agentStatusPath(root))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	var m map[string]agentStatusEntry
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestRunAgentStatus_Start_RecordsDispatch(t *testing.T) {
	root := makeCoauthorRepo(t, config.Config{CodingTool: "Claude Code"})
	orig := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = orig })

	withAgentStatusFlags(t, true, false)
	withPipedStdin(t, `{"session_id":"abc","tool_use_id":"tu1","tool_input":{"subagent_type":"morpheus"}}`)

	if err := runAgentStatus(nil, nil); err != nil {
		t.Fatalf("runAgentStatus: %v", err)
	}

	m := readAgentStatusFile(t, root)
	entry, ok := m["abc"]
	if !ok {
		t.Fatalf("expected key %q, got %v", "abc", m)
	}
	if entry.Agent != "morpheus" {
		t.Errorf("agent = %q, want morpheus", entry.Agent)
	}
	if entry.ToolUseID != "tu1" {
		t.Errorf("tool_use_id = %q, want tu1", entry.ToolUseID)
	}
	if _, err := time.Parse(time.RFC3339, entry.StartedAt); err != nil {
		t.Errorf("started_at %q not RFC3339: %v", entry.StartedAt, err)
	}
}

func TestRunAgentStatus_Start_NoSubagentType_LeavesFileUntouched(t *testing.T) {
	root := makeCoauthorRepo(t, config.Config{CodingTool: "Claude Code"})
	orig := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = orig })

	withAgentStatusFlags(t, true, false)
	withPipedStdin(t, `{"session_id":"abc","hook_event_name":"PreToolUse"}`)

	if err := runAgentStatus(nil, nil); err != nil {
		t.Fatalf("runAgentStatus: %v", err)
	}

	if m := readAgentStatusFile(t, root); m != nil {
		t.Errorf("expected no file written, got %v", m)
	}
}

func TestRunAgentStatus_Start_PrunesStaleEntryFromOtherSession(t *testing.T) {
	root := makeCoauthorRepo(t, config.Config{CodingTool: "Claude Code"})
	orig := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = orig })

	statusPath := agentStatusPath(root)
	if err := os.MkdirAll(filepath.Dir(statusPath), 0o755); err != nil {
		t.Fatal(err)
	}
	stale := map[string]agentStatusEntry{
		"old": {Agent: "phobetor", StartedAt: time.Now().UTC().Add(-20 * time.Minute).Format(time.RFC3339)},
	}
	data, _ := json.Marshal(stale)
	if err := os.WriteFile(statusPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	withAgentStatusFlags(t, true, false)
	withPipedStdin(t, `{"session_id":"abc","tool_input":{"subagent_type":"morpheus"}}`)

	if err := runAgentStatus(nil, nil); err != nil {
		t.Fatalf("runAgentStatus: %v", err)
	}

	m := readAgentStatusFile(t, root)
	if _, ok := m["old"]; ok {
		t.Errorf("expected stale entry %q to be pruned, got %v", "old", m)
	}
	if _, ok := m["abc"]; !ok {
		t.Errorf("expected new entry %q to be present, got %v", "abc", m)
	}
}

func TestRunAgentStatus_Stop_RemovesExistingEntry(t *testing.T) {
	root := makeCoauthorRepo(t, config.Config{CodingTool: "Claude Code"})
	orig := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = orig })

	statusPath := agentStatusPath(root)
	if err := os.MkdirAll(filepath.Dir(statusPath), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := map[string]agentStatusEntry{
		"abc": {Agent: "morpheus", StartedAt: time.Now().UTC().Format(time.RFC3339)},
	}
	data, _ := json.Marshal(existing)
	if err := os.WriteFile(statusPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	withAgentStatusFlags(t, false, true)
	withPipedStdin(t, `{"session_id":"abc"}`)

	if err := runAgentStatus(nil, nil); err != nil {
		t.Fatalf("runAgentStatus: %v", err)
	}

	m := readAgentStatusFile(t, root)
	if _, ok := m["abc"]; ok {
		t.Errorf("expected key %q removed, got %v", "abc", m)
	}
}

func TestRunAgentStatus_Stop_NoMatchingEntry_NoOp(t *testing.T) {
	root := makeCoauthorRepo(t, config.Config{CodingTool: "Claude Code"})
	orig := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = orig })

	withAgentStatusFlags(t, false, true)
	withPipedStdin(t, `{"session_id":"no-such-session"}`)

	if err := runAgentStatus(nil, nil); err != nil {
		t.Fatalf("runAgentStatus: %v", err)
	}
}

func TestRunAgentStatus_MalformedPayload_ExitsZero(t *testing.T) {
	root := makeCoauthorRepo(t, config.Config{CodingTool: "Claude Code"})
	orig := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = orig })

	for _, start := range []bool{true, false} {
		withAgentStatusFlags(t, start, !start)
		withPipedStdin(t, `not json`)

		if err := runAgentStatus(nil, nil); err != nil {
			t.Fatalf("runAgentStatus (start=%v): %v", start, err)
		}
	}
}
