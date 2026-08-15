## 1. Graph node taxonomy and cache

- [ ] 1.1 Define Go structs for the four node types: `ProjectNode` (singleton, no fields beyond repo root), `AgentNode` (`id`, `description`, `tier` enum, `broadRouting` bool, `instructionBody`, `platformFiles map[string]string`, `pos [2]float64`), `HookNode` (`command`, `event` enum: `turn_end`/`turn_start`/`pre_tool_use`/`session_start`, `scope`: `project`/`agent`), `SkillNode` (`id`, `description`, `owner`: `dreamland`/`external`)
- [ ] 1.2 Define edge/slot types matching the litegraph typed-slot model: `routing` (Agent.routes_to output -> Agent.routed_from input), `hookbinding` (Hook.bound_to output -> Project.hooks or Agent.hooks input), `attachment` (Skill.available_to output -> Agent.skills input)
- [ ] 1.3 Add `.dreamland/workflow-graph.json` to `.gitignore` — regenerated cache, not committed
- [ ] 1.4 Implement the live-rebuild importer for `AgentNode`s: scan the six live platform directories (`.claude/agents`, `.cursor/rules`, `.codex/agents`, `.kiro/steering`, `.agents/skills`, `.github/agents`), mapping `id`/`description`/`instructionBody` per the platform file mapping in design.md's Graph Node Taxonomy table; derive `tier` by reverse-mapping `tools:`/capability keys on Claude Code, Copilot, and Codex (deferring to whatever `internal/scaffold`'s existing Codex writer already emits — do not invent a new Codex capability-key scheme); Cursor/Kiro/Antigravity agents import with no `tier` value (pre-existing, no representation on those platforms)
- [ ] 1.5 Implement best-effort `routes_to` extraction from each agent's stripped instruction-body prose ("hand off directly to `X`" canonical pattern); leave unresolved routing edge-less and flag the node instead of guessing
- [ ] 1.6 Implement the live-rebuild importer for `HookNode`s: parse `.claude/settings.json`'s `hooks.<Event>[]` arrays into project-scoped nodes (one node per distinct command+event pair, `scope: project`); parse Claude Code and GitHub Copilot per-agent frontmatter `hooks:` blocks into agent-scoped nodes bound to that one agent; a command that appears both ways (e.g. `telemetry write` in both the workspace `SubagentStop` array and every agent's frontmatter `Stop` block) imports as two separate `HookNode`s, not one deduplicated node — they're independent bindings
- [ ] 1.7 Implement the live-rebuild importer for `SkillNode`s: scan `.claude/skills/*/SKILL.md` (and platform equivalents), `owner: external` unless the file carries the `dreamland-managed` marker
- [ ] 1.8 Implement the singleton `ProjectNode` (always present, exactly one instance, non-deletable in the UI)
- [ ] 1.9 Wire the importer to run at server start and to be re-triggered by the filesystem watcher (§3), not just when the cache file is missing
- [ ] 1.10 Tests: importer against this repo's actual installed files produces the expected node set (ten agents, the four `openspec-*` skills as `owner: external`, the project-scoped and agent-scoped hook nodes matching `.claude/settings.json` and the per-agent frontmatter); unresolved-edge flagging path; cache never required to exist for a rebuild to succeed

## 2. litegraph.js node classes and dynamic multi-input slots

- [ ] 2.1 Register the four litegraph node classes (`dreamland/project`, `dreamland/agent`, `dreamland/hook`, `dreamland/skill`) via `LiteGraph.registerNodeClass`, with the typed slots from §1.2
- [ ] 2.2 Implement the dynamic multi-input-slot pattern in `onConnectionsChange` for `AgentNode.routed_from`, `AgentNode.hooks`, and `AgentNode.skills`: always keep one trailing empty slot of the slot's type; connecting to it appends a new empty slot; disconnecting the last link on a non-trailing slot removes it and closes the gap
- [ ] 2.3 Confirm `routes_to`'s native output fan-out needs no special handling (litegraph outputs already support multiple outgoing links)
- [ ] 2.4 Tests (JS, headless litegraph if feasible, otherwise manual test plan documented): connecting a third source to an agent's `routed_from` grows a third input slot; disconnecting a middle link on a dynamic slot closes the gap without leaving an orphaned empty slot in the middle

## 3. Graph-driven scaffold writer integration

- [ ] 3.1 Extract/adapt `internal/scaffold`'s per-platform agent-formatting functions (used today by `dreamland init`/`hypnos`) to accept an `AgentNode` as input instead of only a static embedded template string
- [ ] 3.2 Implement per-platform sync: given a changed node/edge, determine the minimal set of platform files to regenerate
- [ ] 3.3 Implement the instruction-body hand-off sentence regeneration from `routes_to` (add/remove sentence on edge add/remove), in the canonical form the importer (1.5) parses back
- [ ] 3.4 Implement agent creation (new `AgentNode` -> six platform files: tool-tier assignment per existing router-excluded/full-edit/write-only-no-edit matrix, plus the platform's standard telemetry/coauthor/commit hook baseline — Claude Code's 5-command `hooks.Stop` block, GitHub Copilot's `SubagentStart`/4-command `SubagentStop` block, both parameterized by the new agent's name — matching what every existing agent already has; Cursor/Codex/Kiro/Antigravity get standard frontmatter/instruction-body boilerplate only, no hook block)
- [ ] 3.5 Implement agent deletion/archival (six platform files removed/archived, same process `mengpo` uses; dependent `routes_to` edges cleaned up and their sources regenerated)
- [ ] 3.6 Implement hook attach/detach with project-vs-agent scope resolution per platform: a `HookNode` wired to the `ProjectNode` writes to `.claude/settings.json`'s workspace `hooks.<Event>[]` array (mapping the abstracted `event` enum to the real key); a `HookNode` wired directly to one or more `AgentNode`s writes to each of their per-agent `hooks:` frontmatter blocks; on GitHub Copilot (no workspace mechanism) a `ProjectNode`-wired hook falls back to writing the identical binding onto every agent's frontmatter individually
- [ ] 3.7 Implement skill attach/detach: wiring an existing `SkillNode` to an `AgentNode` records the reference on the agent's side only — never modifies the skill's own file (see §4 for skill creation)
- [ ] 3.8 Every write handler rebuilds the graph from current on-disk state immediately before applying its mutation (no mutating a stale in-memory snapshot)
- [ ] 3.9 Tests: edge add/remove syncs all six platforms; agent create/delete syncs all six platforms; hook attach lands project-scoped on Claude Code (in `.claude/settings.json`) and agent-scoped on GitHub Copilot; a hand edit made just before a save is preserved, not overwritten; a newly created agent's Claude Code and GitHub Copilot hook blocks match an existing agent's, modulo agent name; attaching an existing skill does not modify the skill's own file

## 4. Skill-authoring writer (new capability, not a reuse)

- [ ] 4.1 Confirm current state before building: `internal/scaffold` has no skill-writing code path; this repo's four `.claude/skills/openspec-*` files are stamped `generatedBy` the external `openspec` CLI, not `dreamland` — do not extend the agent writer for this, write new code
- [ ] 4.2 Define the `SKILL.md` (and platform-equivalent) output format for a `dreamland`-authored skill: `name`, `description` frontmatter at minimum, matching the shape of the existing `openspec-*` skills closely enough to render/behave the same way on each platform, without claiming the `openspec` CLI's own `metadata.generatedBy`/`license`/`compatibility` fields
- [ ] 4.3 Implement per-platform skill file writers (Claude Code `.claude/skills/<id>/SKILL.md`, and platform-equivalent skill directories on Codex/Antigravity; note which platforms have no skill concept at all and skip them)
- [ ] 4.4 Implement `SkillNode` creation through this writer, distinct from `AgentNode` creation's writer path
- [ ] 4.5 Tests: creating a new skill node writes the new `SKILL.md` correctly and does not touch any existing `openspec-*` skill file; a skill node created this way carries `owner: dreamland`, distinguishing it from the four `owner: external` ones on next live-rebuild

## 5. Filesystem watcher and live refresh

- [ ] 5.1 Add a filesystem watcher over the six live platform directories and the active OpenSpec change directory
- [ ] 5.2 On a watched change, trigger a graph rebuild (§1) and push a refresh event to connected clients
- [ ] 5.3 Add an SSE endpoint the browser UI subscribes to for refresh events
- [ ] 5.4 Tests: a change to a watched file triggers a rebuild and an SSE event within a bounded time; unrelated file changes outside the watched paths don't trigger a rebuild

## 6. Concurrency: advisory lock

- [ ] 6.1 Add a `.dreamland/hypnos.lock` advisory-lock helper (OS-level `flock` semantics)
- [ ] 6.2 Wrap every write operation (interactive save request, full `apply-plan` run) in acquire-lock / rebuild-from-disk / apply / release-lock
- [ ] 6.3 Tests: two concurrent write operations against the same repo serialize correctly, neither corrupting the result; lock is released promptly after a single operation completes, not held across a browser session

## 7. Local server

- [ ] 7.1 Add `cmd/hypnosserve.go`: `dreamland hypnos-serve --mode=view|interactive|apply-plan`, binds `127.0.0.1:0` (ephemeral port) for view/interactive, following the `cmd/serve.go` cobra-subcommand pattern
- [ ] 7.2 Mount read-only graph routes (fetch current graph, fetch task/change status, SSE refresh endpoint) in view and interactive modes
- [ ] 7.3 Mount write routes (node/edge create/update/delete) only when `--mode=interactive`; verify no route exists for them in view mode
- [ ] 7.4 Serve the embedded litegraph.js UI as static assets via `go:embed`
- [ ] 7.5 Print the local URL and open it in the default browser on start
- [ ] 7.6 Tests: view-mode server has no mutation route reachable (404, not 403); interactive-mode server accepts a mutation and it lands in the graph cache and platform files

## 8. Plan-driven headless apply (hypnos) and Janus dispatch

- [ ] 8.1 Define the plan file format: an ordered JSON array reusing the same operation types (`create_node`, `update_node`, `delete_node`, `create_edge`, `delete_edge`) the interactive editor's write routes accept
- [ ] 8.2 Add `--mode=apply-plan --plan <file>` to `cmd/hypnosserve.go`: acquires the lock, rebuilds the graph from current disk state, validates all referenced node ids resolve (creations earlier in the plan count), applies each operation via the same handlers/sync from §3-4, releases the lock, reports per-operation success/failure, exits without opening a port
- [ ] 8.3 Reject the whole plan before any write if an operation references a node id that doesn't exist and isn't created earlier in the same plan
- [ ] 8.4 Update `hypnos`'s own instructions in `internal/scaffold/templates/agents/*/hypnos.*` (all six platforms) to state the plan-apply responsibility and the Janus in-place-of-`morpheus` dispatch rule for graph-structural OpenSpec changes
- [ ] 8.5 Update `janus`'s own instructions in `internal/scaffold/templates/agents/*/janus.*` (all six platforms): widen the existing "dispatches agent-roster tasks to Hypnos or Meng Po via /opsx:apply" rule to also route workflow-graph-structural tasks (routing-edge changes, hook attach/detach, skill creation) to `hypnos`, per the modified `janus-router-agent` requirement
- [ ] 8.6 Tests: applying a plan produces the same end state as the equivalent manual edits; no server starts in apply-plan mode; invalid node reference rejects the whole plan; a plan-apply run and an interactive save overlapping serialize via the lock (§6); a scaffold-installer test asserting all six `janus.*` files route workflow-graph-structural tasks to `hypnos`

## 9. litegraph.js UI

- [ ] 9.1 Vendor `litegraph.js` as an embedded static asset (no CDN dependency)
- [ ] 9.2 Render the four node types and their edges from the server's graph endpoint
- [ ] 9.3 Interactive mode: node drag/position save, edge draw/delete, agent/skill node create dialog (role/description input), node delete with confirmation
- [ ] 9.4 Surface hook scope in the UI: connecting a hook to the `Project` node vs. a specific agent node is how the user expresses project vs. agent scope — no separate scope toggle widget needed, the wire target *is* the scope choice
- [ ] 9.5 Subscribe to the SSE refresh endpoint and re-render on incoming events, both view and interactive mode
- [ ] 9.6 Visual flag for nodes with unresolved routing (from the live-rebuild importer)
- [ ] 9.7 Visual distinction for `owner: external` skill nodes (e.g. locked/greyed attach point) vs. `owner: dreamland` ones

## 10. Slash commands

- [ ] 10.1 Add `/hypnos-view` template for all six platforms (`internal/scaffold/templates/commands/*`), following `router-slash-commands` per-platform conventions; invokes `dreamland hypnos-serve --mode=view`
- [ ] 10.2 Add `/hypnos-interactive` template for all six platforms; invokes `dreamland hypnos-serve --mode=interactive`
- [ ] 10.3 Register both commands in the scaffold installer so `dreamland init` writes them alongside the existing per-agent commands
- [ ] 10.4 Tests: both commands installed on all six platforms per existing scaffold installer test conventions

## 11. Self-hosting bootstrap (validation instance, not the feature's primary target)

- [ ] 11.1 Run the live-rebuild importer against this repository's actual installed files, confirming it produces a correct graph without needing a committed cache
- [ ] 11.2 Hand-resolve any routing edges the importer flagged as unresolved by fixing the source platform file's hand-off sentence to the canonical pattern
- [ ] 11.3 Update this repository's own live `hypnos` and `janus` agent files to match the template changes in 8.4 and 8.5

## 12. Validation

- [ ] 12.1 End-to-end: add a routing edge via `/hypnos-interactive`, confirm all six platform files updated and content matches the regenerated hand-off sentence
- [ ] 12.2 End-to-end: create a new agent via the editor, confirm tool-tier assignment, hook baseline, and six-platform file creation match what `hypnos` would produce by hand
- [ ] 12.3 End-to-end: create a new skill via the editor, confirm the new `SKILL.md` is written and no existing `openspec-*` skill file is touched
- [ ] 12.4 End-to-end: hand-edit a platform file while `/hypnos-view` is open, confirm the change appears via refresh-on-update without restarting the server or reloading the page
- [ ] 12.5 End-to-end: `/hypnos-view` open during an in-progress OpenSpec task shows the status indicator update without a manual reload
- [ ] 12.6 End-to-end: run `dreamland hypnos-serve --mode=apply-plan --plan <file>` with a plan equivalent to 12.1's manual edit, confirm identical resulting files
- [ ] 12.7 End-to-end: start an interactive save and an `apply-plan` run concurrently against the same repo, confirm both complete correctly and serially, with no corrupted file
