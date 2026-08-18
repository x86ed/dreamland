## MODIFIED Requirements

### Requirement: Janus dispatches agent-roster tasks to Hypnos or Meng Po via /opsx:apply

For a change drafted by `phantasos` whose `tasks.md` describes creating or retiring an agent, or describes a workflow-graph-structural change (a routing-edge change, or a hook/skill attach/detach) rather than application code, Janus's routing table SHALL name `hypnos` and `mengpo` as `/opsx:apply` task-implementer targets, the same dispatch mechanism used for `nyx`/`morpheus` on code tasks: `hypnos` when the task creates a new agent or describes a workflow-graph-structural change, `mengpo` when the task retires an agent.

#### Scenario: Agent-creation task dispatched to Hypnos

- **WHEN** Janus routes a task from an OpenSpec change whose `tasks.md` describes authoring a new agent
- **THEN** it delegates to `hypnos`

#### Scenario: Agent-retirement task dispatched to Meng Po

- **WHEN** Janus routes a task from an OpenSpec change whose `tasks.md` describes retiring an existing agent
- **THEN** it delegates to `mengpo`

#### Scenario: Workflow-graph-structural task dispatched to Hypnos, in place of a coding agent

- **WHEN** Janus routes a task from an OpenSpec change whose `tasks.md` describes a routing-edge change or a hook/skill attach/detach, rather than application code
- **THEN** it delegates to `hypnos`, in place of `morpheus`, and `hypnos` implements the task by authoring a plan and running `dreamland hypnos-serve --mode=apply-plan`
