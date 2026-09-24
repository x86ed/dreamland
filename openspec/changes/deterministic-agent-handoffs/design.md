## Context

Verified in the repository (not assumed):

- `morpheus`, `iktomi`: `tools: Read, Edit, Write, Bash`; `phobetor`: `Read, Bash` (`.claude/agents/*.md`). None has `Agent`. A hand-off is a sentence in the final report; the dispatcher (plain main session, or Janus) must make the next `Agent` call. The user decided against granting `Agent` (option 1: a deterministic next-step mechanism).
- `.claude/agents/morpheus.md` item 4 and `phobetor.md` item 3 carry the fixed edges as prose. The live `.claude/agents/iktomi.md` still says "report completion or blockers to Janus when done" (older than both the templates and `iktomi-always-handoff-phobetor`); the claude-code and github-copilot iktomi templates carry the file-changed-only split; cursor/codex/kiro/antigravity carry the original text.
- `settings-patch.json` (Claude Code) binds `SubagentStop` (`telemetry write`, `version-bump --patch`, `version-bump --minor --if-agent janus`, `commit --reason handoff --hook`), `PreToolUse` `Task|Agent` (`coauthor --hook`) and `Write|Edit` (`guard-artifact`), and `Stop`. `cmd/hookexit.go` (`Blocking`, `IsBlocking`) maps a blocking error to exit 2; every other dreamland error exits 1, which Claude Code treats as non-blocking (`dev-workflow-hooks`: "Hook-invoked commands distinguish blocking failures from advisory skips via exit code").
- `internal/workflowgraph/import.go` has `handOffPattern` matching ``hand(?:s)? off directly to `<agent>` ``; the drift test reuses it.
- `harden-commit-hook-enforcement` is archived (`openspec/changes/archive/2026-09-20-...`). `deterministic-routing-and-janus-guard` is open (0/67); `cmd/route.go`, `cmd/guard_router.go`, and `internal/sessionidentity` do not exist yet. Nothing here may assume them.
- Platform hook facts that this design needs and that are NOT yet confirmed on the installed Claude Code version (task group 0 is the gate): `SubagentStop` payload fields (`session_id`, `agent_type`, and where the final report text is: `last_assistant_message` or `agent_transcript_path`); `PostToolUse` for the `Agent`/`Task` tool returning `hookSpecificOutput.additionalContext` into the dispatcher's context; `Stop` exit 2 preventing the stop and feeding stderr back to the model; the relative ordering of `SubagentStop` and the dispatcher's `PostToolUse`.

## Goals / Non-Goals

**Goals**
- After `morpheus`, `iktomi`, `nyx`, or `phobetor` finishes, the dispatcher is told, in-context and by a machine, exactly which agent to call next, and cannot quietly do anything else or end the turn instead.
- The edge table is one Go table; templates cannot drift from it unnoticed.
- Every decision is made from tags, never from judging prose.
- The failure counter is correct across subagent turns and parallel sessions.
- Fails open on any internal fault: enforcement blocks only on a valid, readable pending directive.

**Non-Goals**
- Granting `Agent` to any agent; making a hook itself spawn a subagent (hooks cannot); changing `baku`, `phantasos` (beyond counter reset), `hypnos`, `mengpo`, `zhougong` edges.
- Enforcing on Cursor, Codex, Kiro, Antigravity (no verified subagent lifecycle hook) or, in this change, Copilot.
- Re-deciding routing of the *first* dispatch (that is `dreamland route`).

## Decisions

### Decision 1: One edge table, five outcomes

`internal/handoff/edges.go` is the only place edges live:

| From | Tag read | Directive |
| --- | --- | --- |
| `nyx` | `[handoff: complete]` (or absent) | dispatch `morpheus` |
| `morpheus` | `[handoff: complete]` (or absent) | dispatch `phobetor` |
| `iktomi` | `[handoff: complete]` (or absent) | dispatch `phobetor` |
| `nyx`/`morpheus`/`iktomi` | `[handoff: blocked]` | report (no dispatch): surface blocker to Janus/user |
| `phobetor` | `[verdict: pass]` | delete the change's counter; then tasks fully ticked (or unknown) -> dispatch `baku`; tasks remaining -> report (partial pass, no dispatch; see Decision 14) |
| `phobetor` | `[verdict: fail]` | counter 0 -> dispatch `morpheus` (counter becomes 1); counter >= 1 -> dispatch `phantasos` (counter becomes 2) |
| `phobetor` | `[verdict: spec-defect]` | dispatch `phantasos` (counter unchanged) |
| `phobetor` | `[verdict: unverified]` | report (no dispatch): tests could not be run; counter unchanged |
| `phobetor` | tag absent or malformed | dispatch `phobetor` again with "end your report with a verdict tag", at most once per change per turn-chain (`verdict_retries`); a second miss becomes a report |
| `phantasos` | any | no directive; reset the change's counter (`[change:]` tag, else this session's most recent counter) |
| `baku` | any | no directive; delete the change's counter (same key rule as `phantasos`) |

`nyx` -> `morpheus` is included although the user's list did not name it: it is a fixed edge in the existing `janus-router-agent` spec with the same skip exposure, and a table that omits it would be incomplete on purpose. Cheap to drop (one row, one template line); flagged for confirmation.

An absent handoff tag on `nyx`/`morpheus`/`iktomi` means complete, because the failure mode being fixed is a skipped validation, so the default must lean toward validating. The cost is that a blocked-but-untagged `iktomi` turn goes to `phobetor`, which finds nothing to verify and reports `pass` (already accepted in `iktomi-always-handoff-phobetor`'s risks).

`unverified` exists because of the stale-binary history: earlier runs skipped `dreamland test` because the binary was stale. A `phobetor` that could not run its tests must not say `pass` (that would advance to `baku` unverified) and must not say `fail` (that would burn the retry budget and send `morpheus` to fix nothing). The hook never runs tests itself, so a stale binary cannot make the mechanism skip validation, only make `phobetor` report `unverified`.

### Decision 2: Why record, inject, enforce, stop-check (and not block at `SubagentStop`)

`SubagentStop` with a block only keeps the *subagent* running; that subagent cannot dispatch, so a block there cannot produce the next call. The next call must be made by the dispatcher, so the mechanism has to act on the dispatcher's side:

1. **record** (`SubagentStop`): the only place both `agent_type` and the final report are reliably available. Computes the directive from the table, updates the counter, writes a pending entry. This is the sole writer of counters.
2. **inject** (`PostToolUse`, `Task|Agent`, and, per Decision 10, `UserPromptSubmit` via `prompt`): read-only and stateless with respect to ordering: it emits `additionalContext` for whatever pending entries exist when it runs: `REQUIRED NEXT STEP (dreamland handoff): call Agent with subagent_type=phobetor now. Do not call any other agent and do not end your turn.` plus the change slug and reason. With no entry it emits nothing.
3. **enforce** (`PreToolUse`, `Task|Agent`): with a pending `dispatch` entry, an `Agent` call whose `subagent_type` is not a pending target exits 2 with a message naming the required target. A matching call clears the entry (oldest first when several are pending, which covers parallel subagents in one session). Any dispatcher `Agent` call also clears pending `report` entries (delivered, never blocking). It reads state at call time, so it works when the entry was written after a background launch.
4. **stop-check** (`Stop`): pending `dispatch` entries make the stop exit 2, stderr = the required call. `stop_hook_active` is ignored (Verification 0.4: honoring it would defeat enforcement); the block bound ends the blocking. This is what catches the "just stops" failure that motivated the change.
5. **prompt** (`UserPromptSubmit`): classifies the prompt. A real human prompt (including a typed slash command) releases pending entries as `released-by-user`; a harness-generated prompt (background-completion notification) injects instead and never releases. See Decision 11.

Each mode is its own hook entry in `settings-patch.json`, not a command appended to an existing entry (a live-test finding: appended commands share an entry's matcher and lifetime and make the drift test and merge harder to reason about). Claude Code runs the hooks of one event in parallel, so nothing may depend on the relative order of two dreamland hooks on the same event; the earlier "record ordered before `commit --reason handoff`" constraint is dropped (they do not read each other's output). What must hold across events is only: `record` (SubagentStop) has finished before `enforce`/`stop-check`/`prompt` read state, which Verification 0.3 confirmed for foreground dispatch and Decision 10 handles for background dispatch.

### Decision 3: Bounded, with a stated escape

A hard block with no exit can trap a session (an agent that cannot be dispatched because it was deleted, a permission denial, a mis-tagged report). So: each pending entry has `blocks`. `enforce` and `stop-check` increment it on each block; the hook blocks (exit 2) twice per entry, and the third violating event marks the entry `abandoned` instead of blocking, exits 0 with a stderr warning, and the abandonment is appended to the transition log and listed by `dreamland status`. The skip is therefore never silent, and it needs two separate refusals from the mechanism first. `handoff_enforcement` in `.dreamland.json` accepts `"block"` (default), `"warn"` (inject and log, never exit 2), `"off"`. `dreamland handoff clear [--session <id>]` is a human command. The `UserPromptSubmit` release is bound to a real human prompt only (Decision 11).

### Decision 4: Failure counter: location, key, resets (answers the explicit question)

Interpretation confirmed as the user stated: per change, the first `phobetor` failure goes to `morpheus`; a failure after that retry goes to `phantasos`.

- **Where**: `<root>/handoff/<repo-id>/<change>.json`, `<root>` the same per-user state root as `internal/sessionidentity` (`$DREAMLAND_STATE_DIR`, else `<UserCacheDir>/dreamland`, else `<TempDir>/dreamland`). It is per-user and outside the repo so it survives across subagent turns and across parallel sessions of the same repository, and does not dirty the working tree or get committed. `<repo-id>` is the first 16 hex chars of sha256 of the cleaned absolute repository root (lower-cased on Windows), because the state root is shared by all repos. Content: `{"phobetor_failures":n,"verdict_retries":n,"session_id":"...","updated_at":"..."}`; `session_id` is the last writer, and gives the fallback below.
- **Key**: the change slug: `[change: <slug>]` in the `phobetor` report, else the sole active change from `openspec list --json`, else `_session-<session_id>` (free-form `iktomi` work has no change). Slugs must match `^[a-z0-9][a-z0-9-]{0,63}$`.
- **Concurrency**: two sessions can validate the same change. Read-modify-write takes a lock file (`<change>.lock`, `O_CREATE|O_EXCL`, 5 s timeout, stale after 30 s, removed after use), then writes via temp file and `os.Rename`. Lock timeout means fail open: no directive is written, a warning is printed.
- **Untagged reset**: `baku` completes after the change is archived, so "sole active change" cannot resolve it; `phantasos`/`baku` reports therefore end with `[change: <slug>]` (Decision 5), and when they do not, the reset applies to the most recently updated counter file whose `session_id` equals the completing session (none: no-op). Other sessions' counters are never touched.
- **Resets**: deleted on `[verdict: pass]`; reset to 0 when `phantasos` completes (re-spec) or `baku` completes; entries older than 14 days are pruned on write. Not reset by a `morpheus` turn (the retry must not clear the evidence it failed).
- **Why not in the repo** (e.g. in `tasks.md`): it would create commits and merge conflicts, and the `Stop`/handoff commits would sweep it up. **Why not in the session id only**: a fresh session for the retry would forget the failure; the change is the unit the user described.

### Decision 5: Machine-readable tags

Own-line tags, last occurrence per key wins, matched by `^\s*\[(handoff|verdict|change): ([a-z0-9-]+)\]\s*$` after normalizing `\r\n`. Values outside the closed set for that key are "malformed" (treated as absent). Own-line plus last-wins prevents a report that quotes an earlier tag from steering the mechanism. The report text comes from the `SubagentStop` payload's final message (field `last_assistant_message`, verified in task 0.1; `agent_transcript_path` is only a fallback). `phobetor` MUST add `[change: <slug>]` when working a change (Decision 14 reads it), and `phantasos` and `baku` end their reports with `[change: <slug>]` too so the counter reset has a key. The agent templates say: the final line group of the report is the tag lines, nothing after them.

### Decision 6: Plain main session versus Janus

The hooks live in `.claude/settings.json`, so they fire identically for a plain `claude` session and for `claude --agent janus`, keyed by `session_id`. They act only when the payload has no `agent_id` (the dispatcher; Verification, dispatcher discrimination: under `claude --agent X` the dispatcher carries `agent_type: X` but never `agent_id`); a subagent's own hook calls, which always carry `agent_id`, are ignored. `record` is the mirror: it acts only on a payload with `agent_id` and `agent_type`. For a plain session the mandatory call is made by the session model itself; for Janus (which under `deterministic-routing-and-janus-guard` has `Agent(<roster>)`) by Janus. Every possible target (`morpheus`, `phobetor`, `baku`, `phantasos`) is in that roster, so the two mechanisms cannot deadlock. `dreamland handoff next` should join that change's read-only Bash allowlist; because that change is unimplemented, this is a coordination note (tasks 8.2), not an edit to its artifacts. Janus's own turns, when spawned as a routing subagent (legacy flow), produce no directive (Janus is not in the table).

### Decision 7: Platform scope

- **Claude Code**: full (five hook modes in `settings-patch.json`).
- **GitHub Copilot**: has a `SubagentStop`-shaped `hooks.json`, but whether it can inject context or block is unverified. This change adds `record` there only if task 0.6 confirms the payload; otherwise Copilot is prose plus `handoff next`. Its frontmatter `agents:` graph already declares the edges structurally.
- **Cursor, Codex, Kiro, Antigravity**: tags plus one line in `morpheus`/`iktomi`/`nyx`/`phobetor` templates telling the dispatcher to run `dreamland handoff next --from <agent> --verdict <v> --change <slug>` and follow its printed directive. No enforcement, said plainly: without a subagent lifecycle hook there is nothing to enforce with.

### Decision 8: Windows safety and the stale binary

- No shell syntax in bindings (`dreamland handoff record --hook` etc.), stdin JSON only, `filepath` for every path, `os.Rename` (replaces on Windows), lock via `O_EXCL` (no `flock`), CRLF-tolerant parsing, and a `_windows` build/test in the style of `cmd/otel_receiver_windows_build_test.go`. No signals, no process inspection.
- **Fail open**: any unreadable state, invalid JSON, lock timeout, or unknown `session_id` (invalid per the `[A-Za-z0-9._-]{1,128}` rule) exits 0 with a stderr warning. Only a successfully read `dispatch` entry blocks.
- **Stale binary**: a binary predating this change lacks the `handoff` subcommand; Cobra exits 1, non-blocking, so enforcement silently disappears. Mitigation: `dreamland status` and `dreamland init` parse `.claude/settings.json` for `dreamland <sub>` bindings and warn when the running binary does not know the subcommand; the existing build-freshness diagnostic still covers dreamland's own source tree. The mechanism never calls `dreamland test`, so a skipped test cannot make it skip a hand-off; the only interaction is `[verdict: unverified]` (Decision 1). The hook does not attempt to detect its own absence, because code that is not running cannot.

### Decision 9: Folding in `iktomi-always-handoff-phobetor`

That change (0/13, six templates plus two `scaffold_test.go` assertions) and this one both edit the same requirement in `janus-router-agent`. Rather than sequencing two edits of one sentence, this change carries the whole scope: the same unconditional rule on all six platforms, the same blocker rule (now tagged), the `scaffold_test.go` updates (its tasks 7.4/7.5), and a MODIFIED requirement that is the superset of its text, order-independently correct. Reconciliation: this change is implemented and archived; `iktomi-always-handoff-phobetor` is then archived with `openspec archive iktomi-always-handoff-phobetor --skip-specs` (its delta would otherwise re-apply the tag-less text over ours). Its unchecked tasks are recorded here as superseded and its `proposal.md` gets a one-line pointer here (task 9.1). If the user prefers to ship the prose-only change first, nothing here breaks: it archives first normally and this change's MODIFIED text replaces it. The live `.claude/agents/iktomi.md` is re-synced by `dreamland init` (claude-code-parity task 9.4's re-sync), never hand-edited.

### Decision 10: Foreground versus background dispatch (live finding B)

The group-0 spike ran foreground dispatch only and concluded `SubagentStop` finishes before `PostToolUse(Agent)`. In the live interactive session the dispatches were background (asynchronous `Agent` launch): `PostToolUse(Agent)` fires at launch, `SubagentStop` later, so `inject` ran when no entry existed and the directive never reached the dispatcher. The design no longer assumes an ordering; each mode reads state when it runs.

| Path | Foreground | Background |
| --- | --- | --- |
| Entry written | `SubagentStop` before `PostToolUse` | `SubagentStop` after `PostToolUse`, at completion |
| Directive reaches the dispatcher by | `inject` on `PostToolUse(Agent)` | `prompt` on the `UserPromptSubmit` the harness raises for the completion notification (live-verified to carry `additionalContext`), else `stop-check`/`enforce` |
| Hard layer | `enforce`, `stop-check` | `enforce`, `stop-check` (read state at call time) |

Consequences: `inject` and `prompt` share one read-only routine; the routine never inspects the `Agent` result's status; `record` is the only writer. Residual gap, stated: if the dispatcher ends its turn while a background agent is still running, `stop-check` finds no entry yet and allows the stop; the mechanism then acts when the completion notification starts the next turn (`prompt` injects; `enforce` guards the dispatch). A pending entry that no notification ever surfaces is still caught by `stop-check` at the next stop. The live tasks (group 12) prove both paths.

### Decision 11: Release only on a real human prompt (live finding A)

The `release` hook on `UserPromptSubmit` fired on the harness's "background agent finished" task-notification, which reaches the hook as an ordinary `UserPromptSubmit` in an interactive session, so the entry became `released-by-user` before the dispatcher could act. Verification 0.5 tested only `claude -p` and explicitly left interactive sessions untested.

Design: mode `prompt` replaces `release`. Release requires positive identification of a human prompt, in order of preference: (1) a payload field naming the prompt origin, if one exists; (2) the prompt text not matching the harness's notification envelope (a text match is a negative test, so it counts as "human" only if task group 10 shows the envelope is a stable marker for every system-generated prompt kind seen: background-completion notification, hook feedback, scheduled/cron prompts). Uncertain means no release. A typed slash command (for example `/drmlnd:morpheus`) is a human prompt and releases; a queued human prompt typed while the agent ran also releases. Fallback if group 10 finds neither discriminator dependable: remove release entirely, keep notification injection bound on `UserPromptSubmit`, and rely on the two-block bound and `dreamland handoff clear`. The task list makes the spike a gate: no `prompt` classifier code is written before the payloads are captured.

### Decision 12: Keeping live agent files in sync (live finding C)

`dreamland init` skipped existing `.claude/agents/*.md` without `--force`, so the live `morpheus|iktomi|phobetor|nyx|phantasos|baku` files lacked the tags and the hooks had nothing to read. Agent files carry no managed marker today (only bare commands and skills do). Decision: installed agent files get `DreamlandManagedMarker` appended (after the body, so frontmatter stays on line 1, as `bareCommandMarker` does). Plain `init` then overwrites any marked file that differs from its template (reported `updated`); unmarked existing files are never silently overwritten (they may be user-owned) but are warned about by name, and `init --force` adopts them (overwrite plus marker); after one adoption, sync is automatic. `dreamland status` lists out-of-date and unmarked-different agent files. No separate `dreamland sync` command: `init` is already the documented re-sync (claude-code-parity 9.4), and a second command adds a surface with the same semantics. Trade-off: users who customized a marked file lose the edit on next init; the marker text already says so.

### Decision 13: Binary staleness check (live finding D): deferred to its own change

`checkBinaryFreshness` compares the ldflags build SHA to source `HEAD`; turn-complete checkpoint commits advance `HEAD` each turn with no source change, so it warns constantly and `dreamland test` skips. A stable check compares content, not history: embed at build time (`-X ...buildSourceHash=`) a SHA-256 over `go.mod`, `go.sum`, non-test `*.go` under `cmd/` and `internal/`, and `internal/scaffold/templates/**` (sorted path plus bytes), and the check recomputes it over the working tree; a mismatch is a real stale binary, and `HEAD` movement alone is not. Belongs in a separate change (suggested `stable-binary-staleness-check`): it touches build tooling and `dreamland test`'s skip logic, not the hand-off mechanism, and folding it in would block this change's live verification on unrelated build work. This change's only dependency is that `phobetor` reports `[verdict: unverified]` when tests are skipped (already specified); no task here implements the hash. Not created by this update; flagged for the user.

### Decision 14: Partial pass does not close the change

`phobetor` validates one task's work; a `pass` in the middle of a change must not send `baku` to close it. `Next` takes `tasksRemaining`, computed by the store layer from `openspec/changes/<slug>/tasks.md` (`[change: <slug>]` tag) as the count of lines matching `^\s*- \[ \]`; -1 when there is no tag, no readable file, or no checkbox lines. Pass with 0 or -1 -> dispatch `baku`; pass with N>0 -> a `report` directive: "partial pass: N tasks of <slug> unticked; dispatch morpheus for the next task or stop (Janus/user decides); baku only after all tasks are ticked". This reads a checkbox file, not prose, so the "mechanism never judges prose" rule holds. Unknown keeps the user-fixed `pass -> baku` edge (free-form `iktomi` work has no change; `baku` reports there is nothing to close). The counter is deleted on any pass. Requires a user confirmation (below).

## Overlap and sequencing

- `iktomi-always-handoff-phobetor`: folded in (Decision 9). Archive this first, then that with `--skip-specs`.
- `deterministic-routing-and-janus-guard` (0/67): both edit `settings-patch.json` (different hook entries: theirs `PreToolUse` guard and `Stop`; ours `SubagentStop`, `PostToolUse`, `PreToolUse` `Task|Agent`, `UserPromptSubmit`; ours are separate entries, not commands appended to the existing `coauthor --hook` matcher) and share the state-root convention; ours defines `internal/handoff` paths through its own small helper and switches to `internal/sessionidentity`'s root if that lands first (task 2.2). Either archives first. Task 0 live verification overlaps their 0.1/0.7 (payload shapes, `session_id`); one scratch session can answer both.
- `claude-code-parity` (36/47): task 9.4 (live agent re-sync) is the same re-sync this change needs; it also modifies the Iktomi requirement, and its delta is superseded on that point by ours (annotate at archive time, no edit now).
- `harden-commit-hook-enforcement`: archived; reused (`Blocking`, exit 2).
- `parallel-session-otel-receiver`: only the state-root idea is reused; no code overlap.
- Main specs: `janus-router-agent` (two MODIFIED requirements); `dev-workflow-hooks` "Hook-invoked commands distinguish blocking failures..." is extended in spirit: `handoff enforce`/`stop-check` join the exit-2 set, specified in our own capability rather than by editing that header, to avoid a header collision with the guard change.

## Risks / Trade-offs

- A dispatcher that ignores the injected text and the PreToolUse block three times still gets through (bounded escape). Chosen over a hard trap; the abandonment is logged and shown.
- Hook payload assumptions (Decision 2 list) may be wrong on the installed version; task group 0 gates all Go work, and the design has a stated fallback for ordering.
- Blocking `Stop` costs extra model turns whenever a hand-off is pending; that is the point.
- Missing-tag defaults lean toward validation; a wrongly-`complete` blocked turn causes one pointless `phobetor` run.
- Concurrent sessions on one change share a counter by design (both see the same failures); a false escalation to `phantasos` is possible if two sessions each fail once. Accepted; visible in the counter file.
- Two hooks (`inject` and `enforce`) both fire on `Task|Agent`; a dispatcher can still call the required agent with an unrelated prompt. Enforcement guarantees the call happens, not its quality.

## Decisions for the user to confirm

1. `nyx` -> `morpheus` is in the table (not in the required list).
2. Missing handoff tag = complete; missing/malformed verdict = one re-dispatch of `phobetor`, then report.
3. A fourth verdict `unverified` (tests could not run) that neither passes nor fails.
4. `spec-defect` verdict kept as a third `phobetor` outcome (-> `phantasos`, counter unchanged), from the existing spec.
5. Escape hatch: 3 blocks then abandon, `UserPromptSubmit` releases, `handoff_enforcement` opt-out.
6. Counter keyed by change slug (fallback session), per-user state dir, reset on pass/phantasos/baku.
7. Iktomi fold-in: this change owns it; the other is archived `--skip-specs` afterwards.
8. Copilot and the four other platforms get no enforcement in this change.
9. Partial pass (Decision 14): `pass` with unticked tasks reports instead of dispatching `baku`; unknown task state still goes to `baku`.
10. Agent-file sync (Decision 12): marker appended to agent files, marked files auto-refresh on `init`, unmarked legacy files need `init --force` once.
11. Staleness check (Decision 13) is a separate change, not folded in; say if you want it created now.
12. Release fallback (Decision 11): if the live spike finds no reliable human/system discriminator, `release` is dropped entirely.

## Verification results (task group 0)

Run 2026-09-24 against Claude Code 2.1.281 with real `claude -p` sessions in a scratch repo outside this repo (scratch settings binding logging hooks to every event; a one-line `echoer` agent; a second run with `claude -p --agent <dispatcher>`). No `dreamland` build was used; only payload and hook-protocol behavior was tested.

- **0.1 SubagentStop payload (confirmed).** Fields: `session_id` (equal to the dispatcher's), `transcript_path` (dispatcher's), `prompt_id`, `permission_mode`, `agent_id`, `agent_type`, `effort`, `hook_event_name`, `stop_hook_active`, `agent_transcript_path`, `last_assistant_message`, `background_tasks`, `session_crons`. The final report is in `last_assistant_message` verbatim (`"SPIKE-REPORT-9913\n[handoff: complete]"`); no transcript read is needed. **Decision 5: use `last_assistant_message`; `agent_transcript_path` is only a fallback if the field is empty.**
- **0.2 PostToolUse on `Task|Agent` (confirmed).** Matcher `Agent` fires with `tool_name: "Agent"`, `tool_input.subagent_type`, and the report in `tool_response.content[0].text` (also `tool_response.agentId`, `agentType`, `status`). `hookSpecificOutput.additionalContext` reached the dispatcher in the same turn, before its next action (the model quoted it as arriving "after the subagent result"). Caveat: in that run the model declined to act on the injected line because the user had not asked for it, so injection is visible but not binding on its own; this supports keeping `enforce` and `stop-check` as the hard layer. The tool name in 2.1.281 is `Agent`; the `Task|Agent` matcher covers both.
- **0.3 Ordering (confirmed, no fallback needed).** With `SubagentStop` sleeping 3 seconds, the dispatcher's `PostToolUse` started 3.2 seconds after `SubagentStop` started, i.e. after it finished. Observed order: PreToolUse(Agent) -> subagent tool hooks -> SubagentStop (completes) -> PostToolUse(Agent). `record` in `SubagentStop` therefore runs before `inject`. The idempotent-`inject` fallback in Decision 2 is not required, though it is harmless.
- **0.4 Exit 2 (confirmed).** `Stop` exit 2: the model continued and the stderr text was delivered as `Stop hook feedback: [<command>]: <stderr>`; the second `Stop` carries `stop_hook_active: true`, so `stop-check` must NOT treat `stop_hook_active` as "allow" (it would defeat enforcement); the design's own 3-block bound applies instead. `PreToolUse` exit 2 on `Agent`: the call did not run and the model saw a tool error `PreToolUse:Agent hook error: [<command>]: <stderr>` and quoted it exactly.
- **0.5 UserPromptSubmit (confirmed).** Payload has `session_id` and `prompt`. It fired once at the start of the prompt and did not fire between the subagent's return and the dispatcher's next call or `Stop`. `-p` mode only; an interactive session was not tested.
- **Dispatcher discrimination (new, needed by Decision 6).** Subagent-originated hooks (its own `PreToolUse` on `Read`, `SubagentStop`) carry `agent_id` and `agent_type=<subagent>`. Dispatcher hooks carry no `agent_id`; in a plain session they also carry no `agent_type`, and under `claude --agent X` they carry `agent_type: "X"` and still no `agent_id`. **Decision 6 should filter on absence of `agent_id`** (dispatcher) rather than on `agent_type` being absent or `janus`; the `agent_type` rule as written also works for the plain and Janus cases but is brittle. `Stop` under `--agent` has `agent_type` set and no `agent_id`.
- **0.6 GitHub Copilot (UNVERIFIED).** The `copilot` binary found on this machine is a VS Code extension shim that only offers to install the CLI and needs an interactive install and GitHub authentication, so no `SubagentStop` payload was captured. Per the task, `record` is not bound on Copilot and Copilot stays prose plus `handoff next` (task 4.5) until someone captures the payload.

### Live interactive findings (2026-09-24, after rebuild and `dreamland init`)

Recorded from an interactive session; these correct the group-0 conclusions above where they conflict:

- `record` worked: after a `morpheus` probe ending `[handoff: complete]`, `dreamland status` showed the pending entry (from=morpheus kind=dispatch target=phobetor blocks=0).
- No `REQUIRED NEXT STEP` reached the dispatcher, and a wrong-target `Agent` (Explore) was not blocked; the entry was already `released-by-user`.
- Root causes: the `UserPromptSubmit` release fired on the background-completion task-notification (0.5 was `-p` only; superseded by Decision 11), and dispatches were background so `PostToolUse(Agent)` preceded `SubagentStop` (0.3 was foreground only; superseded by Decision 10). Live agent files lacked the tags (Decision 12), and the SHA staleness check warned constantly (Decision 13).

## Open Questions

Task group 10 answers, before any `prompt` classifier code: what the `UserPromptSubmit` payload carries for human, slash-command, queued, and task-notification prompts; what the `Agent` `PostToolUse` payload looks like for a background launch; and whether the notification `UserPromptSubmit` fires after `SubagentStop`.
