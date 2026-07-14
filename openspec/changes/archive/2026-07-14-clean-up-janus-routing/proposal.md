## Why

Janus's routing is pure natural-language judgment with no explicit config, and two parallel entry paths exist for the same OpenSpec lifecycle: the new `/opsx:*` commands (which go through Janus) and the legacy `.claude/skills/openspec-{propose,explore,apply-change,archive-change}/` skills (auto-discovered by the skill matcher, which **completely bypass Janus** — no routing decision, no `dreamland coauthor`/`telemetry write` hand-off bookkeeping). Neither `janus.md` nor `iktomi.md` disambiguates the literal keyword "openspec" from Janus's own definition of "OpenSpec context," so requests that mention OpenSpec in free text risk being routed to `iktomi` (the no-context/free-form fallback) instead of `phantasos`/`nyx`/`morpheus`/`baku`. There is also no enumerated, machine-checkable routing table — correctness depends entirely on an LLM parsing prose — and no explicit scope guardrails stopping Janus from acting outside its router role beyond the tool-binding restriction.

## What Changes

- Reconcile the legacy `openspec-{propose,explore,apply-change,archive-change}` skills with the `opsx:*` command set: **BREAKING** for any workflow relying on the legacy skill names being auto-discovered outside Janus's routing. Either retire the legacy skills in favor of `opsx:*` (with a redirect stub so old muscle-memory invocations still resolve), or make them thin wrappers that route through Janus like every other entry point — pick one so there is exactly one non-bypassing path per lifecycle stage.
- Add an explicit, enumerated routing table to Janus's instructions (all six platform variants) that lists every known command/skill spelling per target agent, including historical `openspec-*` names, so routing does not depend solely on free-text judgment of ambiguous phrases like "openspec."
- Fix the `iktomi` fallback definition (`janus.md`, `iktomi.md`, and platform equivalents) so a request merely mentioning the word "openspec" is not treated as "no OpenSpec context" — disambiguate "the request names/uses the OpenSpec tooling" from "the request has no proposal/task list and needs free-form coding."
- Add explicit scope/guardrail language to Janus's instructions restricting it to routing decisions only — no partial implementation, no direct file edits, no answering the underlying request itself — reinforcing the existing tool restriction (`Read, Bash`, no `Edit`/`Write`) with an instruction-level refusal rule for any out-of-scope action a user or hand-off might request of it.
- Validate and correct routing/command enumeration across all six platform templates (Claude Code, Codex, Cursor, Kiro, Antigravity, GitHub Copilot) so each lists the same complete command/agent set with no platform silently missing an entry.
- Update the governing OpenSpec spec files (`janus-router-agent`, `router-slash-commands`) to capture the enumerated routing table and the legacy-skill reconciliation decision as testable requirements/scenarios, so future changes can't silently reintroduce a bypass path.

## Capabilities

### New Capabilities
(none — this change corrects requirements of existing capabilities, it does not introduce new ones)

### Modified Capabilities
- `janus-router-agent`: Janus's routing decision must be driven by an explicit, enumerated command/skill-to-agent table (including historical `openspec-*` names) rather than free-text judgment alone; the `iktomi` fallback criterion must not trigger on the literal keyword "openspec"; Janus's instructions must include explicit out-of-scope refusal guardrails beyond the existing tool restriction.
- `router-slash-commands`: The legacy `openspec-{propose,explore,apply-change,archive-change}` skills must not bypass Janus — they must either be retired in favor of `/opsx:*` (with redirect) or routed through Janus like every other entry point; the full command/skill enumeration must be consistent across all six platform templates.

## Impact

- **Modified files**: `internal/scaffold/templates/agents/*/janus.*` and `internal/scaffold/templates/agents/*/iktomi.*` (all six platform directories) — routing table, iktomi disambiguation, scope guardrails.
- **Modified or removed files**: `.claude/skills/openspec-{propose,explore,apply-change,archive-change}/SKILL.md` — reconciled with `.claude/commands/opsx/{propose,explore,apply,archive}.md`.
- **Modified spec files**: `openspec/changes/janus-router-agent/specs/janus-router-agent/spec.md`, `openspec/changes/janus-router-agent/specs/router-slash-commands/spec.md` (this change's requirements amend that not-yet-archived change's specs directly, since it has not been archived into `openspec/specs/` yet).
- **No new dependencies, no new CLI commands.**
