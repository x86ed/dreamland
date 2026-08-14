## Why

The agent/skill/hook graph (ten agents, six platform templates, Janus's routing table, per-agent hooks) is currently only inspectable by reading template files and spec prose across `internal/scaffold/templates/`. There is no single view of the whole system, and changing routing or attaching a skill/hook means hand-editing every platform's file. A litegraph.js-based visual editor gives users one place to see the framework's structure and to change it, with edits fanning out through the same scaffold-sync mechanism `hypnos`/`mengpo` already use to keep all six platforms consistent.

## What Changes

- Add a local web server, launched via slash command, that renders every agent, skill, and hook as a litegraph.js node graph, with edges for Janus routing links, skill/hook attachments, and hand-off targets.
- Add `/hypnos-interactive`: opens the graph in editable mode. Node/edge edits in the browser (move, connect, detach, create, delete) write back to the six-platform template files and Janus's routing tables — the same target files `hypnos`/`mengpo`/janus-router-agent scaffolding already own — so the graph is a UI over those files, not a separate store.
- Add `/hypnos-view`: opens the same graph read-only, rendering current repo state (including in-flight OpenSpec change/task status) for observing work as it happens, with no write path.
- Editor writes route through the existing per-platform scaffold writers (the same code `dreamland init`/`hypnos` use) so a change made in the graph is reflected in Claude Code, Cursor, Codex, Kiro, Antigravity, and GitHub Copilot templates without manual per-platform edits.
- Creating/attaching/detaching a skill or agent from the editor SHALL go through the same validation and file layout as the `hypnos`/`mengpo` agents (tool-tier rules, per-platform file conventions) rather than writing ad hoc content.

## Capabilities

### New Capabilities
- `litegraph-workflow-view`: local server + `/hypnos-view` that renders agents, skills, hooks, and their connections (routing, attachments, hand-offs) as a read-only litegraph.js graph reflecting current repo/task state.
- `litegraph-workflow-editor`: local server + `/hypnos-interactive` that renders the same graph in editable mode; node/edge mutations (create/attach/detach agents, skills, hooks; change routing edges) are written back through the existing per-platform scaffold-sync mechanism so every platform's templates and Janus's routing table stay consistent.

### Modified Capabilities
(none — this change adds new entry points and a new UI layer on top of the existing scaffold-sync mechanism; it does not change the requirements of `agent-scaffolding`, `router-slash-commands`, or `janus-router-agent`)

## Impact

- New: a local HTTP server (embedded in the `dreamland` binary, alongside existing `cmd/` subcommands) serving a litegraph.js single-page app.
- New: `/hypnos-interactive` and `/hypnos-view` slash-command templates across all six platforms (`internal/scaffold/templates/commands/*`), following the existing per-platform command conventions in `router-slash-commands`.
- Modified (write path only, not requirements): editor mutations call into the same writer logic used by `hypnos` (agent authoring), `mengpo` (archival), and the Janus routing-table update path, so those code paths gain a second caller.
- New dependency: litegraph.js, vendored/bundled for the server's static assets (no external CDN at runtime, consistent with the project's offline-friendly CLI).
