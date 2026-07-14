## ADDED Requirements

### Requirement: Janus is installed as the router agent, replacing Hypnos in that role

The scaffold installer SHALL install an agent named `janus` (in place of `orchestrator`, and in place of the router role Hypnos held earlier in this change's own history) on every supported platform. `hypnos` is not deleted — it is redefined elsewhere as the agent-authoring agent (see the `agent-lifecycle-management` capability); it no longer performs any routing function. Janus's definition SHALL carry a router-only marker distinguishing it from the other nine agents:

- Claude Code / GitHub Copilot (YAML frontmatter): `role: router` field, plus an opening instruction paragraph stating it never edits files, writes code, or writes specs itself.
- Codex CLI (`.toml`): top-level `role = "router"` key.
- Cursor (`.mdc`) / Kiro (plain markdown): the same "pure router" statement in the instruction body (no `role` frontmatter convention available on these platforms).
- Antigravity (`SKILL.md`): the same "pure router" statement in the instruction body.

#### Scenario: Claude Code Janus agent installed with router marker

- **WHEN** the selected coding tool is "Claude Code" and `dreamland init` completes successfully
- **THEN** `.claude/agents/janus.md` exists with frontmatter containing `name: janus`, `role: router`, and a `tools` list that excludes `Edit` and `Write`
- **AND** the instruction body opens with a statement that Janus never edits files, writes code, or writes specs itself

#### Scenario: Codex Janus agent installed with router marker

- **WHEN** the selected coding tool is "Codex CLI" and `dreamland init` completes successfully
- **THEN** `.codex/agents/janus.toml` exists with `name = "janus"` and `role = "router"`

### Requirement: Janus's tool bindings exclude file-editing capabilities

On every platform, Janus's tool/capability binding SHALL exclude any tool capable of writing or editing files (e.g. `Edit`, `Write`, `apply_patch`), retaining only read and dispatch tools (e.g. `Read`, `Bash`, sub-agent invocation).

#### Scenario: Janus cannot edit files on Claude Code

- **WHEN** `.claude/agents/janus.md` is installed
- **THEN** its `tools` frontmatter field does not include `Edit` or `Write`

#### Scenario: Janus cannot apply patches on Codex

- **WHEN** `.codex/agents/janus.toml` is installed
- **THEN** it does not grant `apply_patch` capability

### Requirement: Janus's instructions define a routing table to the other nine agents

Janus's instruction body SHALL include an explicit routing table stating which requests are delegated to `phantasos`, `nyx`, `morpheus`, `phobetor`, `baku`, `iktomi`, `zhougong`, `hypnos`, and `mengpo`, and that ambiguous or general requests are routed by Janus's own judgment after checking `openspec status`. Janus's own dispatch covers the *entry point* into implementation work and the judgment/terminal cases (see the "Deterministic hand-offs go directly to the next agent; ambiguous or terminal hand-offs go through Janus" requirement below) — once a task is underway, most of its subsequent hops are direct agent-to-agent hand-offs Janus does not mediate. The table SHALL document that implementation work is not strictly linear — there are two valid entry flows for spec-driven work, and Janus chooses between them per task (see the "Janus chooses between the acceptance-test and direct-implementation flows per task" requirement below):

- **Direct-implementation flow**: Janus dispatches to `morpheus`, which then hands off directly (no further Janus involvement) through `phobetor` to `baku`.
- **Acceptance-test flow**: Janus dispatches to `nyx`, which then hands off directly through `morpheus` and `phobetor` to `baku`.

Both flows converge at `phobetor` (validation) and `baku` (finalization); they differ only in whether `nyx` writes an acceptance test before `morpheus` implements — and only in which agent Janus's entry dispatch targets, since everything downstream of that entry point is a direct hand-off. For requests that don't fit the OpenSpec-driven flow at all (no proposal, no task list — free-form coding requests), the table SHALL name `iktomi` as the catch-all target. For requests about agent performance, usage analytics, authoring a new agent, or retiring an unused one, the table SHALL name `zhougong` (analytics/reports), `hypnos` (agent authoring), and `mengpo` (agent archival/deletion) respectively — these three are agent-roster maintenance requests, not implementation work, and don't participate in either implementation flow.

#### Scenario: Routing table present in installed Janus instructions

- **WHEN** any platform's `janus.*` agent file is installed
- **THEN** its instruction body names all nine of `phantasos`, `nyx`, `morpheus`, `phobetor`, `baku`, `iktomi`, `zhougong`, `hypnos`, and `mengpo` as delegation targets
- **AND** states that Janus checks `openspec status` (or the platform's equivalent) before making a routing decision on an ambiguous request
- **AND** describes both the direct-implementation flow (entry at `morpheus`) and the acceptance-test flow (entry at `nyx`) as valid, non-mutually-exclusive entry points for a given task, noting that the hops after entry are direct hand-offs Janus does not mediate
- **AND** names `iktomi` as the fallback for requests with no matching specialized agent
- **AND** names `zhougong`, `hypnos`, and `mengpo` as the targets for agent-roster analytics, authoring, and archival requests respectively

### Requirement: Janus chooses between the acceptance-test and direct-implementation flows per task

For each task Janus routes toward implementation, it SHALL decide whether the task goes through `nyx` first (acceptance-test flow) or directly to `morpheus` (direct-implementation flow), rather than applying one fixed sequence to every task. Janus routes to `nyx` first when the task implements new, externally-observable behavior described by one or more spec scenarios (`WHEN`/`THEN`) that don't yet have a covering test. Janus routes directly to `morpheus` when the task is mechanical/internal (e.g. a rename, a config or template edit, a refactor with no behavior change) or when a covering acceptance test already exists.

#### Scenario: New capability task routed through Nyx first

- **WHEN** Janus routes a task that implements a new spec scenario with no existing covering test
- **THEN** it delegates to `nyx` first, and only reaches `morpheus` afterward (via Janus)

#### Scenario: Mechanical task routed directly to Morpheus

- **WHEN** Janus routes a task that is a rename, config change, or other change with no new spec scenario to cover
- **THEN** it delegates directly to `morpheus`, without first routing through `nyx`

### Requirement: Janus routes free-form requests to Iktomi, and Iktomi can route back or onward

For requests that have no matching specialized agent — no OpenSpec proposal/change to draft or continue, no task list to work, nothing that fits the `phantasos`/`nyx`/`morpheus`/`phobetor`/`baku` flow, and no agent-roster-maintenance intent — Janus SHALL delegate to `iktomi`, a general-purpose free-form coding agent, rather than attempting the work itself (Janus has no file-editing tools) or forcing the request into one of the specialized roles.

This is a two-way flow, not a one-shot dispatch: Iktomi's work is inherently open-ended, so unlike the narrow pipeline agents (`nyx`, `morpheus`, `phobetor`) it is not limited to a single fixed hand-off target. Iktomi MAY route directly to any other agent when it discovers mid-task that the work actually fits a specialized role (e.g. it starts as free-form exploration and turns out to need `phantasos` to draft a spec, or `morpheus` to implement against an existing one) — the same broad dispatch capability Janus itself has (see the "Iktomi and the agent-roster-maintenance agents can route directly to any agent" requirement below). When no specific agent fits, or Iktomi's turn simply ends, it reports back to Janus rather than guessing.

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

### Requirement: Janus routes agent-roster-maintenance requests to Zhou Gong, Hypnos, or Meng Po

Requests about tuning agent performance, understanding agent/token usage, authoring a new agent, or retiring one are not implementation work and SHALL NOT be routed into either implementation flow. Janus routes:

- Analytics/reporting requests ("how much is X agent costing us", "which agent is slow", "suggest tuning changes") to `zhougong`.
- New-agent-authoring requests ("add an agent that does X", or a direct follow-up to a `zhougong` report recommending a new agent) to `hypnos`.
- Agent-retirement requests ("remove the agent we don't use anymore", "archive X") to `mengpo`.

#### Scenario: Analytics request routed to Zhou Gong

- **WHEN** a request asks about agent performance, token burn, time taken, or requests a tuning recommendation
- **THEN** Janus delegates to `zhougong`

#### Scenario: New-agent request routed to Hypnos

- **WHEN** a request asks to create/author a new agent, including one that references a `zhougong` report's recommendation
- **THEN** Janus delegates to `hypnos`

#### Scenario: Archival request routed to Meng Po

- **WHEN** a request asks to remove, delete, or archive an agent that is no longer needed
- **THEN** Janus delegates to `mengpo`

### Requirement: Janus invokes identity and telemetry commands around each delegation

Janus's instructions SHALL direct it to run `dreamland coauthor` and `dreamland telemetry write` immediately before delegating to another agent and immediately after that agent's turn completes, on platforms where no native sub-agent lifecycle hook exists to do this automatically (see the modified `dev-workflow-hooks` capability for platforms where this is instead enforced by a hook binding).

#### Scenario: Janus instructions include manual pre/post delegation calls on Cursor

- **WHEN** `.cursor/rules/janus.mdc` is installed
- **THEN** its instruction body directs Janus to run `dreamland coauthor` and `dreamland telemetry write` before and after each delegation

### Requirement: Deterministic hand-offs go directly to the next agent; ambiguous or terminal hand-offs go through Janus

Not every hand-off needs Janus's judgment. Where a task's next step is fixed and unconditional (or is one of a small number of outcomes the current agent itself determines, like Phobetor's pass/fail branches), that agent's instructions SHALL name the next agent directly and dispatch to it without first reporting to Janus. Where the next step requires broader context or is genuinely ambiguous, the agent's instructions SHALL report to Janus instead, which then applies its routing table (or asks the user) to decide what happens next. A third category — the broad-routing agents (`iktomi`, `zhougong`, `hypnos`, `mengpo`) — is neither: their work is open-ended enough that no fixed edge list captures it, so they may dispatch directly to *any* other agent when their own work clearly points there, and report to Janus only when it doesn't (see the "Iktomi and the agent-roster-maintenance agents can route directly to any agent" requirement below).

The narrow deterministic edges are:

- `nyx` → `morpheus` (the acceptance test is written; implementation is always next).
- `morpheus` → `phobetor` (implementation is complete; validation is always next).
- `phobetor` → `baku` (validation passed).
- `phobetor` → `morpheus` (validation failed because of an implementation bug).
- `phobetor` → `phantasos` (validation failed because the spec scenario itself was wrong).

The through-Janus cases (judgment or terminal, no fixed edge) are:

- `morpheus` escalating a genuinely ambiguous requirement (not a fixed hand-off — Janus decides who's best positioned to resolve it, typically `phantasos`, but this is a judgment call, not a mechanical next step).
- `baku` confirming a change is closed (terminal — no fixed next agent).
- Any of the broad-routing agents (`iktomi`, `zhougong`, `hypnos`, `mengpo`), when their own work doesn't clearly point to a specific next agent.

This applies uniformly on every platform. On GitHub Copilot specifically, both the narrow deterministic edges and the broad-routing agents' fan-out are additionally declared structurally in frontmatter (`agents:` — see the `agent-scaffolding` capability), because that platform requires the invocation graph declared upfront rather than left to prose alone; the prose instructions say the same thing there as everywhere else.

#### Scenario: Nyx hands off directly to Morpheus after writing the acceptance test

- **WHEN** any platform's `nyx.*` agent file is installed
- **THEN** its instruction body directs it, once the failing acceptance test is written, to hand off directly to `morpheus`, not to Janus

#### Scenario: Morpheus escalates ambiguity to Janus, not directly to Phantasos

- **WHEN** any platform's `morpheus.*` agent file is installed
- **THEN** its instruction body directs it to escalate ambiguous requirements to Janus (a judgment call, not a fixed next step), and does not name `phantasos` (or the legacy `spec-writer` name) as a direct escalation target for this case

#### Scenario: Morpheus hands off directly to Phobetor when implementation is complete

- **WHEN** any platform's `morpheus.*` agent file is installed
- **THEN** its instruction body directs it, once a task's implementation is complete (test passing, or no test for a mechanical task), to hand off directly to `phobetor`, not to Janus

#### Scenario: Phobetor routes directly to Baku, Morpheus, or Phantasos depending on the validation outcome

- **WHEN** any platform's `phobetor.*` agent file is installed
- **THEN** its instruction body directs it to hand off directly to `baku` on success, directly to `morpheus` when the failure is an implementation bug, and directly to `phantasos` when the failure is a defect in the spec scenario itself — none of these three go through Janus

#### Scenario: Baku confirms closure with Janus, not "the orchestrator"

- **WHEN** any platform's `baku.*` agent file is installed
- **THEN** its instruction body directs it to confirm the change is closed with Janus (a terminal report, not a hand-off to a fixed next agent), and does not reference the legacy `orchestrator` name

### Requirement: Iktomi and the agent-roster-maintenance agents can route directly to any agent

`iktomi`, `zhougong`, `hypnos`, and `mengpo` are not restricted to a narrow set of fixed hand-off targets the way `nyx`/`morpheus`/`phobetor` are. Each MAY hand off directly to any other agent when its own work clearly points there, and SHALL report to Janus when it doesn't (no specific target, or the next step needs Janus's broader context). This reflects the nature of their work — free-form coding, cross-cutting analytics, agent authoring, and agent retirement all routinely touch parts of the system a narrow pipeline agent never would.

#### Scenario: Zhou Gong hands off directly to Hypnos when its report clearly recommends one action

- **WHEN** any platform's `zhougong.*` agent file is installed and its report's "recommended new agent" section names a specific, unambiguous next step
- **THEN** its instruction body permits it to hand off directly to `hypnos`, in addition to the default of reporting to Janus when the report doesn't point to one specific action

#### Scenario: Hypnos hands off directly to Phobetor to validate a newly authored agent

- **WHEN** any platform's `hypnos.*` agent file is installed
- **THEN** its instruction body directs it, once a new agent definition is authored, to hand off directly to `phobetor` for validation — this is Hypnos's default deterministic next step, and its instruction body also notes it may hand off directly to any other agent (e.g. `mengpo`, if authoring the new agent also revealed an old one should be retired) when its own work points there

#### Scenario: Meng Po hands off directly to another agent when archival reveals a follow-up

- **WHEN** any platform's `mengpo.*` agent file is installed and archiving/deleting an agent reveals a clear, specific follow-up (e.g. a routing table entry on another agent's file still needs cleanup)
- **THEN** its instruction body permits it to hand off directly to that agent, in addition to the default of reporting completion to Janus

#### Scenario: Iktomi and the roster-maintenance agents report to Janus absent a specific target

- **WHEN** any of `iktomi.*`, `zhougong.*`, `hypnos.*`, or `mengpo.*` finishes work with no specific next agent implied
- **THEN** its instruction body directs it to report to Janus, the same as any terminal case
