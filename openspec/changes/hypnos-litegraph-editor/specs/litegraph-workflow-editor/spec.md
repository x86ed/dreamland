## ADDED Requirements

### Requirement: `/hypnos-interactive` opens the graph in editable mode

The scaffold-installed `/hypnos-interactive` slash command SHALL start a local, `127.0.0.1`-bound HTTP server in interactive mode, rendering the same graph as `/hypnos-view` but with write routes mounted, so node/edge edits in the browser can be saved back to the six live platform directories.

#### Scenario: Interactive mode exposes write routes

- **WHEN** `/hypnos-interactive` starts the local server
- **THEN** HTTP routes for node create/update/delete and edge create/delete are mounted and reachable

#### Scenario: Saving a moved node does not itself trigger a platform sync

- **WHEN** a user drags a node to a new position and saves
- **THEN** the graph cache records the new position
- **AND** no platform file changes, since position alone carries no information the six platform files represent

### Requirement: Editor writes always apply against freshly re-imported on-disk state

Before applying any mutation, the interactive server SHALL rebuild the graph from the current on-disk state of the six live platform directories, rather than mutating a snapshot that may be stale relative to concurrent hand edits or another process's writes.

#### Scenario: A hand edit made just before a save is not silently discarded

- **WHEN** `.claude/agents/hypnos.md` is hand-edited on disk, and moments later a user saves an unrelated mutation (e.g. moving a different node) in an already-open `/hypnos-interactive` tab
- **THEN** the save rebuilds from current disk state before applying the mutation, so the hand edit is present in the resulting graph rather than overwritten by a stale in-memory copy

### Requirement: Concurrent writers are serialized with an advisory lock

Every write operation (an interactive save request, or a full `--mode=apply-plan` run) SHALL acquire an OS-level advisory lock on a single lock file before mutating any platform file or the graph cache, and release it immediately after that operation completes.

#### Scenario: A browser save and a plan-apply run overlap

- **WHEN** a user saves an edit in `/hypnos-interactive` at the same moment a `hypnos`-driven `dreamland hypnos-serve --mode=apply-plan` run is writing to the same repository
- **THEN** one operation acquires the lock and completes its write first
- **AND** the other blocks briefly until the lock is released, then proceeds against the now-current on-disk state — neither operation's write is lost or corrupted

#### Scenario: Lock is released after a single operation, not held across a session

- **WHEN** an interactive save completes
- **THEN** the advisory lock is released immediately, so a subsequent unrelated save or a concurrent `apply-plan` run is not blocked by an idle browser tab

### Requirement: Editing a routing edge updates the hand-off sentence in the affected agent's instruction body

Connecting or removing an edge between two agent nodes SHALL regenerate the source agent's hand-off sentence in its shared instruction body, in the same canonical, re-parseable form on every platform, then sync the change to every platform file that carries that agent's instructions.

#### Scenario: Adding a routing edge updates all six platforms

- **WHEN** a user draws an edge from `hypnos`'s node to `mengpo`'s node in `/hypnos-interactive` and saves
- **THEN** `hypnos`'s instruction body is regenerated to state the new hand-off to `mengpo`, in the canonical sentence form
- **AND** `.claude/agents/hypnos.md`, `.cursor/rules/hypnos.mdc`, `.codex/agents/hypnos.toml`, `.kiro/steering/hypnos.md`, `.agents/skills/hypnos/SKILL.md`, and `.github/agents/hypnos.agent.md` are all updated to reflect it

#### Scenario: Removing a routing edge removes the corresponding hand-off sentence

- **WHEN** a user deletes an existing routing edge and saves
- **THEN** the regenerated instruction body no longer states a hand-off to that target
- **AND** the change is synced to all six platform files for that agent

### Requirement: Creating and deleting agent nodes goes through existing scaffold writer logic

Creating a new agent node, or deleting one, SHALL apply the same tool-tier rules and per-platform file conventions that `hypnos` (agent authoring) and `mengpo` (archival) already enforce, rather than writing ad hoc content.

#### Scenario: Creating a new agent from the editor applies tool-tier rules

- **WHEN** a user creates a new agent node in `/hypnos-interactive`, sets its role description, and saves
- **THEN** the new agent's tool bindings are assigned per the same router-excluded/full-edit/write-only-no-edit tier matrix `hypnos` applies when authoring an agent by hand
- **AND** a file is written for the new agent on all six platforms following each platform's existing format conventions

#### Scenario: Creating a new agent includes the platform's standard telemetry/coauthor/commit hook baseline

- **WHEN** a user creates a new agent node in `/hypnos-interactive` and saves, on a repository with Claude Code and GitHub Copilot installed
- **THEN** the new agent's Claude Code file includes the same 5-command `hooks.Stop` block (coauthor, telemetry write, version-bump patch, version-bump minor if-agent janus, commit) every existing agent has, parameterized with the new agent's name
- **AND** the new agent's GitHub Copilot file includes the platform's `SubagentStart`/`SubagentStop` hook equivalent, parameterized the same way
- **AND** the new agent is not missing this baseline the way a hand-authored file might be if someone forgot to copy it

#### Scenario: Deleting an agent node archives it like `mengpo` would

- **WHEN** a user deletes an agent node in `/hypnos-interactive` and confirms
- **THEN** the agent's files are archived/removed on all six platforms following the same process `mengpo` uses to retire an agent
- **AND** any remaining routing edges pointing at the deleted agent are removed and their source agents' instruction bodies regenerated

### Requirement: Hook attachment defaults to project (workspace) scope where the platform supports it

Attaching a hook to the graph SHALL default to a project-level (workspace-wide) binding on any platform whose native format supports one, applying to every agent rather than being duplicated per agent. Per-agent scope SHALL only be used on a platform that has no project-level mechanism for that binding.

#### Scenario: Attaching a hook on Claude Code creates a workspace-level binding

- **WHEN** a user attaches a hook to the graph in `/hypnos-interactive` on a repository with Claude Code installed
- **THEN** the hook is added to `.claude/settings.json`'s unscoped workspace hook array, not duplicated into every individual agent's frontmatter

#### Scenario: Attaching a hook on GitHub Copilot falls back to per-agent scope

- **WHEN** a user attaches a hook to a specific agent's node in `/hypnos-interactive` on a repository with GitHub Copilot installed
- **THEN** the hook is added to that agent's `.agent.md` frontmatter `hooks:` block, since Copilot has no unscoped workspace-level hook mechanism

#### Scenario: Detaching a project-scoped hook removes it from the workspace binding

- **WHEN** a user detaches a project-scoped hook node and saves
- **THEN** the workspace-level binding is removed on platforms where it was project-scoped
- **AND** any per-agent bindings for that hook are removed on platforms where it was necessarily agent-scoped

### Requirement: Existing skills are attach/detach-only; creating a new skill requires the new skill-authoring writer

Wiring an existing skill node to an agent (recording that the agent may invoke it), or removing that wire, SHALL NOT modify the skill's own file — those files are owned by whatever tool generated them (e.g. the external `openspec` CLI for this repo's current skills), not by `dreamland`. Creating a brand-new skill node SHALL write a `SKILL.md` (or platform equivalent) through a dedicated skill-authoring writer, separate from the agent-authoring writer, since no such writer exists prior to this change.

#### Scenario: Attaching an existing skill to an agent does not touch the skill's own file

- **WHEN** a user wires the `openspec-propose` skill node to an agent's `skills` input in `/hypnos-interactive` and saves
- **THEN** the agent's file records that it can invoke `openspec-propose`
- **AND** `.claude/skills/openspec-propose/SKILL.md` is not modified

#### Scenario: Creating a new skill writes a new SKILL.md through the skill-authoring writer

- **WHEN** a user creates a new skill node in `/hypnos-interactive`, sets its description, and saves
- **THEN** a new `SKILL.md` (or platform equivalent) is written for it via the skill-authoring writer, distinct from the agent-authoring writer's code path

### Requirement: `hypnos` can implement a workflow-graph plan headlessly

`dreamland hypnos-serve --mode=apply-plan --plan <file>` SHALL read an ordered list of node/edge mutation operations from `<file>` and apply each one through the same mutation handlers and sync logic the interactive editor's write routes use, without starting an HTTP listener or requiring a browser. `hypnos` authors this plan file itself while implementing a `phantasos`-authored OpenSpec change dispatched to it by Janus in place of a coding agent, when the change is graph-structural rather than code.

#### Scenario: Applying a plan produces the same result as the equivalent manual edits

- **WHEN** a plan file containing a `create_node` operation for a new agent, followed by a `create_edge` operation connecting `hypnos` to it, is applied via `--mode=apply-plan`
- **THEN** the graph cache and all six platform files reflect the same end state that performing the equivalent create-node-then-connect-edge actions in `/hypnos-interactive` would have produced

#### Scenario: Plan apply exits without starting a server

- **WHEN** `dreamland hypnos-serve --mode=apply-plan --plan <file>` runs
- **THEN** no HTTP port is opened
- **AND** the process acquires the advisory lock, rebuilds the graph from current disk state, applies the plan, releases the lock, and exits, reporting per-operation success or failure

#### Scenario: Invalid operation in a plan is rejected before any write

- **WHEN** a plan file contains an operation referencing a node id that does not exist and is not created earlier in the same plan
- **THEN** the entire plan is rejected before any file is written, with the invalid operation identified in the error
