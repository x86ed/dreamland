## ADDED Requirements

### Requirement: A single edge table defines every fixed hand-off and is the only source the mechanism consults

`internal/handoff/edges.go` SHALL hold the fixed hand-off edges as one Go table, and `handoff.Next(from, tags, counter, tasksRemaining)` SHALL be a pure function over it; `tasksRemaining` is the number of unticked checkboxes in the change's `tasks.md`, or -1 when unknown. The edges SHALL be: `nyx` complete -> dispatch `morpheus`; `morpheus` complete -> dispatch `phobetor`; `iktomi` complete -> dispatch `phobetor`, unconditionally, whether or not files changed; `nyx`/`morpheus`/`iktomi` blocked -> report (no dispatch); `phobetor` `pass` -> dispatch `baku` when `tasksRemaining` is 0 or -1 (unknown), otherwise a `report` directive stating a partial pass with the number of unticked tasks and leaving the choice (next task via `morpheus`, or stop) to the dispatcher and user; `phobetor` `fail` -> dispatch `morpheus` when the change's failure counter is 0, otherwise dispatch `phantasos`; `phobetor` `spec-defect` -> dispatch `phantasos`; `phobetor` `unverified` -> report (no dispatch). An agent not in the table SHALL produce no directive. A directive SHALL be either kind `dispatch` (with a target agent) or kind `report` (no dispatch required).

#### Scenario: Morpheus completion requires Phobetor

- **WHEN** `Next("morpheus", {handoff: "complete"}, counter=0)` is evaluated
- **THEN** it returns kind `dispatch`, target `phobetor`

#### Scenario: Iktomi completion requires Phobetor regardless of file changes

- **WHEN** `Next("iktomi", {handoff: "complete"}, counter=0)` is evaluated, whether or not the turn changed files
- **THEN** it returns kind `dispatch`, target `phobetor`

#### Scenario: A blocked turn reports instead of dispatching

- **WHEN** `Next("iktomi", {handoff: "blocked"}, counter=0)` is evaluated
- **THEN** it returns kind `report` and no target, and the report names Janus as the recipient of the blocker

#### Scenario: Phobetor pass goes to Baku

- **WHEN** `Next("phobetor", {verdict: "pass"}, counter=1)` is evaluated
- **THEN** it returns kind `dispatch`, target `baku`, and the counter after the call is deleted

#### Scenario: Partial pass does not close the change

- **WHEN** `Next("phobetor", {verdict: "pass", change: "c1"}, counter=0, tasksRemaining=3)` is evaluated
- **THEN** it returns kind `report` (no dispatch, no `baku`) whose text says 3 tasks of `c1` remain unticked, and the failure counter is deleted as for any pass

#### Scenario: Fully ticked tasks go to Baku

- **WHEN** `Next("phobetor", {verdict: "pass", change: "c1"}, counter=0, tasksRemaining=0)` is evaluated
- **THEN** it returns kind `dispatch`, target `baku`

#### Scenario: Unknown task state keeps the fixed edge

- **WHEN** a `phobetor` `pass` has no `[change: <slug>]` tag or the change has no readable `tasks.md` with checkboxes (`tasksRemaining=-1`)
- **THEN** it returns kind `dispatch`, target `baku`

#### Scenario: An unlisted agent produces no directive

- **WHEN** `Next("baku", ...)` or `Next("hypnos", ...)` is evaluated
- **THEN** it returns no directive

### Requirement: Reports carry machine-readable tags and the mechanism never judges prose

`morpheus`, `iktomi`, and `nyx` SHALL end their final report with an own-line tag `[handoff: complete]` or `[handoff: blocked]`. `phobetor` SHALL end its final report with exactly one own-line tag `[verdict: pass]`, `[verdict: fail]`, `[verdict: spec-defect]`, or `[verdict: unverified]`, and MUST add `[change: <slug>]` whenever it is working a change (the partial-pass guard reads it). `phantasos` and `baku` SHALL end their final report with `[change: <slug>]` when the turn concerns a change (they produce no directive; the tag only selects which counter to reset). Parsing SHALL match lines against `^\s*\[(handoff|verdict|change): ([a-z0-9-]+)\]\s*$` after normalizing `\r\n`; the last matching line per key wins; a value outside the closed set for its key is treated as absent. An absent `handoff` tag on `nyx`/`morpheus`/`iktomi` SHALL be treated as `complete`. An absent or malformed `verdict` from `phobetor` SHALL yield a `dispatch` directive to `phobetor` again with an instruction to emit the verdict tag, at most once per change until a valid verdict is seen; a second miss SHALL yield kind `report`. `phobetor` SHALL emit `unverified` and never `pass` when `dreamland test` could not be run.

#### Scenario: Last tag wins and quoted tags are ignored

- **WHEN** a `phobetor` report quotes `[verdict: fail]` inline in a sentence and ends with the own-line tag `[verdict: pass]`
- **THEN** the parsed verdict is `pass`

#### Scenario: Missing handoff tag means complete

- **WHEN** a `morpheus` report has no `[handoff: ...]` line
- **THEN** the directive is `dispatch` `phobetor`, and the pending entry records `tag_missing: true`

#### Scenario: Missing verdict re-dispatches Phobetor once, then reports

- **WHEN** a `phobetor` report has no valid verdict tag, twice in a row for the same change
- **THEN** the first yields `dispatch` `phobetor` with the instruction to emit the tag, and the second yields kind `report` and does not change the failure counter

#### Scenario: Unverified neither passes nor fails

- **WHEN** a `phobetor` report ends with `[verdict: unverified]`
- **THEN** the directive is kind `report`, the failure counter is unchanged, and no dispatch is required

### Requirement: The failure counter is per change, stored per user, and survives across subagent turns and parallel sessions

For each `phobetor` `fail` the mechanism SHALL consult a counter stored at `<root>/handoff/<repo-id>/<change>.json`, where `<root>` is `$DREAMLAND_STATE_DIR` if set, else `<os.UserCacheDir()>/dreamland`, else `<os.TempDir()>/dreamland`, and `<repo-id>` is the first 16 hexadecimal characters of the SHA-256 of the cleaned absolute repository root (lower-cased on Windows). `<change>` is the `[change: <slug>]` tag, else the sole active change from `openspec list --json`, else `_session-<session_id>`; slugs SHALL match `^[a-z0-9][a-z0-9-]{0,63}$`. With counter 0 a `fail` yields `dispatch` `morpheus` and sets the counter to 1; with counter of 1 or more a `fail` yields `dispatch` `phantasos` and sets it to 2. A `spec-defect` verdict yields `dispatch` `phantasos` and leaves the counter unchanged. The counter SHALL be deleted on `pass`, and reset to 0 when `phantasos` or `baku` completes; a `morpheus` turn SHALL NOT change it. Every counter file SHALL also record the `session_id` that last wrote it. A `phantasos` or `baku` completion with no `[change: <slug>]` tag SHALL reset the most recently updated counter file written by that same `session_id` (the sole-active-change fallback is not used for these two agents, because `baku` completes after the change has been archived); if that session wrote none, it is a no-op. Files older than 14 days SHALL be pruned on write. Every read-modify-write SHALL hold a lock file created with `O_CREATE|O_EXCL` (5 second acquire timeout, treated as stale after 30 seconds) and write via a temp file renamed over the target; a lock timeout SHALL fail open (no directive, stderr warning, exit 0).

#### Scenario: First failure goes to Morpheus

- **WHEN** `phobetor` reports `[verdict: fail]` for change `c1` and no counter file exists
- **THEN** the directive is `dispatch` `morpheus` and the counter for `c1` is 1

#### Scenario: Failure after a Morpheus retry escalates to Phantasos

- **WHEN** `phobetor` reports `[verdict: fail]` for `c1` and its counter is 1
- **THEN** the directive is `dispatch` `phantasos` and the counter is 2

#### Scenario: Pass clears the counter

- **WHEN** `phobetor` reports `[verdict: pass]` for `c1` with counter 1
- **THEN** the directive is `dispatch` `baku` and the counter file for `c1` no longer exists

#### Scenario: Phantasos completion resets the counter

- **WHEN** `phantasos` completes a turn and `c1`'s counter is 2
- **THEN** the counter for `c1` is 0 and the next `phobetor` failure yields `dispatch` `morpheus`

#### Scenario: Untagged baku completion resets this session's counter

- **WHEN** `baku` completes with no `[change: ...]` tag and the most recent counter file written by this session is for `c1`
- **THEN** the counter for `c1` is reset and counters written by other sessions are untouched

#### Scenario: Two parallel sessions do not lose a failure

- **WHEN** two sessions in the same repository each record a `phobetor` failure for `c1` concurrently
- **THEN** the counter ends at 2, never 1, and neither write is lost

#### Scenario: State outside the repository

- **WHEN** a failure is recorded
- **THEN** no file inside the repository working tree is created or modified

### Requirement: The dispatcher's next call is made mandatory through hook modes on Claude Code, for foreground and background dispatch alike

`dreamland handoff` SHALL provide these modes, each bound as its own hook entry in `settings-patch.json` (never appended to another hook's command list, never in agent frontmatter), each reading the hook payload from stdin once. A payload is the dispatcher's when it has no `agent_id` (a plain session carries no `agent_type`; a `claude --agent X` session carries `agent_type: "X"` and still no `agent_id`); every mode except `record` SHALL ignore payloads that carry `agent_id`. `record` SHALL act only on a payload that carries `agent_id` and `agent_type` (a subagent's `SubagentStop`) and otherwise exit 0 without writing.

- `handoff record --hook` on `SubagentStop`: parse `last_assistant_message`, evaluate `Next`, update the counter, and write a pending entry at `<root>/handoff/<repo-id>/pending/<session_id>.json` (atomic write) whether the dispatch was foreground or background. It is the only writer of counters.
- `handoff inject --hook` on `PostToolUse` with matcher `Task|Agent`, and `handoff prompt --hook` on `UserPromptSubmit` (see the release requirement): both call the same read-only routine, which for every pending `dispatch` entry emits `hookSpecificOutput.additionalContext` beginning `REQUIRED NEXT STEP (dreamland handoff): call Agent with subagent_type=<target> now. Do not call any other agent and do not end your turn.` and for a pending `report` entry emits the report text instead. The routine SHALL depend only on the pending entries present when it runs, never on hook ordering or on the `Agent` result's status: when no entry exists it emits nothing and exits 0 (for example a `PostToolUse` at the launch of a background dispatch, before `SubagentStop` has run).
- `handoff enforce --hook` on `PreToolUse` with matcher `Task|Agent`: reads the pending entries at call time (so an entry written after a background launch is enforced). While a `dispatch` entry is pending, an `Agent` call whose `subagent_type` is not a pending target SHALL exit 2 with a message naming the required target; a call to a pending target SHALL clear the oldest matching entry and exit 0. Any dispatcher `Agent` call SHALL also clear pending `report`-kind entries (they have been delivered and never block).
- `handoff stop-check --hook` on `Stop`: while a `dispatch` entry is pending the process SHALL exit 2 with the required call on stderr. It SHALL NOT treat `stop_hook_active: true` as permission to stop; only the block bound below ends the blocking.

Foreground dispatch: `SubagentStop` (and so `record`) completes before the dispatcher's `PostToolUse`, so `inject` delivers the directive at the result. Background dispatch (the `Agent` call returns immediately and the subagent finishes later): `PostToolUse` fires at launch with no entry, `record` runs at the later `SubagentStop`, and the directive reaches the dispatcher through `prompt --hook` on the `UserPromptSubmit` event the harness raises for the completion notification, and, if the dispatcher tries to stop or dispatch first, through `enforce` and `stop-check`. The `session_id` SHALL be valid per `^[A-Za-z0-9._-]{1,128}$` (not `.` or `..`); otherwise the mode exits 0 with a warning. `record` SHALL be idempotent by directive id (SHA-256 of session id, agent, and report), so a replayed `SubagentStop` does not create a second entry.

#### Scenario: Foreground injection names the required target

- **WHEN** a foreground `morpheus` subagent finishes with `[handoff: complete]` and the dispatcher's `PostToolUse` for the `Agent` call fires
- **THEN** the dispatcher's context receives text beginning `REQUIRED NEXT STEP (dreamland handoff): call Agent with subagent_type=phobetor now`

#### Scenario: Background launch injects nothing, completion notification injects

- **WHEN** a `morpheus` subagent is dispatched in the background, the `PostToolUse` for the launch fires (no entry exists), `SubagentStop` then writes a `dispatch` entry for `phobetor`, and the harness raises a `UserPromptSubmit` for the completion notification
- **THEN** the `PostToolUse` mode exits 0 with no output, and the `prompt` mode on the notification emits text beginning `REQUIRED NEXT STEP (dreamland handoff): call Agent with subagent_type=phobetor now`

#### Scenario: Entry written after launch is enforced

- **WHEN** a background `morpheus` dispatch has launched, its `SubagentStop` has since written a pending `phobetor` entry, and the dispatcher calls `Agent` with `subagent_type=Explore`
- **THEN** `handoff enforce` exits 2 and its message names `phobetor`

#### Scenario: Dispatching a different agent is blocked

- **WHEN** a `dispatch` entry for `phobetor` is pending and the dispatcher calls `Agent` with `subagent_type=nyx`
- **THEN** `handoff enforce` exits 2 and its message names `phobetor`

#### Scenario: The required call clears the entry

- **WHEN** a `dispatch` entry for `phobetor` is pending and the dispatcher calls `Agent` with `subagent_type=phobetor`
- **THEN** `handoff enforce` exits 0 and the entry is removed

#### Scenario: A report entry never blocks and is cleared by the next dispatch

- **WHEN** a `report` entry is pending, `Stop` fires, and later the dispatcher calls any `Agent`
- **THEN** `handoff stop-check` exits 0, and `handoff enforce` exits 0 and removes the `report` entry

#### Scenario: Ending the turn without the hand-off is blocked, including on the second Stop

- **WHEN** a `dispatch` entry is pending and the `Stop` hook fires, and fires again with `stop_hook_active: true`
- **THEN** `handoff stop-check` exits 2 with the required call on stderr both times (until the block bound abandons the entry)

#### Scenario: A subagent's own calls are ignored

- **WHEN** a `PreToolUse` payload carries `agent_id` (with `agent_type` `morpheus`)
- **THEN** `handoff enforce` exits 0 without reading pending entries

#### Scenario: A dispatcher under --agent is still the dispatcher

- **WHEN** a `PreToolUse` payload carries `agent_type: "janus"` and no `agent_id`
- **THEN** `handoff enforce` treats it as the dispatcher and applies pending entries

#### Scenario: Record ignores a payload without agent_id

- **WHEN** a `SubagentStop`-shaped payload has no `agent_id`
- **THEN** `handoff record` exits 0 and writes nothing

#### Scenario: Parallel subagents each require a call

- **WHEN** two `morpheus` subagents finish in one session, creating two pending `phobetor` entries, and the dispatcher calls `phobetor` once
- **THEN** one entry remains pending and the `Stop` hook still blocks

### Requirement: Enforcement is bounded, releases only on a real human prompt, never silent to skip, and fails open on internal faults

Each pending entry SHALL count `blocks`; `enforce` and `stop-check` SHALL block (exit 2) at most twice per entry and increment the count each time, and the third violating call or `Stop` SHALL instead mark the entry `abandoned`, exit 0 with a stderr warning, append the abandonment to the transition log, and list it in `dreamland status`.

A `UserPromptSubmit` binding `handoff prompt --hook` SHALL classify each prompt. Only a prompt positively identified as a human prompt (typed text, including a typed slash command such as `/drmlnd:morpheus`) SHALL mark pending entries `released-by-user`. A prompt the harness generates itself, in particular the background-agent completion task-notification, SHALL NOT release; it SHALL run the inject routine instead. The classifier SHALL use only a discriminator proven in a live interactive session (task group 10): a payload field that names the prompt's origin if one exists, else a match of the prompt text against the harness's notification envelope. If task group 10 shows neither is reliable, the `UserPromptSubmit` release SHALL be removed (no `prompt` binding for release), the notification `inject` SHALL be bound on `UserPromptSubmit` without any release, and the exits from a trapped session are the two-block bound and `dreamland handoff clear`. When the classifier is unsure it SHALL NOT release.

`.dreamland.json` `handoff_enforcement` SHALL accept `"block"` (default), `"warn"` (`inject` and logging only; no mode exits 2), and `"off"` (no mode acts). Any unreadable or invalid state file, lock timeout, malformed payload, or invalid `session_id` SHALL cause exit 0 with a stderr warning; only a successfully read `dispatch` entry blocks. `dreamland handoff clear [--session <id>]` SHALL remove entries and is a human command.

#### Scenario: Third violation abandons the entry

- **WHEN** an entry has been blocked twice and a third violating call or `Stop` occurs
- **THEN** the hook exits 0 with a warning, the entry is `abandoned`, and `dreamland status` lists it

#### Scenario: A human prompt releases a trapped session

- **WHEN** entries are pending and a `UserPromptSubmit` event for a typed prompt or typed slash command fires
- **THEN** the entries are marked `released-by-user` and later `enforce`/`stop-check` calls exit 0

#### Scenario: A background-completion notification does not release

- **WHEN** a `dispatch` entry is pending and the `UserPromptSubmit` event is the harness's background-agent task-notification
- **THEN** the entry stays `pending` (not `released-by-user`), and the dispatcher's context receives the `REQUIRED NEXT STEP` text

#### Scenario: Unclassifiable prompt does not release

- **WHEN** the classifier cannot tell whether a `UserPromptSubmit` prompt is human
- **THEN** the entry stays `pending`

#### Scenario: Corrupt state never blocks

- **WHEN** the pending file contains invalid JSON
- **THEN** every mode exits 0 and prints a warning

#### Scenario: Warn mode never blocks

- **WHEN** `handoff_enforcement` is `"warn"` and a violating call occurs
- **THEN** `handoff enforce` exits 0 after logging the violation

### Requirement: Plain sessions and Janus are enforced identically, keyed by session

The hooks SHALL be bound at the workspace `settings.json` level so they fire for a plain `claude` session and for a session started with `--agent janus`, keyed by the payload `session_id`; the dispatcher is identified by the absence of `agent_id`, not by `agent_type`. Every directive target (`morpheus`, `phobetor`, `baku`, `phantasos`) SHALL be a registered agent that a Janus main thread with `Agent(<registered roster>)` may dispatch. Janus turns spawned as routing subagents SHALL produce no directive.

#### Scenario: Plain session

- **WHEN** a session started without `--agent` receives a `morpheus` report
- **THEN** the injection, `PreToolUse` block, and `Stop` block apply to that session

#### Scenario: Janus main thread

- **WHEN** a session started with `--agent janus` receives a `phobetor` `pass` report
- **THEN** the same modes require `Agent(subagent_type=baku)`

### Requirement: The mechanism is Windows-safe and tolerates a stale binary

Hook bindings SHALL contain no shell operators, SHALL use only stdin JSON, `filepath`, `os.Rename`, and `O_EXCL` lock files (no `flock`, signals, or process inspection), and parsing SHALL tolerate CRLF. A `_windows` build and test SHALL exist in the style of `cmd/otel_receiver_windows_build_test.go`. The mechanism SHALL NOT invoke `dreamland test`. `dreamland status` and `dreamland init` SHALL parse `.claude/settings.json` and warn when a bound `dreamland <subcommand>` is unknown to the running binary.

#### Scenario: Old binary is reported

- **WHEN** `.claude/settings.json` binds `dreamland handoff record --hook` and the running binary has no `handoff` subcommand
- **THEN** `dreamland status` warns naming the binding

#### Scenario: A skipped test does not skip a hand-off

- **WHEN** `phobetor` could not run `dreamland test` because the binary was stale and reports `[verdict: unverified]`
- **THEN** no counter changes, no `baku` dispatch is required, and the report is surfaced

### Requirement: Drift between the edge table and agent templates fails the build

A Go test SHALL parse, for every platform's `nyx`, `morpheus`, `iktomi`, and `phobetor` templates, the ``hand off directly to `<agent>` `` targets (the pattern in `internal/workflowgraph/import.go`) and the tag instruction, and assert them equal to the edge table's targets for that agent; it SHALL also assert that the Claude Code `settings-patch.json` binds every mode (`record`, `inject`, `enforce`, `stop-check`, `prompt`) as its own hook entry, and that `iktomi` templates contain no file-changed conditional.

#### Scenario: Template disagrees with table

- **WHEN** the `iktomi` template names `janus` as its completion target
- **THEN** the drift test fails naming the platform and agent

#### Scenario: Missing binding

- **WHEN** `settings-patch.json` omits `handoff stop-check`
- **THEN** the drift test fails

### Requirement: Platforms without enforceable hooks receive tags and a CLI, not enforcement

`dreamland handoff next --from <agent> [--verdict <v>] [--handoff <h>] [--change <slug>]` SHALL print the directive as JSON, applying and persisting the counter exactly as `record` does. Cursor, Codex, Kiro, and Antigravity templates for `morpheus`, `iktomi`, `nyx`, and `phobetor` SHALL include the tag instruction and one line telling the dispatcher to run `dreamland handoff next`. GitHub Copilot SHALL get `record` only if its `SubagentStop` payload is verified; otherwise the same prose and CLI.

#### Scenario: CLI applies the counter

- **WHEN** `dreamland handoff next --from phobetor --verdict fail --change c1` is run twice
- **THEN** the first prints target `morpheus` and the second prints target `phantasos`

### Requirement: Dreamland-managed agent files are re-synced when the template changes

Agent files installed by `dreamland init` SHALL carry the `DreamlandManagedMarker` (`<!-- dreamland-managed: safe to overwrite on `dreamland init` -->`, appended after the body so frontmatter stays on line 1). On every platform `dreamland init` SHALL overwrite an existing agent file that carries the marker and differs from the current template, without `--force`, and report `updated`. An existing agent file without the marker SHALL NOT be overwritten by a plain `dreamland init` (it may be user-owned); `init` SHALL warn naming each such file that differs from its template and stating that `dreamland init --force` adopts it (overwrites and adds the marker). `dreamland status` SHALL list agent files that are out of date or unmarked-and-different.

#### Scenario: Marked file is refreshed

- **WHEN** `.claude/agents/morpheus.md` carries the marker and lacks the `[handoff:]` instruction present in the current template, and `dreamland init` runs without `--force`
- **THEN** the file is overwritten with the template plus marker and reported `updated`

#### Scenario: Legacy unmarked file is not silently overwritten

- **WHEN** `.claude/agents/iktomi.md` has no marker and differs from the template, and `dreamland init` runs without `--force`
- **THEN** it is left unchanged and a warning names it and `--force`

#### Scenario: Force adopts a legacy file

- **WHEN** `dreamland init --force` runs over that unmarked file
- **THEN** it is overwritten with the template plus marker, and the next plain `dreamland init` refreshes it
