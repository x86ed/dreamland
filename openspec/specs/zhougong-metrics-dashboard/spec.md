# zhougong-metrics-dashboard Specification

## Purpose
TBD - created by archiving change zhougong-metrics-dashboard. Update Purpose after archive.
## Requirements
### Requirement: Zhougong-only MCP server exposes collect and dashboard tools

`dreamland mcp-zhougong` SHALL start a stdio MCP server named `dreamland-zhougong` exposing `zhougong_collect`, `zhougong_dashboard_start` and `zhougong_dashboard_stop`, plus any tool added to this server by another capability (`zhougong_snapshot` in `zhougong-report-cache`, `zhougong_new_agent_issue` in `new-agent-issue-template`); the server's complete tool set is the union across capabilities and is pinned by its Go tests. The server SHALL be declared only in `zhougong`'s agent frontmatter and SHALL NOT appear in the project `.mcp.json`.

#### Scenario: Only zhougong can call the tools

- **WHEN** any agent other than `zhougong` is scaffolded
- **THEN** its frontmatter has no `mcpServers` entry for `dreamland-zhougong` and no `mcp__dreamland-zhougong__*` entry in `tools`

#### Scenario: Server lists its tools

- **WHEN** an in-process MCP client lists tools on `newZhougongMCPServer(repoRoot)`
- **THEN** the returned names include `zhougong_collect`, `zhougong_dashboard_start` and `zhougong_dashboard_stop`, and every tool defined for this server by other capabilities, and no others

### Requirement: Collect tool derives per-run datasets from git

`zhougong_collect` SHALL, for each requested branch, parse `git log` commits and `Tokens:` trailers into runs. Per run it SHALL record agent, commit count, input/output/cached/total tokens (deltas between consecutive cumulative trailers, never negative), lines added and removed in code files (excluding `openspec/**` and `*.md`), and token-to-code ratio (output tokens divided by lines added plus removed; `null` when that sum is 0).

#### Scenario: Token delta

- **WHEN** two consecutive agent commits carry `total=1000` then `total=1600`
- **THEN** the second commit is attributed 600 total tokens

#### Scenario: Zero code lines

- **WHEN** a run changed only markdown files
- **THEN** its token-to-code ratio is `null` and the dashboard shows "n/a"

#### Scenario: Unknown branch

- **WHEN** a requested branch does not exist
- **THEN** the tool returns `IsError: true` naming the branch and writes nothing to the cache

### Requirement: Merged features are preserved as archived run records

`dreamland zhougong-archive --branch <b>` SHALL write `.dreamland/runs/<change-slug>.json` (committed, not gitignored) containing the branch's runs, `sourceBranch`, `mergedAt` and `schemaVersion`. The dashboard overview and compare view SHALL include archived records as selectable entries marked `source: archived`. `scripts/pre-merge-check.sh` SHALL invoke the command.

#### Scenario: Archived feature compared with a live branch

- **WHEN** an archived record `foo` and a live branch `bar` are selected
- **THEN** the compare view shows both columns with `foo` labelled archived

#### Scenario: Squash-merged branch remains analysable

- **WHEN** a branch was archived and then squash-merged and deleted
- **THEN** its metrics are still available from `.dreamland/runs/`

### Requirement: Runs are branch-scoped and unattributed commits are surfaced

A run SHALL never span branches. Commits whose author is not a known agent SHALL be reported in a per-branch `unattributed` bucket and the dashboard SHALL display a note that attribution is commit-author based.

#### Scenario: Unknown author

- **WHEN** a commit is authored by a non-agent user
- **THEN** it counts toward `unattributed`, not any agent

### Requirement: Analysis is capped at eight branches

Every multi-branch analysis surface SHALL reject more than 8 branches/records with an error naming the limit.

#### Scenario: Cap enforced

- **WHEN** 9 branches are requested
- **THEN** the request fails naming the limit of 8

### Requirement: Dashboard serves the metric views on localhost only

`zhougong_dashboard_start` SHALL serve an embedded static site on `127.0.0.1` and return its URL. The site SHALL show: calls per agent, tokens per agent, token-to-code ratio per run, number of runs per branch, and the typical flow path (most frequent ordered agent sequence and an agent-to-agent transition count table). Calling start while running SHALL return the existing URL.

#### Scenario: Loopback binding

- **WHEN** the dashboard starts
- **THEN** the listener address is on `127.0.0.1` and a request from a non-loopback address is not accepted

#### Scenario: Stop releases the port

- **WHEN** `zhougong_dashboard_stop` is called
- **THEN** the port is closed and a subsequent request fails to connect

### Requirement: Dashboard compares multiple branches or features side by side

The dashboard SHALL provide an all-branches overview of every collected branch, and a compare view for 2 to 8 selected branches or OpenSpec change slugs showing each metric in one column per branch, a selectable baseline (default: first selected), the absolute and percent delta of every other branch versus the baseline, grouped charts across all selected branches, and a flow-path comparison. `/api/compare` SHALL accept `branches=a,b,c&baseline=a`.

#### Scenario: Delta shown

- **WHEN** baseline A has 10 runs, B has 15 and C has 5
- **THEN** the compare view shows runs 10, 15, 5 with B at +5 / +50% and C at -5 / -50%

#### Scenario: Selection limits

- **WHEN** fewer than 2 or more than 8 branches are passed to `/api/compare`
- **THEN** it responds 400 naming the allowed range

#### Scenario: Overview lists all collected branches

- **WHEN** three branches are in the cache
- **THEN** `/api/summary` and the landing view list all three, each sortable by any metric

#### Scenario: Feature slug resolves to a branch

- **WHEN** the user selects change slug `foo` and a branch containing that slug exists
- **THEN** the compare view uses that branch's dataset; if none exists it shows an explicit "no data" column

