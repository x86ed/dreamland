## Why

Live use of dreamland's Claude Code scaffold surfaces three related handoff-integrity bugs the `claude-code-parity` change didn't catch because its own live-verification tasks (10.x) were never completed: (1) `guard-artifact` treats `tasks.md` as exclusively `phantasos`-owned, so `morpheus`/`nyx` are blocked from checking off the very tasks they're dispatched to implement; (2) the per-agent `hooks.Stop` blocks added to every `.claude/agents/*.md` file duplicate the workspace-level `.claude/settings.json` `SubagentStop`/`PreToolUse` bindings — Claude Code runs all matching hooks for an event in parallel with no ordering guarantee, so both fire for the same handoff and race on `git config user.name` and the commit subject, occasionally losing the correctly-dispatched agent's identity to the janus fallback; (3) `dreamland commit --reason handoff` treats ordinary git-mechanics failures as blocking (exit 2), which on `SubagentStop` prevents the subagent from stopping at all instead of letting the fixed pipeline (`nyx`→`morpheus`→`phobetor`→`baku`) continue.

## What Changes

- **`guard-artifact` no longer blocks checkbox-only edits to `tasks.md` by non-`phantasos` agents.** Ownership of `tasks.md`'s prose (task descriptions, scope) stays with `phantasos`; marking a task `- [ ]` → `- [x]` is the documented job of whichever agent implements it (`morpheus`, `nyx`, `hypnos`, `mengpo`), so the guard's ownership table stops applying to that file for that specific kind of edit.
- **Remove the duplicate, unscoped identity/handoff hook entries from `.claude/settings.json`'s `SubagentStop`.** Per-agent `hooks.Stop` blocks (added by `claude-code-parity`, converted to `SubagentStop` at runtime per Claude Code's own subagent-frontmatter-hooks mechanism) already run `coauthor --agent-name`, `telemetry write`, `version-bump`, and `commit --reason handoff` deterministically scoped to the exact dispatched agent. The workspace-level copy of those same commands, which resolves identity from the hook payload instead of a hardcoded flag, is now redundant and — because Claude Code runs both in parallel — a source of nondeterministic identity loss. The workspace-level `PreToolUse(Task|Agent)` `coauthor --hook` binding is unaffected (it runs before dispatch, in the parent's turn, not the subagent's).
- **`dreamland commit --reason handoff` stops treating ordinary git-mechanics failures (`git add -A`, `git commit`) as blocking.** These become non-blocking (exit 1, per Claude Code's documented "any exit code other than 2 doesn't block" rule for `SubagentStop`) so a transient failure surfaces to the user without trapping the subagent mid-handoff. The `--reason turn-complete` test-failure gate is unchanged — that one's blocking behavior is intentional.
- **Correct the stale `dev-workflow-hooks` spec text** claiming Claude Code's `SubagentStop` payload carries no sub-agent identifier — it does (`agent_type`), confirmed against Claude Code's current hooks reference; the code path already accounts for this (see `agentidentity.FromPayload`'s doc comment), only the spec prose is out of date.
- Tests covering all of the above; a self-hosting dogfood pass on this repo's own `.claude/` scaffold to confirm the duplicate-hook removal doesn't regress telemetry/version-bump coverage.

## Capabilities

### New Capabilities

(none — this change corrects behavior within existing capabilities, it doesn't introduce a new one)

### Modified Capabilities

- `dev-workflow-hooks`: removes the duplicate workspace-level `SubagentStop` identity/handoff bindings now that per-agent scoped hooks cover the same commands deterministically; `commit --reason handoff` no longer blocks on git-mechanics failures; corrects the stale claim about Claude Code's `SubagentStop` payload.

`session-agent-identity`'s own requirements (the `janus` fallback default, the closed set of registered names) are unchanged by this fix — the race this change removes was causing that spec's already-documented fallback to trigger more often than intended, not violating the spec itself, so no delta is needed there.

Note: `guard-artifact`'s ownership table (implemented by the still-unarchived `claude-code-parity` change) has no archived spec in `openspec/specs/` yet, so the `tasks.md` checkbox fix below is a correction to that pending implementation, not a delta against an existing spec — it will be folded into `fixed-pipeline-enforcement`'s spec when `claude-code-parity` is archived.

## Impact

- `cmd/guard_artifact.go` — ownership check gains a checkbox-edit exception for `tasks.md` (corrects `claude-code-parity`'s not-yet-archived implementation).
- `internal/scaffold/templates/hooks/bindings/claude-code/settings-patch.json` — remove the duplicate `SubagentStop` identity/handoff commands (keep `PreToolUse(Task|Agent)` coauthor binding and the unscoped `telemetry write`/`version-bump --patch` entries).
- `cmd/commit.go` — `--reason handoff` git-mechanics errors return unwrapped (non-blocking) instead of `Blocking(...)`.
- `openspec/specs/dev-workflow-hooks/spec.md`, `openspec/specs/session-agent-identity/spec.md` — deltas for the above.
- `openspec/changes/claude-code-parity/tasks.md` — note referencing this correction (that change's own guard-artifact task, 7.1, is affected).
- Tests: `cmd/guard_artifact_test.go`, `cmd/commit_test.go`, `internal/scaffold/scaffold_test.go`.
- Repo self-hosting: this repo's own `.claude/settings.json` needs the same duplicate entries removed to dogfood the fix.
