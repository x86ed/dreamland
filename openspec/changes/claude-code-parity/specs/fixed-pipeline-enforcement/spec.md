## ADDED Requirements

### Requirement: A PreToolUse write-guard enforces fixed artifact ownership during a dispatched agent's own turn

`dreamland init` SHALL register a `PreToolUse` hook (matcher `Write|Edit`) in `.claude/settings.json` running `dreamland guard-artifact`, which reads the hook's JSON payload from stdin and enforces a fixed path → agent ownership table:

| Path pattern | Sole permitted writer |
| --- | --- |
| `openspec/changes/**/proposal.md`, `openspec/changes/**/design.md`, `openspec/changes/**/tasks.md`, `openspec/changes/**/specs/**/*.md`, `openspec/specs/**/spec.md` | `phantasos` |
| `internal/scaffold/templates/agents/**`, `internal/scaffold/templates/commands/**` (all six platform template trees) | `hypnos` |
| `.dreamland/reports/*.md` | `zhougong` |

When the payload's `tool_input.file_path` matches a protected pattern and `agent_type` is present but does not equal that pattern's declared owner, `dreamland guard-artifact` SHALL write a reason to stderr and exit with status 2, causing Claude Code to block the tool call. In every other case — an unprotected path, a protected path written by its correct owner, or `agent_type` absent from the payload entirely — it SHALL exit 0 and allow the write.

This requirement exists because GitHub Copilot's structural `agents:` dispatch-allowlist (see the `agent-scaffolding` capability) has no working equivalent on Claude Code: a hand-off relayed by the main thread after a subagent's turn ends reports the *session's* `agent_type`, not the completing subagent's, so a dispatch-time check can never distinguish "Janus's own first dispatch" from "Janus relaying Nyx's recommended hand-off" — both look identical, and Janus can legitimately reach every agent. The enforcement point this requirement uses instead — a hook firing during a subagent's own tool call — is the one place `agent_type` is reliably scoped to the agent that's actually about to act, because subagent identity takes precedence over session identity for any hook firing inside that subagent's own turn.

**Fail-open on unknown identity is deliberate, not a gap being ignored elsewhere.** A payload with no `agent_type` at all (a main-thread-direct tool call with no subagent dispatched) is allowed through unconditionally — there is no identity to check, and no mechanism in this capability closes that gap. This is a known, accepted limitation: enforcement covers "a dispatched agent writing outside its role, mid-turn" and does not cover "no one was dispatched at all." No change to any agent's `tools:` frontmatter or to Claude Code's main-thread configuration is made to close it.

**Bash-mediated file mutation is out of scope.** `mengpo` retires agents via `Bash rm`/`git rm`, not `Edit`/`Write` — a `Write|Edit`-scoped hook never observes these calls. Parsing shell command text to detect and gate file-path targets was considered and rejected as too fragile to trust for a blocking check.

#### Scenario: A non-owning agent's write to a protected path is blocked

- **WHEN** an agent with `agent_type` `morpheus` calls `Write` or `Edit` on a path matching `openspec/changes/**/proposal.md`
- **THEN** `dreamland guard-artifact` exits 2 with a non-empty stderr reason, and Claude Code blocks the write

#### Scenario: The owning agent's write to its own protected path succeeds

- **WHEN** an agent with `agent_type` `phantasos` calls `Write` or `Edit` on a path matching `openspec/changes/**/proposal.md`
- **THEN** `dreamland guard-artifact` exits 0 and the write proceeds

#### Scenario: Writes to unprotected paths are never blocked, regardless of agent

- **WHEN** any agent (dreamland or Claude Code built-in) calls `Write` or `Edit` on a path that matches none of the protected patterns
- **THEN** `dreamland guard-artifact` exits 0

#### Scenario: A main-thread-direct write with no active subagent is not blocked

- **WHEN** `dreamland guard-artifact` receives a hook payload with no `agent_type` field, for a `Write`/`Edit` call targeting a protected path
- **THEN** it exits 0 — this is the accepted coverage gap, not an error condition

#### Scenario: Janus's own permissions are unchanged by this requirement

- **WHEN** `dreamland init` completes with "Claude Code" selected
- **THEN** `.claude/agents/janus.md`'s `tools:` frontmatter is exactly `Read, Bash`, unchanged from before this capability existed, and `.claude/settings.json` contains no `"agent"` top-level key
