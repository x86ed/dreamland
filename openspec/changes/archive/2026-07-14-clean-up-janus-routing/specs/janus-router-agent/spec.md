## MODIFIED Requirements

### Requirement: Janus's instructions define a routing table to the other nine agents

Janus's instruction body SHALL include an explicit routing table stating which requests are delegated to `phantasos`, `nyx`, `morpheus`, `phobetor`, `baku`, `iktomi`, `zhougong`, `hypnos`, and `mengpo`, and that ambiguous or general requests are routed by Janus's own judgment after checking `openspec status`. Janus's own dispatch covers the *entry point* into implementation work and the judgment/terminal cases (see the "Deterministic hand-offs go directly to the next agent; ambiguous or terminal hand-offs go through Janus" requirement below) — once a task is underway, most of its subsequent hops are direct agent-to-agent hand-offs Janus does not mediate. The table SHALL document that implementation work is not strictly linear — there are two valid entry flows for spec-driven work, and Janus chooses between them per task (see the "Janus chooses between the acceptance-test and direct-implementation flows per task" requirement below):

- **Direct-implementation flow**: Janus dispatches to `morpheus`, which then hands off directly (no further Janus involvement) through `phobetor` to `baku`.
- **Acceptance-test flow**: Janus dispatches to `nyx`, which then hands off directly through `morpheus` and `phobetor` to `baku`.

Both flows converge at `phobetor` (validation) and `baku` (finalization); they differ only in whether `nyx` writes an acceptance test before `morpheus` implements — and only in which agent Janus's entry dispatch targets, since everything downstream of that entry point is a direct hand-off. For requests that don't fit the OpenSpec-driven flow at all (no proposal, no task list — free-form coding requests), the table SHALL name `iktomi` as the catch-all target. For requests about agent performance, usage analytics, authoring a new agent, or retiring an unused one, the table SHALL name `zhougong` (analytics/reports), `hypnos` (agent authoring), and `mengpo` (agent archival/deletion) respectively — these three are agent-roster maintenance requests, not implementation work, and don't participate in either implementation flow.

The routing table entry for each agent SHALL additionally enumerate every current and historical command or skill spelling that resolves to that agent, including the legacy `openspec-propose`, `openspec-explore`, `openspec-apply-change`, and `openspec-archive-change` skill names alongside their current `/opsx:propose`, `/opsx:explore`, `/opsx:apply`, and `/opsx:archive` equivalents, so routing does not depend solely on free-text judgment of an unlabeled request.

#### Scenario: Routing table present in installed Janus instructions

- **WHEN** any platform's `janus.*` agent file is installed
- **THEN** its instruction body names all nine of `phantasos`, `nyx`, `morpheus`, `phobetor`, `baku`, `iktomi`, `zhougong`, `hypnos`, and `mengpo` as delegation targets
- **AND** states that Janus checks `openspec status` (or the platform's equivalent) before making a routing decision on an ambiguous request
- **AND** describes both the direct-implementation flow (entry at `morpheus`) and the acceptance-test flow (entry at `nyx`) as valid, non-mutually-exclusive entry points for a given task, noting that the hops after entry are direct hand-offs Janus does not mediate
- **AND** names `iktomi` as the fallback for requests with no matching specialized agent
- **AND** names `zhougong`, `hypnos`, and `mengpo` as the targets for agent-roster analytics, authoring, and archival requests respectively

#### Scenario: Routing table lists historical openspec-* spellings alongside current ones

- **WHEN** any platform's `janus.*` agent file is installed
- **THEN** its routing table entry for `phantasos` lists both `/opsx:propose`/`/opsx:explore` and the legacy `openspec-propose`/`openspec-explore` skill names
- **AND** its routing table entry for `baku` lists both `/opsx:archive` and the legacy `openspec-archive-change` skill name
- **AND** its routing table entry describing the `/opsx:apply` two-flow decision also lists the legacy `openspec-apply-change` skill name

### Requirement: Janus routes free-form requests to Iktomi, and Iktomi can route back or onward

For requests that have no matching specialized agent — no OpenSpec proposal/change to draft or continue, no task list to work, nothing that fits the `phantasos`/`nyx`/`morpheus`/`phobetor`/`baku` flow, and no agent-roster-maintenance intent — Janus SHALL delegate to `iktomi`, a general-purpose free-form coding agent, rather than attempting the work itself (Janus has no file-editing tools) or forcing the request into one of the specialized roles.

This is a two-way flow, not a one-shot dispatch: Iktomi's work is inherently open-ended, so unlike the narrow pipeline agents (`nyx`, `morpheus`, `phobetor`) it is not limited to a single fixed hand-off target. Iktomi MAY route directly to any other agent when it discovers mid-task that the work actually fits a specialized role (e.g. it starts as free-form exploration and turns out to need `phantasos` to draft a spec, or `morpheus` to implement against an existing one) — the same broad dispatch capability Janus itself has (see the "Iktomi and the agent-roster-maintenance agents can route directly to any agent" requirement below). When no specific agent fits, or Iktomi's turn simply ends, it reports back to Janus rather than guessing.

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

#### Scenario: Iktomi reports back to Janus when no specific agent fits

- **WHEN** Iktomi's free-form work is complete, blocked, or doesn't point to any specific next agent
- **THEN** it reports to Janus, the same as any terminal/judgment case

#### Scenario: Request mentioning "openspec" is not misrouted to Iktomi on keyword alone

- **WHEN** a request's text contains the word "openspec" but clearly asks to draft a proposal, work a task, or close a change (i.e. it matches a specialized agent's criteria)
- **THEN** Janus routes to that specialized agent, not to `iktomi`, even though the literal word "openspec" appears in the request

## ADDED Requirements

### Requirement: Janus refuses to act outside the dispatch role

Beyond the existing tool-binding restriction (no `Edit`/`Write`), Janus's instructions SHALL state explicitly that its only valid action is deciding a target agent and dispatching to it. If a request or a mid-routing situation asks Janus to implement a change, answer the substance of a question, or investigate beyond what `openspec status` (or platform equivalent) provides, Janus SHALL delegate that request to the appropriate agent (typically `iktomi` if no specialized agent fits) rather than performing or attempting the work itself.

#### Scenario: Janus declines to answer a substantive question directly

- **WHEN** a request asks Janus to explain, implement, or otherwise directly resolve something rather than route it
- **THEN** Janus's instructions direct it to delegate the request to the matching agent (or `iktomi` if none fits) instead of responding to the substance itself

#### Scenario: Janus's diagnostic reads stay bounded to routing decisions

- **WHEN** Janus needs information to decide where to route a request
- **THEN** its instructions limit that investigation to what's needed for the routing decision (e.g. `openspec status`), not open-ended exploration of the codebase on Janus's own behalf
