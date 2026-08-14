package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dreamland/internal/config"
)

func TestRenderStatusline_NoStateFile(t *testing.T) {
	root := makeCoauthorRepo(t, config.Config{CodingTool: "Claude Code"})
	orig := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = orig })

	got := renderStatusline(strings.NewReader(`{"session_id":"abc"}`))
	if got != statuslineIdleIndicator {
		t.Errorf("got %q, want idle indicator %q", got, statuslineIdleIndicator)
	}
}

func TestRenderStatusline_ActiveRecentDispatch(t *testing.T) {
	root := makeCoauthorRepo(t, config.Config{CodingTool: "Claude Code"})
	orig := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = orig })

	writeAgentStatusFile(t, root, map[string]agentStatusEntry{
		"abc": {Agent: "phobetor", StartedAt: time.Now().UTC().Add(-30 * time.Second).Format(time.RFC3339)},
	})

	got := renderStatusline(strings.NewReader(`{"session_id":"abc"}`))
	if !strings.Contains(got, "phobetor") {
		t.Errorf("got %q, want it to contain phobetor", got)
	}
}

func TestRenderStatusline_StaleDispatch(t *testing.T) {
	root := makeCoauthorRepo(t, config.Config{CodingTool: "Claude Code"})
	orig := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = orig })

	writeAgentStatusFile(t, root, map[string]agentStatusEntry{
		"abc": {Agent: "phobetor", StartedAt: time.Now().UTC().Add(-15 * time.Minute).Format(time.RFC3339)},
	})

	got := renderStatusline(strings.NewReader(`{"session_id":"abc"}`))
	if got != statuslineIdleIndicator {
		t.Errorf("got %q, want idle indicator for a stale entry, not the stale agent name", got)
	}
}

func TestRenderStatusline_MissingSessionID(t *testing.T) {
	root := makeCoauthorRepo(t, config.Config{CodingTool: "Claude Code"})
	orig := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = orig })

	got := renderStatusline(strings.NewReader(`{"hook_event_name":"whatever"}`))
	if got != statuslineIdleIndicator {
		t.Errorf("got %q, want idle indicator", got)
	}
}

func TestRenderStatusline_MalformedPayload(t *testing.T) {
	root := makeCoauthorRepo(t, config.Config{CodingTool: "Claude Code"})
	orig := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = orig })

	got := renderStatusline(strings.NewReader(`not json`))
	if got != statuslineIdleIndicator {
		t.Errorf("got %q, want idle indicator", got)
	}
}

func TestRunStatusline_WritesOutput(t *testing.T) {
	root := makeCoauthorRepo(t, config.Config{CodingTool: "Claude Code"})
	orig := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = orig })

	withPipedStdin(t, `{"session_id":"abc"}`)

	var buf strings.Builder
	statuslineCmd.SetOut(&buf)
	t.Cleanup(func() { statuslineCmd.SetOut(nil) })

	if err := runStatusline(statuslineCmd, nil); err != nil {
		t.Fatalf("runStatusline: %v", err)
	}
	if buf.String() != statuslineIdleIndicator {
		t.Errorf("got %q, want %q", buf.String(), statuslineIdleIndicator)
	}
}

func writeAgentStatusFile(t *testing.T, root string, m map[string]agentStatusEntry) {
	t.Helper()
	statusPath := agentStatusPath(root)
	if err := os.MkdirAll(filepath.Dir(statusPath), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statusPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
