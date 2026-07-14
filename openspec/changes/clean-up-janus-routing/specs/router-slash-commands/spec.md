## ADDED Requirements

### Requirement: Legacy openspec-* skills route to the same target as their current equivalents, not around Janus

The legacy `openspec-propose`, `openspec-explore`, `openspec-apply-change`, and `openspec-archive-change` skills SHALL NOT be auto-discoverable entry points that bypass the routing this capability defines for their current equivalents. Each legacy skill SHALL be rewritten as a redirect stub that resolves to the exact same target as its `/opsx:*` counterpart:

- `openspec-propose` and `openspec-explore` → `phantasos` directly (matching `/opsx:propose`/`/opsx:explore`)
- `openspec-archive-change` → `baku` directly (matching `/opsx:archive`)
- `openspec-apply-change` → `janus`, which then chooses `nyx` or `morpheus` per task (matching `/opsx:apply`)

No legacy skill SHALL perform its own independent routing logic or duplicate a command's instructions; each stub references or reuses its `/opsx:*` counterpart's routing rather than maintaining parallel text that can drift.

#### Scenario: Legacy openspec-propose skill redirects to Phantasos like /opsx:propose

- **WHEN** the `openspec-propose` skill is invoked
- **THEN** it resolves to the same `phantasos` target that `.claude/commands/opsx/propose.md` documents, without introducing a separate routing decision

#### Scenario: Legacy openspec-apply-change skill routes through Janus like /opsx:apply

- **WHEN** the `openspec-apply-change` skill is invoked
- **THEN** it delegates to `janus`, which chooses `nyx` or `morpheus` per task exactly as it does for `/opsx:apply`, rather than picking a fixed target itself

#### Scenario: No entry point reaches the filesystem/agent layer without routing through Janus or a documented direct target

- **WHEN** any OpenSpec-lifecycle command or skill (current or legacy name) is invoked
- **THEN** it either routes through Janus or documents a single deterministic target agent, matching this capability's other requirements — no entry point silently skips both

### Requirement: Command and skill enumeration is consistent across all six platform templates

The full set of user-invocable entry points for the OpenSpec lifecycle and per-agent direct routes SHALL be present and consistent across all six supported platform templates (Claude Code, Codex CLI, Cursor, Kiro, Antigravity, GitHub Copilot) — no platform SHALL be missing an entry point, a legacy redirect, or a routing-table reference that another platform has.

#### Scenario: Per-agent commands present on every platform

- **WHEN** `dreamland init` completes successfully for any of the six supported platforms
- **THEN** the platform's equivalent of all nine per-agent direct-invoke commands (`phantasos`, `nyx`, `morpheus`, `phobetor`, `baku`, `iktomi`, `zhougong`, `hypnos`, `mengpo`) and the generic `/route` command are present

#### Scenario: Legacy redirect coverage matches across platforms wherever the platform supports skills/commands

- **WHEN** a platform supports an auto-discoverable skill mechanism equivalent to Claude Code's `.claude/skills/`
- **THEN** that platform's legacy `openspec-*` redirect stubs exist and resolve to the same targets documented for Claude Code
