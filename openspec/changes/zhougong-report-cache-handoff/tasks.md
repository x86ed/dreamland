## 1. Cache

- [ ] 1.1 Create `internal/zhougongdata/cache.go`: `SchemaVersion` const, `Entry` struct, `Write(repoRoot, Entry)` (temp file plus rename), `Read(repoRoot, branch)`, `IsStale(entry, currentSha)`, branch-slug function per design decision 5.
- [ ] 1.2 Tests in `internal/zhougongdata/cache_test.go`: round trip, stale on sha change, stale on schema change, missing file, slug collision.
- [ ] 1.3 Add `.dreamland/cache/` to `.gitignore` and to the gitignore list in `cmd/init.go` / `internal/scaffold/gitignore.go`; test it.

## 2. Tools

- [ ] 2.1 Make `zhougong_collect` write cache entries and skip fresh ones when `refresh=false` (in `cmd/mcp_zhougong.go`).
- [ ] 2.2 Add `zhougong_snapshot` to `cmd/mcp_zhougong.go` taking `branches` (empty means all cached) and `baseline`, returning per-branch `headSha`, `collectedAt`, `stale`, `missing`, summary and runs, and a `comparison` computed by `zhougongdata.Compare` (from the dashboard change, task 1.3); add it to zhougong's `tools` list.
- [ ] 2.3 Point the dashboard `/api/*` routes at the cache reader; tests in `cmd/mcp_zhougong_test.go`.

## 3. Instructions

- [ ] 3.1 Add the snapshot-first requirement to `.claude/agents/zhougong.md` and its scaffold template; scaffold test asserts the text.
