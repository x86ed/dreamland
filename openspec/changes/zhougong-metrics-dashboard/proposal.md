## Why

`zhougong` produces a static markdown report from `git log` and `Tokens:` trailers, but humans cannot compare inference performance across branches or features, see how agents chain together, or drill into a run. This change adds a localhost dashboard, launched and fed by MCP tools only `zhougong` can use, so humans can evaluate and diff runs visually.

## What Changes

- **New MCP server `dreamland-zhougong`** (`dreamland mcp-zhougong`, stdio), built with the same `github.com/modelcontextprotocol/go-sdk/mcp` pattern as `cmd/mcp_serve.go`. Tools:
  - `zhougong_collect(branches []string, refresh bool)` pulls per-run datasets from git for the named branches and writes them to the cache (cache internals are in `zhougong-report-cache-handoff`).
  - `zhougong_dashboard_start(port int)` / `zhougong_dashboard_stop()` start and stop the localhost microsite (binds `127.0.0.1` only; default port chosen by the OS and returned).
- **Access restriction**: the server is declared inline in `.claude/agents/zhougong.md` frontmatter (`mcpServers`) and is NOT registered in the project `.mcp.json`, so no other agent has the tools. The `tools:` allowlist names the three `mcp__dreamland-zhougong__*` tools.
- **Dataset (`internal/zhougongdata`)**: a run is one branch-scoped sequence of agent commits between handoffs. Per run: agent, commit count, tokens (input/output/cached/total, diffed between consecutive cumulative `Tokens:` trailers), lines added/removed (from `git diff --numstat`, excluding `openspec/`, docs and `*.md`), token-to-code ratio = output tokens / lines added+removed in code files.
- **Dashboard views (`internal/zhougongdash`, embedded static HTML/JS, no CDN)**: agent usage (call count per agent), tokens per agent, token-to-code ratio per run, runs per branch, typical flow path (most frequent ordered agent sequence plus a transition-count table).
- **Multi-branch compare view**: pick 2 to 8 branches/features (feature = OpenSpec change slug resolved to its branch). One column per selected branch in a metric table, a designated baseline column (default: first selected) with absolute and percent delta for every other column, grouped bar charts across all selected branches, and a flow-path comparison (per-branch typical path, plus a transition-count matrix with one column per branch). The default landing view is the all-branches overview (every collected branch, sortable by any metric), so single-branch is the N=1 case.
- **Report**: the markdown report under `.dreamland/reports/` gains a cross-branch comparison table (one row per metric, one column per branch, deltas vs baseline) whenever more than one branch is collected.
- Modifies the `agent-lifecycle-management` requirement that says zhougong introduces no new data sources or commands: MCP tools over the same git/trailer data are now allowed.

## Capabilities

### New Capabilities

- `zhougong-metrics-dashboard`: MCP tools, dataset definition, localhost dashboard and compare view.

### Modified Capabilities

- `agent-lifecycle-management`: zhougong may use its dedicated MCP tools; the "no new CLI commands" wording is narrowed.

## Impact

- New: `cmd/mcp_zhougong.go`, `internal/zhougongdata/`, `internal/zhougongdash/` (with embedded assets), tests for each.
- `.claude/agents/zhougong.md` and its scaffold template under `internal/scaffold/templates` (frontmatter `mcpServers`, `tools`).
- Assumptions to confirm with the user: run definition, token-to-code ratio definition, excluded file globs, 127.0.0.1-only binding.
