// Package agentidentity is the single source of truth for the closed set of ten
// registered dreamland agents and for extracting a sub-agent identity from a hook
// payload, shared between cmd/coauthor.go and the telemetry collectors so neither
// duplicates (and risks drifting from) the other's allow-list or extraction logic.
package agentidentity

// Registered is the closed set of the ten dreamland agents. A candidate identity
// resolved from a hook payload that isn't in this set is treated as unresolved
// rather than used verbatim.
var Registered = map[string]bool{
	"janus": true, "phantasos": true, "nyx": true, "morpheus": true,
	"phobetor": true, "baku": true, "iktomi": true, "zhougong": true,
	"hypnos": true, "mengpo": true,
}

// IsRegistered reports whether name is one of the ten registered dreamland agents.
func IsRegistered(name string) bool {
	return Registered[name]
}

// FromPayload extracts a sub-agent identity from a hook payload already unmarshaled
// into a generic map, checking every shape a supported platform is confirmed to emit:
//   - GitHub Copilot: top-level "agent_type" (e.g. "morpheus") on SubagentStart/SubagentStop payloads.
//   - Claude Code: "tool_input.subagent_type" (e.g. "morpheus") on the PreToolUse/PostToolUse
//     payload for the Task/Agent tool call — Claude Code emits no top-level "agent_type".
//
// Returns "" if neither shape is present.
func FromPayload(payload map[string]any) string {
	if v, ok := payload["agent_type"].(string); ok && v != "" {
		return v
	}
	if toolInput, ok := payload["tool_input"].(map[string]any); ok {
		if v, ok := toolInput["subagent_type"].(string); ok && v != "" {
			return v
		}
	}
	return ""
}
