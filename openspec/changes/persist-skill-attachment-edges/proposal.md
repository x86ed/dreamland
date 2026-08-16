## Why

`litegraph-workflow-editor`'s spec requires two things that currently collide:

- Every editor mutation SHALL rebuild the graph from fresh on-disk state (`spec.md:21-28`) — `rebuildGraph` in `cmd/hypnosserve.go:116-130`, called on every `/api/mutate` request and by the ~2s file-watcher poll.
- Skill attach/detach SHALL be edge-only and SHALL NOT touch the skill's own file (`spec.md:104-114`) — `AttachSkill`/`DetachSkill` in `internal/workflowgraph/writer.go:308-335`.

Because attach/detach is edge-only, the resulting `EdgeAttachment` lives nowhere but the in-memory `*Graph`. `rebuildGraph` only carries forward one piece of state across a fresh `Import` — agent position, via `AgentPosition`/`SavePositions`/`LoadPositions` (`internal/workflowgraph/graph.go:136-179`) — because that's the only thing the prior implementation needed to survive a rebuild. It does not carry forward attachment edges. The result: any skill attached through `/hypnos-interactive` is silently dropped on the very next mutation (which rebuilds before applying) or the next watcher poll (which rebuilds every ~2s regardless), typically well under the time it takes to make a second edit or even release the mouse button on a slow poll cycle.

This gap was known and explicitly deferred when skill attach/detach was first built: `internal/workflowgraph/writer.go:302-307`'s doc comment and the archived `openspec/changes/archive/2026-08-15-hypnos-litegraph-editor/tasks.md:29` both call closing it "future work," not a bug in that change. It has since become directly visible: no agent node in the litegraph UI shows any attached skill, because every attachment made in the UI has already been lost by the time the next graph is rendered.

This change closes that gap by persisting attachment edges the same way agent position already is — a small, low-risk local cache merged forward on every rebuild — rather than inventing a new disk-backed platform field, which would be materially larger in scope and risk (see design.md).

## What Changes

- **New `internal/workflowgraph.SkillAttachment` cache type** plus `SaveSkillAttachments`/`LoadSkillAttachments`, in `internal/workflowgraph/graph.go`, structurally mirroring the existing `AgentPosition`/`SavePositions`/`LoadPositions`.
- **`cmd/hypnosserve.go`'s `rebuildGraph`** additionally loads `.dreamland/workflow-skill-attachments.json` and merges each still-valid `(skillID, agentID)` pair back in as an `EdgeAttachment` edge after every fresh `Import` — the same merge-after-import shape it already uses for position, applied consistently to both mutation-triggered rebuilds and watcher-poll rebuilds since both call `rebuildGraph`.
- **`/api/mutate` and `runApplyPlan`** (`cmd/hypnosserve.go`) each gain a `SaveSkillAttachments` call alongside their existing `SavePositions` call, so an attach/detach made through either path is captured to disk in the same save cycle.
- **`internal/workflowgraph/writer.go`'s `DeleteAgent`** additionally drops any `EdgeAttachment` edge that targets the deleted agent. This is a latent, pre-existing gap (found during investigation, not introduced by prior work) that was inconsequential while attachment edges were never persisted — it becomes consequential once they are, since a stale entry could otherwise be written straight into the new cache file.
- **`AttachSkill`/`DetachSkill` themselves are unmodified** — persistence is added at the call-site layer (`cmd/hypnosserve.go`), exactly where `SavePositions` already lives, not inside the graph-edge mutation functions. This keeps `internal/workflowgraph/writer_test.go:322-377`'s `TestAttachDetachSkillNeverTouchesSkillFile` and `internal/workflowgraph/skillwriter_test.go` passing unchanged.
- **`.gitignore` and `cmd/init.go`** gain a `.dreamland/workflow-skill-attachments.json` entry, mirroring the existing `.dreamland/workflow-positions.json` entry — a regenerated local cache, not committed.
- **Spec**: new requirement in `litegraph-workflow-editor` stating graph-only edges (skill attachment today; any future non-file-backed edge type) SHALL survive a graph rebuild.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `litegraph-workflow-editor` — adds the requirement that graph-only edges survive a graph rebuild, closing the gap between the existing "rebuild from fresh disk state on every mutation" requirement and the existing "skill attachment is edge-only" requirement.

## Impact

- `internal/workflowgraph/graph.go` — new `SkillAttachment` type, `SaveSkillAttachments`, `LoadSkillAttachments`.
- `cmd/hypnosserve.go` — `rebuildGraph` merges attachment edges forward; `skillAttachmentsPathFor` helper; `/api/mutate` and `runApplyPlan` each save attachments after applying operations.
- `internal/workflowgraph/writer.go` — `DeleteAgent` drops dangling `EdgeAttachment` edges targeting the deleted agent.
- `.gitignore`, `cmd/init.go` — new gitignored cache path.
- Tests: `internal/workflowgraph/graph_test.go` (save/load round trip, missing-file, deleted-endpoint filtering), `cmd/hypnosserve_test.go` (attachment survives a subsequent unrelated mutation, survives a simulated restart, watcher-poll rebuild carries it forward), `internal/workflowgraph/writer_test.go` (`DeleteAgent` drops dangling attachment edges).
- No changes to `internal/workflowgraph/writer.go`'s `AttachSkill`/`DetachSkill`, `internal/workflowgraph/render.go`, any per-platform renderer, or any platform agent/skill file format. `TestAttachDetachSkillNeverTouchesSkillFile` and `internal/workflowgraph/skillwriter_test.go` are unaffected and must still pass unmodified.
