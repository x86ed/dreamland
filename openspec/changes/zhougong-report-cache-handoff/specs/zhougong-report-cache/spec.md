## ADDED Requirements

### Requirement: Collected datasets are cached per branch

`zhougong_collect` SHALL write one JSON file per branch to `.dreamland/cache/zhougong/<branch-slug>.json` containing `branch`, `headSha`, `collectedAt` (RFC3339), `schemaVersion` and `runs`. The directory SHALL be gitignored.

#### Scenario: Cache written

- **WHEN** `zhougong_collect` succeeds for branch `feat/x`
- **THEN** `.dreamland/cache/zhougong/feat-x.json` exists with the branch's current HEAD sha

#### Scenario: Cache is gitignored

- **WHEN** `dreamland init` runs
- **THEN** `.gitignore` contains `.dreamland/cache/`

### Requirement: Staleness is determined by HEAD sha and schema version

A cache entry SHALL be stale when the branch's current HEAD sha differs from `headSha` or `schemaVersion` differs from the current constant. `zhougong_collect` with `refresh=false` SHALL recollect only stale or missing entries.

#### Scenario: New commit makes entry stale

- **WHEN** a commit is added to the branch after collection
- **THEN** `zhougong_snapshot` returns that branch with `stale: true`

#### Scenario: Fresh entry not recollected

- **WHEN** `zhougong_collect(refresh=false)` runs and HEAD is unchanged
- **THEN** the file's `collectedAt` is unchanged

### Requirement: Snapshot tool returns cached source data

`zhougong_snapshot(branches, baseline)` SHALL accept any number of branches (an empty list means every cached branch) and SHALL return, for each branch, its `headSha`, `collectedAt`, `stale`, per-branch summary (runs, tokens per agent, calls per agent, flow path) and per-run rows, read from the cache without recomputation. It SHALL also return a `comparison` object with each metric per branch and the absolute and percent delta of each branch versus `baseline` (default: first branch), using the same function the dashboard's compare view uses. A branch with no cache entry SHALL be returned as `missing: true` and excluded from `comparison`.

#### Scenario: Multi-branch comparison

- **WHEN** the snapshot is requested for branches A, B and C with baseline A
- **THEN** `comparison` has values for all three and deltas for B and C versus A, identical to the dashboard's `/api/compare` output

#### Scenario: Empty list means all

- **WHEN** `branches` is empty and three branches are cached
- **THEN** all three are returned

#### Scenario: Missing branch

- **WHEN** a requested branch has never been collected
- **THEN** it appears with `missing: true` and no numbers

### Requirement: Zhougong interprets only from the snapshot

`zhougong`'s instructions SHALL require calling `zhougong_snapshot` before answering the first question about a report or a branch/feature diff, answering from the returned numbers only, and disclosing `collectedAt` and any stale or missing branches.

#### Scenario: Instruction present

- **WHEN** `.claude/agents/zhougong.md` is scaffolded
- **THEN** its body contains the snapshot-first requirement
