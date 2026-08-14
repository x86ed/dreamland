## ADDED Requirements

### Requirement: `/hypnos-view` renders the current agent/skill/hook graph read-only

The scaffold-installed `/hypnos-view` slash command SHALL start a local, `127.0.0.1`-bound HTTP server in view mode and open a litegraph.js graph in the browser showing every agent, skill, and hook installed in the current repository's six live platform directories, with edges for Janus routing (`routes_to`), skill/hook attachments, and hand-off targets. The view-mode server SHALL NOT mount any route capable of writing to `.dreamland/workflow-graph.json` or any platform file.

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
- **AND** no file on disk (including `.dreamland/workflow-graph.json`) changes as a result

### Requirement: The view reflects live OpenSpec task/change status

While the view-mode server is running, node rendering SHALL include the current status (not-started, in-progress, done) of any OpenSpec change/task the corresponding agent is actively working on, refreshed periodically without requiring the browser tab to be reloaded.

#### Scenario: In-progress task highlighted on its agent's node

- **WHEN** `morpheus` is actively working through `tasks.md` for an open OpenSpec change
- **THEN** the `morpheus` node in an open `/hypnos-view` session shows an in-progress indicator tied to that change, and the indicator updates as task status changes without a manual page reload

### Requirement: First run bootstraps the graph from installed platform files

If `.dreamland/workflow-graph.json` does not exist when `/hypnos-view` or `/hypnos-interactive` is first invoked, the server SHALL import it from the six live platform directories and Janus's routing prose before rendering, rather than showing an empty graph or erroring.

#### Scenario: No graph.json present yet

- **WHEN** `/hypnos-view` is invoked in a repository that has run `dreamland init` but has no `.dreamland/workflow-graph.json`
- **THEN** the server builds an initial graph by scanning the installed platform files, writes `.dreamland/workflow-graph.json`, and renders the imported graph

#### Scenario: Unresolved routing edge flagged, not guessed

- **WHEN** the bootstrap importer encounters an agent whose hand-off target cannot be confidently extracted from its instruction-body prose
- **THEN** the agent's node is still created in the graph
- **AND** the node is visually flagged as having unresolved routing, with no edge fabricated on its behalf
