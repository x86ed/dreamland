## MODIFIED Requirements

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

The scaffold installer SHALL install one routing command per non-router agent — `phantasos`, `nyx`, `morpheus`, `phobetor`, `baku`, `iktomi`, `zhougong`, `hypnos`, `mengpo` — on every supported platform, each carrying the `drmlnd` prefix (platform-native separator, per the table in the generic-routing-command requirement above). Each command invokes Janus with an explicit instruction to route directly to the named agent, overriding Janus's own judgment about which agent fits the request. Unlike the generic routing command, these commands do not ask Janus to decide; they force a specific destination while still going through Janus, so the identity/telemetry/hand-off machinery (`dreamland coauthor`, `dreamland telemetry write`) applies exactly as it does for every other delegation.

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

### Requirement: No unprefixed dreamland command artifacts remain after install

`dreamland init` SHALL NOT leave any unprefixed `route`/per-agent command installed with its old identifier alongside the `drmlnd`-prefixed replacement, on any supported platform. If a prior scaffold run left unprefixed Cursor command files/identifiers in place (e.g. a command file whose `name:` frontmatter is still `phantasos` instead of `drmlnd-phantasos`), a subsequent `dreamland init` run SHALL remove/replace them. The other five platforms have no prior unprefixed installs to clean up, since command installation on those platforms is new as of this change. `/opsx:*` command files are not affected by this requirement — they are not dreamland-specific and keep their existing names and locations.

#### Scenario: Re-running init on Cursor replaces old identifiers, not just old filenames

- **WHEN** `.cursor/commands/phantasos.md` exists from a prior version (filename `phantasos.md`, no `name:` frontmatter or `name: phantasos`) and the user runs `dreamland init` again with "Cursor" selected
- **THEN** the file's content is replaced so its frontmatter reads `name: drmlnd-phantasos`, so `/phantasos` no longer resolves and `/drmlnd-phantasos` does
- **AND** `.claude/commands/opsx/*.md` is left untouched
