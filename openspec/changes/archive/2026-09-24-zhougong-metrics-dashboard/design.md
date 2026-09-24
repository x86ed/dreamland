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

## Confirmed scope decisions

- Merged features are comparable: squash merges erase attribution, so durable archived run records are written at pre-merge time (`.dreamland/runs/`, committed). Cache entries stay local and disposable.
- A run is branch-scoped.
- Eight is the upper limit on any analysis.
- Known limitations (attribution gap, cumulative trailers, access enforcement) are scoped explicitly in proposal.md; the harness-level identity gap is not fixed here.

## Verification

### Access spike and real dispatch (task 4.4), 2026-09-23, Claude Code 2.281

Inline `mcpServers` in subagent frontmatter DOES scope an MCP server to that subagent. Result of a real `claude -p` dispatch in this repo with the actual `.claude/agents/zhougong.md`:

- `zhougong` connected `dreamland-zhougong` (debug log: `[Agent: zhougong] Connected to MCP server 'dreamland-zhougong' with 3 tools`), saw exactly `mcp__dreamland-zhougong__{zhougong_collect,zhougong_dashboard_start,zhougong_dashboard_stop}` and successfully called all three (collect on `38-zhougong-hook` and `main`, start returned a `127.0.0.1` URL, stop returned `{}`).
- A `morpheus` dispatch in the same session reported no tool containing `zhougong` (Read, Edit, Write, Bash only).
- The main (Janus) session had no `zhougong` tools either.

The list form (`mcpServers: [ {name: {type, command, args}} ]`) is what the repo uses. Caveats found while spiking:

- Claude Code skips frontmatter MCP servers for agents from an untrusted project folder (`Skipping frontmatter MCP servers for agent ...: the folder ... is not trusted`). A fresh clone must accept the workspace trust dialog once, otherwise zhougong silently gets no MCP tools.
- The `dreamland` on PATH must include `mcp-zhougong` (the server is `command: dreamland`); the dispatch above used a freshly built binary prefixed on PATH because the installed one predates this change.
- Enforcement is client-side scoping by frontmatter, as scoped in proposal.md; there is no server-side identity check.
- Only the claude-code template carries `mcpServers`; other platform templates have no equivalent and do not reference the server.

### Trailer and attribution spot-check (task 3b.5), 2026-09-23

Parsed this repo's own branch `38-zhougong-hook` (`main..38-zhougong-hook`, a snapshot at HEAD when run) with the real `dreamland zhougong-archive` and compared against `git log` by hand:

- Commit counts per agent match `git log --format=%an` exactly (janus 420, phantasos 86, morpheus 49, baku 22, phobetor 20, iktomi 13, nyx 11, zhougong 1 = 622).
- Unattributed = 27 = the non-agent authors in git log (`Adam Siegel` 15, `x86ed` 9, `Claude Code` 3).
- Untracked = 52 agent commits without a `Tokens:` line, identical to an independent script count.
- Sum of token deltas = 91,222,610, identical to an independent recomputation (two resets detected, counted as raw trailer values).

Discrepancy found and fixed: the parser initially counted the `AI-InputTokens:` trailer lines as malformed `Tokens:` trailers (`skipped=1`); the check now requires a line starting with `Tokens:` (regression test added). Observations, not fixed: (1) the first tracked commit on a branch has no baseline so it is attributed 0 tokens; (2) because only `openspec/**` and `*.md` are excluded (per spec), checkpoint commits that touch `.dreamland/*.json`/`transition.log` inflate code-line counts and depress token-to-code for janus runs (31,125 lines changed on this branch); (3) attribution is by commit author, so the harness-level identity gap (out of scope) still applies. Also: a `main` comparison shows 0 runs because main's history is squashed and mostly non-agent authors.
