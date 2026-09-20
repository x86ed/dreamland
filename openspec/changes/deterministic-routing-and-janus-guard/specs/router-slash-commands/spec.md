## MODIFIED Requirements

### Requirement: A generic slash command routes directly to Janus

The scaffold installer SHALL install a `drmlnd`-prefixed generic routing command on every supported platform that invokes Janus's routing decision, for requests not already covered by an existing `/opsx:*` command. On Claude Code that decision is made in the invoking session itself: the command's instructions run `dreamland route` (see the `deterministic-routing` capability) and dispatch the returned target directly, and SHALL NOT spawn a `janus` subagent to make the decision. On every other platform the command still invokes Janus as before. This command is also the entry point for free-form requests that don't fit the OpenSpec-driven flow at all — Janus delegates those to `iktomi` (see the `janus-router-agent` capability). The prefix's separator and the invocation mechanism are platform-native:

- Claude Code: `/drmlnd:route`
- Cursor, GitHub Copilot, Kiro, Antigravity: `/drmlnd-route`
- Codex CLI: `$drmlnd-route` (or the `/skills` menu)

#### Scenario: Claude Code /drmlnd:route command installed

- **WHEN** the selected coding tool is "Claude Code" and `dreamland init` completes successfully
- **THEN** `.claude/commands/drmlnd/route.md` exists and its instructions direct the invoking session to run `dreamland route` for the request and dispatch the returned target itself, without delegating to a `janus` subagent

#### Scenario: Free-form request via the generic routing command reaches Iktomi

- **WHEN** a user invokes the generic routing command with a request that has no OpenSpec change/task context (e.g. an ad hoc coding question)
- **THEN** the routing decision (on Claude Code, `dreamland route` rule `no-openspec-context` applied in the invoking session; on other platforms, Janus per the `janus-router-agent` capability) delegates to `iktomi`

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

The scaffold installer SHALL install one routing command per non-router agent — every one of the fixed nine (`phantasos`, `nyx`, `morpheus`, `phobetor`, `baku`, `iktomi`, `zhougong`, `hypnos`, `mengpo`) and every agent seeded via `dreamland oneiroi seed`/`fork` (see the `oneiroi-seed-naming` capability) — on every supported platform, each carrying the `drmlnd` prefix (platform-native separator, per the table in the generic-routing-command requirement above). Each command forces a specific destination, overriding any judgment about which agent fits the request. On Claude Code the command's instructions direct the invoking session to dispatch the named agent itself with the `Agent` tool, forwarding the request verbatim; they SHALL NOT spawn a `janus` subagent as an intermediary, and no routing decision is made at all (the destination is already fixed by the command). The identity/telemetry/hand-off machinery (`dreamland coauthor` via the `PreToolUse` hook, `dreamland telemetry write` via the `SubagentStop` hook) applies exactly as it does for every other dispatch. On every other platform each command still invokes Janus with an explicit instruction to route directly to the named agent, so the machinery applies through Janus as before. A seeded oneiroi's slash command is written by `dreamland oneiroi seed`/`fork` itself, using the same per-platform template/target-path conventions the scaffold installer uses for the fixed nine (see the `agent-scaffolding` capability's generalized-installer requirement) — not a separate mechanism.

This gives three tiers of entry point: the `/opsx:*` commands (OpenSpec-lifecycle-specific, either a deterministic bypass or the two-flow decision — unchanged by this capability's `drmlnd` naming, since they are not dreamland-specific), the generic routing command (Janus decides which agent), and these per-agent commands (explicit — the caller decides which agent; on Claude Code the invoking session performs the dispatch itself, elsewhere Janus still performs the hand-off).

#### Scenario: /drmlnd:morpheus dispatches Morpheus directly on Claude Code

- **WHEN** a user invokes `/drmlnd:morpheus` on Claude Code
- **THEN** the invoking session dispatches `morpheus` with the `Agent` tool without first deciding whether the request fits `nyx`, `phobetor`, or any other agent, and without spawning a `janus` subagent
- **AND** the workspace-level `PreToolUse` (`coauthor`) and `SubagentStop` (`telemetry write`, `commit --reason handoff --hook`) hooks run for that dispatch exactly as for any other

#### Scenario: /drmlnd:hypnos routes directly to the agent-authoring agent, not the router

- **WHEN** a user invokes `/drmlnd:hypnos` on any platform where it is installed
- **THEN** `hypnos` (the agent-authoring agent) is dispatched — by the invoking session on Claude Code, by Janus elsewhere — and the router is not invoked to decide

#### Scenario: All nine per-agent commands installed on Claude Code

- **WHEN** the selected coding tool is "Claude Code" and `dreamland init` completes successfully
- **THEN** `.claude/commands/drmlnd/phantasos.md`, `.claude/commands/drmlnd/nyx.md`, `.claude/commands/drmlnd/morpheus.md`, `.claude/commands/drmlnd/phobetor.md`, `.claude/commands/drmlnd/baku.md`, `.claude/commands/drmlnd/iktomi.md`, `.claude/commands/drmlnd/zhougong.md`, `.claude/commands/drmlnd/hypnos.md`, and `.claude/commands/drmlnd/mengpo.md` all exist, each instructing the invoking session to dispatch that agent directly with the `Agent` tool and not to delegate to `janus`

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

### Requirement: /opsx:apply routes through Janus, which chooses the flow per task

Unlike `/opsx:propose`/`/opsx:explore`/`/opsx:archive`, `/opsx:apply` does not have one deterministic target agent — the next agent depends on whether the current task needs the acceptance-test flow (`nyx` first) or the direct-implementation flow (straight to `morpheus`), a per-task decision only the routing decision makes (see the `janus-router-agent` and `deterministic-routing` capabilities). `/opsx:apply`'s own definition file SHALL document that it routes through the routing decision, not directly to a fixed agent, and SHALL name both possible entry points (`nyx`, `morpheus`). On Claude Code that decision is `dreamland route --command opsx:apply` run in the invoking session (task-flow tag, else `ambiguous` resolved in-session — no `janus` subagent is spawned); on other platforms it remains Janus's own judgment.

#### Scenario: /opsx:apply documents that it routes through Janus

- **WHEN** `.claude/commands/opsx/apply.md` is read
- **THEN** it states that it resolves the routing decision (on Claude Code via `dreamland route --command opsx:apply` in the invoking session, not a spawned `janus` subagent) which routes the current task to either `nyx` (acceptance-test flow) or `morpheus` (direct-implementation flow), or to `hypnos`/`mengpo` for a roster or workflow-graph task
- **AND** it does not claim a single fixed target agent

### Requirement: Legacy openspec-* skills route to the same target as their current equivalents, not around Janus

The legacy `openspec-propose`, `openspec-explore`, `openspec-apply-change`, and `openspec-archive-change` skills SHALL NOT be auto-discoverable entry points that bypass the routing this capability defines for their current equivalents. Each legacy skill SHALL be rewritten as a redirect stub that resolves to the exact same target as its `/opsx:*` counterpart:

- `openspec-propose` and `openspec-explore` → `phantasos` directly (matching `/opsx:propose`/`/opsx:explore`)
- `openspec-archive-change` → `baku` directly (matching `/opsx:archive`)
- `openspec-apply-change` → the same routing decision as `/opsx:apply` (on Claude Code, `dreamland route --command openspec-apply-change` resolved in-session; on other platforms `janus`), which then chooses `nyx` or `morpheus` per task

No legacy skill SHALL perform its own independent routing logic or duplicate a command's instructions; each stub references or reuses its `/opsx:*` counterpart's routing rather than maintaining parallel text that can drift.

#### Scenario: Legacy openspec-propose skill redirects to Phantasos like /opsx:propose

- **WHEN** the `openspec-propose` skill is invoked
- **THEN** it resolves to the same `phantasos` target that `.claude/commands/opsx/propose.md` documents, without introducing a separate routing decision

#### Scenario: Legacy openspec-apply-change skill routes through Janus like /opsx:apply

- **WHEN** the `openspec-apply-change` skill is invoked
- **THEN** it resolves the same routing decision `/opsx:apply` does, which chooses `nyx` or `morpheus` per task, rather than picking a fixed target itself

#### Scenario: No entry point reaches the filesystem/agent layer without routing through Janus or a documented direct target

- **WHEN** any OpenSpec-lifecycle command or skill (current or legacy name) is invoked
- **THEN** it either routes through Janus or documents a single deterministic target agent, matching this capability's other requirements — no entry point silently skips both
