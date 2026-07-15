## MODIFIED Requirements

### Requirement: Janus's instructions define a routing table to the other nine agents

Janus's instruction body SHALL include an explicit routing table stating which requests are delegated to `phantasos`, `nyx`, `morpheus`, `phobetor`, `baku`, `iktomi`, `zhougong`, `hypnos`, and `mengpo`, and that ambiguous or general requests are routed by Janus's own judgment after checking `openspec status`. Janus's own dispatch covers the *entry point* into implementation work and the judgment/terminal cases (see the "Deterministic hand-offs go directly to the next agent; ambiguous or terminal hand-offs go through Janus" requirement below) — once a task is underway, most of its subsequent hops are direct agent-to-agent hand-offs Janus does not mediate. The table SHALL document that implementation work is not strictly linear — there are two valid entry flows for spec-driven work, and Janus chooses between them per task (see the "Janus chooses between the acceptance-test and direct-implementation flows per task" requirement below):

- **Direct-implementation flow**: Janus dispatches to `morpheus`, which then hands off directly (no further Janus involvement) through `phobetor` to `baku`.
- **Acceptance-test flow**: Janus dispatches to `nyx`, which then hands off directly through `morpheus` and `phobetor` to `baku`.

Both flows converge at `phobetor` (validation) and `baku` (finalization); they differ only in whether `nyx` writes an acceptance test before `morpheus` implements — and only in which agent Janus's entry dispatch targets, since everything downstream of that entry point is a direct hand-off. For requests that don't fit the OpenSpec-driven flow at all (no proposal, no task list — free-form coding requests), the table SHALL name `iktomi` as the catch-all target. For requests about agent performance or usage analytics, the table SHALL name `zhougong`. For requests to author a new agent or retire one, the table SHALL name `phantasos` as the entry dispatch — the same drafting agent used for every other change — since agent-roster changes are drafted (`proposal.md`/`design.md`/`tasks.md`) before any agent file is created or removed; `hypnos` and `mengpo` are then dispatched as `/opsx:apply` task implementers for that drafted change, the same way `nyx`/`morpheus` are dispatched for code tasks (see the "Janus dispatches agent-roster tasks to Hypnos or Meng Po via /opsx:apply" requirement below).

The routing table entry for each agent SHALL additionally enumerate every current and historical command or skill spelling that resolves to that agent, including the legacy `openspec-propose`, `openspec-explore`, `openspec-apply-change`, and `openspec-archive-change` skill names alongside their current `/opsx:propose`, `/opsx:explore`, `/opsx:apply`, and `/opsx:archive` equivalents, so routing does not depend solely on free-text judgment of an unlabeled request.

#### Scenario: Routing table present in installed Janus instructions

- **WHEN** any platform's `janus.*` agent file is installed
- **THEN** its instruction body names all nine of `phantasos`, `nyx`, `morpheus`, `phobetor`, `baku`, `iktomi`, `zhougong`, `hypnos`, and `mengpo` as delegation targets
- **AND** states that Janus checks `openspec status` (or the platform's equivalent) before making a routing decision on an ambiguous request
- **AND** describes both the direct-implementation flow (entry at `morpheus`) and the acceptance-test flow (entry at `nyx`) as valid, non-mutually-exclusive entry points for a given task, noting that the hops after entry are direct hand-offs Janus does not mediate
- **AND** names `iktomi` as the fallback for requests with no matching specialized agent
- **AND** names `zhougong` as the target for agent-performance/analytics requests, and `phantasos` — not `hypnos` or `mengpo` directly — as the entry target for new-agent-authoring or agent-retirement requests

#### Scenario: Routing table lists historical openspec-* spellings alongside current ones

- **WHEN** any platform's `janus.*` agent file is installed
- **THEN** its routing table entry for `phantasos` lists both `/opsx:propose`/`/opsx:explore` and the legacy `openspec-propose`/`openspec-explore` skill names
- **AND** its routing table entry for `baku` lists both `/opsx:archive` and the legacy `openspec-archive-change` skill name
- **AND** its routing table entry describing the `/opsx:apply` two-flow decision also lists the legacy `openspec-apply-change` skill name

### Requirement: Janus routes agent-roster-maintenance requests to Zhou Gong for analysis and to Phantasos for drafting

Requests about tuning agent performance, understanding agent/token usage, authoring a new agent, or retiring one are not implementation work and SHALL NOT be routed into either implementation flow. Janus routes:

- Analytics/reporting requests ("how much is X agent costing us", "which agent is slow", "suggest tuning changes") to `zhougong`.
- New-agent-authoring requests ("add an agent that does X"), including one that follows a `zhougong` report's recommendation, to `phantasos` — which drafts a `proposal.md`/`design.md`/`tasks.md` describing the agent's role and rationale, the same as any other change. `phantasos` does not author the agent itself.
- Agent-retirement requests ("remove the agent we don't use anymore", "archive X") to `phantasos` for the same reason — the retirement rationale is drafted before `mengpo` acts.

`hypnos` and `mengpo` are never Janus's *entry* dispatch target for a raw request; they are dispatched afterward as `/opsx:apply` task implementers once `phantasos`'s change exists (see the "Janus dispatches agent-roster tasks to Hypnos or Meng Po via /opsx:apply" requirement below).

#### Scenario: Analytics request routed to Zhou Gong

- **WHEN** a request asks about agent performance, token burn, time taken, or requests a tuning recommendation
- **THEN** Janus delegates to `zhougong`

#### Scenario: New-agent request routed to Phantasos, not directly to Hypnos

- **WHEN** a request asks to create/author a new agent, including one that references a `zhougong` report's recommendation
- **THEN** Janus delegates to `phantasos` to draft the change first, not directly to `hypnos`

#### Scenario: Archival request routed to Phantasos, not directly to Meng Po

- **WHEN** a request asks to remove, delete, or archive an agent that is no longer needed
- **THEN** Janus delegates to `phantasos` to draft the change first, not directly to `mengpo`

### Requirement: Iktomi and the agent-roster-maintenance agents can route directly to any agent

`iktomi`, `zhougong`, `hypnos`, and `mengpo` are not restricted to a narrow set of fixed hand-off targets the way `nyx`/`morpheus`/`phobetor` are. Each MAY hand off directly to any other agent when its own work clearly points there, and SHALL report to Janus when it doesn't (no specific target, or the next step needs Janus's broader context). This reflects the nature of their work — free-form coding, cross-cutting analytics, agent authoring, and agent retirement all routinely touch parts of the system a narrow pipeline agent never would.

#### Scenario: Zhou Gong hands off directly to Phantasos when its report clearly recommends a new agent

- **WHEN** any platform's `zhougong.*` agent file is installed and its report's "recommended new agent" section names a specific, unambiguous next step
- **THEN** its instruction body permits it to hand off directly to `phantasos` — which drafts the change describing the new agent — in addition to the default of reporting to Janus when the report doesn't point to one specific action
- **AND** its instruction body does not name `hypnos` as this hand-off's direct target

#### Scenario: Hypnos hands off directly to Phobetor to validate a newly authored agent

- **WHEN** any platform's `hypnos.*` agent file is installed
- **THEN** its instruction body directs it, once a new agent definition is authored, to hand off directly to `phobetor` for validation — this is Hypnos's default deterministic next step, and its instruction body also notes it may hand off directly to any other agent (e.g. `mengpo`, if authoring the new agent also revealed an old one should be retired) when its own work points there

#### Scenario: Meng Po hands off directly to another agent when archival reveals a follow-up

- **WHEN** any platform's `mengpo.*` agent file is installed and archiving/deleting an agent reveals a clear, specific follow-up (e.g. a routing table entry on another agent's file still needs cleanup)
- **THEN** its instruction body permits it to hand off directly to that agent, in addition to the default of reporting completion to Janus

#### Scenario: Iktomi and the roster-maintenance agents report to Janus absent a specific target

- **WHEN** any of `iktomi.*`, `zhougong.*`, `hypnos.*`, or `mengpo.*` finishes work with no specific next agent implied
- **THEN** its instruction body directs it to report to Janus, the same as any terminal case

## ADDED Requirements

### Requirement: Janus dispatches agent-roster tasks to Hypnos or Meng Po via /opsx:apply

For a change drafted by `phantasos` whose `tasks.md` describes creating or retiring an agent (rather than writing code), Janus's routing table SHALL name `hypnos` and `mengpo` as `/opsx:apply` task-implementer targets, the same dispatch mechanism used for `nyx`/`morpheus` on code tasks: `hypnos` when the task creates a new agent, `mengpo` when the task retires one.

#### Scenario: Agent-creation task dispatched to Hypnos

- **WHEN** Janus routes a task from an OpenSpec change whose `tasks.md` describes authoring a new agent
- **THEN** it delegates to `hypnos`

#### Scenario: Agent-retirement task dispatched to Meng Po

- **WHEN** Janus routes a task from an OpenSpec change whose `tasks.md` describes retiring an existing agent
- **THEN** it delegates to `mengpo`
