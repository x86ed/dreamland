## Why

The Claude Code binding of dreamland was designed on the assumption that Claude Code exposes a `CLAUDE_AGENT_ID` environment variable and a GitHub-Copilot-shaped `agent_type` field on hook stdin payloads. Neither exists: Claude Code's actual Task-tool payload carries the sub-agent name as `tool_input.subagent_type`, so `agentNameFromHookPayload()` (`cmd/coauthor.go`) never matches on Claude Code and silently falls back to a generic identity. Combined with the fact that this repo never actually ran its own scaffolder on itself — there is no `.claude/agents/` directory, and `.claude/settings.json` carries none of the SessionStart/Stop/SubagentStop hooks that `internal/scaffold/templates/hooks/bindings/claude-code/settings-patch.json` already defines — every Claude Code session in this repo runs as a bare, unbranded assistant with only prose-based "pretend to be Janus" slash commands (`.claude/commands/drmlnd/*.md`) instead of real, tool-restricted sub-agents, and gets none of the automatic version-bump/telemetry/commit hooks that the GitHub Copilot binding already receives.

This produces exactly the symptoms reported: commits and telemetry attributed to a generic/unrelated identity instead of the active dreamland agent, no way to address a specific agent (`nyx`, `phobetor`, ...) as a real isolated Task-tool sub-agent, GitHub-Copilot-only automation that looks "ignored" on Claude Code, and version bumps that only ever land as `--patch` because the minor/major hooks were never installed and the change-scoped bump depends on an agent remembering a manual command.

## What Changes

- Fix Claude Code agent-identity resolution: read `tool_input.subagent_type` from the actual Claude Code `PreToolUse`/`SubagentStop` hook payload shape instead of the nonexistent `CLAUDE_AGENT_ID` env var and the Copilot-only `agent_type` key; keep the existing env-var and `agent_type` paths as fallbacks for the platforms that do provide them.
- Add a `dreamland` and per-agent (`janus`, `nyx`, `phobetor`, etc.) bare slash command per agent on Claude Code, alongside the existing `/drmlnd:*` namespaced set, so agents are directly addressable without the namespace prefix.
- Add session-start enforcement so a Claude Code session always has an explicit, valid dreamland agent identity (`janus` by default) recorded for the session, and reject/relabel telemetry or commits from an identity that isn't one of the ten registered agents.
- Make the OpenSpec-change minor version bump deterministic: fire `dreamland version-bump --change <slug>` from a real hook (`PostToolUse` on `openspec new change`, or equivalent) instead of relying on prose instructions in `phantasos.md` telling the agent to remember to run it.
- Self-host: apply the (now-fixed) scaffolder output to the dreamland repo itself — install real `.claude/agents/*.md` sub-agent definitions, merge the full Claude Code hook set from `settings-patch.json` into `.claude/settings.json` (preserving the existing `pre-merge-check.sh` Stop hook), and install the new bare commands.

## Capabilities

### New Capabilities
- `session-agent-identity`: every Claude Code session under dreamland resolves to one, and only one, of the ten registered dreamland agents (default `janus`); an unresolvable or unregistered identity is rejected rather than silently defaulting to a generic/blank identity for commits and telemetry.

### Modified Capabilities
- `dev-workflow-hooks`: agent-identity resolution for `dreamland coauthor` gains a Claude Code-native path (`tool_input.subagent_type` from the real hook payload) in place of the nonexistent `CLAUDE_AGENT_ID` env var; the OpenSpec change-scoped minor bump (`--change`) is triggered by a real hook instead of a manual/prose step.
- `router-slash-commands`: adds bare (non-`drmlnd:`-namespaced) slash commands — `/dreamland`, `/janus`, `/nyx`, `/phobetor`, and the remaining agent names — as direct-addressing aliases for the existing namespaced commands.
- `otel-commit-hook`: adds an `AI-Agent` trailer key (and corresponding `SnapshotResult.Agent` field) so commit telemetry records which of the ten dreamland agents produced the change, not only which coding tool (`AI-Tool` already exists but is always `"claude-code"` regardless of agent).

## Impact

- Code: `cmd/coauthor.go` (`agentNameFromHookPayload`, `resolveAgentName`), `cmd/version_bump.go` (`--change` trigger point), `internal/scaffold/templates/hooks/bindings/claude-code/settings-patch.json`, `internal/scaffold/templates/agents/claude-code/*.md`, `internal/scaffold/scaffold.go` (bare-command generation).
- This repository: `.claude/settings.json` gains the merged hook set; `.claude/agents/` is created; `.claude/commands/` gains bare-name commands alongside `drmlnd/`.
- Specs: `openspec/specs/dev-workflow-hooks/spec.md` and `openspec/specs/router-slash-commands/spec.md` get delta updates; `openspec/specs/session-agent-identity/spec.md` is new.
- No breaking change to the `dreamland` CLI's public command surface — `version-bump`, `coauthor`, etc. keep their existing flags.
