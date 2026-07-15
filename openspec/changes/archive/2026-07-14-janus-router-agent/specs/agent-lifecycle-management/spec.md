## ADDED Requirements

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

When the report identifies a recurring pattern not well-served by the current agent roster (e.g. `iktomi` handling many similar free-form requests that share a common shape), the report SHALL include an explicit "recommended new agent" section describing the gap. `zhougong` never authors the recommended agent itself — it has no `Edit`/agent-authoring capability — but per its broad-routing capability (see the `janus-router-agent` capability's "Iktomi and the agent-roster-maintenance agents can route directly to any agent" requirement), it hands off directly to `hypnos` when the recommendation names a specific, unambiguous next step, and reports to Janus otherwise.

#### Scenario: Report flags a gap with a recommendation

- **WHEN** `zhougong`'s analysis finds a recurring pattern not covered by an existing specialized agent
- **THEN** the report includes a "recommended new agent" section naming the gap and a suggested role
- **AND** `zhougong` hands off directly to `hypnos` when that section names one specific, unambiguous next step, or reports to Janus when it doesn't

### Requirement: Hypnos authors new agent definitions across every platform template

`hypnos`'s instructions SHALL direct it, given a role description (from a direct request routed via Janus, or from a `zhougong` report's recommendation), to author a complete new agent definition:

1. Create the new agent's template file for all six platforms (Claude Code `.md`, Codex `.toml`, Cursor `.mdc`, Kiro `.md`, Antigravity `SKILL.md`, GitHub Copilot `.agent.md`), following the same frontmatter/instruction-body conventions as the existing ten agents.
2. Decide the new agent's tool tier (router-excluded, full-edit, or write-only — see the `agent-scaffolding` capability's tool-binding matrix) based on its described role, and apply it consistently across all six platform files.
3. Add the new agent as a delegation target in all six `janus.*` files' routing tables, including a hand-off instruction in the new agent's own files directing it to report to Janus by default — unless the described role warrants the broad-routing tier (see the `janus-router-agent` capability's "Iktomi and the agent-roster-maintenance agents can route directly to any agent" requirement), in which case `hypnos` grants it the same direct-to-any-peer capability and states that explicitly in the new agent's own files.
4. Add the new agent's explicit per-agent slash command (`/<new-agent-name>`, or platform equivalent) per the `router-slash-commands` capability.
5. Report completion (including the new agent's name and role) to Janus.

#### Scenario: New agent gets a file on every platform

- **WHEN** `hypnos` authors a new agent
- **THEN** a template file for that agent exists for all six platforms, each following the existing frontmatter/instruction-body conventions

#### Scenario: New agent is registered in Janus's routing table

- **WHEN** `hypnos` finishes authoring a new agent
- **THEN** all six `janus.*` files' routing tables name the new agent as a delegation target

#### Scenario: New agent gets an explicit slash command

- **WHEN** `hypnos` finishes authoring a new agent named, for example, `oneiros`
- **THEN** a `/oneiros` slash command (or platform equivalent) exists and routes to it via Janus, per the `router-slash-commands` capability

### Requirement: Meng Po archives or deletes agent definitions no longer needed

`mengpo`'s instructions SHALL direct it, given an agent name to retire, to default to archiving rather than hard-deleting:

- **Archive (default)**: move that agent's template files, across all six platforms, to `internal/scaffold/templates/agents/_archive/<platform>/<name>.*` (preserving content and platform-specific format), remove the agent from all six `janus.*` routing tables and remove its per-agent slash command files, and append an entry to `.dreamland/archived-agents.md` recording the agent name, the date, and the reason for archival.
- **Hard-delete (only on explicit instruction)**: when the request explicitly asks for permanent deletion rather than archival, remove the agent's template files and command files entirely (no `_archive/` copy) and still record the deletion in `.dreamland/archived-agents.md`.

In both modes, `mengpo` reports completion to Janus by default — but per its broad-routing capability (see the `janus-router-agent` capability's "Iktomi and the agent-roster-maintenance agents can route directly to any agent" requirement), it hands off directly to another agent instead when the archive/delete operation reveals a specific, unambiguous follow-up (e.g. a stale routing-table reference on another agent's file).

#### Scenario: Archiving preserves the agent's file content

- **WHEN** `mengpo` archives an agent (default mode, no explicit "delete permanently" instruction)
- **THEN** the agent's template files exist under `internal/scaffold/templates/agents/_archive/<platform>/` for all six platforms, with their original content intact
- **AND** the agent no longer appears in any `janus.*` routing table or has an active per-agent slash command

#### Scenario: Hard-delete removes files without archiving

- **WHEN** a request explicitly asks `mengpo` to permanently delete an agent (not archive it)
- **THEN** the agent's template files and command files are removed with no copy placed under `_archive/`
- **AND** the deletion is still recorded in `.dreamland/archived-agents.md`

#### Scenario: Archival is recorded and auditable

- **WHEN** `mengpo` completes an archive or delete operation
- **THEN** `.dreamland/archived-agents.md` gains a new entry naming the agent, the date, the mode (archived/deleted), and the stated reason
