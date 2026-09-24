## MODIFIED Requirements

### Requirement: Janus routes free-form requests to Iktomi, and Iktomi can route back or onward

For requests that have no matching specialized agent — no OpenSpec proposal/change to draft or continue, no task list to work, nothing that fits the `phantasos`/`nyx`/`morpheus`/`phobetor`/`baku` flow, and no agent-roster-maintenance intent — Janus SHALL delegate to `iktomi`, a general-purpose free-form coding agent, rather than attempting the work itself (Janus has no file-editing tools) or forcing the request into one of the specialized roles.

Before Iktomi's own work is complete, it is not limited to a single fixed hand-off target the way the narrow pipeline agents (`nyx`, `morpheus`, `phobetor`) are: it MAY redirect directly to any other agent when it discovers mid-task that the work actually fits a specialized role (e.g. it starts as free-form exploration and turns out to need `phantasos` to draft a spec, or `morpheus` to implement against an existing one) — the same broad dispatch capability Janus itself has (see the "Iktomi and the agent-roster-maintenance agents can route directly to any agent" requirement below).

Once Iktomi's own work is complete, its hand-off is fixed and unconditional on every platform: it SHALL end its report with `[handoff: complete]`, and the next step is `phobetor` regardless of whether the completed work involved file changes — never Janus for a completed turn. Where the platform has the hand-off hooks (see `deterministic-handoffs`), the dispatcher is required by the hooks to make that `phobetor` call; elsewhere the instruction says to run `dreamland handoff next`. `phobetor` then applies its fixed rules (`baku` on pass; `morpheus`, then `phantasos` after a failed retry, on failure; `phantasos` on a spec defect). Iktomi SHALL end a blocked turn (unable to proceed, or the next step needs broader context only Janus has) with `[handoff: blocked]` and report the blocker to Janus; a blocked turn has nothing for `phobetor` to validate.

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

- **WHEN** Iktomi's free-form work is complete, on any platform, whether or not files changed
- **THEN** its instructions direct it to hand off directly to `phobetor` and end its report with `[handoff: complete]`
- **AND** no template on any of the six platforms contains a file-changed conditional or directs a completed turn to Janus

#### Scenario: Iktomi reports a blocker to Janus

- **WHEN** Iktomi's free-form work is blocked or doesn't point to any specific next agent, and is not complete
- **THEN** it ends its report with `[handoff: blocked]` and reports the blocker to Janus

#### Scenario: Request mentioning "openspec" is not misrouted to Iktomi on keyword alone

- **WHEN** a request's text contains the word "openspec" but clearly asks to draft a proposal, work a task, or close a change (i.e. it matches a specialized agent's criteria)
- **THEN** Janus routes to that specialized agent, not to `iktomi`, even though the literal word "openspec" appears in the request

### Requirement: Deterministic hand-offs go directly to the next agent; ambiguous or terminal hand-offs go through Janus

Not every hand-off needs Janus's judgment. Where a task's next step is fixed and unconditional (or is one of a small number of outcomes the current agent itself determines, like Phobetor's verdict), the agent's instructions SHALL name the next agent directly, and the next call SHALL NOT be routed through Janus's judgment. Because these agents lack the `Agent` tool, the fixed edges SHALL be executed by the `dreamland handoff` mechanism (see the `deterministic-handoffs` capability), whose single edge table is the source of truth and is drift-tested against the agent templates. Where the next step requires broader context or is genuinely ambiguous, the agent SHALL report to Janus instead, which then applies its routing table (or asks the user). A third category — the broad-routing agents (`iktomi`, `zhougong`, `hypnos`, `mengpo`) — is neither: their work is open-ended enough that no fixed edge list captures it before completion, so they may dispatch directly to *any* other agent when their own work clearly points there (see the "Iktomi and the agent-roster-maintenance agents can route directly to any agent" requirement below).

The narrow deterministic edges are:

- `nyx` → `morpheus` (the acceptance test is written; implementation is always next).
- `morpheus` → `phobetor` (implementation is complete; validation is always next).
- `iktomi` → `phobetor` (its own work is complete; validation is always next).
- `phobetor` → `baku` (`[verdict: pass]` and the change's `tasks.md` is fully ticked, or its task state is unknown).
- `phobetor` → `morpheus` (`[verdict: fail]`, the first failure for the change).
- `phobetor` → `phantasos` (`[verdict: fail]` after a `morpheus` retry has already failed for the change, or `[verdict: spec-defect]`).

The through-Janus cases (judgment or terminal, no fixed edge) are:

- `nyx`, `morpheus`, or `iktomi` ending with `[handoff: blocked]`, including `morpheus` escalating a genuinely ambiguous requirement (Janus decides who's best positioned to resolve it, typically `phantasos`).
- `phobetor` ending with `[verdict: unverified]` (tests could not be run).
- `phobetor` ending with `[verdict: pass]` while the change still has unticked tasks (a partial pass: reported to Janus/the user with the remaining count; Janus decides between the next task via `morpheus` and stopping, and `baku` is not called until the tasks are ticked).
- `baku` confirming a change is closed (terminal — no fixed next agent).
- Any of the broad-routing agents (`iktomi`, `zhougong`, `hypnos`, `mengpo`), when their own work doesn't clearly point to a specific next agent.

This applies uniformly on every platform as instruction text. On GitHub Copilot specifically, both the narrow deterministic edges and the broad-routing agents' fan-out are additionally declared structurally in frontmatter (`agents:` — see the `agent-scaffolding` capability).

#### Scenario: Nyx hands off directly to Morpheus after writing the acceptance test

- **WHEN** any platform's `nyx.*` agent file is installed
- **THEN** its instruction body directs it, once the failing acceptance test is written, to hand off directly to `morpheus`, not to Janus, and to end its report with a `[handoff: ...]` tag

#### Scenario: Morpheus escalates ambiguity to Janus, not directly to Phantasos

- **WHEN** any platform's `morpheus.*` agent file is installed
- **THEN** its instruction body directs it to escalate ambiguous requirements to Janus with `[handoff: blocked]` (a judgment call, not a fixed next step), and does not name `phantasos` (or the legacy `spec-writer` name) as a direct escalation target for this case

#### Scenario: Morpheus hands off directly to Phobetor when implementation is complete

- **WHEN** any platform's `morpheus.*` agent file is installed
- **THEN** its instruction body directs it, once a task's implementation is complete, to hand off directly to `phobetor` and end its report with `[handoff: complete]`, not to Janus

#### Scenario: Phobetor reports a machine-readable verdict and the counter picks the target

- **WHEN** any platform's `phobetor.*` agent file is installed
- **THEN** its instruction body directs it to end its report with exactly one of `[verdict: pass]`, `[verdict: fail]`, `[verdict: spec-defect]`, `[verdict: unverified]`, and states that a first failure goes to `morpheus` while a failure after a failed `morpheus` retry goes to `phantasos`, with the hand-off mechanism, not the agent, applying the counter

#### Scenario: Baku confirms closure with Janus, not "the orchestrator"

- **WHEN** any platform's `baku.*` agent file is installed
- **THEN** its instruction body directs it to confirm the change is closed with Janus (a terminal report, not a hand-off to a fixed next agent), and does not reference the legacy `orchestrator` name
