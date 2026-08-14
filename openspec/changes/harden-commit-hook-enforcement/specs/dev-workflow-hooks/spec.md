## MODIFIED Requirements

### Requirement: coauthor sets agent identity and installs prepare-commit-msg hook

`dreamland coauthor` SHALL run at the session-start lifecycle event and perform two actions:

**a. Set agent git identity (repository-local scope):**

AgentName resolution tries, in order:

1. A hook stdin payload for the current invocation, read only when `dreamland coauthor` is invoked with `--hook` — a flag set exclusively by dreamland's own scaffold-installed hook-binding templates (every platform: Claude Code, GitHub Copilot, Cursor, Codex, Kiro), never by a human running the command manually. Without `--hook`, stdin is never opened or read at all, so a manual invocation in any terminal returns immediately with no possibility of blocking, regardless of what kind of stdin is attached. With `--hook`, the payload is read synchronously to completion (no timeout) — correct because every hook-binding caller writes its payload and closes its end of the pipe promptly, so the read completes as soon as the real data arrives, however long that legitimately takes, checked for an agent identifier in whichever shape the platform actually emits:
   - GitHub Copilot: top-level `agent_type` (e.g. `"morpheus"`) on `SubagentStart`/`SubagentStop` payloads.
   - Claude Code: `tool_input.subagent_type` (e.g. `"morpheus"`) on the `PreToolUse`/`PostToolUse` payload for the `Task`/`Agent` tool call — Claude Code does not emit a top-level `agent_type` field, and `SessionStart`/`Stop`/`SubagentStop` payloads on Claude Code do not carry a sub-agent identifier at all (only `session_id`/`transcript_path`/`hook_event_name`), so this path only resolves anything on the `PreToolUse`/`PostToolUse` hook for that tool.
2. The platform's current-agent env var, if the platform sets one at runtime (no currently-supported platform does; this path exists for forward compatibility and is not exercised by Claude Code or GitHub Copilot).
3. The coding tool name in `.dreamland.json`.
4. If a hook payload resolved a candidate value (step 1) that is not one of the ten registered dreamland agent names (`janus`, `phantasos`, `nyx`, `morpheus`, `phobetor`, `baku`, `iktomi`, `zhougong`, `hypnos`, `mengpo`), that candidate is discarded — treated the same as if step 1 had resolved nothing — rather than used verbatim.
5. On Claude Code specifically, if steps 1-2 resolve nothing, AgentName is `janus`, not the coding-tool name — see the `session-agent-identity` capability for the full "no default, no unknown agent" requirement this satisfies. On every other platform, step 3 (coding-tool name) remains the fallback when steps 1-2 resolve nothing, unchanged from prior behavior.

This full resolution sequence (steps 1-5) is exposed as a single internal function so no other command re-implements it independently; the `dreamland commit` requirement below reads the *result already persisted by this logic* (the git config value this requirement sets, per part **a** below) rather than re-running steps 1-5 itself, since `commit` may run at a different hook event with a different payload shape than the `coauthor` invocation that last set identity — see that requirement for why independent re-resolution at a different lifecycle event would be incorrect.

AgentEmail is derived by cleaning AgentName and appending `email_suffix` from `.dreamland.json` (default `@github.com`).

Email cleaning: lowercase → replace spaces and underscores with `-` → strip characters not in `[a-z0-9.\-]` → trim leading/trailing `-` and `.`.

`git config --local user.name` is set to AgentName. `git config --local user.email` is set to AgentEmail. A failure to write either value with `git config` is a genuine, blocking failure (exit code 2 — see the "Hook-invoked commands distinguish blocking failures from advisory skips" requirement), not an advisory one: identity enforcement is the entire purpose of this command.

If no git repository exists at or above the current working directory, `dreamland coauthor` SHALL exit 0 silently (a skip, not a failure) — there is no identity to set in a location `git config --local` cannot apply to. Any other failure while loading `.dreamland.json` (e.g. malformed JSON) is a genuine, blocking failure.

This identity logic is identical for every scaffolded agent (Janus, Phantasos, Nyx, Morpheus, Phobetor, Baku, Iktomi, Zhou Gong, Hypnos, Meng Po) — none of them get special-cased behavior; only the AgentName value resolved per invocation differs.

**b. Install a `prepare-commit-msg` git hook:**

Write (or update) `.git/hooks/prepare-commit-msg` as a minimal shell wrapper that delegates to `dreamland`:

```sh
#!/bin/sh
dreamland coauthor --trailer "$1" "$2" "$3"
```

When invoked with `--trailer`, `dreamland coauthor` reads `$1` (commit message file path), constructs model identity from `.dreamland.json`, and appends to the file if no matching trailer is already present:

```text
Co-authored-by: <model-name> <model-email>
```

`model-name` is the name portion of `model_id` (text before the first space). `model-email` is the cleaned model-name plus `email_suffix`. All logic in Go; no shell tools required.

Immediately after the Co-authored-by trailer, `dreamland coauthor --trailer` appends a token-usage report line sourced from the current turn's telemetry snapshot (the same data `dreamland telemetry write` collects — model name and input/output/cached/total token counts):

```text
Tokens: input=<n> output=<n> cached=<n> total=<n>
```

If telemetry data is unavailable for the current turn (e.g. the platform doesn't expose usage stats, or no turn has completed yet), the `Tokens:` line is omitted and only the Co-authored-by trailer is appended — this is not a failure condition.

The hook file is written with mode 0755. If `.git/hooks/prepare-commit-msg` already contains `dreamland coauthor --trailer`, the file is left unchanged.

This hook script intentionally has no `command -v dreamland` guard: if `dreamland` is not resolvable when git invokes it, the shell wrapper fails with a nonzero exit and git aborts the commit. This fail-closed behavior is a deliberate decision (not an accident of the script's simplicity) — see the `otel-commit-hook` capability's "Hook appends AI telemetry as git trailers" requirement, which makes the parallel `commit-msg` hook fail the same way for the same condition, for consistency. The standard `git commit --no-verify` bypass remains available for a developer who genuinely needs to commit without `dreamland` installed.

#### Scenario: Agent git identity set from env var at session start

- **WHEN** `dreamland coauthor` runs and the platform env var for current agent is set (e.g., `CLAUDE_AGENT_ID=hypnos`)
- **THEN** `git config --local user.name` is set to `"hypnos"` and `git config --local user.email` to `"hypnos@github.com"` (with configured suffix)

#### Scenario: Agent git identity falls back to coding tool name on non-Claude-Code platforms

- **WHEN** `dreamland coauthor` runs with `.dreamland.json` coding tool `"GitHub Copilot"`, no platform agent env var is set, and no hook payload resolves an identity
- **THEN** `git config --local user.name` is set to `"GitHub Copilot"` and email to `"github-copilot@github.com"`

#### Scenario: Claude Code falls back to janus, not the coding tool name

- **WHEN** `dreamland coauthor` runs with `.dreamland.json` coding tool `"Claude Code"`, no platform agent env var is set, and no hook payload resolves an identity
- **THEN** `git config --local user.name` is set to `"janus"`, not `"Claude Code"`

#### Scenario: Email cleaning applied to agent name

- **WHEN** AgentName is `"Spec Writer"` and `email_suffix` is `@github.com`
- **THEN** AgentEmail is `"spec-writer@github.com"`

#### Scenario: prepare-commit-msg hook installed

- **WHEN** `dreamland coauthor` runs and `.git/hooks/prepare-commit-msg` does not exist
- **THEN** the file is created with mode 0755 containing `#!/bin/sh` and `dreamland coauthor --trailer "$1" "$2" "$3"`

#### Scenario: Claude Code identity resolved from tool_input.subagent_type

- **WHEN** `dreamland coauthor` runs via Claude Code's `PreToolUse` hook for a `Task` tool call whose payload is `{"tool_input": {"subagent_type": "phobetor", ...}}`
- **THEN** `git config --local user.name` is set to `"phobetor"`, not the generic coding-tool fallback

#### Scenario: Unrecognized resolved identity falls back to janus, not the raw value

- **WHEN** `dreamland coauthor` resolves a candidate AgentName that is not one of the ten registered dreamland agent names (e.g. a malformed or unexpected payload value)
- **THEN** AgentName falls back to `"janus"` rather than using the unrecognized value verbatim

#### Scenario: coauthor without --hook never touches stdin

- **WHEN** `dreamland coauthor` runs without the `--hook` flag (a human invoking it directly, in any kind of terminal)
- **THEN** no read of stdin is attempted, AgentName resolution proceeds directly to step 2, and the command cannot block regardless of what is or isn't attached to stdin

#### Scenario: coauthor --hook flag added to every platform's hook binding

- **WHEN** `dreamland init` completes for Claude Code, GitHub Copilot, Cursor, Codex, or Kiro
- **THEN** every default-mode `dreamland coauthor` entry in that platform's installed hook-binding file includes `--hook`
- **AND** `--trailer`-mode invocations (the installed `prepare-commit-msg` git hook) do not include `--hook`, since that code path never reads stdin for an agent-identity payload

#### Scenario: GitHub Copilot identity resolution is unaffected by the --hook gating change

- **WHEN** `dreamland coauthor --hook` runs via GitHub Copilot's `SubagentStart`/`SubagentStop` hook with a payload of `{"agent_type": "iktomi", ...}`
- **THEN** `git config --local user.name` is set to `"iktomi"`, exactly as it would resolve before this change — only when the payload is read changed, not how it is parsed

#### Scenario: coauthor skips silently outside a git repository

- **WHEN** `dreamland coauthor` runs in a directory with no git repository at or above it
- **THEN** the command exits 0 without error and does not attempt to write git config

#### Scenario: A genuine git config write failure blocks the lifecycle event

- **WHEN** `dreamland coauthor` resolves an AgentName/AgentEmail but the `git config --local user.name` or `git config --local user.email` write itself fails
- **THEN** the command exits with the blocking code (2), not the advisory code

### Requirement: test only runs when executable source files changed

`dreamland test` SHALL inspect `git status --porcelain` for unstaged and staged changes to source files matching the configured language's extensions. If matching files are present, it runs `test_command` from `.dreamland.json`, connecting the child process's stdout/stderr to `dreamland test`'s own so the underlying test output is visible (not discarded), and records the outcome as described below. If no matching files are found, it exits 0 silently and writes no result record.

Source file extensions by language:

| Language | Extensions |
| --- | --- |
| Go | `.go` |
| Node/TypeScript | `.ts`, `.tsx`, `.js`, `.jsx`, `.mts`, `.cts` |
| Rust | `.rs` |
| Python | `.py` |

When the configured `test_command` is actually run, `dreamland test` SHALL write `.dreamland/last-test-result.json` (creating the `.dreamland/` directory if absent) recording the outcome:

```json
{"status": "pass" | "fail", "head_sha": "<git rev-parse HEAD>", "written_at": "<RFC3339 timestamp>"}
```

This record exists so `dreamland commit` (see the modified "dreamland commit" requirement below) can refuse to checkpoint a commit over a test failure at the same `HEAD`, without depending on any particular ordering guarantee between separate hook-bound command invocations.

If `test_command` exits nonzero (the configured tests actually failed) or fails to execute at all (e.g. the command isn't found), `dreamland test` SHALL exit with the blocking code (2) — not the raw exit code the underlying test command produced, and not the advisory code (1) — so Claude Code recognizes a real test failure as blocking. `git`, config-loading, and "nothing to test" conditions remain non-blocking skips exiting 0, unchanged from prior behavior.

#### Scenario: Tests run after source code changes

- **WHEN** `dreamland test` runs and `git status --porcelain` shows at least one file with a matching source extension
- **THEN** `test_command` from `.dreamland.json` is executed with its stdout/stderr connected to `dreamland test`'s own, and `.dreamland/last-test-result.json` is written recording the outcome and the current `HEAD` sha

#### Scenario: Tests skipped when no source changed

- **WHEN** `dreamland test` runs and no source files have changes
- **THEN** the command exits 0 without running anything and without writing or modifying `.dreamland/last-test-result.json`

#### Scenario: Test failure blocks the lifecycle event

- **WHEN** `dreamland test` runs, source files changed, and `test_command` exits non-zero
- **THEN** `.dreamland/last-test-result.json` records `"status": "fail"` for the current `HEAD`, and `dreamland test` exits with the blocking code (2), regardless of the underlying test command's own specific exit code

#### Scenario: Test success is recorded

- **WHEN** `dreamland test` runs, source files changed, and `test_command` exits 0
- **THEN** `.dreamland/last-test-result.json` records `"status": "pass"` for the current `HEAD`, and `dreamland test` exits 0

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
4. Otherwise, stage all changes (`git add -A`) and run `git commit -m "chore: <reason> checkpoint (<agent-name>)"`, where `<agent-name>` is read from the currently configured `git config --local user.name` (the value `dreamland coauthor`'s identity-resolution logic, per the modified `coauthor` requirement above, most recently set) — not independently re-resolved from the current invocation's own hook payload or env vars, since `commit` may run at a different lifecycle event (e.g. `Stop`) than the `coauthor` invocation that last set identity (e.g. `PreToolUse`/`SubagentStop`), which would see a different payload shape and could resolve a different value. If `git config --local user.name` is unset (e.g. `coauthor` has genuinely never run in this repository), `<agent-name>` falls back to the same resolution logic `coauthor` uses.
5. Because this shells out to `git commit`, the already-installed `prepare-commit-msg` hook fires normally and appends the Co-authored-by trailer and token-usage report (see the modified `coauthor` requirement) to the commit message — `dreamland commit` does not duplicate that logic.
6. A failure in `git add -A` or `git commit` itself (distinct from the test-gating refusal in step 3) is a genuine, blocking failure — exit code 2 — since the entire purpose of this command is to guarantee a commit exists for every turn/handoff.

On any platform whose `Stop`-equivalent hook binding chains `dreamland test` immediately before `dreamland commit --reason turn-complete`, the scaffold installer SHALL bind `dreamland test-and-commit --reason turn-complete` (see the added `dreamland test-and-commit` requirement below) in place of that pair, instead of the two commands separately — this closes a race where the hook runner does not guarantee `test` finishes writing `.dreamland/last-test-result.json` before a separately-dispatched `commit` reads it. `dreamland commit --reason handoff` is bound to Claude Code's `SubagentStop` event unchanged (alongside `dreamland coauthor` and `dreamland telemetry write --tool claude-code`, per the requirement above) — no platform binds `test` under its `SubagentStop`-equivalent event, and the test-gating step in behavior item 3 above only applies to `commitReason == "turn-complete"`, so the race does not apply to handoff commits. On platforms without a `SubagentStop`-equivalent event, only the `Stop`-bound `test-and-commit --reason turn-complete` invocation applies; handoff commits on those platforms rely on the same agent-driven convention described in the requirement above for `coauthor`/`telemetry write`.

#### Scenario: Commit created when a turn completes with pending changes

- **WHEN** `dreamland commit --reason turn-complete` runs via the `Stop` hook, `git status --porcelain` shows pending changes, and either no `.dreamland/last-test-result.json` record exists or it records a `"pass"` for the current `HEAD`
- **THEN** the changes are staged and committed with subject `chore: turn-complete checkpoint (<agent-name>)`, where `<agent-name>` matches the current `git config --local user.name`

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

- **WHEN** `dreamland coauthor` most recently set `git config --local user.name` to `"phobetor"` (e.g. from a `PreToolUse` hook payload) and `dreamland commit --reason turn-complete` subsequently runs via the `Stop` hook, whose payload carries no sub-agent identity of its own
- **THEN** the resulting commit's subject reads `chore: turn-complete checkpoint (phobetor)`, matching the commit's actual git author — not a value independently re-resolved from the `Stop` hook's own payload

#### Scenario: Handoff commit created when Janus hands off to another agent

- **WHEN** a sub-agent's turn ends via `SubagentStop` and `git status --porcelain` shows pending changes
- **THEN** `dreamland commit --reason handoff` stages and commits those changes with subject `chore: handoff checkpoint (<outgoing-agent-name>)` before Janus regains control

#### Scenario: Claude Code settings.json binds test-and-commit to Stop and commit to SubagentStop

- **WHEN** `dreamland init` completes with "Claude Code" selected
- **THEN** `.claude/settings.json` contains `dreamland test-and-commit --reason turn-complete` under the `Stop` event key, and does not contain a separate `dreamland test` entry followed by `dreamland commit --reason turn-complete`
- **AND** contains `dreamland commit --reason handoff` under the `SubagentStop` event key, unchanged

#### Scenario: commit skips silently outside a git repository

- **WHEN** `dreamland commit` runs in a directory with no git repository at or above it
- **THEN** the command exits 0 without error and does not attempt `git status`, `git add`, or `git commit`

#### Scenario: A genuine git failure during commit blocks the lifecycle event

- **WHEN** `dreamland commit` has pending changes to commit (and is not refused by the test-gating step) but `git add -A` or `git commit` itself fails
- **THEN** the command exits with the blocking code (2), not the advisory code

## ADDED Requirements

### Requirement: dreamland test-and-commit runs test then commit in one process to remove the Stop-hook race

`dreamland` SHALL expose `dreamland test-and-commit --reason <turn-complete|handoff>`, which runs the same logic as `dreamland test` followed by the same logic as `dreamland commit --reason <reason>`, within a single process and a single invocation. If the test step returns an error, that error SHALL be written to stderr and execution SHALL still proceed to the commit step (which, per the modified `commit` requirement above, will itself observe the just-written `.dreamland/last-test-result.json` and refuse to commit when appropriate) — `test-and-commit`'s own exit code and final error SHALL be those of the commit step.

This exists because a `Stop`-equivalent hook binding that runs `dreamland test` and `dreamland commit --reason turn-complete` as two separately hook-bound commands does not have a documented guarantee that the first finishes writing `.dreamland/last-test-result.json` before the second reads it — confirmed empirically, not merely theoretically, when this exact pair produced a false "no result recorded" refusal on an ordinary turn where `test` had in fact run and was about to record a failure. Running both steps as one process removes the race structurally: there is no second process for a hook runner to schedule concurrently against.

#### Scenario: A failing test step still leads to commit's own refusal, not a separate ambiguous failure

- **WHEN** `dreamland test-and-commit --reason turn-complete` runs, source files changed, and the configured `test_command` fails
- **THEN** the test failure is written to `.dreamland/last-test-result.json` and to stderr, and the subsequent commit step observes that record for the current `HEAD` and refuses to commit, exactly as `dreamland commit --reason turn-complete` would given that same record
- **AND** `dreamland test-and-commit` exits with the blocking code (2)

#### Scenario: A passing test step allows the commit step to proceed

- **WHEN** `dreamland test-and-commit --reason turn-complete` runs, source files changed, and the configured `test_command` succeeds
- **THEN** `.dreamland/last-test-result.json` records `"status": "pass"` for the current `HEAD`, and the commit step stages and commits pending changes exactly as `dreamland commit --reason turn-complete` would given that record

### Requirement: Hook-invoked commands distinguish blocking failures from advisory skips via exit code

`dreamland` SHALL exit with code 2 — the only code Claude Code's hook lifecycle treats as blocking a `PreToolUse`/`Stop`/`SubagentStop` transition — when `coauthor`, `commit`, or `test` (the three commands the audit identified as having a genuine failure mode with real consequences) encounter a genuine, unrecoverable failure, as detailed in each command's own requirement above. Every other error from any other command, and every already-existing skip condition on these three commands, SHALL continue to exit 1 (advisory, unchanged default) or 0 (silent skip) exactly as before this change — this requirement narrows to the three named commands, not a blanket change to every command's error handling.

This is implemented as a single dispatch point in `Execute()` (the CLI's top-level error handler): a command's `RunE` marks a genuine failure by wrapping it before returning, and `Execute()` checks for that marker once, mapping it to exit code 2; an unmarked error keeps exiting 1 exactly as every command's errors do today.

#### Scenario: A blocking failure exits with code 2

- **WHEN** `dreamland coauthor`, `dreamland commit`, `dreamland test` (on an actual test-command failure), or `dreamland test-and-commit` (which inherits its exit code from the commit step, per its own requirement) encounters the genuine failure condition described in its own requirement
- **THEN** the process exits with code 2

#### Scenario: A skip condition still exits 0

- **WHEN** any of `coauthor`, `commit`, or `test` encounters one of its documented skip conditions (no git repository, clean working tree, nothing configured, no matching source files changed)
- **THEN** the process exits 0, unchanged from prior behavior

#### Scenario: Commands outside this change's scope are unaffected

- **WHEN** `dreamland version-bump`, `dreamland transition-log`, or `dreamland telemetry write` returns any error
- **THEN** the process exits with code 1, exactly as it did before this change

### Requirement: dreamland detects and reports when its own binary is stale relative to its build source

`dreamland` SHALL expose a `dreamland version` subcommand that prints the git commit SHA the running binary was built from, sourced from a `buildCommit` value embeddable at build time via `-ldflags "-X dreamland/cmd.buildCommit=<sha>"`. When built without this flag (e.g. a plain `go build -o dreamland .`), `buildCommit` SHALL default to `"unknown"`.

On every invocation, before dispatching to the requested subcommand, `dreamland` SHALL check whether the current working directory is inside dreamland's own source repository (a `go.mod` at the repository root declaring `module dreamland`) and, if so, whether `buildCommit` (when not `"unknown"`) matches that repository's current `git rev-parse HEAD`. On a mismatch, `dreamland` SHALL print a single diagnostic line to stderr naming both SHAs and directing the operator to rebuild and reinstall, and SHALL NOT alter the invoked command's exit code or behavior in any other way — this check is advisory, intended to surface exactly the kind of silent binary/source drift that let a prior identity-resolution bug persist unnoticed, not a new enforcement gate in its own right.

This check SHALL NOT run, and SHALL NOT print anything, when invoked from any repository other than dreamland's own source tree — comparing an embedded dreamland build SHA against an unrelated consumer project's `HEAD` would be meaningless.

#### Scenario: dreamland version reports the embedded build SHA

- **WHEN** `dreamland` was built with `-ldflags "-X dreamland/cmd.buildCommit=<sha>"` and `dreamland version` is run
- **THEN** the output includes `<sha>`

#### Scenario: dreamland version reports unknown when built without ldflags

- **WHEN** `dreamland` was built with a plain `go build -o dreamland .` (no `-ldflags`) and `dreamland version` is run
- **THEN** the output includes `"unknown"` in place of a build SHA

#### Scenario: Stale binary warning fires inside dreamland's own source repository

- **WHEN** any `dreamland` command is invoked with a working directory inside a git repository whose root `go.mod` declares `module dreamland`, the binary's embedded `buildCommit` is not `"unknown"`, and it does not match that repository's current `git rev-parse HEAD`
- **THEN** a diagnostic naming both SHAs is printed to stderr, and the invoked command's own exit code and behavior are otherwise unaffected

#### Scenario: No warning outside dreamland's own source repository

- **WHEN** any `dreamland` command is invoked with a working directory inside a consumer project (a repository whose `go.mod`, if any, does not declare `module dreamland`)
- **THEN** no build-freshness diagnostic is printed, regardless of the embedded `buildCommit` value
