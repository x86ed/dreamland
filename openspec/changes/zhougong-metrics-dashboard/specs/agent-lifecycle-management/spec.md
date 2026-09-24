## MODIFIED Requirements

### Requirement: Zhou Gong generates agent-performance reports from git history and telemetry

`zhougong`'s instructions SHALL direct it to build a report from data already produced by this project's lifecycle hooks (git history and `Tokens:` trailers, `.dreamland/transition.log`), and MAY obtain and visualize that data through its dedicated `dreamland-zhougong` MCP tools (see the `zhougong-metrics-dashboard` capability). It introduces no other CLI commands or data sources. The report SHALL still be written as a new markdown file under `.dreamland/reports/` including a per-agent breakdown of commit count and aggregate token totals and a narrative section of tuning suggestions. When more than one branch is analysed, the report SHALL also include a cross-branch comparison table (one row per metric, one column per branch, deltas versus a named baseline branch).

#### Scenario: Cross-branch table in the report

- **WHEN** zhougong reports on branches A, B and C
- **THEN** the report contains a comparison table with columns A, B, C and delta columns for B and C versus A

#### Scenario: Report includes a per-agent breakdown

- **WHEN** `zhougong` generates a report
- **THEN** the report file lists, for every agent that has authored at least one commit, its commit count and aggregate token totals parsed from `Tokens:` trailers

#### Scenario: Zhougong frontmatter grants the MCP tools

- **WHEN** `.claude/agents/zhougong.md` is scaffolded
- **THEN** its `tools` includes the three `mcp__dreamland-zhougong__*` tools and its `mcpServers` declares `dreamland-zhougong`
