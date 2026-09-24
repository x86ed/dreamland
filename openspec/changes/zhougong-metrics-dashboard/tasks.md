## 1. Dataset

- [ ] 1.1 Create `internal/zhougongdata/parse.go` with `ParseBranch(repoRoot, branch string) ([]Run, error)` using `git log --format` and `--numstat`; parse `Tokens: input=<n> output=<n> cached=<n> total=<n>` trailers; return an error naming the branch if it does not exist.
- [ ] 1.2 Implement token deltas (negative delta means baseline reset), code-line counting with the `openspec/**` and `*.md` exclusions, and `TokenToCode *float64` (nil when lines is 0).
- [ ] 1.3 Implement `FlowPath(runs)` and `Transitions(runs)` per design decision 3, and `Compare(datasets []Dataset, baseline string) CompareResult` producing per-metric values per branch plus absolute/percent deltas versus baseline.
- [ ] 1.4 Table-driven tests in `internal/zhougongdata/parse_test.go` using a temp git repo covering each scenario in the spec.

## 2. MCP server

- [ ] 2.1 Create `cmd/mcp_zhougong.go` with `mcp-zhougong` Cobra command and `newZhougongMCPServer(repoRoot)`, mirroring `cmd/mcp_serve.go`; tools `zhougong_collect`, `zhougong_dashboard_start`, `zhougong_dashboard_stop`.
- [ ] 2.2 Tests in `cmd/mcp_zhougong_test.go` with an in-process client: tool list, unknown branch error, start/stop idempotence.

## 3. Dashboard

- [ ] 3.1 Create `internal/zhougongdash` with an `embed.FS` of `index.html`, `app.js`, `style.css` and JSON routes `/api/summary`, `/api/compare?a=&b=`.
- [ ] 3.2 Views: all-branches overview (sortable), agent calls, tokens per agent, token-to-code per run, runs per branch, flow path; multi-branch compare view (2-8 branches, baseline selector, delta columns, grouped charts, per-branch flow path and transition matrix). Show totals and per-run averages.
- [ ] 3.4 `/api/compare?branches=a,b,c&baseline=a` with 400 outside 2-8; tests for delta math with three branches and the limits.
- [ ] 3.3 Bind `127.0.0.1:<port>`; tests for loopback binding, stop releasing the port, and compare delta math.

## 3b. Archive records, attribution, and verification

- [ ] 3b.1 Create `dreamland zhougong-archive --branch <b>` (`cmd/zhougong_archive.go`) writing `.dreamland/runs/<change-slug>.json` using `zhougongdata.ParseBranch`; call it from `scripts/pre-merge-check.sh`; tests.
- [ ] 3b.2 Load `.dreamland/runs/*.json` into overview/compare as `source: archived`; test mixing archived and live in one compare.
- [ ] 3b.3 Add the `unattributed` bucket and `untracked` commit list to the dataset; dashboard banner text; tests with a non-agent author and a trailer-less commit.
- [ ] 3b.4 Fixture tests for token deltas: monotonic, reset (negative delta), missing trailer.
- [ ] 3b.5 Spot-check: run the parser on one real branch of this repo, compare against `git log` by hand, and record findings and any discrepancy in design.md "Verification".
- [ ] 3b.6 Enforce the 8-branch cap in one shared helper used by `/api/compare`, the report table and (in the cache change) `zhougong_snapshot`.

## 4. Access wiring

- [ ] 4.1 Update `.claude/agents/zhougong.md` and its scaffold template: `mcpServers` inline entry running `dreamland mcp-zhougong`, and the three tools in `tools`.
- [ ] 4.2 Scaffold test asserting no other agent template references `dreamland-zhougong`, and `.mcp.json` does not.
- [ ] 4.4 Manual verification in a real dispatch: a non-zhougong agent cannot see `mcp__dreamland-zhougong__*`; record the result in design.md "Verification".
- [ ] 4.3 Update the zhougong instructions to describe the tools and note the spec's requirement change.
