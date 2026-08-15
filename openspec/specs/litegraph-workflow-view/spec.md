# litegraph-workflow-view Specification

## Purpose
TBD - created by archiving change hypnos-litegraph-editor. Update Purpose after archive.
## Requirements
### Requirement: `/hypnos-view` renders the current agent/skill/hook graph read-only

The scaffold-installed `/hypnos-view` slash command SHALL start a local, `127.0.0.1`-bound HTTP server in view mode and open a litegraph.js graph in the browser showing every agent, skill, and hook installed in the current repository's six live platform directories, with edges for Janus routing (`routes_to`), skill/hook attachments, and hand-off targets. The view-mode server SHALL NOT mount any route capable of writing to any platform file.

#### Scenario: Opening the view shows all installed agents

- **WHEN** a user invokes `/hypnos-view` in a repository where `dreamland init` has completed for at least one platform
- **THEN** a local server starts bound to `127.0.0.1` on an ephemeral port
- **AND** the browser opens a graph with one node per installed agent, one node per installed skill, and one node per installed hook binding, with edges reflecting each agent's `routes_to` targets

#### Scenario: View mode has no write route

- **WHEN** the view-mode server is running
- **THEN** no HTTP route exists that accepts a graph mutation (node create/update/delete, edge create/delete) — such a request receives `404 Not Found`, not a permission error

#### Scenario: Dragging a node in view mode does not persist

- **WHEN** a user drags a node to a new position in the `/hypnos-view` UI
- **THEN** the visual position updates in the browser only
- **AND** no platform file, and nothing in the local graph cache, changes as a result

### Requirement: The graph cache is a live-reloaded view of the six platform files, not a persisted source of truth

The graph rendered by `/hypnos-view` and `/hypnos-interactive` SHALL be rebuilt from the six live platform directories (and Janus's routing prose) whenever it's served — at server start and on every relevant filesystem change — never trusted as an unrefreshed snapshot between rebuilds. The cache file backing this SHALL be excluded from version control.

#### Scenario: No cache present yet

- **WHEN** `/hypnos-view` is invoked in a repository that has run `dreamland init` but has no existing graph cache on disk
- **THEN** the server builds the graph by scanning the installed platform files and renders it — no error, no empty graph

#### Scenario: Cache file is not committed

- **WHEN** the repository's `.gitignore` is inspected after this change is applied
- **THEN** the graph cache file's path is listed, so `git status` never reports it as an untracked or modified file to commit

#### Scenario: Unresolved routing edge flagged, not guessed

- **WHEN** the live rebuild encounters an agent whose hand-off target cannot be confidently extracted from its instruction-body prose
- **THEN** the agent's node is still created in the graph
- **AND** the node is visually flagged as having unresolved routing, with no edge fabricated on its behalf

### Requirement: The view refreshes on update, pushed to the browser without a manual reload

While `/hypnos-view` (or `/hypnos-interactive`) is open, a server-side watcher on the six live platform directories and the active OpenSpec change directory SHALL detect a change and push a refresh to the open browser tab over the SSE connection, without the browser ever needing to re-fetch on a timer or the user manually reloading the page. (The server's own change-detection mechanism is a short-interval mtime poll, not an OS-native filesystem-event API — see design.md's revised decision; that's an internal implementation detail, not something the browser side does or waits on.)

#### Scenario: Hand-off edge change appears without reload

- **WHEN** an agent's routing edge changes on disk (via the editor, a `hypnos` plan-apply run, or a hand edit) while `/hypnos-view` is open
- **THEN** the open browser tab reflects the updated edge shortly after the change, without the user reloading the page or the browser polling for it

### Requirement: The view surfaces OpenSpec task/change progress and the currently active agent

The graph SHALL include, alongside the node/edge structure, every in-progress OpenSpec change's task-completion progress and the session's currently active agent (the same identity `.dreamland-session.json`'s coauthor/telemetry hooks already maintain every turn). **Scope note, corrected from the original wording**: there is no per-task agent-assignment data anywhere in this system — OpenSpec tracks task completion, not who is doing a given task, and that association only ever exists transiently inside a live agent session. The requirement is therefore scoped to what's genuinely knowable: a change-level progress overlay, plus a highlight on the one agent node matching the session's current agent — not a specific-task-to-specific-agent-node binding, which isn't real data to bind from.

#### Scenario: In-progress change progress is visible

- **WHEN** an OpenSpec change has `tasks.md` items completed but not archived, and `/hypnos-view` is open
- **THEN** the view shows that change's name and completed/total task count, sourced from `openspec list --json`
- **AND** the display updates as task status changes, pushed to the browser over the same SSE connection rather than the browser re-fetching on a timer

#### Scenario: The currently active agent's node is highlighted

- **WHEN** `.dreamland-session.json` names an agent as the session's current agent
- **THEN** that agent's node in an open `/hypnos-view` session is visually highlighted
- **AND** no other agent's node is highlighted on the basis of task content alone

#### Scenario: Status overlay degrades gracefully without the openspec CLI

- **WHEN** the `openspec` CLI is not installed or `openspec list --json` fails
- **THEN** the graph itself still renders normally
- **AND** the change-progress overlay is simply empty, not an error blocking the rest of the view

