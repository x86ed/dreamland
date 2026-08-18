## Why

Today, once Iktomi's own free-form work concludes, its instructions leave the next step conditional or optional rather than fixed. The still-open `claude-code-parity` change (already landed in `internal/scaffold/templates/agents/{claude-code,github-copilot}/iktomi.*`, not yet archived) narrowed this partway: it routes Iktomi to `phobetor` for validation only when the completed work touched files, and still reports straight to Janus when it didn't — and it explicitly left `cursor`/`codex`/`kiro`/`antigravity` untouched, still on the original "report completion or blockers to Janus" text with no `phobetor` mention at all. The result is three different behaviors across six platforms for the same agent's completion step, and even the two platforms with the split still bypass `phobetor` for non-file-change turns.

The user wants a single, simple rule instead: Iktomi always hands off to `phobetor` once its own work is complete, on every platform, with no file-changed/no-file-changed branching to maintain. This also folds `phobetor`'s existing pass/fail/spec-defect routing (`baku`/`morpheus`/`phantasos`) into Iktomi's own completion path as a validation gate, the same quality checkpoint the structured `nyx`/`morpheus` pipeline already goes through before a change is considered done.

## What Changes

- **MODIFIED** (agent-instruction behavior, all six platforms): Iktomi's completion step changes from "hand off to `phobetor` only if files changed, otherwise report to Janus" (claude-code, github-copilot) or "report to Janus" (cursor, codex, kiro, antigravity) to a single unconditional rule: once Iktomi's own work is complete, it always hands off directly to `phobetor` — never straight to Janus for a completed turn, regardless of whether the work involved file changes.
- Iktomi's other two responsibilities are unchanged: it still handles a request directly using its own judgment, and it still may redirect mid-task, before completion, straight to a specialist agent whose role clearly fits (e.g. `phantasos`, `morpheus`) — this is a different moment (before the work is done) from the completion hand-off this change fixes.
- Iktomi still reports a blocker to Janus, unchanged — a blocked turn has nothing for `phobetor` to validate, so this is not folded into the new fixed edge.
- `phobetor` itself is unchanged: it already applies its existing, unmodified pass/fail/spec-defect routing (`baku`/`morpheus`/`phantasos`) regardless of which agent handed off to it.
- Update all six platform templates (`internal/scaffold/templates/agents/{claude-code,cursor,codex,kiro,antigravity,github-copilot}/iktomi.*`) to the same unconditional rule, resolving both the two-platform split and the four-platform gap in one pass.

## Capabilities

### New Capabilities
(none)

### Modified Capabilities
- `janus-router-agent`: the "Janus routes free-form requests to Iktomi, and Iktomi can route back or onward" requirement changes so Iktomi's completion hand-off to `phobetor` is unconditional and platform-uniform, not gated on whether the work touched files, and not limited to two of six platforms.

## Impact

- 6 template files: `internal/scaffold/templates/agents/{claude-code,cursor,codex,kiro,antigravity,github-copilot}/iktomi.*`.
- `openspec/specs/janus-router-agent/spec.md` — MODIFIED requirement (same requirement the still-open `claude-code-parity` change also modifies; see design.md's note on reconciling the two at archive time).
- No Go source or CLI command changes — this is agent-instruction content only.
- This repo's own live, self-hosted `.claude/agents/iktomi.md` is intentionally **not** touched by this change — see design.md. It is already out of sync with the templates for unrelated reasons (`claude-code-parity`'s task 9.4, blocked on user confirmation) and will pick up both changes' edits together whenever that re-sync runs.
