# agent-lifecycle-management
## Requirements
### Requirement: Zhou Gong generates agent-performance reports from git history and telemetry

`zhougong`'s instructions SHALL direct it to build a report from data already produced by this project's lifecycle hooks — it introduces no new CLI commands or data sources:

- **Git history**: `git log`, filtered/grouped by author (`git config user.name`, which `dreamland coauthor` sets to the acting agent's name per handoff — see the `dev-workflow-hooks` capability), to derive per-agent commit counts and time-between-commits.
- **Token burn**: the `Tokens: input=<n> output=<n> cached=<n> total=<n>` line `dreamland coauthor --trailer` appends to commit messages, aggregated per agent.
- **Turn/handoff timing**: `.dreamland/transition.log`, appended to by `dreamland transition-log` on every turn.

The report SHALL be written as a new markdown file under `.dreamland/reports/` (e.g. `.dreamland/reports/<YYYY-MM-DD>-agent-report.md`) and SHALL include, at minimum: a per-agent breakdown of commit count and aggregate token totals, and a narrative section of tuning suggestions (e.g. an agent whose commits show disproportionate token burn relative to commit count, or unusually long time-between-handoffs).

#### Scenario: Report includes a per-agent breakdown

- **WHEN** `zhougong` generates a report
- **THEN** the report file lists, for every agent that has authored at least one commit, its commit count and aggregate token totals parsed from `Tokens:` trailers

#### Scenario: Report is written as a new file, not an edit to an existing one

- **WHEN** `zhougong` finishes generating a report
- **THEN** a new file is created under `.dreamland/reports/`
- **AND** no existing file is modified (`zhougong` has no `Edit` tool — see the `agent-scaffolding` capability)

### Requirement: Zhou Gong's report may recommend authoring a new agent

When the report identifies a recurring pattern not well-served by the current agent roster (e.g. `iktomi` handling many similar free-form requests that share a common shape), the report SHALL include an explicit "recommended new agent" section describing the gap. `zhougong` never authors the recommended agent itself, and never drafts the change either — it has no `Edit`/agent-authoring/spec-drafting capability — but per its broad-routing capability (see the `janus-router-agent` capability's "Iktomi and the agent-roster-maintenance agents can route directly to any agent" requirement), it hands off directly to `phantasos` when the recommendation names a specific, unambiguous next step, and reports to Janus otherwise. `phantasos` is responsible for turning the recommendation into a drafted `proposal.md`/`design.md`/`tasks.md`; `zhougong`'s role ends at producing the report and (optionally) making that hand-off.

#### Scenario: Report flags a gap with a recommendation

- **WHEN** `zhougong`'s analysis finds a recurring pattern not covered by an existing specialized agent
- **THEN** the report includes a "recommended new agent" section naming the gap and a suggested role
- **AND** `zhougong` hands off directly to `phantasos` when that section names one specific, unambiguous next step, or reports to Janus when it doesn't
- **AND** `zhougong`'s instructions do not name `hypnos` as a direct hand-off target

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

### Requirement: Meng Po archives or deletes agent definitions no longer needed

`mengpo`'s instructions SHALL direct it, given an agent name to retire captured in a `phantasos`-authored change's `proposal.md`/`design.md`/`tasks.md` — dispatched to `mengpo` by Janus via `/opsx:apply` (see the `janus-router-agent` capability's "Janus dispatches agent-roster tasks to Hypnos or Meng Po via /opsx:apply" requirement) — to default to archiving rather than hard-deleting:

- **Archive (default)**: move that agent's template files, across all six platforms, to `internal/scaffold/templates/agents/_archive/<platform>/<name>.*` (preserving content and platform-specific format), remove the agent from all six `janus.*` routing tables and remove its per-agent slash command files, and append an entry to `.dreamland/archived-agents.md` recording the agent name, the date, and the reason for archival.
- **Hard-delete (only on explicit instruction)**: when the change explicitly calls for permanent deletion rather than archival, remove the agent's template files and command files entirely (no `_archive/` copy) and still record the deletion in `.dreamland/archived-agents.md`.

In both modes, `mengpo` reports completion to Janus by default — but per its broad-routing capability (see the `janus-router-agent` capability's "Iktomi and the agent-roster-maintenance agents can route directly to any agent" requirement), it hands off directly to another agent instead when the archive/delete operation reveals a specific, unambiguous follow-up (e.g. a stale routing-table reference on another agent's file).

#### Scenario: Archiving preserves the agent's file content

- **WHEN** `mengpo` archives an agent (default mode, no explicit "delete permanently" instruction)
- **THEN** the agent's template files exist under `internal/scaffold/templates/agents/_archive/<platform>/` for all six platforms, with their original content intact
- **AND** the agent no longer appears in any `janus.*` routing table or has an active per-agent slash command

#### Scenario: Hard-delete removes files without archiving

- **WHEN** a `phantasos`-drafted change explicitly calls for `mengpo` to permanently delete an agent (not archive it)
- **THEN** the agent's template files and command files are removed with no copy placed under `_archive/`
- **AND** the deletion is still recorded in `.dreamland/archived-agents.md`

#### Scenario: Archival is recorded and auditable

- **WHEN** `mengpo` completes an archive or delete operation
- **THEN** `.dreamland/archived-agents.md` gains a new entry naming the agent, the date, the mode (archived/deleted), and the stated reason

#### Scenario: Meng Po is triggered by a drafted change, not a raw agent name

- **WHEN** any platform's `mengpo.*` agent file is installed
- **THEN** its instruction body describes its trigger as a `phantasos`-authored change naming the agent(s) to retire and why, dispatched via `/opsx:apply`

