## ADDED Requirements

### Requirement: Every session has an explicit dreamland agent identity, defaulting to Janus

A dreamland-scaffolded session SHALL always have an explicit, resolved agent identity recorded for git/telemetry purposes — one of the ten registered dreamland agents (`janus`, `phantasos`, `nyx`, `morpheus`, `phobetor`, `baku`, `iktomi`, `zhougong`, `hypnos`, `mengpo`). If no sub-agent has been dispatched yet in the current session (the common case for a session that starts as plain conversation rather than an immediate `/opsx:*` or `/drmlnd:*` command), the identity defaults to `janus`, the router agent — never a generic/blank/coding-tool-name identity and never left unset.

The `SessionStart` hook binding SHALL run `dreamland coauthor` (per the `dev-workflow-hooks` capability), which performs this resolution and writes the result to git-local identity (`user.name`/`user.email`) immediately, before any commit or telemetry write can occur for the session.

#### Scenario: Fresh session identity defaults to janus before any sub-agent dispatch

- **WHEN** a new Claude Code session starts under a dreamland-scaffolded repo and no `Task`/`Agent` tool call has yet occurred
- **THEN** the session's git identity (`user.name`) is `"janus"`, not the coding-tool name or a blank value

#### Scenario: Identity updates once a named sub-agent is dispatched

- **WHEN** the session's first `Task`/`Agent` tool call dispatches to `nyx`
- **THEN** the session's git identity updates to `"nyx"` per the `PreToolUse` hook, per the `dev-workflow-hooks` capability's Claude Code handoff binding

### Requirement: An unrecognized identity value never propagates into commits or telemetry

If any resolution path (hook payload, env var, or otherwise) surfaces a candidate agent identity that is not one of the ten registered dreamland agents, that value SHALL be treated as unresolved, not used verbatim — it SHALL NOT be written to `git config user.name`/`user.email` or to a telemetry snapshot's tool/agent field. The identity instead falls back to `janus`.

This directly addresses commits/telemetry being attributed to a stray or unrelated identity: the set of values that can ever reach `git config user.name` is closed to the ten registered names plus the documented coding-tool-name fallback (used only when no hook-based resolution is available at all, e.g. very first invocation before any payload has been read) — an arbitrary string from a malformed or unexpected payload can never reach it.

#### Scenario: Malformed payload value does not become the git identity

- **WHEN** a hook payload's identity field contains a value that is not one of the ten registered agent names (e.g. empty string, unrelated text, or a value from an unrecognized platform-specific field)
- **THEN** `git config user.name` is set to `"janus"`, not the unrecognized value

#### Scenario: Telemetry snapshot never records an unregistered agent name

- **WHEN** `dreamland telemetry write` runs and the resolved identity for the session is `"janus"` due to a fallback per the above scenario
- **THEN** the resulting snapshot's agent-identifying field records `"janus"`, never the discarded unrecognized value
