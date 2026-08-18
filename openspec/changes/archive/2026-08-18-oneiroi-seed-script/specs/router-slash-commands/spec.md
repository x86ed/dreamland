## MODIFIED Requirements

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
