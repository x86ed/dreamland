## MODIFIED Requirements

### Requirement: A generic slash command routes directly to Janus

The scaffold installer SHALL install a `/drmlnd:route` slash command (or platform equivalent) on every supported platform that invokes Janus directly, for requests not already covered by an existing `/opsx:*` command. `/drmlnd:route` is also the entry point for free-form requests that don't fit the OpenSpec-driven flow at all — Janus delegates those to `iktomi` (see the `janus-router-agent` capability).

#### Scenario: Claude Code /drmlnd:route command installed

- **WHEN** the selected coding tool is "Claude Code" and `dreamland init` completes successfully
- **THEN** `.claude/commands/drmlnd/route.md` exists and its instructions direct the invoking session to delegate to the `janus` agent

#### Scenario: Free-form request via /drmlnd:route reaches Iktomi

- **WHEN** a user invokes `/drmlnd:route` with a request that has no OpenSpec change/task context (e.g. an ad hoc coding question)
- **THEN** Janus, per the `janus-router-agent` capability, delegates to `iktomi`

#### Scenario: Cursor /drmlnd-route command installed

- **WHEN** the selected coding tool is "Cursor" and `dreamland init` completes successfully
- **THEN** `.cursor/commands/route.md` exists with frontmatter `name: drmlnd-route`, so it is invoked as `/drmlnd-route` — Cursor's command naming is flat kebab-case and does not support the colon separator Claude Code uses

### Requirement: Each non-router agent has an explicit, named direct-invoke slash command

The scaffold installer SHALL install one slash command per non-router agent — `/drmlnd:phantasos`, `/drmlnd:nyx`, `/drmlnd:morpheus`, `/drmlnd:phobetor`, `/drmlnd:baku`, `/drmlnd:iktomi`, `/drmlnd:zhougong`, `/drmlnd:hypnos`, `/drmlnd:mengpo` (or platform equivalents) — on every platform that supports user-invocable commands. Each command invokes Janus with an explicit instruction to route directly to the named agent, overriding Janus's own judgment about which agent fits the request. Unlike `/drmlnd:route`, these commands do not ask Janus to decide; they force a specific destination while still going through Janus, so the identity/telemetry/hand-off machinery (`dreamland coauthor`, `dreamland telemetry write`) applies exactly as it does for every other delegation.

This gives three tiers of entry point: the `/opsx:*` commands (OpenSpec-lifecycle-specific, either a deterministic bypass or the two-flow decision — unchanged by this capability's `drmlnd:` naming, since they are not dreamland-specific), `/drmlnd:route` (generic — Janus decides which agent), and these per-agent commands (explicit — the caller decides which agent, Janus still performs the hand-off).

#### Scenario: /drmlnd:morpheus routes directly to Morpheus via Janus

- **WHEN** a user invokes `/drmlnd:morpheus` on any platform where it is installed
- **THEN** Janus delegates to `morpheus` without first deciding whether the request fits `nyx`, `phobetor`, or any other agent
- **AND** Janus still runs its normal pre/post-delegation identity and telemetry steps for that hand-off

#### Scenario: /drmlnd:hypnos routes directly to the agent-authoring agent, not the router

- **WHEN** a user invokes `/drmlnd:hypnos` on any platform where it is installed
- **THEN** Janus delegates to `hypnos` (the agent-authoring agent), not to itself — `/drmlnd:hypnos` does not invoke the router

#### Scenario: All nine per-agent commands installed on Claude Code

- **WHEN** the selected coding tool is "Claude Code" and `dreamland init` completes successfully
- **THEN** `.claude/commands/drmlnd/phantasos.md`, `.claude/commands/drmlnd/nyx.md`, `.claude/commands/drmlnd/morpheus.md`, `.claude/commands/drmlnd/phobetor.md`, `.claude/commands/drmlnd/baku.md`, `.claude/commands/drmlnd/iktomi.md`, `.claude/commands/drmlnd/zhougong.md`, `.claude/commands/drmlnd/hypnos.md`, and `.claude/commands/drmlnd/mengpo.md` all exist, each instructing the invoking session to delegate to `janus` with an explicit "route to `<agent>`" instruction

#### Scenario: All nine per-agent commands installed on Cursor with hyphenated names

- **WHEN** the selected coding tool is "Cursor" and `dreamland init` completes successfully
- **THEN** `.cursor/commands/phantasos.md`, `.cursor/commands/nyx.md`, `.cursor/commands/morpheus.md`, `.cursor/commands/phobetor.md`, `.cursor/commands/baku.md`, `.cursor/commands/iktomi.md`, `.cursor/commands/zhougong.md`, `.cursor/commands/hypnos.md`, and `.cursor/commands/mengpo.md` all exist, each with frontmatter `name: drmlnd-<agent>` so it is invoked as `/drmlnd-<agent>` (e.g. `/drmlnd-phantasos`)

### Requirement: No unprefixed dreamland command artifacts remain after install

`dreamland init` SHALL NOT leave any unprefixed `route`/per-agent command installed with its old identifier alongside the `drmlnd`-prefixed replacement, on any supported platform, using whatever separator that platform's naming model supports (`drmlnd:` colon-namespace for Claude Code, `drmlnd-` hyphen prefix for Cursor). If a prior scaffold run left unprefixed command files/identifiers in place (e.g. `.claude/commands/route.md`, or a Cursor command file whose `name:` frontmatter is still `phantasos` instead of `drmlnd-phantasos`), a subsequent `dreamland init` run SHALL remove/replace them. `/opsx:*` command files are not affected by this requirement — they are not dreamland-specific and keep their existing names and locations.

#### Scenario: Re-running init on a repo with stale unprefixed commands cleans them up

- **WHEN** a repo contains unprefixed dreamland command files from a prior version (e.g. `.claude/commands/route.md`) and the user runs `dreamland init` again
- **THEN** the unprefixed files are removed
- **AND** only the `drmlnd:`-prefixed equivalents remain
- **AND** `.claude/commands/opsx/*.md` is left untouched

#### Scenario: Re-running init on Cursor replaces old identifiers, not just old filenames

- **WHEN** `.cursor/commands/phantasos.md` exists from a prior version (filename `phantasos.md`, no `name:` frontmatter or `name: phantasos`) and the user runs `dreamland init` again with "Cursor" selected
- **THEN** the file's content is replaced so its frontmatter reads `name: drmlnd-phantasos`, so `/phantasos` no longer resolves and `/drmlnd-phantasos` does
