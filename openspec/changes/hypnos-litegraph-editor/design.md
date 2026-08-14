## Context

Today the framework's structure — ten agents, their per-agent slash commands, their hooks, and Janus's routing table — exists only as six parallel sets of platform-native files (`.claude/agents/*.md`, `.cursor/rules/*.mdc`, `.codex/agents/*.toml`, `.kiro/steering/*.md`, `.agents/skills/*/SKILL.md`, `.github/agents/*.agent.md`), written by `internal/scaffold`'s per-platform embed templates and kept in sync by hand whenever `hypnos` authors a new agent or `mengpo` retires one. Routing/hand-off edges between agents are **prose**, not data: an agent's file says "hand off directly to `phobetor`" inside its markdown instruction body. There is no structured, parseable graph today — `openspec/specs/agent-scaffolding` and `router-slash-commands` describe the six file formats per agent/command, not a queryable relationship model.

This change is scoped to the current repository's own **installed** platform files (the six directories above, as they exist after `dreamland init` has run here), not `internal/scaffold/templates/` (the factory-default source used only when scaffolding a brand-new repo). "Syncs to all workflows in the repo" means those six live directories.

## Goals / Non-Goals

**Goals:**
- One litegraph.js graph, not six: a single canonical model of agents/skills/hooks/routing that the six platform files are generated *from*, so editing once fans out everywhere.
- `/hypnos-view`: read-only snapshot of the graph, including live OpenSpec task/change status, safe to open mid-session without risk of mutating anything.
- `/hypnos-interactive`: full editor — move/connect/disconnect nodes, create/attach/detach a skill, agent, or hook — writes land in the six platform files via the same writer logic `hypnos`/`mengpo` use today (new caller, not new logic).
- `hypnos` (the agent) can implement a workflow-graph change on its own, headlessly, from a plan file — the same mutation operations the browser UI issues, applied through the identical writer/sync/drift-detection path, with no server listener or browser required.
- Local-only server (`localhost`, ephemeral port), no new runtime dependency beyond a vendored `litegraph.js` static asset embedded in the binary (`go:embed`), consistent with `agent-scaffolding`'s "templates embedded at compile time, nothing read from the filesystem at install time" constraint.

**Non-Goals:**
- Multi-user/remote/collaborative editing. Single local user, single local server instance.
- Editing OpenSpec artifact *content* (`proposal.md`/`design.md`/`tasks.md` prose) through the graph. `/hypnos-view`'s task-status overlay is read-only telemetry about those artifacts, not an editor for them.
- Platforms beyond the existing six.
- Free-form scripting of new hook *behavior* in the UI. The editor attaches/detaches existing known hook commands (`dreamland coauthor`, `dreamland telemetry write`, etc.) to nodes; it does not let a user type arbitrary shell into a node and have it become a hook.
- Replacing `hypnos`'s own agent-authoring responsibility. The editor is a second front end onto the same writer logic Hypnos already owns — it does not bypass Hypnos's tool-tier/routing-table rules.
- A bespoke plan-file DSL or a mapping from `tasks.md` prose to graph mutations. The plan format is the same structured operation list the interactive editor's write routes already accept — no new parsing surface.

## Decisions

**A canonical `graph.json` becomes the source of truth; the six platform files become generated output.** Considered parsing the six platform files live on every server start and writing edits back into their prose in place (no new file). Rejected: hand-off edges are free-text ("hand off directly to `phobetor`") with no stable location to parse or rewrite reliably across six different formats, and a bidirectional prose-diff editor is far more fragile than a structured model with a codegen step. Instead: `.dreamland/workflow-graph.json` stores, per agent/skill/hook node, its id, description, tool tier, hook bindings, structured `routes_to` edges, and the shared instruction-body markdown; per-platform frontmatter differences (Claude Code `tools:` list vs. Codex capability keys, etc.) stay exactly where they already live — in `internal/scaffold`'s per-platform formatting logic — now driven by graph nodes instead of static embedded strings.

**Structured `routes_to` replaces "figure it out from prose."** Every agent's hand-off targets become an explicit field the graph (and Hypnos, going forward) maintains, instead of only being asserted in instruction-body sentences. The instruction body keeps its human-readable "hand off directly to X" sentence too (regenerated from `routes_to` at sync time, so they can't drift from each other) — this is additive to the existing convention, not a replacement of the prose agents actually read.

**Bootstrap import, not a hand-authored graph.json from scratch.** First run of either slash command with no `.dreamland/workflow-graph.json` present triggers a one-time importer that scans the six live directories and Janus's routing prose to build an initial graph. Hand-off edges it can't confidently extract are still created as nodes but left edge-less, and flagged in the UI for manual connection — safer than silently guessing wrong routing.

**Sync writes are diffed per platform file, not full re-scaffold.** Reuses `internal/scaffold`'s existing per-platform formatting functions (the same ones `dreamland init`/`hypnos` call) but only rewrites files whose derived content actually changed, and only the files touched by the specific node/edge edited — avoids clobbering unrelated hand-edits elsewhere in a platform's directory the way a full `--force` re-scaffold would.

**Server: `dreamland hypnos-serve --mode=view|interactive`, `net/http` stdlib, localhost-only, ephemeral port.** Matches the existing `cmd/serve.go` (MCP server) pattern of a dedicated cobra subcommand; no new HTTP framework dependency. The two slash commands (`/hypnos-view`, `/hypnos-interactive`) just invoke this command with the matching `--mode` and open the printed URL — mirrors how other dreamland slash commands shell out to the CLI binary. `--mode=view` never mounts the write/save HTTP routes at all (not just a disabled UI button), so a view-mode server has no code path capable of mutating `.dreamland/workflow-graph.json` or any platform file.

**Plan file is a list of the same mutation operations the interactive editor's write routes accept — not a second schema.** `dreamland hypnos-serve --mode=apply-plan --plan <file>` reads an ordered JSON array of operations (`create_node`, `update_node`, `delete_node`, `create_edge`, `delete_edge` — the same operation types §3's writer integration already has to define for the HTTP write routes) and applies them one at a time through the identical handler functions, including the drift check before each write. Considered a bespoke "plan" DSL closer to `tasks.md`'s checklist prose; rejected — it would need its own parser and its own mapping onto graph mutations, duplicating work the interactive editor already requires, and reintroducing the same prose-parsing fragility the `routes_to` decision above moved away from. Reusing the operation list means the interactive editor and `hypnos`'s headless mode are two callers of one mutation API, not two implementations. `--mode=apply-plan` performs no `net/http` listen — it loads the plan, applies it, reports results, and exits.

**`hypnos`'s own instructions gain the plan-apply responsibility in the same place every other agent capability lives: its per-platform template files.** Because `hypnos` is one of the ten framework-shipped agents, its role description is authored in `internal/scaffold/templates/agents/*/hypnos.*` (the source `dreamland init` uses for every repo, not just this one) — distinct from the live per-repo `.dreamland/workflow-graph.json`/platform files this change's editor operates on. This change updates both: the shipped templates (so every future `dreamland init` gives `hypnos` this ability) and this repository's own installed `hypnos` files (self-hosting bootstrap, §Migration Plan step 5), the same two-tier pattern prior changes (e.g. `claude-code-parity`) already followed when an agent's responsibilities changed.

**litegraph.js vendored as a single embedded static asset, no CDN fetch at runtime.** Matches the project's offline-friendly CLI posture (already true of every other scaffolded artifact, which is embedded at compile time per `agent-scaffolding`).

## Risks / Trade-offs

[Someone hand-edits a platform file directly (e.g. `.claude/agents/hypnos.md`) after `graph.json` exists, bypassing the editor] → sync includes a drift check: before writing, compare the file's current on-disk content against what `graph.json` last generated; on mismatch, surface it in the UI as a conflict requiring an explicit "keep disk" or "overwrite from graph" choice, rather than silently clobbering the hand-edit.

[Vendored `litegraph.js` increases binary size] → single minified asset, embedded like existing template content; acceptable one-time cost, no runtime download.

[Regenerating instruction-body prose from `routes_to` loses hand-tuned phrasing] → only the hand-off sentence(s) are templated from `routes_to`; the rest of the instruction body is stored and round-tripped verbatim as free text in `graph.json`, not regenerated.

[Local server accidentally binds non-localhost and exposes the editor's write routes] → hard-bind to `127.0.0.1` with an ephemeral port (`:0`), never configurable to `0.0.0.0`; interactive-mode write routes only exist in that mode's route table, per the decision above.

## Migration Plan

1. Add `.dreamland/workflow-graph.json` schema + the bootstrap importer (scan six live directories, best-effort `routes_to` extraction, flag unresolved edges).
2. Add graph-driven writer entrypoints in `internal/scaffold`, delegating to the existing per-platform formatting logic `hypnos`/`mengpo` already call, plus the drift check.
3. Add `dreamland hypnos-serve` (view and interactive modes), embedded `litegraph.js` UI, ephemeral localhost port.
4. Add `dreamland hypnos-serve --mode=apply-plan`, reusing the mutation handlers from step 2.
5. Add `/hypnos-view` and `/hypnos-interactive` slash-command templates across all six platforms, following `router-slash-commands` conventions.
6. Update `hypnos`'s own instructions in `internal/scaffold/templates/agents/*/hypnos.*` (all six platforms) to state the new plan-apply responsibility.
7. Run the importer against this repository (dreamland is self-hosted) as the first real bootstrap, hand-resolve any flagged edges, and update this repo's own live `hypnos` agent files to match step 6.

No rollback concerns beyond deleting `.dreamland/workflow-graph.json` and the two commands — the six platform files remain valid, hand-editable files with or without the graph layer present.

## Open Questions

- Is `.dreamland/workflow-graph.json` committed to git as canonical source, or treated as a derived/regenerable cache? Leaning committed (same status as OpenSpec artifacts), needs confirmation.
- Should `/hypnos-view`'s task-status overlay poll the filesystem or does it need a push mechanism for near-real-time "watch work as it happens"? Polling is simpler and likely sufficient given task cadence; revisit if it feels laggy in practice.
- Workspace-level hooks (e.g. Claude Code's `SubagentStop` array in `.claude/settings.json`, not per-agent frontmatter) need a node representation distinct from per-agent hook bindings — exact shape TBD in tasks.
