## Why

Today, agent-roster-maintenance requests skip spec drafting entirely: Janus routes a direct "author a new agent" ask straight to `hypnos`, and a `zhougong` report recommending a new agent hands off straight to `hypnos` too — `phantasos` is never involved. Retiring an agent is the same: Janus routes straight to `mengpo`, with no drafted rationale beyond `mengpo`'s own free-text summary. This means agent-roster changes are the one category of work in this project that isn't spec-driven, inconsistent with every other capability change, which always starts with `phantasos` drafting a proposal/design/tasks. It also puts spec-authoring responsibility on `zhougong`, whose job is analysis (git history, token burn, hand-off timing) — deciding *whether* a recommendation is specific enough to act on and drafting the resulting change blurs its scope.

## What Changes

- **BREAKING** (agent-instruction behavior, not CLI): `zhougong` no longer hands off directly to `hypnos`. When its report's "recommended new agent" section names a specific, unambiguous next step, `zhougong` hands off to `phantasos` instead. `phantasos` drafts the proposal/design/tasks for the roster change; `zhougong`'s own job stays confined to producing the report.
- Janus's routing table entries for new-agent-authoring and agent-retirement requests change from dispatching directly to `hypnos`/`mengpo` to dispatching to `phantasos` first — matching how every other implementation request already enters through `phantasos` (Standard SDD) or `nyx` (TDD/BDD). Direct user asks ("add an agent that does X", "retire agent Y") now draft a spec before any agent file is touched.
- `hypnos`'s trigger changes from "a direct request routed via Janus, or a `zhougong` report's recommendation" to "a `phantasos`-authored change's proposal/design/tasks describing the agent to create." `hypnos` still authors the six-platform template files and still hands off directly to `phobetor` to validate.
- `mengpo`'s trigger changes from "an agent name to retire, routed via Janus" to "a `phantasos`-authored change's proposal/design/tasks describing the agent(s) to retire and why." `mengpo`'s own archive/hard-delete mechanics and its default report-to-Janus terminal behavior are unchanged.
- Update all six platform templates (`internal/scaffold/templates/agents/{claude-code,codex,cursor,kiro,antigravity,github-copilot}/{janus,zhougong,hypnos,mengpo,phantasos}.*`) to reflect the new routing and triggers.

## Capabilities

### New Capabilities
(none)

### Modified Capabilities
- `janus-router-agent`: the routing-table requirement for agent-roster-maintenance requests changes so new-agent-authoring and agent-retirement requests dispatch to `phantasos` first, not directly to `hypnos`/`mengpo`.
- `agent-lifecycle-management`: `zhougong`'s recommendation hand-off target changes from `hypnos` to `phantasos`; `hypnos`'s and `mengpo`'s trigger descriptions change from a raw request/name to a `phantasos`-authored change.

## Impact

- 30 template files: `{janus,zhougong,hypnos,mengpo,phantasos}` × 6 platforms (`claude-code`, `codex`, `cursor`, `kiro`, `antigravity`, `github-copilot`).
- `openspec/specs/janus-router-agent/spec.md` and `openspec/specs/agent-lifecycle-management/spec.md` — MODIFIED requirements.
- No Go source or CLI command changes — this is agent-instruction content only.
