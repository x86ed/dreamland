package agentidentity

import (
	"os"
	"path/filepath"
	"testing"
)

// TestIsRegistered exercises the new (repoRoot string) signature (task 2.3) against a
// directory with no .dreamland/oneiroi/registry.json present — the built-in ten must
// still resolve exactly as before this change, with no rebuild/registry-file dependency.
func TestIsRegistered(t *testing.T) {
	root := t.TempDir()

	for name := range Registered(root) {
		if !IsRegistered(name, root) {
			t.Errorf("%q should be registered", name)
		}
	}
	for _, name := range []string{"", "claude-code", "dreamland", "not-a-real-agent"} {
		if IsRegistered(name, root) {
			t.Errorf("%q should not be registered", name)
		}
	}
}

// TestIsRegistered_NoRegistryFile_BuiltinTenOnly is the explicit regression form of the
// oneiroi-seed-naming capability's "Built-in ten remain registered with no registry file
// present" scenario: a repo that has never run `dreamland oneiroi seed` behaves exactly
// as it did before this change, and an as-yet-unseeded name is not registered.
func TestIsRegistered_NoRegistryFile_BuiltinTenOnly(t *testing.T) {
	root := t.TempDir()

	if !IsRegistered("janus", root) {
		t.Error(`"janus" should still be registered with no registry.json present`)
	}
	if IsRegistered("amber-falcon", root) {
		t.Error(`"amber-falcon" should not be registered with no registry.json present`)
	}
}

// TestIsRegistered_RegistryFileGrantsSeededName is the "Freshly seeded agent is
// immediately a registered identity" scenario: a name present only in
// .dreamland/oneiroi/registry.json (never one of the compiled-in ten) must resolve as
// registered as soon as the file exists on disk, with no binary rebuild.
func TestIsRegistered_RegistryFileGrantsSeededName(t *testing.T) {
	root := t.TempDir()
	registryDir := filepath.Join(root, ".dreamland", "oneiroi")
	if err := os.MkdirAll(registryDir, 0o755); err != nil {
		t.Fatal(err)
	}
	registryJSON := `{
		"agents": [
			{
				"name": "amber-falcon",
				"words": ["amber", "falcon"],
				"role": "example role",
				"tool_tier": "full-edit",
				"parent": null,
				"created": "2026-08-17T00:00:00Z",
				"revisions": []
			}
		]
	}`
	if err := os.WriteFile(filepath.Join(registryDir, "registry.json"), []byte(registryJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	if !IsRegistered("amber-falcon", root) {
		t.Error(`"amber-falcon" should be registered once present in registry.json`)
	}
	if !Registered(root)["amber-falcon"] {
		t.Error(`Registered(root) map should contain "amber-falcon"`)
	}
	// The built-in ten remain registered alongside the registry-file entries — additive,
	// not a replacement (decision 4 in design.md).
	if !IsRegistered("janus", root) {
		t.Error(`"janus" should remain registered alongside registry-file entries`)
	}
	// A name never seeded and not one of the ten is still unregistered.
	if IsRegistered("not-a-real-agent", root) {
		t.Error(`"not-a-real-agent" should not be registered`)
	}
}

// TestIsRegistered_RegistryFileCaseInsensitiveNameMatch mirrors the naming
// requirement's case-insensitive collision check — resolving an existing registry
// name should not be case-sensitive-brittle when matched against a hook-payload-derived
// candidate identity (which, per FromPayload's own doc comment, is used verbatim).
func TestIsRegistered_MissingRegistryFileIsNotAnError(t *testing.T) {
	// A repo root that doesn't even exist as a directory must not panic or error out of
	// IsRegistered/Registered — "no error when the registry file does not exist" per the
	// oneiroi-seed-naming capability's registry requirement.
	root := filepath.Join(t.TempDir(), "does-not-exist")

	if !IsRegistered("janus", root) {
		t.Error(`"janus" should still be registered when repoRoot doesn't exist`)
	}
	if Registered(root) == nil {
		t.Error("Registered(root) should never return a nil map")
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
		"session_id":             "abc123",
		"transcript_path":        "~/.claude/projects/.../abc123.jsonl",
		"cwd":                    "/Users/example",
		"permission_mode":        "default",
		"hook_event_name":        "SubagentStop",
		"stop_hook_active":       false,
		"agent_id":               "def456",
		"agent_type":             "phantasos",
		"agent_transcript_path":  "~/.claude/projects/.../abc123/subagents/agent-def456.jsonl",
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
