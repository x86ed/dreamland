## MODIFIED Requirements

### Requirement: Ten agent definition files are installed per platform

After `dreamland init` completes, the scaffold installer SHALL write ten agent definition files into the repository (or user-level plugin directory for Antigravity) using the platform's native path and format.

The ten agents are:

- **janus** — routes tasks and coordinates the other agents (router-only; see the `janus-router-agent` capability for its restricted tool bindings and routing table)
- **phantasos** (Oneiroi, shaper of imagined forms) — drafts and refines spec files
- **nyx** (primordial goddess of Night, mother of Hypnos) — writes the acceptance test for a task before implementation begins, when the task warrants one (TDD red phase; see the `janus-router-agent` capability for when Janus routes through Nyx vs. straight to Morpheus)
- **morpheus** (Oneiroi, shaper of human-form dreams) — writes production code to make the task's test pass, or to implement a task directly when there is no acceptance test (TDD green phase)
- **phobetor** (Oneiroi, bringer of nightmares) — runs the full test suite and validates the implementation against spec scenarios
- **baku** (獏, the dream-eating spirit) — archives/syncs the spec and opens the pull request
- **iktomi** (Lakota trickster spider spirit) — free-form coding agent for requests that don't fit the OpenSpec-driven flow; selected by Janus when no specialized agent matches the task
- **zhougong** (周公, Duke of Zhou — in Chinese folklore, dreams are traditionally interpreted by consulting him; "to visit the Duke of Zhou" is an idiom for sleeping) — analyzes git history, agent/token usage, and turn duration to generate reports and tuning recommendations for the other agents (see the `agent-lifecycle-management` capability)
- **hypnos** (Greek god of sleep, father of the Oneiroi) — authors new agent definitions across all platform templates, from `zhougong`'s reports or a direct request, and registers them with Janus's routing table
- **mengpo** (孟婆, the goddess who serves the Broth of Forgetting at the Bridge of Forgetfulness in Chinese folklore) — archives or deletes agent definitions that are no longer needed for the project

#### Scenario: Claude Code agents installed

- **WHEN** the selected coding tool is "Claude Code" and `dreamland init` completes successfully
- **THEN** ten `.md` files are written to `.claude/agents/` (`janus.md`, `phantasos.md`, `nyx.md`, `morpheus.md`, `phobetor.md`, `baku.md`, `iktomi.md`, `zhougong.md`, `hypnos.md`, `mengpo.md`), each containing valid Claude Code agent YAML frontmatter (`name`, `description`, `tools`) and a role-specific system prompt

#### Scenario: Codex CLI agents installed

- **WHEN** the selected coding tool is "Codex CLI" and `dreamland init` completes successfully
- **THEN** ten `.toml` files are written to `.codex/agents/` (`janus.toml`, `phantasos.toml`, `nyx.toml`, `morpheus.toml`, `phobetor.toml`, `baku.toml`, `iktomi.toml`, `zhougong.toml`, `hypnos.toml`, `mengpo.toml`), each containing the required fields `name`, `description`, and `developer_instructions`

#### Scenario: Cursor agents installed

- **WHEN** the selected coding tool is "Cursor" and `dreamland init` completes successfully
- **THEN** ten `.mdc` files are written to `.cursor/rules/` (`janus.mdc`, `phantasos.mdc`, `nyx.mdc`, `morpheus.mdc`, `phobetor.mdc`, `baku.mdc`, `iktomi.mdc`, `zhougong.mdc`, `hypnos.mdc`, `mengpo.mdc`), each containing YAML frontmatter with `description` and `alwaysApply: false`, followed by role-specific instructions

#### Scenario: Kiro agents installed

- **WHEN** the selected coding tool is "Kiro" and `dreamland init` completes successfully
- **THEN** ten `.md` files are written to `.kiro/steering/` (`janus.md`, `phantasos.md`, `nyx.md`, `morpheus.md`, `phobetor.md`, `baku.md`, `iktomi.md`, `zhougong.md`, `hypnos.md`, `mengpo.md`) as plain markdown steering documents

#### Scenario: Antigravity agents installed

- **WHEN** the selected coding tool is "Antigravity" and `dreamland init` completes successfully
- **THEN** ten skill directories are created at `.agents/skills/<name>/` in the repo root (one per agent: `janus`, `phantasos`, `nyx`, `morpheus`, `phobetor`, `baku`, `iktomi`, `zhougong`, `hypnos`, `mengpo`), each containing a `SKILL.md` file with YAML frontmatter (`name`, `description`) and a markdown body of agent instructions
- **AND** the `SKILL.md` files are auto-discovered by Antigravity via the project-scoped `.agents/skills/` convention (no plugin install step required)

#### Scenario: GitHub Copilot agents installed

- **WHEN** the selected coding tool is "GitHub Copilot" and `dreamland init` completes successfully
- **THEN** ten `.agent.md` files are written to `.github/agents/` (`janus.agent.md`, `phantasos.agent.md`, `nyx.agent.md`, `morpheus.agent.md`, `phobetor.agent.md`, `baku.agent.md`, `iktomi.agent.md`, `zhougong.agent.md`, `hypnos.agent.md`, `mengpo.agent.md`), each with YAML frontmatter (`name`, `description`, `tools`, `agents`, `hooks` — see the "GitHub Copilot frontmatter declares its subagent routing graph and hooks in the header" requirement below)

### Requirement: Existing agent files are not overwritten by default

If any agent file already exists at its target path, the installer SHALL skip that file and report it as skipped to stdout.

#### Scenario: Agent file already exists, no force flag

- **WHEN** `.claude/agents/janus.md` already exists and `dreamland init` is run without `--force`
- **THEN** the existing file is not modified and stdout includes `skipped (already exists): .claude/agents/janus.md`

#### Scenario: Agent file overwritten with force flag

- **WHEN** `.claude/agents/janus.md` already exists and `dreamland init` is run with `--force`
- **THEN** the file is overwritten with the current template content and stdout shows `installed (forced): .claude/agents/janus.md`

#### Scenario: Pre-existing orchestrator file is left untouched

- **WHEN** a repository already contains `.claude/agents/orchestrator.md` from a prior version of `dreamland init`, and `dreamland init` is run with the current version
- **THEN** `.claude/agents/orchestrator.md` is not modified or deleted
- **AND** `.claude/agents/janus.md` is installed alongside it (or reported skipped if it also already exists)

## ADDED Requirements

### Requirement: Each scaffolded agent's tool bindings are scoped to its role

On every platform that exposes a tool/capability list in agent frontmatter (Claude Code, GitHub Copilot: `tools:`; Codex: capability keys), the installed tool bindings SHALL follow this fixed per-role matrix, with three tiers:

| Agent | Tier | `Edit` | `Write` | `Read`, `Bash` |
| --- | --- | --- | --- | --- |
| `janus` | Router (read/dispatch only) | No | No | Yes |
| `phobetor` | Read/dispatch only | No | No | Yes |
| `baku` | Read/dispatch only | No | No | Yes |
| `phantasos` | Full edit | Yes | Yes | Yes |
| `nyx` | Full edit | Yes | Yes | Yes |
| `morpheus` | Full edit | Yes | Yes | Yes |
| `iktomi` | Full edit | Yes | Yes | Yes |
| `hypnos` | Full edit | Yes | Yes | Yes |
| `zhougong` | Write-only (new files) | No | Yes | Yes |
| `mengpo` | Write-only (new files) | No | Yes | Yes |

`phantasos`, `nyx`, `morpheus`, `iktomi`, and `hypnos` create or modify existing specs/tests/code/agent-templates, so they are granted full `Edit`+`Write` (plus `apply_patch` on Codex). `janus`, `phobetor`, and `baku` are read-only/dispatch-only by role (routing, verification, and PR/git operations respectively) and MUST NOT be granted any file-editing tool. `zhougong` and `mengpo` sit in a third tier: each is granted `Write` (to author a report file or an archive-manifest entry) but explicitly NOT `Edit` — neither agent's job is to make targeted modifications to another agent's existing file content; `zhougong` only ever produces new report documents, and `mengpo`'s file removal/relocation is done via `Bash`, not `Edit`.

#### Scenario: Nyx retains file-editing tools

- **WHEN** `.claude/agents/nyx.md` is installed
- **THEN** its `tools` frontmatter field includes `Edit` and `Write`

#### Scenario: Phobetor has no file-editing tools on Claude Code

- **WHEN** `.claude/agents/phobetor.md` is installed
- **THEN** its `tools` frontmatter field is `Read, Bash` and does not include `Edit` or `Write`

#### Scenario: Baku has no file-editing tools on Codex

- **WHEN** `.codex/agents/baku.toml` is installed
- **THEN** it does not grant `apply_patch` capability

#### Scenario: Morpheus retains file-editing tools

- **WHEN** `.claude/agents/morpheus.md` is installed
- **THEN** its `tools` frontmatter field includes `Edit` and `Write`

#### Scenario: Iktomi retains file-editing tools

- **WHEN** `.claude/agents/iktomi.md` is installed
- **THEN** its `tools` frontmatter field includes `Edit` and `Write`

#### Scenario: Hypnos (agent-authoring) retains file-editing tools

- **WHEN** `.claude/agents/hypnos.md` is installed
- **THEN** its `tools` frontmatter field includes `Edit` and `Write`

#### Scenario: Zhou Gong is granted Write but not Edit

- **WHEN** `.claude/agents/zhougong.md` is installed
- **THEN** its `tools` frontmatter field includes `Write` but does not include `Edit`

#### Scenario: Meng Po is granted Write but not Edit

- **WHEN** `.claude/agents/mengpo.md` is installed
- **THEN** its `tools` frontmatter field includes `Write` but does not include `Edit`

### Requirement: GitHub Copilot frontmatter declares its subagent routing graph and hooks in the header

Every platform relies on the same deterministic-vs-judgment hand-off graph (see the `janus-router-agent` capability), but GitHub Copilot's VS Code agent framework is the one platform that requires this graph declared *structurally* in each agent's frontmatter, rather than left to prose alone — unlike Claude Code, where the `Task` tool can technically reach any agent and the deterministic/judgment distinction lives entirely in each agent's instruction body. Every `.github/agents/*.agent.md` file SHALL include, in addition to `name`/`description`/`tools`:

- `agents:` — the list of agent names this agent may invoke as a subagent (its deterministic hand-off targets and/or its broad-routing fan-out, plus `janus` for the ambiguous/terminal case).
- `hooks:` — a normalized, identical set on every one of the ten agents: `coauthor`, `telemetry-write`, `commit`, `version-bump`. GitHub Copilot has no global session-level hook binding file the way Claude Code's `.claude/settings.json` provides, so each agent declares this set itself rather than inheriting it from one shared binding. These hooks exist to guarantee, per the `dev-workflow-hooks` capability, that on GitHub Copilot exactly as everywhere else: `coauthor` sets `git config user.name`/`user.email` to the currently acting agent for every turn; `telemetry-write` (surfaced via `coauthor --trailer`'s `Tokens:` line) pushes token/model usage into the commit message; `commit` guarantees every turn (`--reason turn-complete`) and hand-off (`--reason handoff`) produces a checkpoint commit, so no agent's cycle is left uncaptured in git history; and `version-bump` bumps the patch version by default on every turn, with minor (new change/branch) and major (breaking change) bumps triggered separately per the `dev-workflow-hooks` capability.

The `agents:` list SHALL match the hand-off graph defined in the `janus-router-agent` capability, which has three tiers:

- **Router** — `janus.agent.md` lists all nine other agents (it dispatches to any of them at an entry point).
- **Narrow deterministic** — `nyx.agent.md` lists `janus` plus `morpheus` (the acceptance test is written; implementation is always next); `morpheus.agent.md` lists `janus` plus `phobetor`; `phobetor.agent.md` lists `janus` plus `baku`, `morpheus`, and `phantasos`; `phantasos.agent.md` and `baku.agent.md` list only `janus` (they have no deterministic hand-off of their own — only ambiguous escalation or terminal reporting).
- **Broad-routing** — `iktomi.agent.md`, `zhougong.agent.md`, `hypnos.agent.md`, and `mengpo.agent.md` each list all nine other agents, same as `janus.agent.md`, because their work isn't confined to a fixed set of hand-off targets (see the `janus-router-agent` capability's "Iktomi and the agent-roster-maintenance agents can route directly to any agent" requirement).

#### Scenario: Janus's GitHub Copilot frontmatter lists all nine other agents

- **WHEN** `.github/agents/janus.agent.md` is installed
- **THEN** its `agents:` frontmatter field lists `phantasos`, `nyx`, `morpheus`, `phobetor`, `baku`, `iktomi`, `zhougong`, `hypnos`, and `mengpo`

#### Scenario: Nyx can invoke Morpheus directly on GitHub Copilot

- **WHEN** `.github/agents/nyx.agent.md` is installed
- **THEN** its `agents:` frontmatter field includes `morpheus` in addition to `janus`, and no other agent

#### Scenario: Morpheus can invoke Phobetor directly on GitHub Copilot

- **WHEN** `.github/agents/morpheus.agent.md` is installed
- **THEN** its `agents:` frontmatter field includes `phobetor` in addition to `janus`, and no other agent

#### Scenario: Phobetor's GitHub Copilot frontmatter lists its success and failure targets

- **WHEN** `.github/agents/phobetor.agent.md` is installed
- **THEN** its `agents:` frontmatter field includes `baku`, `morpheus`, and `phantasos`, in addition to `janus`

#### Scenario: A narrow agent with no declared direct edges can still reach Janus

- **WHEN** `.github/agents/baku.agent.md` or `.github/agents/phantasos.agent.md` is installed
- **THEN** its `agents:` frontmatter field contains only `janus`

#### Scenario: The broad-routing agents list all nine peers on GitHub Copilot

- **WHEN** `.github/agents/iktomi.agent.md`, `.github/agents/zhougong.agent.md`, `.github/agents/hypnos.agent.md`, or `.github/agents/mengpo.agent.md` is installed
- **THEN** its `agents:` frontmatter field lists all nine other agents, the same set `janus.agent.md` lists

#### Scenario: Every agent declares the same normalized hook set

- **WHEN** any of the ten `.github/agents/*.agent.md` files is installed
- **THEN** its `hooks:` frontmatter field lists `coauthor`, `telemetry-write`, `commit`, and `version-bump`, identically on every agent regardless of its `agents:` tier (router, narrow, or broad-routing)

#### Scenario: Coauthor hook keeps git identity current per turn

- **WHEN** any agent's turn begins on GitHub Copilot
- **THEN** the `coauthor` hook it declares sets `git config user.name`/`user.email` to that agent's identity, the same as the `PreToolUse`/`SubagentStop` binding does on Claude Code

#### Scenario: Telemetry is pushed into the commit message on every agent

- **WHEN** any agent on GitHub Copilot produces a commit
- **THEN** the `telemetry-write` hook it declares results in a `Tokens:` line in that commit's message, alongside the `Co-authored-by:` trailer `coauthor` appends

#### Scenario: Commit hook guarantees every agent cycle is captured in git history

- **WHEN** any agent's turn ends or hands off on GitHub Copilot
- **THEN** the `commit` hook it declares runs `dreamland commit --reason turn-complete` (or `--reason handoff`), so a checkpoint commit exists for that cycle even if the agent never explicitly ran `git commit`

#### Scenario: Version-bump hook defaults to a patch bump per turn

- **WHEN** any agent's turn ends or hands off on GitHub Copilot
- **THEN** the `version-bump` hook it declares runs `dreamland version-bump --patch`, per the `dev-workflow-hooks` capability
