## Context

Today the framework's structure — ten agents, their per-agent slash commands, their hooks, and Janus's routing table — exists only as six parallel sets of platform-native files (`.claude/agents/*.md`, `.cursor/rules/*.mdc`, `.codex/agents/*.toml`, `.kiro/steering/*.md`, `.agents/skills/*/SKILL.md`, `.github/agents/*.agent.md`), written by `internal/scaffold`'s per-platform embed templates and kept in sync by hand whenever `hypnos` authors a new agent or `mengpo` retires one. Routing/hand-off edges between agents are **prose**, not data: an agent's file says "hand off directly to `phobetor`" inside its markdown instruction body. There is no structured, parseable graph today — `openspec/specs/agent-scaffolding` and `router-slash-commands` describe the six file formats per agent/command, not a queryable relationship model.

This change is scoped to the current repository's own **installed** platform files (the six directories above, as they exist after `dreamland init` has run here), not `internal/scaffold/templates/` (the factory-default source used only when scaffolding a brand-new repo). "Syncs to all workflows in the repo" means those six live directories.

**The six live platform files are and remain the canonical source of truth.** The graph the UI renders is a live-reloaded cache derived from them, not a second store — this shapes every decision below.

## Goals / Non-Goals

**Goals:**
- One litegraph.js graph, not six: a single live view of agents/skills/hooks/routing rebuilt from the six platform files, not a persisted source of truth competing with them.
- `/hypnos-view`: read-only snapshot of the graph, including live OpenSpec task/change status, safe to open mid-session without risk of mutating anything, refreshing itself as the underlying files change ("refresh on update").
- `/hypnos-interactive`: full editor — move/connect/disconnect nodes, create/attach/detach a skill, agent, or hook — writes land in the six platform files via the same writer logic `hypnos`/`mengpo` use today (new caller, not new logic).
- `hypnos` (the agent) can implement a workflow-graph change on its own, headlessly, from a plan it authors itself while working a `phantasos`-authored OpenSpec change — the same mutation operations the browser UI issues, applied through the identical writer/sync path, with no server listener or browser required.
- Hook/skill scope standardized at the project (workspace) level wherever a platform supports it, rather than proliferating per-agent copies — per-agent scope only where a platform requires it.
- Local-only server (`localhost`, ephemeral port), no new runtime dependency beyond a vendored `litegraph.js` static asset embedded in the binary (`go:embed`), consistent with `agent-scaffolding`'s "templates embedded at compile time, nothing read from the filesystem at install time" constraint.

**Non-Goals:**
- Multi-user/remote/collaborative editing. Single local user, single local machine; concurrency handling (below) covers same-machine concurrent processes, not networked collaboration.
- Editing OpenSpec artifact *content* (`proposal.md`/`design.md`/`tasks.md` prose) through the graph. `/hypnos-view`'s task-status overlay is read-only telemetry about those artifacts, not an editor for them.
- Platforms beyond the existing six.
- Free-form scripting of new hook *behavior* in the UI. The editor attaches/detaches existing known hook commands (`dreamland coauthor`, `dreamland telemetry write`, etc.) to nodes; it does not let a user type arbitrary shell into a node and have it become a hook.
- Replacing `hypnos`'s own agent-authoring responsibility. The editor is a second front end onto the same writer logic Hypnos already owns — it does not bypass Hypnos's tool-tier/routing-table rules.
- A bespoke plan-file DSL or a mechanical parser that maps `tasks.md` prose to graph mutations. The plan format is the same structured operation list the interactive editor's write routes already accept; `hypnos` (an LLM agent, not a parser) is the one translating a phantasos-authored change's `design.md`/`tasks.md` into that operation list, exactly as `morpheus` translates the same artifacts into code without `tasks.md` itself needing to be machine-parseable.
- A persisted, committed graph store. `.dreamland/workflow-graph.json` is a regenerable local cache, gitignored, not authoritative.

## Decisions

**The six live platform files are canonical; the graph is a live-reloaded cache, not a persisted source of truth.** Considered making `.dreamland/workflow-graph.json` the source of truth with the platform files generated as output (a prior draft of this design took that direction). Rejected on reconsideration: it would create a second store that can silently outlive or diverge from the actual harness state, and this project's routing/context design intent already favors agents pulling live context over an orchestrator-assembled cache. Instead: the graph is rebuilt from the six live directories every time it's needed (server start, and on every filesystem-watch-triggered change — see below), never trusted as a snapshot between rebuilds. `.dreamland/workflow-graph.json` still exists on disk as a fast-reload convenience for the UI and for `apply-plan` to read/write against within a single run, but it is gitignored and gets rebuilt, not relied upon, at the start of any new run.

**Structured `routes_to` sentences stay in the platform files' instruction-body prose, generated in one canonical, re-parseable form.** Hand-off edges are still expressed as an instruction-body sentence ("hand off directly to `X`"), not a new frontmatter field — adding a field would mean the graph's edit path and its live-reload path disagree about which representation is truth. Because the sentence the editor/plan-apply *writes* is always the same generated pattern, the importer that *reads* it back on the next rebuild can parse it with confidence — the fragility this avoids is only in hand-written, freeform prose (which the importer already tolerates by flagging it unresolved rather than guessing).

**Live rebuild on every server start and on every relevant filesystem change — not a one-time bootstrap.** There is no first-run/steady-state distinction: the importer that scans the six live directories and Janus's routing prose runs whenever the cache needs refreshing, full stop. Hand-off edges it can't confidently extract are still created as nodes but left edge-less, and flagged in the UI for manual connection — safer than silently guessing wrong routing.

**A filesystem watcher drives "refresh on update," not polling.** The server watches the six live platform directories plus the active OpenSpec change directory (for task/change status) and, on any relevant change, rebuilds the cache and pushes a refresh to connected browser clients over Server-Sent Events. Applies to both `/hypnos-view` (its whole reason for existing — watching work as it happens) and `/hypnos-interactive` (so a concurrently hand-edited file, or a `hypnos` plan-apply run, shows up without a manual reload). Rejected: fixed-interval polling — adds latency for no reason when the harness can just tell the server what changed.

**Hook (and skill) scope defaults to project/workspace level wherever the platform supports it; per-agent scope only where the platform requires it.** Matches this project's own existing pattern (Claude Code's `SubagentStop`/`PreToolUse` hooks are already unscoped workspace arrays covering every agent, confirmed in `claude-code-parity`'s design) and reduces the graph to one project-level hook node with edges to "applies to everything" instead of ten duplicate per-agent bindings. GitHub Copilot, which has no unscoped hook mechanism (per the same prior design doc — Copilot hooks are either a fixed workspace event list or per-agent frontmatter each agent declares individually), keeps per-agent scope there; the graph model records a hook node's scope (`project` or `agent`) and the writer picks the right target per platform's actual capability, not a uniform choice.

**Sync writes are diffed per platform file, not full re-scaffold.** Reuses `internal/scaffold`'s existing per-platform formatting functions (the same ones `dreamland init`/`hypnos` call) but only rewrites files whose derived content actually changed, and only the files touched by the specific node/edge edited — avoids clobbering unrelated hand-edits elsewhere in a platform's directory the way a full `--force` re-scaffold would.

**Server: `dreamland hypnos-serve --mode=view|interactive|apply-plan`, `net/http` stdlib, localhost-only, ephemeral port.** Matches the existing `cmd/serve.go` (MCP server) pattern of a dedicated cobra subcommand; no new HTTP framework dependency. The two slash commands (`/hypnos-view`, `/hypnos-interactive`) just invoke this command with the matching `--mode` and open the printed URL — mirrors how other dreamland slash commands shell out to the CLI binary. `--mode=view` never mounts the write/save HTTP routes at all (not just a disabled UI button), so a view-mode server has no code path capable of mutating any platform file.

**Plan file is a list of the same mutation operations the interactive editor's write routes accept — not a second schema.** `dreamland hypnos-serve --mode=apply-plan --plan <file>` reads an ordered JSON array of operations (`create_node`, `update_node`, `delete_node`, `create_edge`, `delete_edge` — the same operation types the writer integration defines for the HTTP write routes) and applies them one at a time through the identical handler functions. `hypnos` authors this plan file itself, from a `phantasos`-authored change's `design.md`/`tasks.md` — Janus dispatches a graph-structural OpenSpec change to `hypnos` in place of `morpheus` the same way it dispatches code changes to `morpheus` today, per `janus-router-agent`. `--mode=apply-plan` performs no `net/http` listen: it acquires the write lock (below), rebuilds the cache from current disk state, applies the plan, reports per-operation results, and exits.

**Concurrent writers are serialized with a single advisory file lock — the least-overhead option for a same-machine, single-user tool.** `/hypnos-interactive`'s server process and a `hypnos`-driven `--mode=apply-plan` run are separate OS processes that can touch the same repo at the same time (e.g. a background subagent applying a plan while a human has the browser editor open). Considered: no locking at all ("don't do that," rejected — this is a real, foreseeable Claude Code scenario, a subagent and a human working the same repo concurrently, not a hypothetical); a long-running daemon owning all writes (rejected — extra process to manage, no benefit over a lock for this access pattern); CRDT/merge-based conflict resolution (rejected — large complexity for a problem two processes taking turns already solves). What ships: a single `.dreamland/hypnos.lock` file, taken with an OS-level advisory lock (`flock` semantics) for the duration of one write operation (one interactive save request, or one full `apply-plan` run) and released immediately after — no daemon, no queue, blocks briefly rather than corrupting anything.

**`hypnos`'s own instructions gain the plan-apply responsibility, and the Janus dispatch rule, in the same place every other agent capability lives: its per-platform template files.** Because `hypnos` is one of the ten framework-shipped agents, its role description is authored in `internal/scaffold/templates/agents/*/hypnos.*` (the source `dreamland init` uses for every repo, not just this one) — distinct from the live per-repo platform files this change's editor operates on. This change updates both: the shipped templates (so every future `dreamland init` gives `hypnos` this ability) and this repository's own installed `hypnos` files (self-hosting bootstrap, §Migration Plan), the same two-tier pattern prior changes (e.g. `claude-code-parity`) already followed when an agent's responsibilities changed.

**litegraph.js vendored as a single embedded static asset, no CDN fetch at runtime.** Matches the project's offline-friendly CLI posture (already true of every other scaffolded artifact, which is embedded at compile time per `agent-scaffolding`).

## Risks / Trade-offs

[Two processes (browser-driven interactive save, and a `hypnos` `apply-plan` run) write at the same moment] → advisory file lock serializes writes; a losing process blocks briefly rather than corrupting the cache or a platform file.

[Vendored `litegraph.js` increases binary size] → single minified asset, embedded like existing template content; acceptable one-time cost, no runtime download.

[Regenerating the hand-off sentence from `routes_to` loses hand-tuned phrasing] → only the hand-off sentence(s) are templated; the rest of an agent's instruction body is never touched by sync — read from and written back to the platform file verbatim.

[Local server accidentally binds non-localhost and exposes the editor's write routes] → hard-bind to `127.0.0.1` with an ephemeral port (`:0`), never configurable to `0.0.0.0`; interactive-mode write routes only exist in that mode's route table, per the decision above.

[Filesystem watcher misses or coalesces rapid changes, leaving the browser stale] → rebuild is idempotent and cheap (same importer used at server start); worst case is a slightly delayed refresh, not incorrect data, since the next watch event or manual reload rebuilds from current disk truth regardless.

## Migration Plan

1. Add the live-rebuild importer (scan six live directories + Janus routing prose, best-effort `routes_to` extraction, flag unresolved edges) and the `.dreamland/workflow-graph.json` cache read/write helpers; add the cache path to `.gitignore`.
2. Add graph-driven writer entrypoints in `internal/scaffold`, delegating to the existing per-platform formatting logic `hypnos`/`mengpo` already call, plus the hook/skill project-vs-agent scope resolution per platform.
3. Add the advisory-lock helper (`.dreamland/hypnos.lock`) wrapping every write operation.
4. Add the filesystem watcher (six live directories + active OpenSpec change dir) driving cache rebuild and an SSE push endpoint.
5. Add `dreamland hypnos-serve` (view and interactive modes), embedded `litegraph.js` UI subscribing to the SSE endpoint, ephemeral localhost port.
6. Add `dreamland hypnos-serve --mode=apply-plan`, reusing the mutation handlers and lock from steps 2-3.
7. Add `/hypnos-view` and `/hypnos-interactive` slash-command templates across all six platforms, following `router-slash-commands` conventions.
8. Update `hypnos`'s own instructions in `internal/scaffold/templates/agents/*/hypnos.*` (all six platforms) to state the plan-apply responsibility and the Janus in-place-of-`morpheus` dispatch rule.
9. Run the importer against this repository (dreamland is self-hosted) as the first real rebuild, hand-resolve any flagged edges by fixing the source platform file's hand-off sentence to the canonical pattern, and update this repo's own live `hypnos` files to match step 8.

No rollback concerns beyond deleting `.dreamland/workflow-graph.json` (gitignored, nothing lost) and the two commands — the six platform files remain valid, hand-editable files with or without the graph layer present.

## Open Questions

None outstanding. All prior open questions resolved above: cache is gitignored and live-reloaded (not committed), refresh is watch-driven (not polling), hook/skill scope standardizes at project level with per-platform fallback, `hypnos` plan authorship comes from a phantasos-authored change dispatched via Janus in place of a coding agent, and concurrent writers are serialized with a single advisory lock.
