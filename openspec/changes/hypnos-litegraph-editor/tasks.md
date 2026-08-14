## 1. Graph schema and data model

- [ ] 1.1 Define the `.dreamland/workflow-graph.json` schema (Go structs): agent/skill/hook nodes with id, description, tool tier, hook bindings, `routes_to` edges, shared instruction-body markdown, and node position; workspace-level (non-per-agent) hook nodes as a distinct type
- [ ] 1.2 Add read/write helpers for `.dreamland/workflow-graph.json` in `internal/scaffold` (or a new `internal/workflowgraph` package)
- [ ] 1.3 Unit tests for schema round-trip (load, mutate, save, reload)

## 2. Bootstrap importer

- [ ] 2.1 Implement a scanner over the six live platform directories (`.claude/agents`, `.claude/commands/drmlnd`, `.cursor/rules`, `.cursor/commands`, `.codex/agents`, `.codex/skills`, `.kiro/steering`, `.agents/skills`, `.github/agents`, `.github/prompts`) that builds agent/skill/hook nodes
- [ ] 2.2 Implement best-effort `routes_to` extraction from each agent's instruction-body prose ("hand off directly to `X`" patterns); leave unresolved routing unflagged with an edge and mark the node instead
- [ ] 2.3 Wire the importer to run automatically when `.dreamland/workflow-graph.json` is missing at server start
- [ ] 2.4 Tests: importer against this repo's actual installed files produces the expected node set; tests for the unresolved-edge flagging path

## 3. Graph-driven scaffold writer integration

- [ ] 3.1 Extract/adapt `internal/scaffold`'s per-platform formatting functions (used today by `dreamland init`/`hypnos`) to accept a graph node as input instead of only a static embedded template string
- [ ] 3.2 Implement per-platform sync: given a changed node/edge, determine the minimal set of platform files to regenerate
- [ ] 3.3 Implement the instruction-body hand-off sentence regeneration from `routes_to` (add/remove sentence on edge add/remove)
- [ ] 3.4 Implement drift detection: before writing a generated file, compare on-disk content against last-generated content recorded in `.dreamland/workflow-graph.json`; block and report conflicts instead of overwriting
- [ ] 3.5 Implement agent creation (new node -> six platform files, tool-tier assignment per existing router-excluded/full-edit/write-only-no-edit matrix)
- [ ] 3.6 Implement agent deletion/archival (six platform files removed/archived, same process `mengpo` uses; dependent `routes_to` edges cleaned up and their sources regenerated)
- [ ] 3.7 Implement hook attach/detach (adds/removes a hook binding on the target node's files across platforms)
- [ ] 3.8 Tests: edge add/remove syncs all six platforms; agent create/delete syncs all six platforms; drift check blocks a hand-edited file; no-drift save proceeds cleanly

## 4. Local server

- [ ] 4.1 Add `cmd/hypnosserve.go`: `dreamland hypnos-serve --mode=view|interactive`, binds `127.0.0.1:0` (ephemeral port), following the `cmd/serve.go` cobra-subcommand pattern
- [ ] 4.2 Mount read-only graph routes (fetch current graph, fetch task/change status) in both modes
- [ ] 4.3 Mount write routes (node/edge create/update/delete) only when `--mode=interactive`; verify no route exists for them in view mode
- [ ] 4.4 Serve the embedded litegraph.js UI as static assets via `go:embed`
- [ ] 4.5 Print the local URL and open it in the default browser on start
- [ ] 4.6 Tests: view-mode server has no mutation route reachable (404, not 403); interactive-mode server accepts a mutation and it lands in `.dreamland/workflow-graph.json`

## 5. litegraph.js UI

- [ ] 5.1 Vendor `litegraph.js` as an embedded static asset (no CDN dependency)
- [ ] 5.2 Render nodes (agents, skills, hooks) and edges (routing, attachments) from the server's graph endpoint
- [ ] 5.3 Interactive mode: node drag/position save, edge draw/delete, node create dialog (role description input), node delete with confirmation, hook attach/detach controls
- [ ] 5.4 Conflict UI: present drift conflicts from 3.4 with "keep disk" / "overwrite from graph" choice
- [ ] 5.5 View mode: task/change status indicator per agent node, refreshed periodically (poll interval per design's open question — start with polling)
- [ ] 5.6 Visual flag for nodes with unresolved routing (from bootstrap import)

## 6. Slash commands

- [ ] 6.1 Add `/hypnos-view` template for all six platforms (`internal/scaffold/templates/commands/*`), following `router-slash-commands` per-platform conventions; invokes `dreamland hypnos-serve --mode=view`
- [ ] 6.2 Add `/hypnos-interactive` template for all six platforms; invokes `dreamland hypnos-serve --mode=interactive`
- [ ] 6.3 Register both commands in the scaffold installer so `dreamland init` writes them alongside the existing per-agent commands
- [ ] 6.4 Tests: both commands installed on all six platforms per existing scaffold installer test conventions

## 7. Self-hosting bootstrap

- [ ] 7.1 Run the importer against this repository's actual installed files to produce the first real `.dreamland/workflow-graph.json`
- [ ] 7.2 Hand-resolve any routing edges the importer flagged as unresolved
- [ ] 7.3 Commit the bootstrapped `.dreamland/workflow-graph.json`

## 8. Validation

- [ ] 8.1 End-to-end: add a routing edge via `/hypnos-interactive`, confirm all six platform files updated and content matches the regenerated hand-off sentence
- [ ] 8.2 End-to-end: create a new agent via the editor, confirm tool-tier assignment and six-platform file creation match what `hypnos` would produce by hand
- [ ] 8.3 End-to-end: hand-edit a platform file, then attempt a conflicting save, confirm it's blocked with the conflict surfaced
- [ ] 8.4 End-to-end: `/hypnos-view` open during an in-progress OpenSpec task shows the status indicator update without a manual reload
