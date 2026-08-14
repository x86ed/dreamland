## ADDED Requirements

### Requirement: `/hypnos-interactive` opens the graph in editable mode

The scaffold-installed `/hypnos-interactive` slash command SHALL start a local, `127.0.0.1`-bound HTTP server in interactive mode, rendering the same graph as `/hypnos-view` but with write routes mounted, so node/edge edits in the browser can be saved back to `.dreamland/workflow-graph.json` and, from there, synced to the six live platform directories.

#### Scenario: Interactive mode exposes write routes

- **WHEN** `/hypnos-interactive` starts the local server
- **THEN** HTTP routes for node create/update/delete and edge create/delete are mounted and reachable

#### Scenario: Saving a moved node does not itself trigger a platform sync

- **WHEN** a user drags a node to a new position and saves
- **THEN** `.dreamland/workflow-graph.json` records the new position
- **AND** no platform file changes, since position alone carries no information the six platform files represent

### Requirement: Editing a routing edge updates `routes_to` and regenerates the affected hand-off sentence

Connecting or removing an edge between two agent nodes SHALL update the source agent's `routes_to` list in `.dreamland/workflow-graph.json` and regenerate that agent's hand-off sentence in its shared instruction body, then sync the change to every platform file that carries that agent's instructions.

#### Scenario: Adding a routing edge updates all six platforms

- **WHEN** a user draws an edge from `hypnos`'s node to `mengpo`'s node in `/hypnos-interactive` and saves
- **THEN** `hypnos`'s `routes_to` in `.dreamland/workflow-graph.json` includes `mengpo`
- **AND** `hypnos`'s instruction body is regenerated to state the new hand-off
- **AND** `.claude/agents/hypnos.md`, `.cursor/rules/hypnos.mdc`, `.codex/agents/hypnos.toml`, `.kiro/steering/hypnos.md`, `.agents/skills/hypnos/SKILL.md`, and `.github/agents/hypnos.agent.md` are all updated to reflect it

#### Scenario: Removing a routing edge removes the corresponding hand-off sentence

- **WHEN** a user deletes an existing routing edge and saves
- **THEN** the target is removed from the source agent's `routes_to`
- **AND** the regenerated instruction body no longer states a hand-off to that target
- **AND** the change is synced to all six platform files for that agent

### Requirement: Creating, attaching, and detaching agents/skills/hooks goes through existing scaffold writer logic

Creating a new agent, skill, or hook node, or attaching/detaching one from another node, SHALL apply the same tool-tier rules and per-platform file conventions that `hypnos` (agent authoring) and `mengpo` (archival) already enforce, rather than writing ad hoc content.

#### Scenario: Creating a new agent from the editor applies tool-tier rules

- **WHEN** a user creates a new agent node in `/hypnos-interactive`, sets its role description, and saves
- **THEN** the new agent's tool bindings are assigned per the same router-excluded/full-edit/write-only-no-edit tier matrix `hypnos` applies when authoring an agent by hand
- **AND** a file is written for the new agent on all six platforms following each platform's existing format conventions

#### Scenario: Detaching a hook from an agent removes its binding on every platform

- **WHEN** a user detaches a hook node from an agent node and saves
- **THEN** that hook's binding is removed from `.dreamland/workflow-graph.json`
- **AND** the corresponding hook entry is removed from that agent's file on every platform where it was present

#### Scenario: Deleting an agent node archives it like `mengpo` would

- **WHEN** a user deletes an agent node in `/hypnos-interactive` and confirms
- **THEN** the agent's files are archived/removed on all six platforms following the same process `mengpo` uses to retire an agent
- **AND** any remaining `routes_to` edges pointing at the deleted agent are removed and their source agents' instruction bodies regenerated

### Requirement: Sync detects and surfaces drift from hand-edited platform files

Before writing a generated file during sync, the server SHALL compare the file's current on-disk content against what `.dreamland/workflow-graph.json` last generated for it. On a mismatch, the write SHALL be blocked and the conflict surfaced in the UI for an explicit choice, rather than silently overwritten.

#### Scenario: Hand-edited file blocks a silent overwrite

- **WHEN** `.claude/agents/hypnos.md` was hand-edited outside the editor since the last sync, and a save in `/hypnos-interactive` would otherwise regenerate that file
- **THEN** the write is blocked
- **AND** the UI presents the conflict, requiring an explicit "keep disk" or "overwrite from graph" choice before proceeding

#### Scenario: No drift means sync proceeds normally

- **WHEN** no platform file targeted by a save has changed on disk since the last sync
- **THEN** the save proceeds and the affected files are regenerated without requiring any conflict choice

### Requirement: `hypnos` can implement a workflow-graph plan headlessly

`dreamland hypnos-serve --mode=apply-plan --plan <file>` SHALL read an ordered list of node/edge mutation operations from `<file>` and apply each one through the same mutation handlers, sync logic, and drift detection the interactive editor's write routes use, without starting an HTTP listener or requiring a browser. This is how the `hypnos` agent implements a workflow-graph change on its own.

#### Scenario: Applying a plan produces the same result as the equivalent manual edits

- **WHEN** a plan file containing a `create_edge` operation from `hypnos` to a new agent node, followed by a `create_node` operation for that agent, is applied via `--mode=apply-plan`
- **THEN** `.dreamland/workflow-graph.json` and all six platform files reflect the same end state that performing the equivalent create-node-then-connect-edge actions in `/hypnos-interactive` would have produced

#### Scenario: Plan apply exits without starting a server

- **WHEN** `dreamland hypnos-serve --mode=apply-plan --plan <file>` runs
- **THEN** no HTTP port is opened
- **AND** the process applies the plan and exits, reporting per-operation success or failure

#### Scenario: Drift during plan apply blocks that operation, not the whole run

- **WHEN** a plan includes an operation targeting a platform file that has drifted from what `.dreamland/workflow-graph.json` last generated
- **THEN** that operation is rejected with the conflict reported in the process output
- **AND** operations that don't target the drifted file still apply normally

#### Scenario: Invalid operation in a plan is rejected before any write

- **WHEN** a plan file contains an operation referencing a node id that does not exist and is not created earlier in the same plan
- **THEN** the entire plan is rejected before any file is written, with the invalid operation identified in the error
