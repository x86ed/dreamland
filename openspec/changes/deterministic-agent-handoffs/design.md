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
| `phobetor` | `[verdict: pass]` | dispatch `baku`; delete the change's counter |
| `phobetor` | `[verdict: fail]` | counter 0 -> dispatch `morpheus` (counter becomes 1); counter >= 1 -> dispatch `phantasos` (counter becomes 2) |
| `phobetor` | `[verdict: spec-defect]` | dispatch `phantasos` (counter unchanged) |
| `phobetor` | `[verdict: unverified]` | report (no dispatch): tests could not be run; counter unchanged |
| `phobetor` | tag absent or malformed | dispatch `phobetor` again with "end your report with a verdict tag", at most once per change per turn-chain (`verdict_retries`); a second miss becomes a report |
| `phantasos` | any | no directive; reset the change's counter |
| `baku` | any | no directive; delete the change's counter |

`nyx` -> `morpheus` is included although the user's list did not name it: it is a fixed edge in the existing `janus-router-agent` spec with the same skip exposure, and a table that omits it would be incomplete on purpose. Cheap to drop (one row, one template line); flagged for confirmation.

An absent handoff tag on `nyx`/`morpheus`/`iktomi` means complete, because the failure mode being fixed is a skipped validation, so the default must lean toward validating. The cost is that a blocked-but-untagged `iktomi` turn goes to `phobetor`, which finds nothing to verify and reports `pass` (already accepted in `iktomi-always-handoff-phobetor`'s risks).

`unverified` exists because of the stale-binary history: earlier runs skipped `dreamland test` because the binary was stale. A `phobetor` that could not run its tests must not say `pass` (that would advance to `baku` unverified) and must not say `fail` (that would burn the retry budget and send `morpheus` to fix nothing). The hook never runs tests itself, so a stale binary cannot make the mechanism skip validation, only make `phobetor` report `unverified`.

### Decision 2: Why record, inject, enforce, stop-check (and not block at `SubagentStop`)

`SubagentStop` with a block only keeps the *subagent* running; that subagent cannot dispatch, so a block there cannot produce the next call. The next call must be made by the dispatcher, so the mechanism has to act on the dispatcher's side:

1. **record** (`SubagentStop`): the only place both `agent_type` and the final report are reliably available. Computes the directive from the table, updates the counter, writes a pending entry. This is the sole writer of counters.
2. **inject** (`PostToolUse`, `Task|Agent`): read-only. Emits `additionalContext`: `REQUIRED NEXT STEP (dreamland handoff): call Agent with subagent_type=phobetor now. Do not call any other agent and do not end your turn.` plus the change slug and reason. This is what makes the instruction appear in the dispatcher's context at the moment it matters instead of relying on the subagent's prose.
3. **enforce** (`PreToolUse`, `Task|Agent`): with a pending `dispatch` entry, an `Agent` call whose `subagent_type` is not a pending target exits 2 with a message naming the required target. A matching call clears the entry (oldest first when several are pending, which covers parallel subagents in one session).
4. **stop-check** (`Stop`): pending `dispatch` entries make the stop exit 2, stderr = the required call. This is what catches the "just stops" failure that motivated the change.
5. **release** (`UserPromptSubmit`): a new user prompt while entries are pending releases them as `released-by-user` (visible in `dreamland status`). Hand-offs happen within one dispatcher turn, so the dispatcher cannot cause a user prompt; the user can always regain control.

If `inject` runs before `record` (ordering is a task 0 question), it finds no entry and emits nothing; `enforce` and `stop-check` still hold because they read the entry that `record` writes before the dispatcher's next tool call or stop. If task 0 shows `SubagentStop` fires after the dispatcher already resumed, the fallback is to run the same pure function from `inject` on the `tool_response` (which contains the report) and make `record` idempotent by directive id (sha256 of session id, agent, report). That fallback is specified now so the design does not depend on the ordering.

### Decision 3: Bounded, with a stated escape

A hard block with no exit can trap a session (an agent that cannot be dispatched because it was deleted, a permission denial, a mis-tagged report). So: each pending entry has `blocks`. `enforce` and `stop-check` increment it on each block; at 3 the entry becomes `abandoned`, the hook exits 0 with a stderr warning, and the abandonment is appended to the transition log and listed by `dreamland status`. The skip is therefore never silent, and it needs three separate refusals from the mechanism first. `handoff_enforcement` in `.dreamland.json` accepts `"block"` (default), `"warn"` (inject and log, never exit 2), `"off"`. `dreamland handoff clear [--session <id>]` is a human command.

### Decision 4: Failure counter: location, key, resets (answers the explicit question)

Interpretation confirmed as the user stated: per change, the first `phobetor` failure goes to `morpheus`; a failure after that retry goes to `phantasos`.

- **Where**: `<root>/handoff/<repo-id>/<change>.json`, `<root>` the same per-user state root as `internal/sessionidentity` (`$DREAMLAND_STATE_DIR`, else `<UserCacheDir>/dreamland`, else `<TempDir>/dreamland`). It is per-user and outside the repo so it survives across subagent turns and across parallel sessions of the same repository, and does not dirty the working tree or get committed. `<repo-id>` is the first 16 hex chars of sha256 of the cleaned absolute repository root (lower-cased on Windows), because the state root is shared by all repos. Content: `{"phobetor_failures":n,"verdict_retries":n,"updated_at":"..."}`.
- **Key**: the change slug: `[change: <slug>]` in the `phobetor` report, else the sole active change from `openspec list --json`, else `_session-<session_id>` (free-form `iktomi` work has no change). Slugs must match `^[a-z0-9][a-z0-9-]{0,63}$`.
- **Concurrency**: two sessions can validate the same change. Read-modify-write takes a lock file (`<change>.lock`, `O_CREATE|O_EXCL`, 5 s timeout, stale after 30 s, removed after use), then writes via temp file and `os.Rename`. Lock timeout means fail open: no directive is written, a warning is printed.
- **Resets**: deleted on `[verdict: pass]`; reset to 0 when `phantasos` completes (re-spec) or `baku` completes; entries older than 14 days are pruned on write. Not reset by a `morpheus` turn (the retry must not clear the evidence it failed).
- **Why not in the repo** (e.g. in `tasks.md`): it would create commits and merge conflicts, and the `Stop`/handoff commits would sweep it up. **Why not in the session id only**: a fresh session for the retry would forget the failure; the change is the unit the user described.

### Decision 5: Machine-readable tags

Own-line tags, last occurrence per key wins, matched by `^\s*\[(handoff|verdict|change): ([a-z0-9-]+)\]\s*$` after normalizing `\r\n`. Values outside the closed set for that key are "malformed" (treated as absent). Own-line plus last-wins prevents a report that quotes an earlier tag from steering the mechanism. The report text comes from the `SubagentStop` payload's final message (or the last assistant message read from `agent_transcript_path`; task 0 picks the field). The agent templates say: the final line group of the report is the tag lines, nothing after them.

### Decision 6: Plain main session versus Janus

The hooks live in `.claude/settings.json`, so they fire identically for a plain `claude` session and for `claude --agent janus`, keyed by `session_id`. They act only when the payload has no `agent_type` or `agent_type` is `janus` (the dispatcher); a subagent's own tool calls are ignored. For a plain session the mandatory call is made by the session model itself; for Janus (which under `deterministic-routing-and-janus-guard` has `Agent(<roster>)`) by Janus. Every possible target (`morpheus`, `phobetor`, `baku`, `phantasos`) is in that roster, so the two mechanisms cannot deadlock. `dreamland handoff next` should join that change's read-only Bash allowlist; because that change is unimplemented, this is a coordination note (tasks 8.2), not an edit to its artifacts. Janus's own turns, when spawned as a routing subagent (legacy flow), produce no directive (Janus is not in the table).

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

## Overlap and sequencing

- `iktomi-always-handoff-phobetor`: folded in (Decision 9). Archive this first, then that with `--skip-specs`.
- `deterministic-routing-and-janus-guard` (0/67): both edit `settings-patch.json` (different hook entries: theirs `PreToolUse` guard and `Stop`; ours `SubagentStop`, `PostToolUse`, `PreToolUse` `Task|Agent`, `UserPromptSubmit`; the `PreToolUse` `Task|Agent` matcher already exists with `coauthor --hook`, so ours appends a second command to it, not a second entry) and share the state-root convention; ours defines `internal/handoff` paths through its own small helper and switches to `internal/sessionidentity`'s root if that lands first (task 2.2). Either archives first. Task 0 live verification overlaps their 0.1/0.7 (payload shapes, `session_id`); one scratch session can answer both.
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

## Open Questions

None that block drafting. Task group 0 answers the platform-payload questions before any Go code.
