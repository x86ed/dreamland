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
