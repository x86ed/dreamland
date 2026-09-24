## Why

The workflow's fixed hand-offs are written into agent instructions ("hand off directly to `phobetor`"), but the agents that carry them cannot execute them. `morpheus` and `iktomi` have `tools: Read, Edit, Write, Bash`; `phobetor` has `Read, Bash`; none has the `Agent` tool. A "hand off" is therefore only a sentence in the agent's final report. The dispatching session (a plain main session, or Janus) must read that sentence and make the next `Agent` call itself. Nothing makes it, and in practice it sometimes does not: the run ends after `morpheus`, the validation step never happens, and nobody is told.

The user's decision (recorded, not re-litigated here): do not grant `Agent` to these agents. Add a deterministic next-step mechanism that makes the dispatcher's next call mandatory and hard to skip. The required edges are fixed by the user:

- `morpheus` completes -> `phobetor`.
- `iktomi` completes -> `phobetor`, unconditionally (this supersedes the file-changed-only branch still in the live `.claude/agents/iktomi.md`).
- `phobetor` pass -> `baku`.
- `phobetor` fail -> `morpheus`, unless `morpheus` already failed once on a retry for that change, then -> `phantasos`.

Facts checked in the repository while drafting:

- `settings-patch.json` (Claude Code) already binds a `SubagentStop` hook (telemetry, version-bump, `commit --reason handoff --hook`) and `PreToolUse` `Task|Agent` (`coauthor --hook`). None of them knows what the next agent should be.
- `nyx` -> `morpheus` is also a fixed edge in the `janus-router-agent` spec, and is exposed to the same skip. It is included in the table (Decision 1) because leaving it out would make the table incomplete by construction; the user's required list did not name it, which is a decision to confirm.
- `deterministic-routing-and-janus-guard` (open, 0/67) adds `dreamland route`, `dreamland guard-router`, `internal/sessionidentity` and the per-user state root. None of that code exists yet in `cmd/` or `internal/`; this change reuses its state-root convention and does not depend on its guard.
- `harden-commit-hook-enforcement` is archived (2026-09-20). Its `Blocking`/exit-2 mechanism (`cmd/hookexit.go`) is in the tree and is reused.
- Hook-invoked `dreamland` commands other than `coauthor`/`commit`/`test` exit 1 on error, and Claude Code treats only exit 2 as blocking. A `dreamland` binary older than the settings file therefore turns an unknown subcommand into a silent no-op (the stale-binary problem seen in earlier runs where `dreamland test` was skipped). Decision 8 addresses it.

## What Changes

- **New `dreamland handoff` command family** (`cmd/handoff.go`, `internal/handoff/`): a single edge table (`internal/handoff/edges.go`) and a pure `Next(from, tag, counter) -> Directive` function, plus hook modes (`record`, `inject`, `enforce`, `stop-check`, `prompt`):
  - `handoff record --hook` (`SubagentStop`): parses the finished subagent's report for its machine-readable tags, computes the next target, updates the per-change failure counter, and writes a pending directive.
  - `handoff inject --hook` (`PostToolUse`, matcher `Task|Agent`): emits the directive as `additionalContext` into the dispatcher's context the moment the subagent's result returns: "Your next call MUST be `Agent(subagent_type=phobetor)`".
  - `handoff enforce --hook` (`PreToolUse`, matcher `Task|Agent`): while a directive is pending, an `Agent` call to any other target is blocked (exit 2) with a message naming the required target; the matching call clears the directive.
  - `handoff stop-check --hook` (`Stop`): the session cannot end while a directive is unsatisfied; it is blocked with the required next call as the reason, up to a bounded number of times.
  - `handoff next` (CLI, no hook): prints the directive for `--from`/`--verdict`/`--change`; used by platforms without hooks and by tests.
- **Live-session corrections (added after an interactive test).** A `handoff prompt --hook` on `UserPromptSubmit` replaces the naive `release`: it releases only on a real human prompt and injects on the harness's background-completion notification (which arrives as a `UserPromptSubmit`); `inject` no longer depends on `PostToolUse` ordering, so background (asynchronous) and foreground dispatch both work; each mode is its own hook entry; the dispatcher is identified by absence of `agent_id`; `stop-check` ignores `stop_hook_active`; `phobetor` `pass` goes to `baku` only when the change's `tasks.md` is fully ticked, otherwise a partial-pass report; `phantasos`/`baku` reports carry `[change: <slug>]` and counter files record the writing session; `dreamland init` refreshes marker-carrying agent files when templates change. The stale-binary SHA check is deliberately left to a separate change (design Decision 13).
- **Machine-readable report tags.** `morpheus` and `iktomi` end their report with `[handoff: complete]` or `[handoff: blocked]`. `phobetor` ends with `[verdict: pass]`, `[verdict: fail]`, or `[verdict: spec-defect]`, plus `[change: <slug>]` when it knows it. Nothing in the mechanism judges prose.
- **Per-change failure counter** in the per-user state directory (`<root>/handoff/<repo-id>/<change>.json`), locked and atomically written, so it survives across subagent turns and parallel sessions. First `phobetor` failure for a change -> `morpheus`; a failure after that retry -> `phantasos`. Reset on pass, and when `phantasos` completes a re-spec turn.
- **`iktomi-always-handoff-phobetor` is folded in**, not duplicated: this change carries all six platform iktomi template edits (superseding that change's unchecked tasks), the same unconditional rule, and the same "blocked reports a blocker to Janus" rule, now with a tag the hook reads. See design.md, Decision 9.
- **Agent template edits** (owned by `hypnos`): `morpheus`, `iktomi`, `phobetor` (and one line in `nyx`) on all six platforms gain the report-tag instruction; `phobetor`'s hand-off text changes from "fail -> morpheus" to the counter rule, with the hook, not the agent, deciding.
- **Drift test**: the edge table versus every platform's agent templates (parsed with the same `hand off directly to` pattern as `internal/workflowgraph/import.go`), so the table and the prose cannot disagree.
- **Claude Code first.** Copilot has a `SubagentStop` hook but its injection semantics are unverified, so it gets record and `handoff next` only in this change. Cursor, Codex, Kiro, and Antigravity get the tags and one instruction line telling the dispatcher to run `dreamland handoff next`, with no enforcement (Decision 7).

## Capabilities

### New Capabilities

- `deterministic-handoffs`: the edge table, report tags, failure counter, hook modes, enforcement and its bounded escape, stale-binary tolerance, Windows safety, drift test, and platform scope.

### Modified Capabilities

- `janus-router-agent`: MODIFIED "Janus routes free-form requests to Iktomi, and Iktomi can route back or onward" (unconditional `phobetor` edge, blocker tag; superset of `iktomi-always-handoff-phobetor`'s text) and MODIFIED "Deterministic hand-offs go directly to the next agent; ..." (the fixed edges are executed by the mechanism, the failure-counter escalation, tags).

## Impact

- **New Go code**: `cmd/handoff.go`, `internal/handoff/*` (edge table, tag parser, counter store, directive store, lock), `internal/config/config.go` (`handoff_enforcement`), a `status`/init check for the bound-but-missing subcommand.
- **Templates** (`hypnos`-owned): `internal/scaffold/templates/agents/{claude-code,cursor,codex,kiro,antigravity,github-copilot}/{morpheus,iktomi,phobetor,nyx}.*`; `hooks/bindings/claude-code/settings-patch.json` (not `hypnos`-owned; `nyx`/`morpheus` work) gains the four bindings.
- **Live files** (`.claude/agents/*.md`, `.claude/settings.json`) are re-synced by `dreamland init`, not hand-edited.
- **Overlaps and order** (details in design.md, "Overlap and sequencing"): `iktomi-always-handoff-phobetor` (folded in; archive this change, then archive that one with `--skip-specs`), `deterministic-routing-and-janus-guard` (shares `settings-patch.json` and the state root; this change is order-independent from it but its live task 0 verification can be shared), `claude-code-parity` (task 9.4 live re-sync is the same re-sync), `harden-commit-hook-enforcement` (archived; reused mechanism).
- **Also affected by the live findings**: `internal/scaffold/scaffold.go` (agent-file marker and refresh), `cmd/status.go`/`cmd/init.go` (out-of-date agent-file report), `internal/handoff` (`tasks.md` reader, counter `session_id`), `phantasos.*`/`baku.*` templates on six platforms (`[change: <slug>]`).
- **Behavior change** on Claude Code: after installing, a dispatcher that receives a `morpheus`/`iktomi`/`phobetor`/`nyx` report cannot end its turn or dispatch a different agent until the required call is made (bounded; escape and opt-out in design.md).
