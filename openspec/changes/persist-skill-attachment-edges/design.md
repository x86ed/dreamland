## Context

Two requirements in `litegraph-workflow-editor`, both correct in isolation, collide:

- `spec.md:21-28` ("Editor writes always apply against freshly re-imported on-disk state"): every mutation, and the watcher's ~2s poll, rebuild the graph from the six live platform directories via `workflowgraph.Import` before doing anything else — `rebuildGraph` in `cmd/hypnosserve.go:116-130`.
- `spec.md:104-114` ("Existing skills are attach/detach-only"): wiring a skill to an agent SHALL NOT modify the skill's own file. `AttachSkill`/`DetachSkill` (`internal/workflowgraph/writer.go:308-335`) implement this as pure graph-edge mutation — `g.Edges = append(g.Edges, Edge{Kind: EdgeAttachment, ...})` — with no corresponding write to any platform file at all, agent's or skill's.

`Import` derives every node and edge type it can from disk. It cannot derive `EdgeAttachment`, because nothing on disk represents it — confirmed by `writer.go:302-307`'s own doc comment ("no platform's agent file format has a field for 'skills this agent may invoke' today \[checked all six\]"). The one existing precedent for state `Import` can't derive — `AgentNode.PosX`/`PosY` — is handled by a local cache (`AgentPosition`/`SavePositions`/`LoadPositions`, `graph.go:136-179`) that `rebuildGraph` merges forward after every `Import`. Attachment edges have no equivalent merge step, so they are unconditionally dropped by the very next rebuild — which, given `rebuildGraph` runs on every mutation and every ~2s poll tick, is effectively immediately.

This was a known, explicitly named gap when skill attach/detach was first built (`writer.go:302-307`; `openspec/changes/archive/2026-08-15-hypnos-litegraph-editor/tasks.md:29`: "closing that gap is future work if/when such a field is established, not invented here"). It is now the confirmed root cause of a live symptom: no agent node in the litegraph UI shows any attached skill, because every attachment made through the UI is lost before the graph is next rendered. (A second, independent contributor to that same symptom — all hooks currently being project-scoped, with none attached per-agent, so no agent shows an attached hook widget either — is a data/usage fact, not a code gap: `AttachHookToAgent` already exists and already persists correctly via each platform's real frontmatter/settings file. It is out of scope here; see Non-Goals.)

## Goals / Non-Goals

**Goals:**
- A skill attached through `/hypnos-interactive` or a `--mode=apply-plan` plan survives every rebuild trigger: the next mutation, the next watcher poll, and a full process restart.
- `AttachSkill`/`DetachSkill` remain edge-only and continue to never modify a skill's own file — the existing, tested invariant is unchanged, not just re-verified.
- Minimize surface area and risk: touch as few already-tested code paths as possible to close this specific gap.

**Non-Goals:**
- Adding a real "skills this agent may invoke" field to any platform's agent file format. Considered as Option 2 below and rejected for this change — not because it's wrong in principle, but because it is materially larger in scope than the gap being closed, and `writer.go`'s existing comment already correctly frames it as separate future work rather than something this fix should silently expand into.
- Fixing "no agent shows an attached hook widget." That is a consequence of no hook currently being attached at agent scope in any real repo, not a code defect — `AttachHookToAgent`/`agentHooksFor`/`syncAgent` already round-trip agent-scoped hooks correctly and are untouched by this change.
- Any change to `internal/workflowgraph/render.go`, any per-platform renderer, or the importer's agent-file parsing.

## Decisions

### Local cache mirroring the position-save pattern (chosen) vs. a real platform-file field (rejected)

**Option 1 — chosen: local persistence, `.dreamland/workflow-skill-attachments.json`, merged forward in `rebuildGraph` exactly like `AgentPosition`.**

- New `SkillAttachment{SkillID, AgentID string}` plus `SaveSkillAttachments`/`LoadSkillAttachments` in `internal/workflowgraph/graph.go`, same shape as `AgentPosition`/`SavePositions`/`LoadPositions`: `Save*` writes an indented JSON array (a slice, not a map, since attachment is many-to-many rather than keyed by a single agent id the way position is); `Load*` returns `nil, nil` on a missing file, matching `LoadPositions`' "absence is not an error" contract.
- `rebuildGraph` loads the cache after `Import` and, for each entry, re-adds an `EdgeAttachment` edge only if both `SkillID` and `AgentID` still resolve in the freshly-imported graph — silently dropping (not erroring on) an entry whose skill or agent was deleted elsewhere in the interim, the same forgiving posture the existing position-merge loop already takes (`if prev, ok := positions[id]; ok`).
- `SaveSkillAttachments` is called at the two existing `SavePositions` call sites (`/api/mutate`'s handler, `runApplyPlan`) — not inside `AttachSkill`/`DetachSkill` themselves. This is the same layering the position cache already uses: `workflowgraph` owns the data shape, `cmd` owns the file-path convention and *when* to save, and the graph-mutation primitives in `writer.go` stay pure in-memory operations callable identically from the interactive HTTP path and the headless `apply-plan` path.
- Gitignored, regenerated, non-canonical — added to `.gitignore` and `cmd/init.go`'s `EnsureGitignoreEntry` calls next to `.dreamland/workflow-positions.json`.

**Option 2 — rejected for this change: a real platform-file field, e.g. `skills:` frontmatter, written through `syncAgent`/the six renderers in `render.go`, importable back by the importer.**

Rejected specifically on the risk criterion this task was scoped against, not on principle:

- It requires new rendering logic in all six per-platform renderers in `render.go` (Claude Code, GitHub Copilot, Cursor, Codex, Kiro, Antigravity) — a materially larger, six-way surface, compared to Option 1's two files (`graph.go`, `hypnosserve.go`) plus one pre-existing latent-bug fix (`writer.go`'s `DeleteAgent`).
- It requires new importer parsing to round-trip that field back into `EdgeAttachment` edges on the next `Import` — a new code path with no existing analog to model it on (unlike hooks, which already have both a renderer and an importer side).
- Six new per-platform format decisions would need to be made and verified from scratch (does Cursor's `.mdc` frontmatter support an arbitrary new key cleanly? Does Antigravity's `SKILL.md`-shaped agent file? etc.) — none of this was investigated as part of confirming this gap, and doing so is exactly the "future work, if/when such a field is established" `writer.go:302-307` already correctly deferred, not something to fold into a rebuild-persistence fix.
- It touches `syncAgent`, the function every existing agent-file-content test (`writer_test.go`, plus every `TestCreateAgent*`/`TestAddRoutingEdge*`-style test that reads back rendered file content) depends on — six renderers' output changing shape is a much larger blast radius against already-passing tests than Option 1's fully additive change.
- It does *not* actually touch the skill's own file either (it would write to the *agent's* file), so it would also satisfy `TestAttachDetachSkillNeverTouchesSkillFile` — but satisfying that one test is not the same as being the lower-risk change against the *whole* existing test suite, which is the actual criterion.

Option 2 remains available as genuine future work if a real disk-backed "what can this agent invoke" record is ever wanted (e.g. to make attachments visible to a human reading an agent's file directly, or to survive someone deleting the `.dreamland/` cache directory) — this decision does not foreclose it, it just declines to bundle it into closing the immediate rebuild-loses-attachments gap.

### `DeleteAgent` also drops dangling `EdgeAttachment` edges targeting the deleted agent

Found during investigation: `DeleteAgent` (`writer.go:206-260`) already strips agent-scoped hook edges and routing edges pointing at the deleted agent, and any edge *sourced from* it, but not an `EdgeAttachment` edge whose `To` is the deleted agent (attachment edges are `From: skillID, To: agentID`). This was inconsequential before this change — nothing persisted attachment edges at all, so a dangling one lived at most until the next `Import`-only rebuild silently dropped every attachment anyway. It becomes consequential once attachment edges are cached to disk: without this fix, a `DeleteAgent` call followed immediately by a save could write a dangling entry into the new cache file. `rebuildGraph`'s existence-filtering (above) means this self-heals within one rebuild cycle either way (at most the next ~2s watcher poll), but fixing it directly at the source is a small, low-risk, one-loop-branch addition directly adjacent to the code this change already touches, so it's included rather than left as a second latent gap.

## Risks / Trade-offs

- **[Risk] Two independent local cache files (`workflow-positions.json`, `workflow-skill-attachments.json`) can each drift from current disk/graph state between saves.** Accepted — this is the same risk profile the position cache already carries alone today; this change doesn't introduce a new category of risk, only a second file with the identical characteristics.
- **[Risk] Hand-deleting `.dreamland/workflow-skill-attachments.json` silently loses every attachment on the next rebuild**, the same failure mode `workflow-positions.json` already has for position. Not a regression — consistent with the existing, accepted risk profile of the pattern being mirrored.
- **[Trade-off] Attachments remain invisible to someone reading an agent's platform file directly** (unlike hooks, which are visible in real frontmatter). This is the direct consequence of choosing Option 1 over Option 2, accepted per the Decisions section above; Option 2 remains available later if that visibility is ever wanted.

## Migration Plan

None required. `.dreamland/workflow-skill-attachments.json` does not exist until the first skill attach/detach is saved after this change lands; `LoadSkillAttachments`' missing-file case returns `nil, nil`, so a repo with no such file (every repo, before its first post-change save) behaves exactly as it does today — zero attachment edges survive a rebuild, same as now, until the new cache path starts capturing them.

## Open Questions

None outstanding — proceeding to specs/tasks.
