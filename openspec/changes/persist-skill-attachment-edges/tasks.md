## 1. Cache type and save/load

- [x] 1.1 In `internal/workflowgraph/graph.go`, add a `SkillAttachment` type (`SkillID`, `AgentID string`, JSON tags `skillId`/`agentId`) directly below `AgentPosition`, with a doc comment cross-referencing `AttachSkill`/`DetachSkill`'s existing doc comment (`writer.go:302-307`) and explaining why this needs the same treatment as position: `Import` has nothing on disk to derive an `EdgeAttachment` from.
- [x] 1.2 Add `SaveSkillAttachments(path string, g *Graph) error`: collect every `EdgeAttachment` edge in `g.Edges` into a `[]SkillAttachment` (sorted deterministically — by `SkillID` then `AgentID` — so repeated saves with the same edge set produce byte-identical output), marshal with `json.MarshalIndent`, create the parent dir if absent, write with a trailing newline — same shape as `SavePositions`.
- [x] 1.3 Add `LoadSkillAttachments(path string) ([]SkillAttachment, error)`: same shape as `LoadPositions` — missing file returns `nil, nil`, not an error.
- [x] 1.4 Tests in `internal/workflowgraph/graph_test.go`, alongside the existing `TestLoadPositionsMissingFileReturnsNilNotError`/`TestSavePositionsCreatesParentDir`/`TestSaveLoadPositionsRoundTrip` tests: equivalent `TestLoadSkillAttachmentsMissingFileReturnsNilNotError`, `TestSaveSkillAttachmentsCreatesParentDir`, `TestSaveLoadSkillAttachmentsRoundTrip` (attach two skills to two different agents, save, load, assert the pairs round-trip), and a determinism test asserting two saves of the same edge set (added in different orders) produce identical file bytes.

## 2. Rebuild merge and save-call-site wiring

- [x] 2.1 In `cmd/hypnosserve.go`, add `skillAttachmentsPathFor(repoRoot string) string` returning `filepath.Join(repoRoot, ".dreamland", "workflow-skill-attachments.json")`, next to the existing `positionsPathFor`/`lockPathFor`.
- [x] 2.2 Extend `rebuildGraph`: after the existing position-merge block, call `workflowgraph.LoadSkillAttachments(skillAttachmentsPathFor(repoRoot))`; for each returned entry, re-add an `EdgeAttachment{Kind: EdgeAttachment, From: SkillID, To: AgentID}` to `g.Edges` only if `g.Skills[SkillID]` and `g.Agents[AgentID]` both exist in the just-imported graph — skip (do not error on) any entry referencing a since-deleted skill or agent. Update `rebuildGraph`'s doc comment to describe both cached fields, not just position.
- [x] 2.3 In the `/api/mutate` handler (inside `newHypnosMux`), add a `workflowgraph.SaveSkillAttachments(skillAttachmentsPathFor(repoRoot), g)` call immediately after the existing `workflowgraph.SavePositions(...)` call, inside the same `WithLock` closure, before `guarded.Set(g)`.
- [x] 2.4 In `runApplyPlan`, add the equivalent `SaveSkillAttachments` call immediately after the existing `SavePositions` call, inside the same `WithLock` closure, with the same "only overwrite `applyErr` if it was nil" pattern the existing `SavePositions` error handling uses.
- [x] 2.5 Tests in `cmd/hypnosserve_test.go`:
  - `TestInteractiveModeAttachSkillSurvivesSubsequentMutation`: POST an `OpCreateEdge`/`EdgeAttachment` operation attaching an existing skill to an existing agent via `/api/mutate`, then POST a second, unrelated mutation (e.g. an `OpUpdateNode` position move on a different agent); assert the attachment edge is still present in `guarded.Get()` after the second call, and that `skillAttachmentsPathFor(root)`'s file contains the attached pair.
  - `TestRebuildGraphPreservesSkillAttachmentAcrossRestart`, directly mirroring `TestRebuildGraphPreservesPositionAcrossRestart`: `rebuildGraph`, `AttachSkill`, `SaveSkillAttachments` (simulating the mutate handler's sequence), then a fresh `rebuildGraph` call simulating a restart; assert the attachment edge is present.
  - `TestRebuildGraphDropsAttachmentForDeletedSkillOrAgent`: save an attachment, then delete the underlying skill's/agent's platform file(s) out from under the cache (or call `DeleteAgent`/`DeleteSkill` and re-save) so the next `Import` no longer finds that id; assert `rebuildGraph`'s resulting graph contains no dangling `EdgeAttachment` edge and does not error.
  - Extend the existing watcher-poll test (the one exercising `OnChange`/`Broadcaster.Publish`, near `TestRebuildGraphPreservesPositionAcrossRestart`) or add a sibling test confirming a poll-triggered `rebuildGraph` call also carries a previously saved attachment forward — covering the "watcher poll" scenario distinctly from the "explicit mutation" and "restart" scenarios already covered above.

## 3. `DeleteAgent` dangling-attachment-edge fix

- [x] 3.1 In `internal/workflowgraph/writer.go`'s `DeleteAgent` edge-filtering loop (`writer.go:233-247`), add a branch dropping any edge where `e.Kind == EdgeAttachment && e.To == id` (attachment edges are `From: skillID, To: agentID`), alongside the existing `EdgeHookBinding`/`EdgeRouting`/`e.From == id` branches.
- [x] 3.2 Test in `internal/workflowgraph/writer_test.go`: create an agent, attach a skill to it via `AttachSkill`, call `DeleteAgent`, assert no `EdgeAttachment` edge referencing the deleted agent id remains in `g.Edges`.
- [x] 3.3 Confirm `TestAttachDetachSkillNeverTouchesSkillFile` (`writer_test.go:322-377`) and every test in `internal/workflowgraph/skillwriter_test.go` still pass unmodified — this task adds a new branch to `DeleteAgent`'s edge filter only; `AttachSkill`/`DetachSkill` themselves are untouched by this whole change.

## 4. Gitignore and scaffold wiring

- [x] 4.1 Add `.dreamland/workflow-skill-attachments.json` to this repo's own `.gitignore`, directly below the existing `.dreamland/workflow-positions.json` line.
- [x] 4.2 In `cmd/init.go`, add a `scaffold.EnsureGitignoreEntry(repoRoot, ".dreamland/workflow-skill-attachments.json")` call directly below the existing `.dreamland/workflow-positions.json` call.
- [x] 4.3 Test in `internal/scaffold` (alongside existing `EnsureGitignoreEntry` coverage, or a `cmd/init_test.go` assertion if that's where the existing `.dreamland/workflow-positions.json`/`.dreamland/hypnos.lock` entries are already asserted post-`dreamland init`): confirm `.dreamland/workflow-skill-attachments.json` is present in `.gitignore` after `dreamland init`.

## 5. Verify

- [x] 5.1 Run `go build ./...` and `go vet ./...` — clean.
- [x] 5.2 Run `go test ./...` — full suite green, including all new tests from §1-4 and the full existing `internal/workflowgraph` and `cmd` suites with no regressions.
- [x] 5.3 Re-read `internal/workflowgraph/writer.go`'s `AttachSkill`/`DetachSkill` functions to confirm they were not modified by this change — persistence is entirely at the `cmd/hypnosserve.go` call-site layer, per design.md's Decisions.
- [ ] 5.4 Manually exercise the originally reported symptom against a real repo: start `dreamland hypnos-serve --mode=interactive`, attach a skill to an agent node, save, trigger a second mutation (or wait for a watcher poll), and confirm the attachment is still shown on the agent node — not silently dropped.
