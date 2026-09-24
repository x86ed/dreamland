## Context

`cmd/mcp_serve.go` already shows the go-sdk stdio MCP pattern with a testable `newXMCPServer(repoRoot)` seam. `cmd/hypnosserve.go` shows a localhost server pattern. zhougong currently has only Read, Write, Bash.

## Decisions

1. **Separate server, not extending `dreamland-oneiroi`.** Access control is per server: an inline `mcpServers` entry in zhougong's frontmatter scopes the server to that subagent, while a `.mcp.json` entry would expose it to all agents. Trade-off: an extra subcommand and process. Alternative rejected: one shared server with server-side identity checks, because agent identity attribution has known gaps (see session-agent-identity).
2. **Data source is git only** (commit author = agent, `Tokens:` trailers, numstat). Trailers are cumulative per session, so per-turn tokens are deltas; a negative delta (session reset) is treated as a new baseline, counting the raw trailer value. `transition.log` has no agent field, so it is not used for flow.
3. **Flow path** = per-branch ordered list of agent authors collapsed for consecutive duplicates; "typical" = most frequent sequence across runs, ties broken by earliest occurrence.
4. **Static embedded dashboard** reading `/api/*` JSON from the Go server; no build step, no CDN, so it works offline. Port 0 by default.
5. **Token-to-code ratio** excludes `openspec/**` and `*.md` so spec-writing agents do not distort it; the exclusion list is a constant in `internal/zhougongdata` and shown on the dashboard.

6. **N-way comparison with a baseline** instead of pairwise diff: a table with one column per branch scales to 8 and pairwise diffs can be derived by choosing any branch as baseline. The cap of 8 keeps charts legible. Metrics are normalised per run (per-run averages alongside totals) so branches with different run counts compare fairly.

## Risks

- Squash merges lose per-agent attribution on main; runs are computed on feature branches.
- Trailer format drift breaks parsing; the parser skips unparseable commits and reports the count.
