# router-slash-commands
## Requirements
### Requirement: A generic slash command routes directly to Janus

The scaffold installer SHALL install a `drmlnd`-prefixed generic routing command on every supported platform that invokes Janus directly, for requests not already covered by an existing `/opsx:*` command. This command is also the entry point for free-form requests that don't fit the OpenSpec-driven flow at all — Janus delegates those to `iktomi` (see the `janus-router-agent` capability). The prefix's separator and the invocation mechanism are platform-native:

- Claude Code: `/drmlnd:route`
- Cursor, GitHub Copilot, Kiro, Antigravity: `/drmlnd-route`
- Codex CLI: `$drmlnd-route` (or the `/skills` menu)

#### Scenario: Claude Code /drmlnd:route command installed

- **WHEN** the selected coding tool is "Claude Code" and `dreamland init` completes successfully
- **THEN** `.claude/commands/drmlnd/route.md` exists and its instructions direct the invoking session to delegate to the `janus` agent

#### Scenario: Free-form request via the generic routing command reaches Iktomi

- **WHEN** a user invokes the generic routing command with a request that has no OpenSpec change/task context (e.g. an ad hoc coding question)
- **THEN** Janus, per the `janus-router-agent` capability, delegates to `iktomi`

#### Scenario: Cursor /drmlnd-route command installed

- **WHEN** the selected coding tool is "Cursor" and `dreamland init` completes successfully
- **THEN** `.cursor/commands/route.md` exists with frontmatter `name: drmlnd-route`, so it is invoked as `/drmlnd-route`

#### Scenario: GitHub Copilot /drmlnd-route command installed

- **WHEN** the selected coding tool is "GitHub Copilot" and `dreamland init` completes successfully
- **THEN** `.github/prompts/route.prompt.md` exists with frontmatter `name: drmlnd-route` and `agent: janus`

#### Scenario: Kiro /drmlnd-route command installed without colliding with an agent file

- **WHEN** the selected coding tool is "Kiro" and `dreamland init` completes successfully
- **THEN** `.kiro/steering/drmlnd-route.md` exists with frontmatter `inclusion: manual`, so it appears as a slash command
- **AND** it does not overwrite or collide with any agent persona steering file in the same directory

#### Scenario: Antigravity /drmlnd-route command installed

- **WHEN** the selected coding tool is "Antigravity" and `dreamland init` completes successfully
- **THEN** `.agents/skills/drmlnd-route.md` exists as a flat file (not a directory), distinct from the directory-per-skill agent personas already installed in `.agents/skills/`

#### Scenario: Codex CLI drmlnd-route skill installed

- **WHEN** the selected coding tool is "Codex CLI" and `dreamland init` completes successfully
- **THEN** `.codex/skills/drmlnd-route/SKILL.md` exists with frontmatter `name: drmlnd-route`

### Requirement: Each non-router agent has an explicit, named direct-invoke slash command

The scaffold installer SHALL install one routing command per non-router agent — every one of the fixed nine (`phantasos`, `nyx`, `morpheus`, `phobetor`, `baku`, `iktomi`, `zhougong`, `hypnos`, `mengpo`) and every agent seeded via `dreamland oneiroi seed`/`fork` (see the `oneiroi-seed-naming` capability) — on every supported platform, each carrying the `drmlnd` prefix (platform-native separator, per the table in the generic-routing-command requirement above). Each command invokes Janus with an explicit instruction to route directly to the named agent, overriding Janus's own judgment about which agent fits the request. Unlike the generic routing command, these commands do not ask Janus to decide; they force a specific destination while still going through Janus, so the identity/telemetry/hand-off machinery (`dreamland coauthor`, `dreamland telemetry write`) applies exactly as it does for every other delegation. A seeded oneiroi's slash command is written by `dreamland oneiroi seed`/`fork` itself, using the same per-platform template/target-path conventions the scaffold installer uses for the fixed nine (see the `agent-scaffolding` capability's generalized-installer requirement) — not a separate mechanism.

This gives three tiers of entry point: the `/opsx:*` commands (OpenSpec-lifecycle-specific, either a deterministic bypass or the two-flow decision — unchanged by this capability's `drmlnd` naming, since they are not dreamland-specific), the generic routing command (Janus decides which agent), and these per-agent commands (explicit — the caller decides which agent, Janus still performs the hand-off).

#### Scenario: /drmlnd:morpheus routes directly to Morpheus via Janus on Claude Code

- **WHEN** a user invokes `/drmlnd:morpheus` on Claude Code
- **THEN** Janus delegates to `morpheus` without first deciding whether the request fits `nyx`, `phobetor`, or any other agent
- **AND** Janus still runs its normal pre/post-delegation identity and telemetry steps for that hand-off

#### Scenario: /drmlnd:hypnos routes directly to the agent-authoring agent, not the router

- **WHEN** a user invokes `/drmlnd:hypnos` on any platform where it is installed
- **THEN** Janus delegates to `hypnos` (the agent-authoring agent), not to itself — it does not invoke the router

#### Scenario: All nine per-agent commands installed on Claude Code

- **WHEN** the selected coding tool is "Claude Code" and `dreamland init` completes successfully
- **THEN** `.claude/commands/drmlnd/phantasos.md`, `.claude/commands/drmlnd/nyx.md`, `.claude/commands/drmlnd/morpheus.md`, `.claude/commands/drmlnd/phobetor.md`, `.claude/commands/drmlnd/baku.md`, `.claude/commands/drmlnd/iktomi.md`, `.claude/commands/drmlnd/zhougong.md`, `.claude/commands/drmlnd/hypnos.md`, and `.claude/commands/drmlnd/mengpo.md` all exist, each instructing the invoking session to delegate to `janus` with an explicit "route to `<agent>`" instruction

#### Scenario: All nine per-agent commands installed on Cursor with hyphenated names

- **WHEN** the selected coding tool is "Cursor" and `dreamland init` completes successfully
- **THEN** `.cursor/commands/phantasos.md`, `.cursor/commands/nyx.md`, `.cursor/commands/morpheus.md`, `.cursor/commands/phobetor.md`, `.cursor/commands/baku.md`, `.cursor/commands/iktomi.md`, `.cursor/commands/zhougong.md`, `.cursor/commands/hypnos.md`, and `.cursor/commands/mengpo.md` all exist, each with frontmatter `name: drmlnd-<agent>`

#### Scenario: All nine per-agent commands installed on GitHub Copilot as prompt files

- **WHEN** the selected coding tool is "GitHub Copilot" and `dreamland init` completes successfully
- **THEN** `.github/prompts/<agent>.prompt.md` exists for each of the nine agents, each with frontmatter `name: drmlnd-<agent>` and `agent: janus`

#### Scenario: All nine per-agent commands installed on Kiro without colliding with agent files

- **WHEN** the selected coding tool is "Kiro" and `dreamland init` completes successfully
- **THEN** `.kiro/steering/drmlnd-<agent>.md` exists for each of the nine agents, each with frontmatter `inclusion: manual`
- **AND** none of them overwrites the always-on agent persona file at `.kiro/steering/<agent>.md`

#### Scenario: All nine per-agent commands installed on Antigravity as flat skill files

- **WHEN** the selected coding tool is "Antigravity" and `dreamland init` completes successfully
- **THEN** `.agents/skills/drmlnd-<agent>.md` exists for each of the nine agents as a flat file
- **AND** the corresponding agent persona directory `.agents/skills/<agent>/SKILL.md` still exists, untouched

#### Scenario: All nine per-agent commands installed on Codex CLI as project skills

- **WHEN** the selected coding tool is "Codex CLI" and `dreamland init` completes successfully
- **THEN** `.codex/skills/drmlnd-<agent>/SKILL.md` exists for each of the nine agents

#### Scenario: A seeded oneiroi's slash command is installed at seed time, not at the next dreamland init

- **WHEN** `dreamland oneiroi seed --role "example role"` generates the name `amber-falcon` on a repository already scaffolded for Claude Code
- **THEN** `.claude/commands/drmlnd/amber-falcon.md` exists immediately after the seed command completes, following the same `drmlnd:<agent>` convention as the fixed nine, without requiring a subsequent `dreamland init` run

#### Scenario: A forked oneiroi gets its own slash command, distinct from its parent's

- **WHEN** `dreamland oneiroi fork --agent amber-falcon --role "variant role"` generates `amber-falcon-onyx`
- **THEN** `.claude/commands/drmlnd/amber-falcon-onyx.md` exists as a new file, and `.claude/commands/drmlnd/amber-falcon.md` (the parent's) is unmodified

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
- **THEN** the platform's equivalent of all nine per-agent direct-invoke commands (`phantasos`, `nyx`, `morpheus`, `phobetor`, `baku`, `iktomi`, `zhougong`, `hypnos`, `mengpo`) and the generic routing command are present, each carrying the `drmlnd` prefix in that platform's native form (`drmlnd:` colon-namespace on Claude Code, `drmlnd-` hyphen prefix everywhere else)

#### Scenario: Legacy redirect coverage matches across platforms wherever the platform supports skills/commands

- **WHEN** a platform supports an auto-discoverable skill mechanism equivalent to Claude Code's `.claude/skills/`
- **THEN** that platform's legacy `openspec-*` redirect stubs exist and resolve to the same targets documented for Claude Code

### Requirement: No unprefixed dreamland command artifacts remain after install

`dreamland init` SHALL NOT leave any unprefixed `route`/per-agent command installed with its old identifier alongside the `drmlnd`-prefixed replacement, on any supported platform. If a prior scaffold run left unprefixed Cursor command files/identifiers in place (e.g. a command file whose `name:` frontmatter is still `phantasos` instead of `drmlnd-phantasos`), a subsequent `dreamland init` run SHALL remove/replace them. The other five platforms have no prior unprefixed installs to clean up, since command installation on those platforms is new as of this change. `/opsx:*` command files are not affected by this requirement — they are not dreamland-specific and keep their existing names and locations.

#### Scenario: Re-running init on Cursor replaces old identifiers, not just old filenames

- **WHEN** `.cursor/commands/phantasos.md` exists from a prior version (filename `phantasos.md`, no `name:` frontmatter or `name: phantasos`) and the user runs `dreamland init` again with "Cursor" selected
- **THEN** the file's content is replaced so its frontmatter reads `name: drmlnd-phantasos`, so `/phantasos` no longer resolves and `/drmlnd-phantasos` does
- **AND** `.claude/commands/opsx/*.md` is left untouched

