## MODIFIED Requirements

### Requirement: Hypnos authors new agent definitions across every platform template

`hypnos`'s instructions SHALL direct it, given a role description captured in a `phantasos`-authored change's `proposal.md`/`design.md`/`tasks.md` — dispatched to `hypnos` by Janus via `/opsx:apply`, the same mechanism `morpheus` uses for code tasks (see the `janus-router-agent` capability's "Janus dispatches agent-roster tasks to Hypnos or Meng Po via /opsx:apply" requirement) — to author a complete new agent definition:

1. Run `dreamland oneiroi seed --role "<one-line role from the change>" [--tool-tier <tier>]` (see the `oneiroi-seed-naming` capability) to generate the new agent's name and its stub scaffold — the six per-platform template files with hook/frontmatter boilerplate already wired, a stub routing-table edge, and a per-agent slash command. `hypnos` does not invent the name itself.
2. Edit each of the six stub files' instruction body (`Edit`) to replace the placeholder with the role-specific persona prose, following the same frontmatter/instruction-body conventions as the existing agents. The frontmatter/hook block `dreamland oneiroi seed` already wrote is left as-is unless the requested tool tier needs correcting (re-run `dreamland oneiroi seed`'s tier flag is not retroactive; adjust the frontmatter directly if the tier was wrong).
3. Finalize the new agent's placement in all six `janus.*` files' routing tables — `dreamland oneiroi seed` already added a stub delegation-target line; `hypnos` confirms or corrects its tier (deterministic hand-off target vs. broad-routing peer) per the described role, including a hand-off instruction in the new agent's own files directing it to report to Janus by default — unless the described role warrants the broad-routing tier (see the `janus-router-agent` capability's "Iktomi and the agent-roster-maintenance agents can route directly to any agent" requirement), in which case `hypnos` grants it the same direct-to-any-peer capability and states that explicitly in the new agent's own files.
4. Confirm the per-agent slash command `dreamland oneiroi seed` already installed matches the new agent's finalized name, per the `router-slash-commands` capability.
5. When the change instead describes a model revision or a fork of an existing oneiroi rather than a brand-new agent, run `dreamland oneiroi revise --agent <name> --reason "<reason>"` or `dreamland oneiroi fork --agent <parent> --role "<role>"` respectively (see the `oneiroi-seed-naming` capability), then apply steps 2-4 to whatever files that command touched.
6. Report completion (including the new agent's name and role) to Janus.

#### Scenario: New agent gets a file on every platform

- **WHEN** `hypnos` authors a new agent
- **THEN** a template file for that agent exists for all six platforms, each following the existing frontmatter/instruction-body conventions

#### Scenario: New agent is registered in Janus's routing table

- **WHEN** `hypnos` finishes authoring a new agent
- **THEN** all six `janus.*` files' routing tables name the new agent as a delegation target

#### Scenario: New agent gets an explicit slash command

- **WHEN** `hypnos` finishes authoring a new agent named, for example, `amber-falcon`
- **THEN** a `/amber-falcon` slash command (or platform equivalent) exists and routes to it via Janus, per the `router-slash-commands` capability

#### Scenario: Hypnos is triggered by a drafted change, not a raw request

- **WHEN** any platform's `hypnos.*` agent file is installed
- **THEN** its instruction body describes its trigger as a `phantasos`-authored change's proposal/design/tasks, dispatched via `/opsx:apply`
- **AND** it does not describe a direct request routed via Janus, or a `zhougong` report's recommendation, as its trigger

#### Scenario: Hypnos generates the name via dreamland oneiroi seed rather than inventing one

- **WHEN** `hypnos` begins authoring a brand-new agent
- **THEN** its first action is running `dreamland oneiroi seed`, not selecting a name of its own choosing

#### Scenario: Hypnos uses revise/fork for model-revision or fork requests

- **WHEN** the dispatched change describes a model revision or fork of an existing oneiroi rather than a wholly new agent
- **THEN** `hypnos` runs `dreamland oneiroi revise` or `dreamland oneiroi fork` respectively, instead of `dreamland oneiroi seed`
