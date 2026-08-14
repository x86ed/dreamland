## ADDED Requirements

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

### Requirement: The view refreshes on update via a filesystem watcher, not polling

While `/hypnos-view` (or `/hypnos-interactive`) is open, a filesystem watcher on the six live platform directories and the active OpenSpec change directory SHALL trigger a graph rebuild and push a refresh to the open browser tab whenever a watched file changes, without requiring a manual reload or a fixed polling interval.

#### Scenario: Hand-off edge change appears without reload

- **WHEN** an agent's routing edge changes on disk (via the editor, a `hypnos` plan-apply run, or a hand edit) while `/hypnos-view` is open
- **THEN** the open browser tab reflects the updated edge shortly after the change, without the user reloading the page

#### Scenario: In-progress task highlighted on its agent's node

- **WHEN** `morpheus` is actively working through `tasks.md` for an open OpenSpec change
- **THEN** the `morpheus` node in an open `/hypnos-view` session shows an in-progress indicator tied to that change, and the indicator updates as task status changes, pushed by the same filesystem-watcher mechanism rather than periodic polling
