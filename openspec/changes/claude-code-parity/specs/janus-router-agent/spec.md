## MODIFIED Requirements

### Requirement: Janus routes free-form requests to Iktomi, and Iktomi can route back or onward

For requests that have no matching specialized agent — no OpenSpec proposal/change to draft or continue, no task list to work, nothing that fits the `phantasos`/`nyx`/`morpheus`/`phobetor`/`baku` flow, and no agent-roster-maintenance intent — Janus SHALL delegate to `iktomi`, a general-purpose free-form coding agent, rather than attempting the work itself (Janus has no file-editing tools) or forcing the request into one of the specialized roles.

This is a two-way flow, not a one-shot dispatch: Iktomi's work is inherently open-ended, so unlike the narrow pipeline agents (`nyx`, `morpheus`, `phobetor`) it is not limited to a single fixed hand-off target. Iktomi MAY route directly to any other agent when it discovers mid-task that the work actually fits a specialized role (e.g. it starts as free-form exploration and turns out to need `phantasos` to draft a spec, or `morpheus` to implement against an existing one) — the same broad dispatch capability Janus itself has (see the "Iktomi and the agent-roster-maintenance agents can route directly to any agent" requirement below).

When Iktomi's own work involved code or file changes, it SHALL hand off to `phobetor` for validation upon completion rather than reporting directly to Janus — the same quality gate the structured `nyx`/`morpheus` pipeline already requires before a change is considered done. `phobetor` then applies its own existing, unmodified hand-off rules (`baku` on pass, `morpheus` on an implementation bug, `phantasos` on a spec defect). When Iktomi's work made no file changes (pure investigation, answering a question, or a genuine dead end with nothing to fit), or when its turn is blocked with nothing to validate, it reports to Janus, unchanged from before.

This gap (Iktomi's instructions previously named no distinction at all) was confirmed present identically in both Claude Code's and GitHub Copilot's `iktomi.*` templates before this change; the `claude-code-parity` change that introduces this requirement fixes it on those two platforms only, matching that change's stated scope. Cursor, Codex, Kiro, and Antigravity's `iktomi.*` templates carry the same underlying gap and are not fixed by that change — this requirement applies to them too (Iktomi's routing behavior is platform-uniform, like the rest of this capability), but bringing their templates into compliance is unfinished follow-up work, not done here.

The "no OpenSpec context at all" criterion SHALL be based on the absence of a matching proposal, task list, or spec-scenario for the request to act against — never on whether the request's text happens to mention "openspec," a tool name, or a command spelling. A request that references OpenSpec by name but clearly asks to draft, apply, or close a change is routed to the matching specialized agent, not to `iktomi`, purely because it mentions the word.

#### Scenario: Ad hoc coding request routed to Iktomi

- **WHEN** a user request has no corresponding OpenSpec change, task list, spec-scenario context, or agent-roster-maintenance intent for Janus to route against
- **THEN** Janus delegates to `iktomi`

#### Scenario: Iktomi is not selected when a specialized agent fits

- **WHEN** a request clearly maps to drafting a spec, writing/implementing/validating a task, closing a change, or maintaining the agent roster
- **THEN** Janus routes to the matching specialized agent instead of `iktomi`

#### Scenario: Iktomi redirects directly to a specialist mid-task

- **WHEN** Iktomi is working a free-form request and determines it actually fits a specialized agent's role
- **THEN** it hands off directly to that agent, not back through Janus first

#### Scenario: Iktomi routes completed code changes to Phobetor, not directly to Janus

- **WHEN** Iktomi's free-form work is complete and included editing or writing files
- **THEN** it hands off to `phobetor` for validation, not directly to Janus

#### Scenario: Phobetor's existing pass/fail routing applies unchanged to Iktomi's work

- **WHEN** `phobetor` validates a change that originated from Iktomi's free-form work
- **THEN** it hands off to `baku` on success, `morpheus` on an implementation bug, or `phantasos` on a spec defect — the same rules this capability already documents for the structured pipeline, unmodified

#### Scenario: Iktomi reports back to Janus when there were no file changes to validate

- **WHEN** Iktomi's free-form work is complete with no file changes, blocked, or doesn't point to any specific next agent
- **THEN** it reports to Janus, the same as any terminal/judgment case

#### Scenario: Request mentioning "openspec" is not misrouted to Iktomi on keyword alone

- **WHEN** a request's text contains the word "openspec" but clearly asks to draft a proposal, work a task, or close a change (i.e. it matches a specialized agent's criteria)
- **THEN** Janus routes to that specialized agent, not to `iktomi`, even though the literal word "openspec" appears in the request
