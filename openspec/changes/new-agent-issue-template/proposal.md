## Why

zhougong recommends new agents in reports, but the recommendation lives in a local markdown file. A structured GitHub issue gives humans a place to review and approve it and gives `phantasos` a consistent input to draft an OpenSpec change from.

## What Changes

- **Issue form** `.github/ISSUE_TEMPLATE/new-agent.yml` with fields: agent name (oneiroi-style), role (one line), rationale and evidence (report link, metrics), tool tier (`router`, `read-dispatch-only`, `full-edit`, `write-only-no-edit`), routing position (receives from / hands off to), and acceptance criteria. Label `new-agent`.
- **Command `dreamland agent-issue`**: `--template` (write or refresh the template file only, idempotent) and `--create --name --role --rationale --tier --routing --criteria` (create a filled issue via `gh issue create --template`-equivalent body and the `new-agent` label). Fails with a clear error if `gh` is missing or unauthenticated.
- **MCP tool `zhougong_new_agent_issue`** on `dreamland-zhougong`, delegating to the same core function as the command.
- **Lifecycle hook**: `dreamland init` writes the template file; a `Stop` hook is NOT added. The command is callable from a hook by users who want it (documented), and via MCP.
- **Phantasos intake**: phantasos's instructions state that given an issue number it reads it with `gh issue view <n> --json title,body` and drafts the change from the fields.
- Scoped independent of the dashboard changes; the MCP tool depends on `zhougong-metrics-dashboard`'s server.

## Capabilities

### New Capabilities

- `new-agent-issue-template`: template file, command, MCP tool, phantasos intake.

### Modified Capabilities

(none)

## Impact

- New: `.github/ISSUE_TEMPLATE/new-agent.yml` (and scaffold template), `cmd/agent_issue.go`, tests; `cmd/mcp_zhougong.go` tool; `.claude/agents/phantasos.md` and template; `dreamland init`.
- Assumptions to confirm: "project's GH page" means the repo's issue templates plus issue creation through `gh`; field set above.
