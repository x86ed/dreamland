package agentidentity

import "testing"

func TestIsRegistered(t *testing.T) {
	for name := range Registered {
		if !IsRegistered(name) {
			t.Errorf("%q should be registered", name)
		}
	}
	for _, name := range []string{"", "claude-code", "dreamland", "not-a-real-agent"} {
		if IsRegistered(name) {
			t.Errorf("%q should not be registered", name)
		}
	}
}

func TestFromPayload_CopilotAgentType(t *testing.T) {
	got := FromPayload(map[string]any{"agent_type": "morpheus"})
	if got != "morpheus" {
		t.Errorf("got %q, want morpheus", got)
	}
}

func TestFromPayload_ClaudeCodeSubagentType(t *testing.T) {
	got := FromPayload(map[string]any{
		"tool_input": map[string]any{"subagent_type": "phobetor"},
	})
	if got != "phobetor" {
		t.Errorf("got %q, want phobetor", got)
	}
}

func TestFromPayload_AgentTypeTakesPriority(t *testing.T) {
	got := FromPayload(map[string]any{
		"agent_type": "morpheus",
		"tool_input": map[string]any{"subagent_type": "phobetor"},
	})
	if got != "morpheus" {
		t.Errorf("got %q, want morpheus", got)
	}
}

func TestFromPayload_Empty(t *testing.T) {
	if got := FromPayload(map[string]any{}); got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}

func TestFromPayload_ToolInputWrongShape(t *testing.T) {
	got := FromPayload(map[string]any{"tool_input": "not-a-map"})
	if got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}

// TestFromPayload_ClaudeCodeSubagentStopAgentType uses the exact SubagentStop input shape
// documented in Anthropic's hooks reference (session_id, transcript_path, cwd,
// permission_mode, hook_event_name, stop_hook_active, agent_id, agent_type,
// agent_transcript_path, last_assistant_message, background_tasks, session_crons) to confirm
// the top-level "agent_type" field is extracted correctly and the extra fields introduced
// specifically on SubagentStop don't interfere with resolution.
func TestFromPayload_ClaudeCodeSubagentStopAgentType(t *testing.T) {
	payload := map[string]any{
		"session_id":            "abc123",
		"transcript_path":       "~/.claude/projects/.../abc123.jsonl",
		"cwd":                   "/Users/example",
		"permission_mode":       "default",
		"hook_event_name":       "SubagentStop",
		"stop_hook_active":      false,
		"agent_id":              "def456",
		"agent_type":            "phantasos",
		"agent_transcript_path": "~/.claude/projects/.../abc123/subagents/agent-def456.jsonl",
		"last_assistant_message": "Analysis complete.",
		"background_tasks":       []any{},
		"session_crons":          []any{},
	}
	got := FromPayload(payload)
	if got != "phantasos" {
		t.Errorf("got %q, want phantasos", got)
	}
}

// TestFromPayload_SubagentStopUnregisteredAgentType confirms a Claude Code built-in
// (non-dreamland) subagent type like "Explore" is extracted verbatim by FromPayload —
// filtering it against the registered allow-list is the caller's responsibility
// (see cmd/coauthor.go's isRegisteredAgent), not FromPayload's.
func TestFromPayload_SubagentStopUnregisteredAgentType(t *testing.T) {
	got := FromPayload(map[string]any{"hook_event_name": "SubagentStop", "agent_type": "Explore"})
	if got != "Explore" {
		t.Errorf("got %q, want Explore", got)
	}
	if IsRegistered(got) {
		t.Errorf("%q should not be a registered dreamland agent", got)
	}
}
