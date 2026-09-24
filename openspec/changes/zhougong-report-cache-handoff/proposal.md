## Why

Datasets pulled from git are expensive to recompute, and when the user starts asking zhougong about a report or diff, the agent must interpret the actual numbers, not recall or invent them. This change adds a cache and a snapshot handoff so zhougong is given the cached source data as soon as questions begin.

## What Changes

- **Cache** at `.dreamland/cache/zhougong/<branch-slug>.json` (gitignored, added to `.gitignore` and `cmd/init.go` like `workflow-positions.json`), written by `zhougong_collect` and read by the dashboard. Each file records `branch`, `headSha`, `collectedAt`, `schemaVersion` and the runs.
- **Staleness**: an entry is stale when the branch's current HEAD sha differs from the stored `headSha`, or `schemaVersion` differs. Stale entries are recollected on the next `zhougong_collect`, or reported as stale by reads; no TTL.
- **New MCP tool `zhougong_snapshot(branches []string)`** on `dreamland-zhougong` returning the cached numbers (per-branch summaries and per-run rows, plus `headSha`/`collectedAt`/`stale` flags) as structured JSON.
- **Handoff**: zhougong's instructions require calling `zhougong_snapshot` before answering the first question about a report or diff and answering only from the returned JSON, quoting `collectedAt`, and stating when data is stale. A `UserPromptSubmit` hook is not used; this is instruction-level plus the tool, and the cached JSON is the single source.
- **Cross-branch**: `zhougong_snapshot` accepts many branches (or `[]` meaning every cached branch) and also returns a precomputed `comparison` (metrics per branch, deltas vs `baseline`) so zhougong interprets multi-branch diffs from the same numbers the dashboard shows. Report generation also writes the cross-branch table from this snapshot.
- Depends on `zhougong-metrics-dashboard` (same MCP server and dataset types).

## Capabilities

### New Capabilities

- `zhougong-report-cache`: cache layout, staleness rule, and snapshot tool.

### Modified Capabilities

- `agent-lifecycle-management`: zhougong must interpret from the snapshot.

## Impact

- New `internal/zhougongdata/cache.go` and tests; `cmd/mcp_zhougong.go` gains `zhougong_snapshot`; `.gitignore`, `cmd/init.go`; zhougong agent file and template.
- Assumption to confirm: no TTL, sha-based staleness only.
