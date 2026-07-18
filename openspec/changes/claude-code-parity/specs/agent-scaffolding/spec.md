## ADDED Requirements

### Requirement: Claude Code frontmatter declares agent-scoped Stop hooks for redundant telemetry/identity binding

Every `.claude/agents/*.md` file SHALL include, in addition to `name`/`description`/`role`/`tools`/`model`, an agent-scoped `hooks:` frontmatter block using Claude Code's per-subagent hook mechanism (frontmatter `Stop:` entries, which Claude Code auto-converts to `SubagentStop` at runtime when the agent is invoked as a subagent). Shown for `nyx.md`:

```yaml
hooks:
  Stop:
    - hooks:
        - type: command
          command: dreamland coauthor --agent-name nyx
        - type: command
          command: dreamland telemetry write --tool claude-code --agent-name nyx
        - type: command
          command: dreamland version-bump --patch
        - type: command
          command: dreamland version-bump --minor --if-agent janus
        - type: command
          command: dreamland commit --reason handoff --agent-name nyx
```

Every file carries the same five commands in the same order, including `dreamland version-bump --minor --if-agent janus` on every file, not only `janus.md` (self-filtering, silent no-op unless the invoking payload's `agent_type` is `janus` — see the `dev-workflow-hooks` capability). The one value that differs per file is the `--agent-name` argument to `coauthor`, `telemetry write`, and `commit`, which SHALL equal that file's own frontmatter `name:` field. `--agent-name` (new CLI flag, see the `dev-workflow-hooks` capability's commit-resolution requirement) takes precedence over the existing env-var/stdin-payload resolution chain and is hardcoded here rather than left to runtime lookup: unlike the workspace-level `SubagentStop` array — one shared block that cannot know in advance which of the ten agents (or which built-in) will trigger it — a per-agent frontmatter block is declared inside that specific agent's own file, so its identity is already known at template-authoring time. This is a deliberate improvement over GitHub Copilot's equivalent mechanism, not a parity mirror: Copilot's own agent-scoped `hooks:` blocks (e.g. `hypnos.agent.md`) take no such parameter and rely on the same runtime `agent_type` stdin lookup the workspace-level path uses.

This requirement is the Claude Code counterpart to GitHub Copilot's existing agent-scoped `hooks:` frontmatter (see this capability's "GitHub Copilot frontmatter declares its subagent routing graph..." requirement) — same identity/telemetry/version-bump/commit coverage — adapted to Claude Code's mechanism and telemetry pipeline rather than copied verbatim: Claude Code has no `SubagentStart` frontmatter event (only `PreToolUse`/`PostToolUse`/`Stop` are supported at the per-agent level, so there is no frontmatter counterpart to Copilot's `SubagentStart` → `coauthor` entry — the existing workspace-level `PreToolUse` hook, matcher `Task|Agent`, already covers that role), and `telemetry write --tool claude-code` reads token counts from the transcript JSONL directly rather than depending on an OTel receiver the way Copilot's `--tool github-copilot` invocation does.

Unlike GitHub Copilot's agent-scoped hooks (which require the `chat.useCustomAgentHooks` setting to fire at all), Claude Code's frontmatter hooks require no equivalent settings flag — they fire whenever the agent is spawned as a subagent through the `Agent` tool or an `@`-mention, or run as the main session via `--agent`.

#### Scenario: Agent-scoped hooks block installed on every Claude Code agent, each with its own agent-name argument

- **WHEN** `dreamland init` completes with "Claude Code" selected
- **THEN** each of `.claude/agents/janus.md`, `phantasos.md`, `nyx.md`, `morpheus.md`, `phobetor.md`, `baku.md`, `iktomi.md`, `zhougong.md`, `hypnos.md`, and `mengpo.md` contains a `hooks.Stop` frontmatter entry running, in order, `dreamland coauthor --agent-name <that-file's-name>`, `dreamland telemetry write --tool claude-code --agent-name <that-file's-name>`, `dreamland version-bump --patch`, `dreamland version-bump --minor --if-agent janus`, and `dreamland commit --reason handoff --agent-name <that-file's-name>`

#### Scenario: Agent-scoped hook commands match the workspace-level SubagentStop commands except for the added --agent-name argument

- **WHEN** any `.claude/agents/*.md` file's `hooks.Stop` frontmatter is compared against `.claude/settings.json`'s `hooks.SubagentStop` command array
- **THEN** both list the same five underlying commands in the same order, differing only in that the frontmatter version adds `--agent-name <that-file's-name>` to the `coauthor`, `telemetry write`, and `commit` invocations

## MODIFIED Requirements

### Requirement: GitHub Copilot frontmatter declares its subagent routing graph, and any agent that dispatches subagents grants itself the `agent` tool

Every platform relies on the same deterministic-vs-judgment hand-off graph (see the `janus-router-agent` capability), but GitHub Copilot's VS Code agent framework is the one platform that requires this graph declared *structurally* in each agent's frontmatter, rather than left to prose alone — unlike Claude Code, where the `Task`/`Agent` tool can technically reach any agent and the deterministic/judgment distinction lives entirely in each agent's instruction body. Every `.github/agents/*.agent.md` file SHALL include, in addition to `name`/`description`/`tools`:

- `agents:` — the list of agent names this agent may invoke as a subagent (its deterministic hand-off targets and/or its broad-routing fan-out, plus `janus` for the ambiguous/terminal case).
- `tools:` includes `agent` whenever `agents:` is non-empty (every one of the ten agents, since even the narrow agents dispatch to `janus`). VS Code's custom-agent framework requires the invoking agent's own `tools:` list to grant the `agent` tool before its `agents:` restriction list has any effect — declaring `agents:` alone, without also granting the `agent` tool, leaves subagent dispatch unavailable to that agent.

Hook bindings (`coauthor`, `telemetry-write`, `commit`, `version-bump`) are declared **twice**, redundantly, on GitHub Copilot: once workspace-wide (`.github/hooks/*.json` — see the `dev-workflow-hooks` capability's "GitHub Copilot binds identity, telemetry, and lifecycle commands via a real hooks file" requirement, which needs no settings flag and fires regardless of which agent is active) and once per-agent, via a real agent-scoped `hooks:` frontmatter field. Every `.github/agents/*.agent.md` file's frontmatter SHALL also include:

- `hooks:` — a map from event name to an array of `{type: command, command: "<cmd>"}` entries: `SubagentStart` runs `dreamland coauthor --agent-name <that-file's-name>`; `SubagentStop` runs `dreamland coauthor --agent-name <that-file's-name>`, `dreamland telemetry write --tool github-copilot --agent-name <that-file's-name>`, `dreamland version-bump --patch`, and `dreamland commit --reason handoff --agent-name <that-file's-name>` — the same commands the workspace-level hooks file already binds to those events, plus an explicit `--agent-name` argument on every command that accepts one. `--agent-name` (see the `dev-workflow-hooks` capability) takes precedence over the existing runtime `agent_type`-from-payload lookup and is hardcoded here rather than left to that lookup: a per-agent frontmatter block is declared inside that specific agent's own file, so its identity is already known at template-authoring time, unlike the workspace-level file's single block shared across all ten agents. This is belt-and-suspenders, not a different behavior: agent-scoped hooks are a real, separate VS Code mechanism (distinct schema location from the workspace file, same event names and command-entry shape) that requires the `chat.useCustomAgentHooks: true` setting — which `dreamland init` writes to `.vscode/settings.json` — to fire at all; the workspace-level file has no such gate. Declaring both means hooks still fire even if a user's environment has the agent-scoped preview flag off (workspace file) or if the workspace-file mechanism is ever restricted (agent-scoped, once the flag is on).

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

#### Scenario: Every agent declares agent-scoped hooks carrying its own agent-name argument

- **WHEN** any of the ten `.github/agents/*.agent.md` files is installed
- **THEN** its `hooks:` frontmatter field declares `SubagentStart` running `dreamland coauthor --agent-name <that-file's-name>`, and `SubagentStop` running `dreamland coauthor --agent-name <that-file's-name>`, `dreamland telemetry write --tool github-copilot --agent-name <that-file's-name>`, `dreamland version-bump --patch`, and `dreamland commit --reason handoff --agent-name <that-file's-name>`
- **AND** the `<that-file's-name>` value matches that file's own frontmatter `name:` field, so the five agent-scoped blocks are identical in structure but not byte-identical to each other

#### Scenario: dreamland init enables the agent-scoped hooks preview flag

- **WHEN** `dreamland init` completes with "GitHub Copilot" selected
- **THEN** `.vscode/settings.json` contains `"chat.useCustomAgentHooks": true`
