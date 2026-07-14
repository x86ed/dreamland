# agent-scaffolding

## Requirements

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
- **THEN** ten `.agent.md` files are written to `.github/agents/` (`janus.agent.md`, `phantasos.agent.md`, `nyx.agent.md`, `morpheus.agent.md`, `phobetor.agent.md`, `baku.agent.md`, `iktomi.agent.md`, `zhougong.agent.md`, `hypnos.agent.md`, `mengpo.agent.md`), each with YAML frontmatter (`name`, `description`, `tools`, `agents`, `hooks` — see the "GitHub Copilot frontmatter declares its subagent routing graph, and any agent that dispatches subagents grants itself the `agent` tool" requirement below)

### Requirement: Agent templates are embedded in the binary

The CLI binary SHALL contain all agent template content at compile time using Go embed directives; no template files SHALL be read from the host filesystem at install time.

#### Scenario: Binary installs agents without template directory present

- **WHEN** `dreamland init` is run in an environment where `internal/scaffold/templates/` does not exist on disk
- **THEN** agents are still installed correctly using templates compiled into the binary

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

### Requirement: Installed agent files are reported to the user

After scaffolding completes, the CLI SHALL print a summary listing each agent file that was written and each that was skipped.

#### Scenario: All agents newly written

- **WHEN** no agent files previously existed and `dreamland init` completes
- **THEN** stdout contains one `installed:` line per agent file written

#### Scenario: Mixed written and skipped

- **WHEN** some agent files exist and some do not
- **THEN** stdout contains `installed:` lines for new files and `skipped (already exists):` lines for existing ones

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

### Requirement: GitHub Copilot frontmatter declares its subagent routing graph, and any agent that dispatches subagents grants itself the `agent` tool

Every platform relies on the same deterministic-vs-judgment hand-off graph (see the `janus-router-agent` capability), but GitHub Copilot's VS Code agent framework is the one platform that requires this graph declared *structurally* in each agent's frontmatter, rather than left to prose alone — unlike Claude Code, where the `Task`/`Agent` tool can technically reach any agent and the deterministic/judgment distinction lives entirely in each agent's instruction body. Every `.github/agents/*.agent.md` file SHALL include, in addition to `name`/`description`/`tools`:

- `agents:` — the list of agent names this agent may invoke as a subagent (its deterministic hand-off targets and/or its broad-routing fan-out, plus `janus` for the ambiguous/terminal case).
- `tools:` includes `agent` whenever `agents:` is non-empty (every one of the ten agents, since even the narrow agents dispatch to `janus`). VS Code's custom-agent framework requires the invoking agent's own `tools:` list to grant the `agent` tool before its `agents:` restriction list has any effect — declaring `agents:` alone, without also granting the `agent` tool, leaves subagent dispatch unavailable to that agent.

Hook bindings (`coauthor`, `telemetry-write`, `commit`, `version-bump`) are declared **twice**, redundantly, on GitHub Copilot: once workspace-wide (`.github/hooks/*.json` — see the `dev-workflow-hooks` capability's "GitHub Copilot binds identity, telemetry, and lifecycle commands via a real hooks file" requirement, which needs no settings flag and fires regardless of which agent is active) and once per-agent, via a real agent-scoped `hooks:` frontmatter field. Every `.github/agents/*.agent.md` file's frontmatter SHALL also include:

- `hooks:` — a map from event name to an array of `{type: command, command: "<cmd>"}` entries, identical on every one of the ten agents: `SubagentStart` runs `dreamland coauthor`; `SubagentStop` runs `dreamland coauthor`, `dreamland telemetry write --tool github-copilot`, `dreamland version-bump --patch`, and `dreamland commit --reason handoff` — the same commands the workspace-level hooks file already binds to those events. This is belt-and-suspenders, not a different behavior: agent-scoped hooks are a real, separate VS Code mechanism (distinct schema location from the workspace file, same event names and command-entry shape) that requires the `chat.useCustomAgentHooks: true` setting — which `dreamland init` writes to `.vscode/settings.json` — to fire at all; the workspace-level file has no such gate. Declaring both means hooks still fire even if a user's environment has the agent-scoped preview flag off (workspace file) or if the workspace-file mechanism is ever restricted (agent-scoped, once the flag is on).

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

#### Scenario: Every agent that dispatches subagents grants itself the agent tool

- **WHEN** any of the ten `.github/agents/*.agent.md` files is installed
- **THEN** its `tools:` frontmatter field includes `agent`, since every agent's `agents:` list is non-empty (at minimum, every agent can reach `janus`)

#### Scenario: Every agent declares identical agent-scoped hooks

- **WHEN** any of the ten `.github/agents/*.agent.md` files is installed
- **THEN** its `hooks:` frontmatter field declares `SubagentStart` running `dreamland coauthor`, and `SubagentStop` running `dreamland coauthor`, `dreamland telemetry write --tool github-copilot`, `dreamland version-bump --patch`, and `dreamland commit --reason handoff` — identically on every agent

#### Scenario: dreamland init enables the agent-scoped hooks preview flag

- **WHEN** `dreamland init` completes with "GitHub Copilot" selected
- **THEN** `.vscode/settings.json` contains `"chat.useCustomAgentHooks": true`
