// Package agentidentity is the single source of truth for the open set of registered
// dreamland agents (the ten built-ins plus any oneiroi seeded via `dreamland oneiroi
// seed`/`revise`/`fork`, per the oneiroi-seed-naming capability) and for extracting a
// sub-agent identity from a hook payload, shared between cmd/coauthor.go and the
// telemetry collectors so neither duplicates (and risks drifting from) the other's
// allow-list or extraction logic.
package agentidentity

import "dreamland/internal/oneiroi"

// builtin is the closed set of the ten dreamland agents compiled into the binary. A
// candidate identity resolved from a hook payload that isn't in the union of this set
// and the current repo's oneiroi registry (see Registered) is treated as unresolved
// rather than used verbatim.
var builtin = map[string]bool{
	"janus": true, "phantasos": true, "nyx": true, "morpheus": true,
	"phobetor": true, "baku": true, "iktomi": true, "zhougong": true,
	"hypnos": true, "mengpo": true,
}

// Registered returns the open set of registered dreamland agents: the ten built-ins
// unioned with every name recorded in repoRoot's `.dreamland/oneiroi/registry.json`, if
// present. With no registry file present (or repoRoot itself absent), this is byte-
// identical to the built-in ten's behavior before the open registry existed — never an
// error, never nil.
func Registered(repoRoot string) map[string]bool {
	result := make(map[string]bool, len(builtin))
	for name := range builtin {
		result[name] = true
	}

	reg, err := oneiroi.Load(repoRoot)
	if err != nil || reg == nil {
		return result
	}
	for _, name := range reg.Names() {
		result[name] = true
	}
	return result
}

// IsRegistered reports whether name is one of the ten built-in dreamland agents or a
// name present in repoRoot's oneiroi registry.
func IsRegistered(name, repoRoot string) bool {
	return Registered(repoRoot)[name]
}

// FromPayload extracts a sub-agent identity from a hook payload already unmarshaled
// into a generic map, checking every shape a supported platform is confirmed to emit:
//   - GitHub Copilot: top-level "agent_type" (e.g. "morpheus") on SubagentStart/SubagentStop payloads.
//   - Claude Code: top-level "agent_type" (e.g. "morpheus") on SubagentStop payloads (confirmed
//     against Anthropic's published hooks reference: SubagentStop input includes "agent_id",
//     "agent_type", "agent_transcript_path", and "last_assistant_message" in addition to the
//     common fields — an earlier assumption in this codebase that Claude Code's SubagentStop
//     payload carries no sub-agent identifier at all was wrong and has been corrected), and
//     "tool_input.subagent_type" (e.g. "morpheus") on the PreToolUse/PostToolUse payload for
//     the Task/Agent tool call itself.
//
// Returns "" if none of these shapes is present (e.g. SessionStart/Stop payloads, which carry
// neither field).
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
