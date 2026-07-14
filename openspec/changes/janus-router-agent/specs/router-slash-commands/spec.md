## ADDED Requirements

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
