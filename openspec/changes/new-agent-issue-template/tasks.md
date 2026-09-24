## 1. Template

- [ ] 1.1 Add embedded `internal/scaffold/templates/github/ISSUE_TEMPLATE/new-agent.yml` with the fields in the spec; wire it into scaffold and `dreamland init` (skip if present unless `--template`).
- [ ] 1.2 Tests in `internal/scaffold` and `cmd/init_test.go`: file created, valid YAML, required fields, idempotent.

## 2. Command and core

- [ ] 2.1 Create `internal/agentissue/agentissue.go` with `Create(Fields) (url string, err error)`, injectable `runGh`, required-field validation before any `gh` call, duplicate-title check.
- [ ] 2.2 Create `cmd/agent_issue.go` with `agent-issue` command, flags `--template`, `--create`, `--name`, `--role`, `--rationale`, `--tier`, `--routing`, `--criteria`.
- [ ] 2.3 Tests: missing flag, gh missing, duplicate, success body contains every section.

## 3. MCP and phantasos

- [ ] 3.1 Add `zhougong_new_agent_issue` to `cmd/mcp_zhougong.go` (requires `zhougong-metrics-dashboard` task 2.1) delegating to `agentissue.Create`; add to zhougong `tools`; parity test.
- [ ] 3.2 Add the issue-intake instruction to `.claude/agents/phantasos.md` and its template; scaffold test.
- [ ] 3.3 Document calling `dreamland agent-issue` from a lifecycle hook in the README.
