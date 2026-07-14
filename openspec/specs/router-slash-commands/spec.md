# router-slash-commands

## Requirements

### Requirement: A generic slash command routes directly to Janus

The scaffold installer SHALL install a `/route` slash command (or platform equivalent) on every supported platform that invokes Janus directly, for requests not already covered by an existing `/opsx:*` command. `/route` is also the entry point for free-form requests that don't fit the OpenSpec-driven flow at all — Janus delegates those to `iktomi` (see the `janus-router-agent` capability).

#### Scenario: Claude Code /route command installed

- **WHEN** the selected coding tool is "Claude Code" and `dreamland init` completes successfully
- **THEN** `.claude/commands/route.md` exists and its instructions direct the invoking session to delegate to the `janus` agent

#### Scenario: Free-form request via /route reaches Iktomi

- **WHEN** a user invokes `/route` with a request that has no OpenSpec change/task context (e.g. an ad hoc coding question)
- **THEN** Janus, per the `janus-router-agent` capability, delegates to `iktomi`

### Requirement: Each non-router agent has an explicit, named direct-invoke slash command

The scaffold installer SHALL install one slash command per non-router agent — `/phantasos`, `/nyx`, `/morpheus`, `/phobetor`, `/baku`, `/iktomi`, `/zhougong`, `/hypnos`, `/mengpo` (or platform equivalents) — on every platform that supports user-invocable commands. Each command invokes Janus with an explicit instruction to route directly to the named agent, overriding Janus's own judgment about which agent fits the request. Unlike `/route`, these commands do not ask Janus to decide; they force a specific destination while still going through Janus, so the identity/telemetry/hand-off machinery (`dreamland coauthor`, `dreamland telemetry write`) applies exactly as it does for every other delegation.

This gives three tiers of entry point: the `/opsx:*` commands (OpenSpec-lifecycle-specific, either a deterministic bypass or the two-flow decision), `/route` (generic — Janus decides which agent), and these per-agent commands (explicit — the caller decides which agent, Janus still performs the hand-off).

#### Scenario: /morpheus routes directly to Morpheus via Janus

- **WHEN** a user invokes `/morpheus` on any platform where it is installed
- **THEN** Janus delegates to `morpheus` without first deciding whether the request fits `nyx`, `phobetor`, or any other agent
- **AND** Janus still runs its normal pre/post-delegation identity and telemetry steps for that hand-off

#### Scenario: /hypnos routes directly to the agent-authoring agent, not the router

- **WHEN** a user invokes `/hypnos` on any platform where it is installed
- **THEN** Janus delegates to `hypnos` (the agent-authoring agent), not to itself — `/hypnos` does not invoke the router

#### Scenario: All nine per-agent commands installed on Claude Code

- **WHEN** the selected coding tool is "Claude Code" and `dreamland init` completes successfully
- **THEN** `.claude/commands/phantasos.md`, `.claude/commands/nyx.md`, `.claude/commands/morpheus.md`, `.claude/commands/phobetor.md`, `.claude/commands/baku.md`, `.claude/commands/iktomi.md`, `.claude/commands/zhougong.md`, `.claude/commands/hypnos.md`, and `.claude/commands/mengpo.md` all exist, each instructing the invoking session to delegate to `janus` with an explicit "route to `<agent>`" instruction

### Requirement: OpenSpec lifecycle commands with one deterministic target route directly, bypassing Janus

`/opsx:propose`, `/opsx:explore`, and `/opsx:archive` each have exactly one possible target agent, so each SHALL document, in its own definition file, that single target and route to it directly, without going through Janus's routing decision:

- `/opsx:propose` and `/opsx:explore` → `phantasos`
- `/opsx:archive` → `baku`

#### Scenario: /opsx:propose documents its direct routing target

- **WHEN** `.claude/commands/opsx/propose.md` is read
- **THEN** it states that it routes directly to the `phantasos` agent rather than through Janus

### Requirement: /opsx:apply routes through Janus, which chooses the flow per task

Unlike `/opsx:propose`/`/opsx:explore`/`/opsx:archive`, `/opsx:apply` does not have one deterministic target agent — the next agent depends on whether the current task needs the acceptance-test flow (`nyx` first) or the direct-implementation flow (straight to `morpheus`), a per-task decision only Janus makes (see the `janus-router-agent` capability). `/opsx:apply`'s own definition file SHALL document that it routes through Janus, not directly to a fixed agent, and SHALL name both possible entry points (`nyx`, `morpheus`).

#### Scenario: /opsx:apply documents that it routes through Janus

- **WHEN** `.claude/commands/opsx/apply.md` is read
- **THEN** it states that it delegates to `janus`, which then routes the current task to either `nyx` (acceptance-test flow) or `morpheus` (direct-implementation flow)
- **AND** it does not claim a single fixed target agent

### Requirement: Janus's own routing table stays consistent with the slash command definitions

For each `/opsx:*` command with one deterministic target (`propose`, `explore`, `archive`), the routing table documented in Janus's instructions (see the `janus-router-agent` capability) SHALL name the same target agent as that command's own definition file states. For `/opsx:apply`, Janus's routing table SHALL document the same two-flow decision (`nyx` vs `morpheus`) that `/opsx:apply`'s own definition file describes.

#### Scenario: Routing table matches command definitions for a deterministic command

- **WHEN** Janus's instructions list a target agent for `/opsx:archive`
- **THEN** that target agent is `baku`, matching `/opsx:archive`'s own documented routing target

#### Scenario: Routing table matches /opsx:apply's two-flow description

- **WHEN** Janus's instructions describe how `/opsx:apply` requests are routed
- **THEN** they name the same two flows (acceptance-test via `nyx`, direct-implementation via `morpheus`) that `/opsx:apply`'s own definition file names

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
