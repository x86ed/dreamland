## MODIFIED Requirements

### Requirement: dreamland commit auto-commits pending changes on turn completion and agent handoff

A new lifecycle command, `dreamland commit --reason <turn-complete|handoff>`, SHALL run at end-of-turn and handoff-complete events so that every agent turn and every agent-to-agent transition is captured as its own commit, even if the agent itself never ran `git commit`.

Behavior:

1. If no git repository exists at or above the current working directory, exit 0 silently (a skip, not a failure) — there is nothing for this command to commit to.
2. Inspect `git status --porcelain`. If there are no staged or unstaged changes, exit 0 silently — no commit is created.
3. Before staging or committing anything, when `commitReason` is `turn-complete`, consult `.dreamland/last-test-result.json` (written by `dreamland test`, see the modified `test` requirement above), and `cfg.TestCommand` from `.dreamland.json`:
   - If the record exists and records `"status": "fail"` for a `head_sha` matching the repository's current `HEAD`, `dreamland commit` SHALL refuse to create the commit — no `git add`, no `git commit` — and SHALL exit with the blocking code (2). The pending changes remain staged/unstaged, to be picked up by the next successful `dreamland commit` invocation once tests pass.
   - If the record does not exist at all, and `cfg.TestCommand` is non-empty (a test command is configured for this project), `dreamland commit` SHALL treat this as a broken invariant, not a skip: it SHALL refuse to create the commit and SHALL exit with the blocking code (2), with a message stating that a test command is configured but no result was recorded for this turn and instructing that this should route to `iktomi` to investigate why `dreamland test` did not run or record a result before this commit attempt.
   - If the record does not exist and `cfg.TestCommand` is empty (no test command configured for this project), this gating step does not apply — there was never an expectation of a result — and `commit` proceeds normally.
   - If the record exists but its `head_sha` does not match the current `HEAD` (a stale record, not a missing one), this gating step does not apply and `commit` proceeds normally — staleness is treated differently from absence because a stale record at least proves tests ran successfully at some point, while absence (with a test command configured) proves they may not have run this turn at all.
4. Otherwise, stage all changes (`git add -A`) and run `git commit -m "chore: <reason> checkpoint (<agent-name>)"`, where `<agent-name>` is the resolved agent identity, resolved in this order (first match wins): (a) an explicit `--agent-name`; (b) for `--reason handoff` only, the `--hook` payload's `agent_type`/`subagent_type` when it is a registered agent (a `SubagentStop` payload's `agent_type` is the subagent that just finished); (c) when `--hook` is set and the payload carries a valid `session_id` (see the `session-agent-identity` capability's per-session identity requirement), the identity recorded for that session in the per-user state directory, or `janus` when the session has no valid recorded identity; (d) otherwise, the configured `git config --local user.name` (a shared, last-writer-wins value; reached only when no valid `session_id` is available, i.e. on platforms whose bindings do not pass `--hook` at their stop event, or a manual invocation); (e) otherwise the same resolution `coauthor` uses, ending in the `janus` default. For `--reason turn-complete` the payload's own `agent_type` is deliberately ignored and the payload is read only for `session_id`: at `Stop` that field is the session's main-thread agent (e.g. `janus` under `claude --agent janus`), not the last dispatched specialist, and using it would relabel a turn that dispatched `nyx` as `janus`. The commit's author and committer are pinned to the resolved identity explicitly (`git -c user.name=<agent-name> -c user.email=<cleaned-name><suffix> commit ...`), independent of the shared `git config` value, so the subject and the author agree by construction.
5. Because this shells out to `git commit`, the already-installed `prepare-commit-msg` hook fires normally and appends the Co-authored-by trailer and token-usage report (see the modified `coauthor` requirement) to the commit message — `dreamland commit` does not duplicate that logic.
6. A failure in `git add -A` or `git commit` itself (distinct from the test-gating refusal in step 3) is a genuine, blocking failure — exit code 2 — for `--reason turn-complete`, since the entire purpose of this command at end of turn is to guarantee a commit exists. For `--reason handoff` specifically, such a failure SHALL be reported to the user but SHALL NOT block the sub-agent's `SubagentStop` event: the command returns a non-blocking error (exit 1), so a transient git failure during hand-off cannot trap the fixed pipeline (`nyx`→`morpheus`→`phobetor`→`baku`) mid-transition. A `git commit` that reports "nothing to commit" because a concurrent session already committed the same staged changes is a benign no-op for both reasons.

The scaffold installer SHALL bind, on Claude Code, `dreamland test-and-commit --reason turn-complete --hook` to the `Stop` event (see the added `dreamland test-and-commit` requirement below, and the `session-agent-identity` capability for why `--hook` is passed and how it reaches the commit step) and `dreamland commit --reason handoff --hook` to the `SubagentStop` event, alongside `dreamland telemetry write --tool claude-code` and the version-bump commands; `dreamland coauthor` is not registered under `SubagentStop`. On any other platform whose `Stop`-equivalent binding chains `dreamland test` immediately before `dreamland commit --reason turn-complete`, the installer SHALL likewise bind `dreamland test-and-commit --reason turn-complete` in place of that pair (no `--hook`, unchanged from before this change), closing the race where the hook runner does not guarantee `test` finishes writing `.dreamland/last-test-result.json` before a separately-dispatched `commit` reads it. The test-gating step in behavior item 3 applies only to `--reason turn-complete`, so that race does not apply to handoff commits. On platforms without a `SubagentStop`-equivalent event, only the `Stop`-bound invocation applies; handoff commits there rely on the agent-driven convention described for `coauthor`/`telemetry write`.

#### Scenario: Commit created when a turn completes with pending changes

- **WHEN** `dreamland commit --reason turn-complete` runs via the `Stop` hook, `git status --porcelain` shows pending changes, and either no `.dreamland/last-test-result.json` record exists or it records a `"pass"` for the current `HEAD`
- **THEN** the changes are staged and committed with subject `chore: turn-complete checkpoint (<agent-name>)`, where `<agent-name>` is the identity resolved by behavior item 4

#### Scenario: No-op when a turn completes with a clean working tree

- **WHEN** `dreamland commit --reason turn-complete` runs and `git status --porcelain` is empty
- **THEN** the command exits 0 without creating a commit

#### Scenario: Commit refused when tests are failing at the current HEAD

- **WHEN** `dreamland commit --reason turn-complete` runs, `git status --porcelain` shows pending changes, and `.dreamland/last-test-result.json` records `"status": "fail"` for a `head_sha` matching the current `HEAD`
- **THEN** no `git add` or `git commit` is run, the pending changes remain uncommitted, and `dreamland commit` exits with the blocking code (2)

#### Scenario: Stale test-result record does not gate a commit

- **WHEN** `dreamland commit --reason turn-complete` runs and `.dreamland/last-test-result.json` records a `head_sha` that does not match the current `HEAD`
- **THEN** the gating step in behavior item 3 does not apply, and the commit proceeds normally per the other scenarios

#### Scenario: Commit refused and routed to iktomi when a configured test command recorded no result

- **WHEN** `dreamland commit --reason turn-complete` runs, `cfg.TestCommand` is non-empty, and no `.dreamland/last-test-result.json` file exists at all
- **THEN** no `git add` or `git commit` is run, `dreamland commit` exits with the blocking code (2), and the error message states that a test command is configured but no result was recorded, instructing that this routes to `iktomi` for investigation

#### Scenario: Missing test-result record does not gate a commit when no test command is configured

- **WHEN** `dreamland commit --reason turn-complete` runs, `cfg.TestCommand` is empty (or `.dreamland.json` is absent), and no `.dreamland/last-test-result.json` file exists
- **THEN** the gating step in behavior item 3 does not apply, and the commit proceeds normally

#### Scenario: Commit subject and git author never diverge

- **WHEN** `dreamland commit --reason turn-complete --hook` resolves agent identity `phobetor` for the session and `git config --local user.name` currently holds a different value
- **THEN** the resulting commit's subject reads `chore: turn-complete checkpoint (phobetor)` and its author and committer name are `phobetor`, independent of the shared git config

#### Scenario: Stop-time identity comes from the session's own record

- **WHEN** `dreamland commit --reason turn-complete --hook` runs with a `Stop` payload whose `session_id` is `S1`, the per-session record for `S1` holds `nyx`, and another session `S2` has since recorded `morpheus` and overwritten `git config --local user.name` with `morpheus`
- **THEN** the commit subject and author are `nyx`

#### Scenario: The Stop payload's own agent_type does not override the session record

- **WHEN** `dreamland commit --reason turn-complete --hook` runs with a payload `{"session_id":"S1","agent_type":"janus"}` and the record for `S1` holds `nyx`
- **THEN** the commit subject and author are `nyx`

#### Scenario: A valid session with no record is janus, not the shared git config

- **WHEN** `dreamland commit --reason turn-complete --hook` runs with a payload carrying a valid `session_id` that has no recorded identity, and `git config --local user.name` holds `morpheus`
- **THEN** the commit subject and author are `janus`

#### Scenario: Without a session_id the shared git config is still the fallback

- **WHEN** `dreamland commit --reason turn-complete` runs with no `--hook` (or a payload with no valid `session_id`) and `git config --local user.name` holds `phobetor`
- **THEN** the commit subject reads `chore: turn-complete checkpoint (phobetor)`


#### Scenario: Commit author is pinned regardless of a concurrent git config change

- **WHEN** `dreamland commit` resolves agent identity `X` and `git config --local user.name` currently holds a different value
- **THEN** the commit's author and committer name/email are set explicitly to `X` (`git -c user.name=X -c user.email=...`), matching the `(X)` in the subject

#### Scenario: Handoff identity comes from the SubagentStop payload

- **WHEN** `dreamland commit --reason handoff --hook` runs with payload `{"agent_type": "morpheus"}`, the session record for its `session_id` holds `nyx`, and `git config --local user.name` reads `janus`
- **THEN** the commit's subject is `chore: handoff checkpoint (morpheus)` and its author and committer are `morpheus`

#### Scenario: A concurrent commit that already covered the changes is a benign no-op

- **WHEN** `git commit` reports "nothing to commit" because another session committed the same changes between the status check and the commit
- **THEN** the command exits 0 for both `turn-complete` and `handoff`

#### Scenario: Handoff commit created when Janus hands off to another agent

- **WHEN** a sub-agent's turn ends via `SubagentStop` and `git status --porcelain` shows pending changes
- **THEN** `dreamland commit --reason handoff` stages and commits those changes with subject `chore: handoff checkpoint (<outgoing-agent-name>)` before Janus regains control

#### Scenario: Claude Code settings.json binds test-and-commit --hook to Stop and commit --hook to SubagentStop

- **WHEN** `dreamland init` completes with "Claude Code" selected
- **THEN** `.claude/settings.json` contains `dreamland test-and-commit --reason turn-complete --hook` under the `Stop` event key, and does not contain a separate `dreamland test` entry followed by `dreamland commit --reason turn-complete`
- **AND** contains `dreamland commit --reason handoff --hook` under the `SubagentStop` event key


#### Scenario: commit skips silently outside a git repository

- **WHEN** `dreamland commit` runs in a directory with no git repository at or above it
- **THEN** the command exits 0 without error and does not attempt `git status`, `git add`, or `git commit`

#### Scenario: A genuine git failure during a turn-complete commit blocks the lifecycle event

- **WHEN** `dreamland commit --reason turn-complete` has pending changes to commit (and is not refused by the test-gating step) but `git add -A` or `git commit` itself fails
- **THEN** the command exits with the blocking code (2), not the advisory code

#### Scenario: A transient git failure during handoff does not block the sub-agent from stopping

- **WHEN** `dreamland commit --reason handoff` is invoked and the underlying `git commit` call fails for a reason other than "nothing to commit"
- **THEN** the command exits with a non-blocking status (exit code 1, not 2) and the sub-agent's `SubagentStop` event completes, with the failure surfaced to the user
