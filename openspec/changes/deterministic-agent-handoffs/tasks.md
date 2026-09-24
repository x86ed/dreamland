Dispatch notes. Go code (`cmd/`, `internal/handoff/`, `internal/config/`, `internal/scaffold/*.go`, and `settings-patch.json`) is `nyx` (tests first) then `morpheus`. Everything under `internal/scaffold/templates/agents/**` is `hypnos`-owned (`guard-artifact`). Tasks carry a `[flow: ...]` tag for `/opsx:apply`. Live `.claude/**` files are re-synced with `dreamland init`, never hand-edited.

Revision after the live interactive test (design.md Decisions 10-14): tasks marked `[REOPENED]` were done against the earlier design and must be redone; tasks marked `[CHANGED]` keep their box but have amended scope; new groups 10-13 hold the code fixes and the live proofs. Group 10 (interactive spikes) gates the `prompt` classifier code in group 11. No Go code before it.

## 0. Live verification of platform assumptions (gate for groups 1-6)

Run in a scratch repo with a build of `dreamland` from this branch; share the session with `deterministic-routing-and-janus-guard` tasks 0.1 and 0.7 where possible. If an assumption fails, stop and return to `phantasos`.

- [x] 0.1 Dump the `SubagentStop` payload (temporary hook): confirm `session_id` (same as the dispatcher's), `agent_type`, and which field holds the final report (`last_assistant_message` or `agent_transcript_path`). Record the field name in design.md Decision 5. [flow: morpheus]
- [x] 0.2 Confirm a `PostToolUse` hook with matcher `Task|Agent` fires when a subagent returns, that `tool_response` contains the report, and that `hookSpecificOutput.additionalContext` reaches the dispatcher's context before its next action. [flow: morpheus]
- [x] 0.3 Confirm ordering: does `SubagentStop` complete before the dispatcher's `PostToolUse`? If not, adopt the idempotent-`inject` fallback in design.md Decision 2. [flow: morpheus]
- [x] 0.4 Confirm `Stop` exit 2 makes the model continue with stderr as the reason (as `harden-commit-hook-enforcement` relied on), and that `PreToolUse` exit 2 on `Agent` blocks the call with the message shown to the model. [flow: morpheus]
- [x] 0.5 [CHANGED: `-p` only; interactive gap closed by 10.1-10.3] Confirm a `UserPromptSubmit` hook payload carries `session_id` and does not fire between a subagent's return and the dispatcher's next call. [flow: morpheus]
- [ ] 0.6 GitHub Copilot: dump its `SubagentStop` payload; decide whether `record` can be bound there (design.md Decision 7). Record the outcome; no Copilot enforcement unless injection is also confirmed. [flow: morpheus]

## 1. Edge table, tags, counter (`internal/handoff`)

- [x] 1.1 Write `internal/handoff/edges_test.go` from every scenario in the `deterministic-handoffs` spec's edge-table, tag, and counter requirements (table-driven `Next`; tag parsing incl. CRLF, quoted tags, last-wins, closed value sets; counter transitions 0->1->2, pass delete, phantasos/baku reset, spec-defect unchanged, unverified unchanged, missing-verdict retry once). [flow: nyx]
- [x] 1.2 Create `internal/handoff/edges.go` (the single table, `Next`, `Directive{Kind,Target,Reason,TagMissing}`) and `internal/handoff/tags.go`. [flow: morpheus]
- [x] 1.3 Write `internal/handoff/store_test.go`: state root resolution (`DREAMLAND_STATE_DIR`, cache dir, temp dir), repo-id hashing (Windows lower-casing), change key resolution (tag, sole active change via a fake `openspec list --json`, `_session-` fallback, slug validation), lock acquire/timeout/stale (fail open), atomic write, 14-day prune, and a concurrency test with two goroutines recording failures (`-race`). [flow: nyx]
- [x] 1.4 Create `internal/handoff/store.go` (counter and pending stores, `O_EXCL` lock, temp-and-rename). [flow: morpheus]

## 2. Commands

- [ ] 2.1 [REOPENED: agent_id discrimination, `record` requires agent_id, inject with no entry emits nothing, report-kind clearing by enforce, stop-check ignores `stop_hook_active`, two blocks then abandon on third, `prompt` mode; see 11.1] Write `cmd/handoff_test.go` for every hook-mode scenario: record writes a pending entry; inject text begins with the exact `REQUIRED NEXT STEP` sentence; enforce blocks a wrong target with exit 2 naming the required one and clears on the right one (oldest-first with two entries); stop-check exits 2; subagent payloads ignored; 3 blocks abandon; release; corrupt state, bad `session_id`, and lock timeout exit 0; `handoff_enforcement` `warn`/`off`; `handoff next` counter behavior; idempotent `record` by directive id. [flow: nyx]
- [ ] 2.2 [REOPENED: see 11.2] Create `cmd/handoff.go` (`record`, `inject`, `enforce`, `stop-check`, `release`, `next`, `clear`) reusing `cmd/hookexit.go` `Blocking` for exit 2. If `internal/sessionidentity` has landed, use its state root; otherwise a local helper with a test tying it to the same `DREAMLAND_STATE_DIR` rule. [flow: morpheus]
- [x] 2.3 Add `HandoffEnforcement string \`json:"handoff_enforcement,omitempty"\`` to `internal/config/config.go`; unknown values behave as `block`. [flow: morpheus]
- [x] 2.4 Append abandonment lines to the transition log and list pending/abandoned/released entries in `dreamland status`; add the bound-but-unknown-subcommand warning to `status` and `init` (spec: Windows and stale-binary requirement). [flow: nyx]
- [x] 2.5 Add `cmd/handoff_windows_build_test.go` in the style of `cmd/otel_receiver_windows_build_test.go`, and a Windows-path test for the lock and rename. [flow: morpheus]

## 3. Bindings (Claude Code)

- [ ] 3.1 [REOPENED: each mode its own entry (no appended command), `UserPromptSubmit` -> `handoff prompt --hook`; see 11.3] Write a scaffold test that the merged `.claude/settings.json` binds `SubagentStop` -> `dreamland handoff record --hook`, `PostToolUse` `Task|Agent` -> `handoff inject --hook`, `PreToolUse` `Task|Agent` -> `handoff enforce --hook` (appended after the existing `coauthor --hook` command, not a second entry), `Stop` -> `handoff stop-check --hook`, `UserPromptSubmit` -> `handoff release --hook`, with no shell operators, and that an existing user hook list is preserved. [flow: nyx]
- [ ] 3.2 [REOPENED: separate entries; drop the `record`-before-`commit` ordering constraint] Edit `internal/scaffold/templates/hooks/bindings/claude-code/settings-patch.json` accordingly; confirm the merge (`mergeJSON`) appends commands to an existing matcher rather than replacing. Keep `record` ordered before `commit --reason handoff --hook` in `SubagentStop`. [flow: morpheus]

## 4. Agent templates (`hypnos`), all six platforms

- [x] 4.1 `morpheus.*` on claude-code, cursor, codex, kiro, antigravity, github-copilot: keep the ``hand off directly to `phobetor` `` sentence; add "end your report with `[handoff: complete]`, or `[handoff: blocked]` when escalating an ambiguity to Janus; tags on their own final lines, nothing after them". [flow: hypnos]
- [x] 4.2 `iktomi.*` on all six: replace the file-changed conditional (claude-code, github-copilot) and the "report completion or blockers to Janus" line (cursor, codex, kiro, antigravity) with: "Once your own work is complete, hand off directly to `phobetor` for validation — a fixed, unconditional next step, regardless of whether the work involved file changes. End your report with `[handoff: complete]`. If you are blocked instead, end with `[handoff: blocked]` and report the blocker to Janus." Leave items 1 and 2 and the broad-routing sentence untouched. This folds in every edit of `iktomi-always-handoff-phobetor`. [flow: hypnos]
- [x] 4.3 [CHANGED: `[change: <slug>]` becomes MUST when working a change; redo text in 11.4] `phobetor.*` on all six: replace the three-way prose with the verdict tags, the counter statement ("the first failure for a change goes to `morpheus`; a failure after a failed `morpheus` retry goes to `phantasos`; the hand-off mechanism applies the counter"), `[change: <slug>]`, and "if `dreamland test` could not be run, report `[verdict: unverified]`, never pass". Keep the substrings ``hand off directly to `baku` ``, ``... `morpheus` ``, ``... `phantasos` `` for the graph importer. [flow: hypnos]
- [x] 4.4 `nyx.*` on all six: add the `[handoff: complete|blocked]` line. [flow: hypnos]
- [x] 4.5 Cursor, Codex, Kiro, Antigravity `morpheus`/`iktomi`/`nyx`/`phobetor`: add one line telling the dispatcher to run `dreamland handoff next --from <agent> ...` and follow its directive; add the same line on Copilot if task 0.6 rules out `record`. [flow: hypnos]
- [x] 4.6 Update `internal/scaffold/scaffold_test.go` assertions at the former lines 420-421 and 440-441 for the new iktomi text, and add per-platform assertions for cursor, codex, kiro, antigravity iktomi (the folded-in tasks 7.4/7.5 of `iktomi-always-handoff-phobetor`). [flow: morpheus]

## 5. Drift test

- [ ] 5.1 [REOPENED: bindings assertion covers the five modes as separate entries incl. `prompt`; phantasos/baku change tag; see 11.4] Write and pass `internal/handoff/drift_test.go` per the drift requirement: templates on six platforms versus the edge table (reusing `handOffPattern` semantics), tag-instruction presence, no file-changed conditional in any `iktomi` template, and the five bindings in `settings-patch.json`. It fails until group 4 lands. [flow: nyx]

## 6. Live verification (after groups 1-5)

- [ ] 6.1 [SUPERSEDED by 12.1-12.6; do not run separately] In a scratch repo run a real `morpheus` dispatch from a plain session; confirm injection, that dispatching `nyx` instead is blocked, and that `phobetor` is then dispatched. Record what was observed. [flow: morpheus]
- [ ] 6.2 Force a `phobetor` fail twice for one change; confirm `morpheus` then `phantasos`; confirm a pass afterwards clears the counter (inspect `<root>/handoff/`). [flow: morpheus]
- [ ] 6.3 Repeat 6.1 under `claude --agent janus` (needs the Janus `Agent(...)` grant from `deterministic-routing-and-janus-guard`; if not landed, record as blocked, not passed). [flow: morpheus]
- [ ] 6.4 [CHANGED: three-block abandon is now two blocks then abandon on the third; the release check is 12.5] Verify a stale binary (one built before this change) yields the `status` warning and no crash; verify the three-block abandon and the `UserPromptSubmit` release with real hooks. [flow: morpheus]

## 7. Windows

- [ ] 7.1 Run the Windows build test in CI or on a Windows machine; run the lock and rename tests there once. [flow: morpheus]

## 8. Coordination with the open routing change

- [x] 8.1 Re-check `deterministic-routing-and-janus-guard` `settings-patch.json` edits for conflicts with group 3; the entries differ, so expect a mechanical merge only. [flow: morpheus]
- [x] 8.2 If `guard-router` has landed, add `dreamland handoff next *` (read-only) to its Bash allowlist and its allowlist test. If not, leave a note in that change's tasks to add it. [flow: morpheus]

## 9. Reconcile, re-sync, archive readiness

- [x] 9.1 Add a one-line pointer to `openspec/changes/iktomi-always-handoff-phobetor/proposal.md` ("Superseded by `deterministic-agent-handoffs`; archive with `--skip-specs` after that change archives") and do not tick its tasks; its scope is done here. [flow: mengpo]
- [ ] 9.2 [CHANGED: depends on 11.5 agent-file sync, and now also covers phantasos and baku] Re-sync live `.claude/agents/{morpheus,iktomi,phobetor,nyx}.md` and `.claude/settings.json` by running `dreamland init` (shared with `claude-code-parity` task 9.4; no hand edits); confirm the live `iktomi.md` no longer says "report completion or blockers to Janus when done". [flow: morpheus]
- [ ] 9.3 [REOPENED: rerun after groups 10-12] Run `openspec validate deterministic-agent-handoffs --strict`, `go vet ./...`, `go test ./...`. [flow: morpheus]
- [ ] 9.4 Archive order: this change first, then `openspec archive iktomi-always-handoff-phobetor --skip-specs`; annotate `claude-code-parity`'s Iktomi delta as superseded when it archives. [flow: mengpo]

## 10. Interactive-session spikes (gate for group 11; scratch repo, real interactive `claude`, temporary logging hooks only; no dreamland code)

If any result contradicts Decisions 10-11, stop and return to `phantasos`.

- [ ] 10.1 Dump the `UserPromptSubmit` payload (all fields) for: (a) a typed prompt, (b) a typed slash command such as `/drmlnd:morpheus`, (c) a human prompt typed while a background agent is running (queued), (d) the background-agent completion task-notification, (e) any other system-generated prompt seen (hook `Stop` feedback continuation, cron). Record for each: every field name, and whether any field or the prompt-text prefix distinguishes human from system. Write the result into design.md Decision 11 and choose: field discriminator, envelope-match discriminator, or drop release. [flow: morpheus]
- [ ] 10.2 Confirm the order in the background case: `PostToolUse(Agent)` at launch, `SubagentStop`, then the notification `UserPromptSubmit`; capture the `PostToolUse` `tool_response` for a background launch (status field, presence of agent id). Confirm `additionalContext` from `UserPromptSubmit` on the notification reaches the dispatcher before its next action. [flow: morpheus]
- [ ] 10.3 Confirm in an interactive session: `Stop` exit 2 feedback and `PreToolUse(Agent)` exit 2 both work when the dispatch was background, and record what the dispatcher does when it ends its turn while a background agent is still running (does the notification wake it). [flow: morpheus]

## 11. Code fixes (after group 10)

- [ ] 11.1 Extend `cmd/handoff_test.go` and `internal/handoff/*_test.go` (tests first): dispatcher discrimination by absence of `agent_id` (plain, `--agent janus`, subagent payloads); `record` ignores a payload without `agent_id`; `inject`/`prompt` emit nothing with no entry and emit the directive when an entry exists (background sequence: PostToolUse first with no entry, SubagentStop, notification prompt); `enforce` blocks a wrong target for an entry written after launch; `enforce` clears report-kind entries; `stop-check` blocks with `stop_hook_active: true`; exactly two blocks then abandon on the third; `prompt` classification per 10.1 (notification does not release, typed prompt and slash command release, unknown does not release); partial pass (`Next` with `tasksRemaining` 0/N/-1, `tasks.md` checkbox counting incl. CRLF and no-checkbox file); counter `session_id` and untagged phantasos/baku reset of this session's most recent counter only. [flow: nyx]
- [ ] 11.2 Implement in `cmd/handoff.go` and `internal/handoff/`: `agent_id` filter, `prompt` mode (replaces `release`; no separate release binding), shared inject routine, report-entry clearing, block bound, `tasksRemaining` reader, `Next` signature change, counter `session_id`, session-fallback reset, `[change:]` parsing for phantasos/baku. [flow: morpheus]
- [ ] 11.3 Scaffold test then `settings-patch.json`: five separate hook entries (`SubagentStop` record, `PostToolUse` `Task|Agent` inject, `PreToolUse` `Task|Agent` enforce as its own entry beside `coauthor`, `Stop` stop-check, `UserPromptSubmit` prompt); merge preserves user hooks and the existing `coauthor`/telemetry/commit entries; confirm `mergeJSON` de-duplicates on re-init. [flow: nyx then morpheus]
- [ ] 11.4 Update the drift test (5.1) and the `phantasos.*`/`baku.*` templates on all six platforms to end reports with `[change: <slug>]`; `phobetor.*` templates say `[change: <slug>]` is required when working a change and describe the partial-pass rule ("if tasks remain unticked the mechanism reports a partial pass instead of calling `baku`"). Template edits are `hypnos`-owned; keep the ``hand off directly to `baku` `` substring. [flow: hypnos]
- [ ] 11.5 Agent-file sync (Decision 12): test then implement in `internal/scaffold/scaffold.go`: agent files carry `DreamlandManagedMarker`; plain `init` overwrites a marked differing file (`updated`), warns on an unmarked differing file, `--force` adopts it; `dreamland status` lists out-of-date and unmarked-different agent files. Cover all six platform layouts (flat and Antigravity per-skill). [flow: nyx then morpheus]
- [ ] 11.6 `handoff prompt` runs on every prompt: it must exit 0 quickly on any fault; add a test that the human-prompt path does no `openspec` shell-out. [flow: morpheus]

## 12. Live verification after groups 10-11 (interactive session in a scratch repo; rebuild the binary and run `dreamland init` first; record observations in design.md)

- [ ] 12.1 Foreground dispatch: dispatch `morpheus` in the foreground; confirm `REQUIRED NEXT STEP` arrives at `PostToolUse`, an `Explore` dispatch is blocked (exit 2 text quoted), `phobetor` then clears the entry, and `Stop` with a pending entry is blocked. [flow: morpheus]
- [ ] 12.2 Background dispatch: dispatch `morpheus` in the background; confirm `PostToolUse` at launch emits nothing, `dreamland status` shows the entry after completion, the notification `UserPromptSubmit` delivers `REQUIRED NEXT STEP` and does NOT set `released-by-user`, an `Explore` dispatch is then blocked, and `phobetor` clears it. [flow: morpheus]
- [ ] 12.3 Background dispatch where the dispatcher ends its turn before completion: confirm the notification turn injects and enforces (Decision 10 residual gap), and `stop-check` catches a stop with a pending entry. [flow: morpheus]
- [ ] 12.4 `phobetor` fail twice for one change then pass, foreground and background: `morpheus` then `phantasos`, counter deleted on pass; partial pass with unticked tasks yields the report and no `baku`; fully ticked yields `baku`; untagged `baku` resets this session's counter only. [flow: morpheus]
- [ ] 12.5 Release and abandon: typed prompt and typed `/drmlnd:morpheus` release a pending entry (`released-by-user`); a notification does not; two blocks then abandon on the third with the transition-log line. [flow: morpheus]
- [ ] 12.6 Repeat 12.1 under `claude --agent janus` (record as blocked, not passed, if the Janus `Agent(...)` grant has not landed). [flow: morpheus]
- [ ] 12.7 Confirm after plain `dreamland init` (no `--force`) that live `morpheus|iktomi|phobetor|nyx|phantasos|baku.md`, once marked, contain the `[handoff:]`/`[verdict:]`/`[change:]` instructions, and that an unmarked legacy file produces the warning and `--force` adopts it. [flow: morpheus]

## 13. Follow-ups outside this change

- [ ] 13.1 Ask the user whether to create `stable-binary-staleness-check` (design.md Decision 13: content hash of Go sources and templates embedded at build, replacing the `HEAD`-SHA comparison in `cmd/version.go`). Do not implement here. [flow: janus]
