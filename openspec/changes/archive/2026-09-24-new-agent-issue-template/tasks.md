## 1. Template

- [x] 1.1 Add embedded `internal/scaffold/templates/github/ISSUE_TEMPLATE/new-agent.yml` with the fields in the spec; wire it into scaffold and `dreamland init` (skip if present unless `--template`).
- [x] 1.2 Tests in `internal/scaffold` and `cmd/init_test.go`: file created, valid YAML, required fields, idempotent.

## 2. Command and core

- [x] 2.1 Create `internal/agentissue/agentissue.go` with `Create(Fields) (url string, err error)`, injectable `runGh`, required-field validation before any `gh` call, duplicate-title check.
- [x] 2.2 Create `cmd/agent_issue.go` with `agent-issue` command, flags `--template`, `--create`, `--name`, `--role`, `--rationale`, `--tier`, `--routing`, `--criteria`.
- [x] 2.2b Add the confirm prompt and `--yes` flag to `agent-issue --create`; add an in-memory `previewId` store (single process, expires after 15 min) used by the MCP tool; tests for declined, `--yes`, preview-then-confirm and unknown `previewId`.
- [x] 2.3 Tests: missing flag, gh missing, duplicate, success body contains every section.

## 3. MCP and phantasos

- [x] 3.1 Add `zhougong_new_agent_issue` to `cmd/mcp_zhougong.go` (requires `zhougong-metrics-dashboard` task 2.1) two-phase (`confirm`, `previewId`) delegating to `agentissue.Create`; add to zhougong `tools`; parity test.
- [x] 3.1b Add to zhougong's instructions: on a recommended new agent, call the tool with `confirm=false`, show the preview to the user, and only call again with `confirm=true` after explicit approval.
- [x] 3.2 Add the issue-intake instruction to `.claude/agents/phantasos.md` and its template; scaffold test.
- [x] 3.3 Document calling `dreamland agent-issue` from a lifecycle hook in the README.
