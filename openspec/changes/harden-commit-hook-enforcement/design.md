## Context

This change follows a hooks audit of `claude-code-self-hosting`'s output (26/28 tasks done; 6.4 and 7.3 blocked on live sub-agent-dispatch verification). The audit's finding was not that identity resolution is wrong — it's now correct — but that nothing in the hook chain actually *enforces* the invariants dreamland's docs and this repo's own `.claude/settings.json` imply: that every turn produces a commit with the right author, that a failing test suite blocks that commit, and that a stale local binary can't silently reintroduce the exact bug `claude-code-self-hosting` just fixed. Six concrete findings (file:line-grounded) plus one structural root cause drive this change; each is addressed by a specific, scoped fix below rather than a general rewrite of the hook architecture.

Relevant current-state facts, confirmed by reading source directly (not assumed from the audit alone):

- `cmd/coauthor.go`'s `hookPayloadReadTimeout` is `200 * time.Millisecond`, a `const`, shared indirectly by `cmd/version_bump.go`'s `--if-agent` path (which calls the same `agentNameFromHookPayload()`) and directly mirrored by `version_bump.go`'s own `bashHookStdinReadTimeout` (also 200ms, for the `--change-from-command` payload read).
- `cmd/coauthor.go:112-121`'s existing doc comment on `hookPayloadReadTimeout` records that stdin-type detection (`os.ModeCharDevice`) was already tried as a way to skip the read for an interactive invocation, and rejected: "VS Code's integrated terminal (and other pty-backed shells) does not reliably present stdin the way a plain interactive terminal does, so a bare `Stat()` check let `dreamland coauthor` hang indefinitely when run directly in that terminal." Any redesign of this read path must not reintroduce a stdin-type/`ModeCharDevice` heuristic — it is proven unreliable in this exact codebase's history, not merely a theoretical concern.
- `dreamland coauthor` (default mode) and `dreamland version-bump`'s `--if-agent`/`--change-from-command` modes are invoked, unmodified, from hook-binding templates for every supported platform, not just Claude Code: `internal/scaffold/templates/hooks/bindings/{claude-code,github-copilot,cursor,codex,kiro}/*` (Antigravity binds `version-bump` but not `coauthor` — no `PreToolUse`/`SubagentStart`-equivalent event exists for it, per `openspec/specs/dev-workflow-hooks/spec.md`). GitHub Copilot's binding (`github-copilot/hooks.json`) calls `dreamland coauthor` under `SessionStart`, `SubagentStart`, and `SubagentStop`; its hook payload shape for sub-agent identity is a top-level `agent_type` field (confirmed from live payloads, per `cmd/coauthor.go:125-126`), parsed by the same `agentidentity.FromPayload` Claude Code's `tool_input.subagent_type` shape uses. Any change to how/when stdin is read must preserve this cross-platform behavior, not just Claude Code's.
- `cmd/root.go`'s `Execute()` is four lines: any error from `rootCmd.Execute()` becomes `os.Exit(1)`, unconditionally, for every command.
- `.claude/settings.json`'s `Stop` event runs, as siblings in one `hooks` array, in this order: `version-bump --patch`, `transition-log`, `test`, `telemetry write --tool claude-code`, `commit --reason turn-complete`. Claude Code's documented hook semantics describe exit code 2 as blocking *the lifecycle transition* (e.g., forcing the agent to keep working instead of stopping) and feeding stderr back to the model — they do not document whether a exit-2 result from one command in a `hooks` array skips the array's remaining sibling commands. This change does not rely on that undocumented behavior.
- `cmd/commit.go`'s `runCommit` computes its commit-message subject via `resolveAgentName(cfg.CodingTool)` — the same low-level helper `coauthor` calls, but without `coauthor`'s registered-agent check or Claude-Code-fallback correction (`cmd/coauthor.go:64-74`).
- `internal/scaffold/templates/hooks/commit-msg` (installed by `dreamland telemetry install` / `internal/scaffold/hook.go`) guards its whole body with `if command -v dreamland >/dev/null 2>&1; then ... fi` and always `exit 0` — this is the literal, specified behavior in `openspec/specs/otel-commit-hook/spec.md`'s "Hook appends AI telemetry as git trailers" requirement ("the hook SHALL exit with code 0 regardless of whether telemetry data is available... dreamland binary not found — commit proceeds normally"). `cmd/coauthor.go`'s `installPrepareCommitMsgHook` writes `.git/hooks/prepare-commit-msg` with no such guard, so a missing `dreamland` binary makes that hook's shell wrapper fail with "command not found" and git aborts the commit — fail-closed, but *by accident*, not by any documented decision.
- Nothing in the repo (`go.mod`, `README.md`, `.github/workflows/ci.yml`, `internal/scaffold`) embeds a build-time git SHA into the `dreamland` binary, and `README.md`'s only documented build command is `go build -o dreamland .` with no install/freshness step.

## Goals / Non-Goals

**Goals:**
- Every genuine failure in `coauthor`, `commit`, or a real test-command failure in `test` becomes a Claude-Code-blocking exit code (2), while every already-legitimate skip condition (no git repo, clean tree, nothing configured, no matching source changes) keeps exiting 0 exactly as today.
- A failing test run cannot be silently checkpointed by `dreamland commit`, verified through a mechanism that does not depend on undocumented Claude Code inter-hook-command sequencing behavior.
- The commit author (`git config user.name`, set by `coauthor`) and the commit subject's agent name (set by `commit`) can never diverge, including across the different Claude Code hook events (`Stop` vs `SubagentStop`) that trigger each independently.
- The two installed git hooks (`commit-msg`, `prepare-commit-msg`) behave identically — both fail loud — for the same "`dreamland` unresolvable" condition, and that behavior is a documented decision, not an accident.
- `claude-code-self-hosting` tasks 6.4/7.3 close via the real sub-agent dispatches this change's own implementation naturally performs, not a hand-crafted verification step.
- A stale local `dreamland` binary (source changed, binary not rebuilt/reinstalled) becomes observable, closing the structural gap that let the original identity-resolution bug persist silently.

**Non-Goals:**
- Redesigning the Stop/SubagentStop hook chain's ordering or which commands are bound to which lifecycle events — this change only changes what individual commands *do* (exit codes, gating, resolution), not `.claude/settings.json`'s hook wiring itself (except where a new command, `dreamland version`, needs no binding at all — it's operator/debug-invoked).
- Making *every* dreamland command's every error path blocking. Only the three commands and conditions the audit named (`coauthor`, `commit`, `test`-on-real-failure) get the exit-2 treatment; `version-bump`, `transition-log`, and `telemetry write`'s existing exit-code behavior is unchanged and out of scope.
- Building a general CI/install pipeline (Makefile, release automation) around the freshness check — the fix here is a runtime warning plus a documented build command, not new build tooling.
- Solving `dreamland test`'s pre-existing, separate gap of discarding the underlying test command's stdout/stderr (`cmd/test.go`'s `c.Stdout = nil; c.Stderr = nil`) as an unscoped fix — see Decision 3, where it's addressed as a small adjacent change because it directly affects whether the new blocking failure is diagnosable, not treated as a general test-command UX overhaul.

## Decisions

**1. Hook-payload reads: remove the timeout entirely — an explicit, dreamland-controlled `--hook` flag instead of any stdin-type heuristic.**

A fixed deadline can never be fully correct in either direction: too short reproduces the exact false negative the audit flagged (a legitimate payload arriving a moment late gets silently treated as absent); too long just delays the same false negative without removing it. Raising `hookPayloadReadTimeout` from 200ms to 500ms (this change's first draft) was still a guess, not a fix. A second draft proposed telling "no payload coming" apart from "payload inbound, not fully arrived yet" via `os.Stdin.Stat().Mode()&os.ModeCharDevice` — rejected on discovering `cmd/coauthor.go:112-121`'s own doc comment: this exact heuristic was already tried and reverted, because pty-backed shells (VS Code's integrated terminal named explicitly) don't reliably present stdin as a character device, so the check let `coauthor` hang indefinitely in exactly the scenario it was meant to catch. Any stdin-type-based detection is disqualified by this codebase's own history, not just theoretically risky.

The correct signal is not a property of stdin at all — it's a property of *who is calling*. `dreamland coauthor` (default mode) and `dreamland version-bump`'s `--change-from-command`/`--if-agent` modes are invoked from exactly two kinds of callers: dreamland's own hook-binding templates (a real payload is always either coming promptly or legitimately not applicable to this event) and a human running the command directly (no payload is ever coming, regardless of what kind of terminal or shell they're in). Only dreamland controls the hook-binding command lines, so dreamland can make that distinction explicit instead of inferring it: a new `--hook` bool flag on `coauthorCmd`'s default mode, set only by hook-binding templates, never by a human invoking the command manually. `--change-from-command` and `--if-agent <name>` already serve this exact role for `version-bump` today — their mere presence is only ever set from a hook-binding template, so no new flag is needed there, just the timeout removal.

Flag absent: stdin is never touched at all — not read, not `Stat()`-checked, nothing — so a manual `dreamland coauthor` run in any terminal (pty-backed or not) returns instantly with no possibility of hanging, categorically, not probabilistically. Flag present: `agentNameFromHookPayloadFrom(r io.Reader)` (`changeSlugFromBashHookStdin` unchanged in shape) does a plain synchronous `io.ReadAll` to EOF with no deadline — correct because every hook-binding template's caller (Claude Code, GitHub Copilot, Cursor, Codex, Kiro) writes its payload and closes its end of the pipe promptly, the same assumption the original 200ms timeout already relied on for its "legitimate case," just no longer paired with an arbitrary cutoff for the case where that assumption is momentarily slow rather than false.

The stderr diagnostic this change originally planned for a timeout branch is dropped along with the timeout — there is no longer a race to distinguish from a legitimate empty payload; a `--hook`-flagged read that comes back empty (e.g. a `Stop` payload, which never carries `tool_input.subagent_type` by design) is unambiguous and needs no diagnostic.

Every hook-binding template invoking `dreamland coauthor` in default mode must add `--hook`: `internal/scaffold/templates/hooks/bindings/{claude-code,github-copilot,cursor,codex,kiro}/*` and this repo's own already-installed `.claude/settings.json` (Antigravity binds `version-bump` but never `coauthor` — no `PreToolUse`/`SubagentStart`-equivalent event exists for it, so it's untouched by this decision). GitHub Copilot's `SessionStart`/`SubagentStart`/`SubagentStop` bindings all get `--hook`; its top-level `agent_type` payload shape is parsed by the same `agentidentity.FromPayload` call as before — this decision changes only *when* stdin is read, not how the payload is parsed once read, so cross-platform identity resolution (Claude Code's `tool_input.subagent_type`, Copilot's `agent_type`) is unaffected. `--trailer` mode (the installed `prepare-commit-msg` git hook) is a separate code path reading commit-message-file positional args, not stdin — untouched by this decision entirely.

Applies to both duplicated read implementations — `cmd/coauthor.go`'s `agentNameFromHookPayload` and `cmd/version_bump.go`'s `changeSlugFromBashHookStdin` (currently forked, near-identical goroutine/`select`/`time.After` implementations sharing the same 200ms-constant pattern) — `hookPayloadReadTimeout` and `bashHookStdinReadTimeout` are deleted outright along with that machinery in both, not adjusted. A test asserting every binding template's `dreamland coauthor` line includes `--hook` (extending `internal/scaffold/scaffold_test.go`'s existing content-assertion pattern) guards against a future template edit silently dropping the flag and regressing that platform back to "never reads its hook payload."

**2. Blocking vs advisory: a `BlockingError` marker type, `cmd/hookexit.go`, checked once in `Execute()`.**

```go
// cmd/hookexit.go
package cmd

// blockingError marks an error as one that must cause dreamland to exit with
// code 2 — the only exit code Claude Code's hook lifecycle treats as blocking
// (see PreToolUse/Stop/SubagentStop hook semantics). Every other error keeps
// exiting 1, unchanged, so commands not named in this change (version-bump,
// transition-log, telemetry write) are not affected.
type blockingError struct{ err error }

func Blocking(err error) error {
	if err == nil {
		return nil
	}
	return &blockingError{err: err}
}

func (e *blockingError) Error() string { return e.err.Error() }
func (e *blockingError) Unwrap() error { return e.err }

func IsBlocking(err error) bool {
	var be *blockingError
	return errors.As(err, &be)
}
```

`cmd/root.go`'s `Execute()` becomes:

```go
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		if IsBlocking(err) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}
```

`coauthor`, `commit`, and `test` wrap their genuine-failure return paths in `Blocking(...)` (see per-command detail in the tasks). Skip conditions (`config.ErrNoGitRepo`, clean tree, no test command configured, no matching source files) continue to `return nil` exactly as today — unchanged, per the audit's explicit instruction that these must keep exiting 0. Alternative considered: make *every* non-nil error from any command blocking — rejected as far broader than the audit's scope and riskier (e.g. a `dreamland telemetry write` hiccup blocking every turn would be a significant regression in disruptiveness for a command whose data is already best-effort per its own spec).

**3. `no git repository` becomes an explicit, named skip — not a generic propagated error.**

`internal/config.FindRepoRoot` currently returns `errors.New("no git repository found in any parent directory")` as a plain error; `config.Load` propagates it as-is. `cmd/test.go`'s `runTest` already treats "git unavailable" as a skip (via its own `runCmd("git", "status", ...)` error check), but `cmd/coauthor.go`'s `runCoauthor` and `cmd/commit.go`'s `runCommit` do not — today, running either outside a git repo returns a generic error, which (once Decision 2 lands) would otherwise become a *blocking* exit 2 for a condition that is not a real enforcement failure, just an environment where the hook doesn't apply. `internal/config` gains a sentinel, `var ErrNoGitRepo = errors.New("no git repository found in any parent directory")`, returned by `FindRepoRoot` in place of the ad hoc `errors.New(...)`. `runCoauthor` and `runCommit` both check `errors.Is(err, config.ErrNoGitRepo)` immediately after `config.Load` and `return nil` (skip) in that case, `Blocking(err)` for any other `config.Load` failure (e.g. malformed `.dreamland.json`, which *is* a genuine failure worth blocking on).

**4. `dreamland test` writes an explicit failure marker; `dreamland commit` gates on it — not on undocumented exit-2 sibling-skipping.**

Rather than relying on Claude Code's `Stop` hook array halting `commit` because `test` exited 2 earlier in the same array (undocumented, and the two run as separate OS processes regardless), `dreamland test` writes `.dreamland/last-test-result.json` — `{"status": "pass"|"fail", "head_sha": "<git rev-parse HEAD>", "written_at": "<RFC3339>"}` — every time it actually runs the configured test command (not on any of its existing skip branches). `dreamland commit` reads that file before staging/committing: if `status == "fail"` and `head_sha` matches the repository's current `HEAD`, `commit` refuses to create the commit (`Blocking(...)`, changes remain staged/unstaged for the next successful run to pick up) instead of checkpointing over a known failure. A `head_sha` mismatch (or missing file) means the record doesn't describe the current tree — treated as "no valid signal," and `commit` proceeds normally; this is the one deliberate fail-open branch in this design, justified because staleness here is genuinely ambiguous (we cannot tell whether tests would still fail on the current tree) and the two commands run sequentially within the same turn with no intervening commits in the current hook wiring, so a mismatch should be rare and is itself observable in `.dreamland/last-test-result.json` if debugged. Alternative considered (from the audit): rely on exit-2 short-circuiting once Decision 2 lands — rejected as the primary mechanism because it's coupled to unverified Claude Code hook-array orchestration semantics; Decision 2's exit-2 signal is kept anyway (it still tells Claude Code the turn isn't really "done" when `test` itself fails), but it is not what actually prevents the commit — the state file is.

As a small, directly adjacent fix (not a separate scope expansion): `cmd/test.go`'s `runTest` currently sets `c.Stdout = nil; c.Stderr = nil`, which discards the configured test command's real output entirely. Once a test failure becomes a blocking condition that stops commits, an operator needs to see *why* it failed — `c.Stdout`/`c.Stderr` are changed to `os.Stdout`/`os.Stderr` so the underlying test command's output reaches wherever `dreamland test`'s own output goes (the Claude Code hook transcript). This is bundled here because it's a one-line change to code this decision already touches and materially affects whether the new blocking behavior is debuggable; it is not a general redesign of `test`'s UX.

**5. `commit`'s subject-line agent name is read from the persisted git identity, not re-resolved independently.**

The audit's ask was "share one resolver." A literal shared-function-call implementation would still be wrong: `coauthor` and `commit` run as separate OS processes, bound to different hook events (`coauthor` on `SessionStart`/`PreToolUse`/`SubagentStop`; `commit --reason turn-complete` on `Stop`, which carries no `tool_input.subagent_type` in its payload — only `PreToolUse` does). Independently re-deriving identity from each command's *own* hook payload at `commit`'s point in the chain would resolve differently than what `coauthor` most recently wrote to git config, because the two run at different lifecycle events with different payload shapes. Instead: `runCoauthor`'s existing inline resolution logic (`cmd/coauthor.go:63-74`) is extracted into `resolveEnforcedAgentName(cfg *config.Config) string`, used by `coauthor` exactly as before (it still needs a *fresh* resolution — that's what it's about to write into git config). A new `currentGitIdentityName(cfg *config.Config) string` reads `git config --local --get user.name` (the value `coauthor` already persisted) and only falls back to `resolveEnforcedAgentName(cfg)` if that's unset/empty (e.g. `coauthor` genuinely hasn't run yet in this repo). `runCommit` calls `currentGitIdentityName`, not `resolveAgentName` or `resolveEnforcedAgentName` directly. This guarantees the commit subject and the actual git-recorded author (which `git commit` reads from the same `user.name` config value) can never diverge, regardless of which hook event triggered `commit`.

**6. `commit-msg` hook becomes fail-closed for "`dreamland` not on PATH"; `prepare-commit-msg`'s existing fail-closed behavior is documented as intentional, not changed.**

`internal/scaffold/templates/hooks/commit-msg` changes from:

```sh
if command -v dreamland >/dev/null 2>&1; then
  TRAILERS="$(dreamland telemetry snapshot --format trailers 2>/dev/null)"
  ...
fi
exit 0
```

to guard-and-fail:

```sh
if ! command -v dreamland >/dev/null 2>&1; then
  echo "dreamland: not found on PATH — commit blocked (bypass with 'git commit --no-verify')" >&2
  exit 1
fi
TRAILERS="$(dreamland telemetry snapshot --format trailers 2>/dev/null)"
...
exit 0
```

"No telemetry data available" (the `TRAILERS` variable ending up empty) remains a silent, non-blocking case — that condition is not an enforcement failure, it's a legitimate "nothing to add" outcome (e.g. a manual `git commit` run outside any dreamland hook context), and the existing spec scenario for it is unchanged. Git's standard `--no-verify` bypass is the documented escape hatch for a developer who needs to commit without `dreamland` installed (e.g. debugging git internals) — no new custom environment variable is introduced for this, since one already exists and is exactly this feature's intended purpose. `prepare-commit-msg` needs no functional change (it already fails this way, via plain shell "command not found" propagating a nonzero exit) — this change only makes that behavior an explicit, spec'd decision instead of an accident, so a future edit doesn't "fix" it into silently degrading to match `commit-msg`'s old behavior.

**7. Binary freshness: embed build SHA via `-ldflags`, warn (not block) on mismatch, scoped to dreamland's own source repo.**

A new `dreamland version` command (`cmd/version.go`) prints a `buildCommit` package variable, default `"unknown"` (plain `go build` with no `-ldflags`), settable via `-ldflags "-X dreamland/cmd.buildCommit=$(git rev-parse HEAD)"`. `README.md`'s build instructions are updated to use this form. A freshness check runs from `rootCmd`'s existing `PersistentPreRunE` (`loadConfig`, `cmd/root.go`) on every invocation: it only does anything when the current working directory's repository is dreamland's *own* source tree (detected via a `go.mod` containing `module dreamland` at the repo root — this repo, or a fork/local clone someone else is developing dreamland itself from) *and* `buildCommit != "unknown"`; in that case, if `buildCommit` doesn't match `git rev-parse HEAD` in that repository, it prints one loud stderr line (`dreamland: binary build SHA <x> does not match source HEAD <y> — rebuild and reinstall before trusting hook output`) and does *not* affect the command's exit code. Scoping to "is this dreamland's own source repo" is required for correctness, not just to reduce noise: comparing the embedded SHA against an arbitrary *consumer* repository's `HEAD` would be meaningless (a downstream project's own commits have nothing to do with when dreamland itself was last built) and would misfire constantly. Making the warning non-blocking is a deliberate trade-off (see Risks) — the six fixes above are what actually enforce identity/commit behavior; this is a developer-facing signal to prevent the *next* silent regression of them, not itself a new hard gate. As part of this change's own implementation, this repo's `~/.local/bin/dreamland` and repo-root `./dreamland` are rebuilt with the new `-ldflags` command so the repo isn't left freshly stale the moment this change merges.

## Risks / Trade-offs

- [Removing the timeout means a `--hook`-flagged invocation whose caller opens the stdin pipe but never writes to or closes it would now block indefinitely, instead of degrading after a fixed wait] → Accepted, and now strictly narrower than before: the flag is only ever set by dreamland's own hook-binding templates, so this can only happen if a real hook runner (Claude Code, GitHub Copilot, Cursor, Codex, Kiro) itself fails to write and close promptly — never from a human's terminal session, which categorically never passes `--hook` regardless of shell or pty implementation. This is a strictly smaller risk surface than the rejected character-device-check approach, which could misfire depending on how a given shell presents stdin.
- [A future hook-binding template edit could add a new `dreamland coauthor` invocation, or copy an existing one, without the `--hook` flag, silently regressing that platform back to never reading its hook payload — no hang, just a silent fallback to generic identity] → Mitigated, not eliminated, by the content-assertion test added in task 3.6 for every current binding; a genuinely new binding added later still depends on whoever writes it remembering this convention.
- [The `.dreamland/last-test-result.json` staleness fallback (mismatched `head_sha` → don't block) is a real, if narrow, fail-open path] → Explicitly scoped to only the case where the record provably doesn't describe the current tree; the record is inspectable on disk for debugging, and the two commands run within the same turn's hook chain today with no intervening commit between them, so a mismatch should not occur under the current, unmodified hook wiring.
- [Making `commit-msg`'s missing-binary case fail-closed means any repo where `dreamland` briefly isn't on `PATH` (e.g. a shell profile issue, a fresh clone before running the install step) now blocks *every* commit, not just dreamland-driven ones] → Accepted per the explicit governing principle (fail loud over fail open where unclear); `git commit --no-verify` remains the standard, already-existing escape hatch, requiring no new mechanism.
- [The binary-freshness warning is advisory only, so a developer can still ignore it and keep running a stale binary indefinitely] → Accepted as an intentionally scoped fix (see Non-Goals); making staleness itself a hard block risks bricking the exact inner dev loop this change's own implementation runs inside, and the primary enforcement is the six behavioral fixes, not this signal.
- [Closing `claude-code-self-hosting` tasks 6.4/7.3 depends on this change's own implementation actually going through a real Task-tool delegation, not a guaranteed property of how any given session executes it] → If, for some reason, this change's implementation is carried out without any real sub-agent dispatch (e.g. a single agent does all the work itself with no delegation), 6.4/7.3 remain open; this change's own tasks make the verification step explicit and conditioned on a real delegation having occurred, rather than assuming it.

## Open Questions

- Should the `.dreamland/last-test-result.json` staleness window be tightened further (e.g. also compare a monotonic session/turn identifier, not just `head_sha`) if, in practice, the mismatch fail-open path is ever observed to fire under the current hook wiring?
- Should the binary-freshness check eventually escalate from advisory to blocking for `coauthor`/`commit` specifically (the two commands this change hardens) once there's evidence developers reliably ignore the warning — or is the warning sufficient given it only fires inside dreamland's own source tree, where the audience is already dreamland contributors who can act on it?
