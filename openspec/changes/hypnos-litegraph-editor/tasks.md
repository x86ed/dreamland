## 1. Graph cache and live rebuild

- [ ] 1.1 Define the graph cache schema (Go structs): agent/skill/hook nodes with id, description, tool tier, hook bindings (with `scope: project|agent`), `routes_to` edges, shared instruction-body markdown, and node position
- [ ] 1.2 Add `.dreamland/workflow-graph.json` to `.gitignore` — regenerated cache, not committed
- [ ] 1.3 Implement the live-rebuild importer: scan the six live platform directories (`.claude/agents`, `.claude/commands/drmlnd`, `.cursor/rules`, `.cursor/commands`, `.codex/agents`, `.codex/skills`, `.kiro/steering`, `.agents/skills`, `.github/agents`, `.github/prompts`) and Janus's routing prose, building agent/skill/hook nodes
- [ ] 1.4 Implement best-effort `routes_to` extraction from each agent's instruction-body prose ("hand off directly to `X`" canonical pattern); leave unresolved routing edge-less and flag the node instead of guessing
- [ ] 1.5 Wire the importer to run at server start and to be re-triggered by the filesystem watcher (§3), not just when the cache file is missing
- [ ] 1.6 Tests: importer against this repo's actual installed files produces the expected node set; unresolved-edge flagging path; cache never required to exist for a rebuild to succeed

## 2. Graph-driven scaffold writer integration

- [ ] 2.1 Extract/adapt `internal/scaffold`'s per-platform formatting functions (used today by `dreamland init`/`hypnos`) to accept a graph node as input instead of only a static embedded template string
- [ ] 2.2 Implement per-platform sync: given a changed node/edge, determine the minimal set of platform files to regenerate
- [ ] 2.3 Implement the instruction-body hand-off sentence regeneration from `routes_to` (add/remove sentence on edge add/remove), in the canonical form the importer (1.4) parses back
- [ ] 2.4 Implement agent creation (new node -> six platform files, tool-tier assignment per existing router-excluded/full-edit/write-only-no-edit matrix)
- [ ] 2.5 Implement agent deletion/archival (six platform files removed/archived, same process `mengpo` uses; dependent `routes_to` edges cleaned up and their sources regenerated)
- [ ] 2.6 Implement hook/skill attach/detach with project-vs-agent scope resolution per platform (project-level binding where the platform supports one — e.g. Claude Code's workspace `.claude/settings.json` hooks array; per-agent frontmatter only where required — e.g. GitHub Copilot)
- [ ] 2.7 Every write handler rebuilds the graph from current on-disk state immediately before applying its mutation (no mutating a stale in-memory snapshot)
- [ ] 2.8 Tests: edge add/remove syncs all six platforms; agent create/delete syncs all six platforms; hook attach lands project-scoped on Claude Code and agent-scoped on GitHub Copilot; a hand edit made just before a save is preserved, not overwritten

## 3. Filesystem watcher and live refresh

- [ ] 3.1 Add a filesystem watcher over the six live platform directories and the active OpenSpec change directory
- [ ] 3.2 On a watched change, trigger a graph rebuild (§1.3) and push a refresh event to connected clients
- [ ] 3.3 Add an SSE endpoint the browser UI subscribes to for refresh events
- [ ] 3.4 Tests: a change to a watched file triggers a rebuild and an SSE event within a bounded time; unrelated file changes outside the watched paths don't trigger a rebuild

## 4. Concurrency: advisory lock

- [ ] 4.1 Add a `.dreamland/hypnos.lock` advisory-lock helper (OS-level `flock` semantics)
- [ ] 4.2 Wrap every write operation (interactive save request, full `apply-plan` run) in acquire-lock / rebuild-from-disk / apply / release-lock
- [ ] 4.3 Tests: two concurrent write operations against the same repo serialize correctly, neither corrupting the result; lock is released promptly after a single operation completes, not held across a browser session

## 5. Local server

- [ ] 5.1 Add `cmd/hypnosserve.go`: `dreamland hypnos-serve --mode=view|interactive|apply-plan`, binds `127.0.0.1:0` (ephemeral port) for view/interactive, following the `cmd/serve.go` cobra-subcommand pattern
- [ ] 5.2 Mount read-only graph routes (fetch current graph, fetch task/change status, SSE refresh endpoint) in view and interactive modes
- [ ] 5.3 Mount write routes (node/edge create/update/delete) only when `--mode=interactive`; verify no route exists for them in view mode
- [ ] 5.4 Serve the embedded litegraph.js UI as static assets via `go:embed`
- [ ] 5.5 Print the local URL and open it in the default browser on start
- [ ] 5.6 Tests: view-mode server has no mutation route reachable (404, not 403); interactive-mode server accepts a mutation and it lands in the graph cache and platform files

## 6. Plan-driven headless apply (hypnos) and Janus dispatch

- [ ] 6.1 Define the plan file format: an ordered JSON array reusing the same operation types (`create_node`, `update_node`, `delete_node`, `create_edge`, `delete_edge`) the interactive editor's write routes accept
- [ ] 6.2 Add `--mode=apply-plan --plan <file>` to `cmd/hypnosserve.go`: acquires the lock, rebuilds the graph from current disk state, validates all referenced node ids resolve (creations earlier in the plan count), applies each operation via the same handlers/sync from §2, releases the lock, reports per-operation success/failure, exits without opening a port
- [ ] 6.3 Reject the whole plan before any write if an operation references a node id that doesn't exist and isn't created earlier in the same plan
- [ ] 6.4 Update `hypnos`'s own instructions in `internal/scaffold/templates/agents/*/hypnos.*` (all six platforms) to state the plan-apply responsibility and the Janus in-place-of-`morpheus` dispatch rule for graph-structural OpenSpec changes
- [ ] 6.5 Update `janus`'s own instructions in `internal/scaffold/templates/agents/*/janus.*` (all six platforms): widen the existing "dispatches agent-roster tasks to Hypnos or Meng Po via /opsx:apply" rule to also route workflow-graph-structural tasks (routing-edge changes, hook/skill attach/detach) to `hypnos`, per the modified `janus-router-agent` requirement
- [ ] 6.6 Tests: applying a plan produces the same end state as the equivalent manual edits; no server starts in apply-plan mode; invalid node reference rejects the whole plan; a plan-apply run and an interactive save overlapping serialize via the lock (§4); a scaffold-installer test asserting all six `janus.*` files route workflow-graph-structural tasks to `hypnos`

## 7. litegraph.js UI

- [ ] 7.1 Vendor `litegraph.js` as an embedded static asset (no CDN dependency)
- [ ] 7.2 Render nodes (agents, skills, hooks) and edges (routing, attachments) from the server's graph endpoint
- [ ] 7.3 Interactive mode: node drag/position save, edge draw/delete, node create dialog (role description input), node delete with confirmation, hook/skill attach/detach controls (surfacing project vs. agent scope per platform)
- [ ] 7.4 Subscribe to the SSE refresh endpoint and re-render on incoming events, both view and interactive mode
- [ ] 7.5 Visual flag for nodes with unresolved routing (from the live-rebuild importer)

## 8. Slash commands

- [ ] 8.1 Add `/hypnos-view` template for all six platforms (`internal/scaffold/templates/commands/*`), following `router-slash-commands` per-platform conventions; invokes `dreamland hypnos-serve --mode=view`
- [ ] 8.2 Add `/hypnos-interactive` template for all six platforms; invokes `dreamland hypnos-serve --mode=interactive`
- [ ] 8.3 Register both commands in the scaffold installer so `dreamland init` writes them alongside the existing per-agent commands
- [ ] 8.4 Tests: both commands installed on all six platforms per existing scaffold installer test conventions

## 9. Self-hosting bootstrap

- [ ] 9.1 Run the live-rebuild importer against this repository's actual installed files, confirming it produces a correct graph without needing a committed cache
- [ ] 9.2 Hand-resolve any routing edges the importer flagged as unresolved by fixing the source platform file's hand-off sentence to the canonical pattern
- [ ] 9.3 Update this repository's own live `hypnos` and `janus` agent files to match the template changes in 6.4 and 6.5

## 10. Validation

- [ ] 10.1 End-to-end: add a routing edge via `/hypnos-interactive`, confirm all six platform files updated and content matches the regenerated hand-off sentence
- [ ] 10.2 End-to-end: create a new agent via the editor, confirm tool-tier assignment and six-platform file creation match what `hypnos` would produce by hand
- [ ] 10.3 End-to-end: hand-edit a platform file while `/hypnos-view` is open, confirm the change appears via refresh-on-update without restarting the server or reloading the page
- [ ] 10.4 End-to-end: `/hypnos-view` open during an in-progress OpenSpec task shows the status indicator update without a manual reload
- [ ] 10.5 End-to-end: run `dreamland hypnos-serve --mode=apply-plan --plan <file>` with a plan equivalent to 10.1's manual edit, confirm identical resulting files
- [ ] 10.6 End-to-end: start an interactive save and an `apply-plan` run concurrently against the same repo, confirm both complete correctly and serially, with no corrupted file
