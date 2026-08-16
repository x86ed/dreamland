## MODIFIED Requirements

### Requirement: Janus routes free-form requests to Iktomi, and Iktomi can route back or onward

For requests that have no matching specialized agent — no OpenSpec proposal/change to draft or continue, no task list to work, nothing that fits the `phantasos`/`nyx`/`morpheus`/`phobetor`/`baku` flow, and no agent-roster-maintenance intent — Janus SHALL delegate to `iktomi`, a general-purpose free-form coding agent, rather than attempting the work itself (Janus has no file-editing tools) or forcing the request into one of the specialized roles.

Before Iktomi's own work is complete, it is not limited to a single fixed hand-off target the way the narrow pipeline agents (`nyx`, `morpheus`, `phobetor`) are: it MAY redirect directly to any other agent when it discovers mid-task that the work actually fits a specialized role (e.g. it starts as free-form exploration and turns out to need `phantasos` to draft a spec, or `morpheus` to implement against an existing one) — the same broad dispatch capability Janus itself has (see the "Iktomi and the agent-roster-maintenance agents can route directly to any agent" requirement below).

Once Iktomi's own work is complete, its hand-off is fixed and unconditional: it SHALL always hand off directly to `phobetor` for validation, regardless of whether the completed work involved file changes — never straight to Janus for a completed turn. `phobetor` then applies its own existing, unmodified hand-off rules (`baku` on pass, `morpheus` on an implementation bug, `phantasos` on a spec defect). This applies uniformly on every platform. Iktomi reports to Janus only when its turn is blocked (unable to proceed, or the next step needs broader context only Janus has) rather than complete — a blocked turn has nothing for `phobetor` to validate.

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

#### Scenario: Iktomi always hands off to Phobetor once its own work is complete

- **WHEN** Iktomi's free-form work is complete, on any platform
- **THEN** it hands off directly to `phobetor` for validation, regardless of whether the completed work involved file changes
- **AND** it does not report completion to Janus first

#### Scenario: Iktomi reports a blocker to Janus

- **WHEN** Iktomi's free-form work is blocked or doesn't point to any specific next agent, and is not complete
- **THEN** it reports the blocker to Janus, the same as any terminal/judgment case

#### Scenario: Request mentioning "openspec" is not misrouted to Iktomi on keyword alone

- **WHEN** a request's text contains the word "openspec" but clearly asks to draft a proposal, work a task, or close a change (i.e. it matches a specialized agent's criteria)
- **THEN** Janus routes to that specialized agent, not to `iktomi`, even though the literal word "openspec" appears in the request
