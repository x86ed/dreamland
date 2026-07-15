## 1. Janus routing table (6 platforms)

- [x] 1.1 `internal/scaffold/templates/agents/claude-code/janus.md`: edit the routing table — extend the `phantasos` bullet to cover agent-roster specs, replace the standalone `hypnos`/`mengpo` bullets with a `/opsx:apply` dispatch bullet alongside `nyx`/`morpheus`.
- [x] 1.2 `internal/scaffold/templates/agents/cursor/janus.mdc`: same routing-table edit.
- [x] 1.3 `internal/scaffold/templates/agents/codex/janus.toml`: same routing-table edit (plain-text list, no backticks).
- [x] 1.4 `internal/scaffold/templates/agents/kiro/janus.md`: same routing-table edit.
- [x] 1.5 `internal/scaffold/templates/agents/github-copilot/janus.agent.md`: same routing-table edit. No frontmatter change — `agents:` already lists `hypnos`/`mengpo`.
- [x] 1.6 `internal/scaffold/templates/agents/antigravity/janus/SKILL.md`: same routing-table edit.

## 2. Zhou Gong hand-off target (6 platforms)

- [x] 2.1 `internal/scaffold/templates/agents/claude-code/zhougong.md`: step 4 — hand off to `phantasos` instead of `hypnos`, note phantasos drafts the change that reaches `hypnos` via the normal task flow.
- [x] 2.2 `internal/scaffold/templates/agents/cursor/zhougong.mdc`: same hand-off edit.
- [x] 2.3 `internal/scaffold/templates/agents/codex/zhougong.toml`: same hand-off edit.
- [x] 2.4 `internal/scaffold/templates/agents/kiro/zhougong.md`: same hand-off edit.
- [x] 2.5 `internal/scaffold/templates/agents/github-copilot/zhougong.agent.md`: same hand-off edit. No frontmatter change — `agents:` already lists `phantasos`.
- [x] 2.6 `internal/scaffold/templates/agents/antigravity/zhougong/SKILL.md`: same hand-off edit.

## 3. Hypnos trigger (6 platforms)

- [x] 3.1 `internal/scaffold/templates/agents/claude-code/hypnos.md`: change trigger from "a direct request routed via Janus, or a `zhougong` report's recommendation" to "a role description in a `phantasos`-authored change's proposal/design/tasks, dispatched via `/opsx:apply`."
- [x] 3.2 `internal/scaffold/templates/agents/cursor/hypnos.mdc`: same trigger edit.
- [x] 3.3 `internal/scaffold/templates/agents/codex/hypnos.toml`: same trigger edit.
- [x] 3.4 `internal/scaffold/templates/agents/kiro/hypnos.md`: same trigger edit.
- [x] 3.5 `internal/scaffold/templates/agents/github-copilot/hypnos.agent.md`: same trigger edit. No frontmatter change.
- [x] 3.6 `internal/scaffold/templates/agents/antigravity/hypnos/SKILL.md`: same trigger edit.

## 4. Meng Po trigger (6 platforms)

- [x] 4.1 `internal/scaffold/templates/agents/claude-code/mengpo.md`: change trigger from "an agent name to retire" to "an agent name to retire, captured in a `phantasos`-authored change's proposal/design/tasks, dispatched via `/opsx:apply`."
- [x] 4.2 `internal/scaffold/templates/agents/cursor/mengpo.mdc`: same trigger edit.
- [x] 4.3 `internal/scaffold/templates/agents/codex/mengpo.toml`: same trigger edit.
- [x] 4.4 `internal/scaffold/templates/agents/kiro/mengpo.md`: same trigger edit.
- [x] 4.5 `internal/scaffold/templates/agents/github-copilot/mengpo.agent.md`: same trigger edit. No frontmatter change.
- [x] 4.6 `internal/scaffold/templates/agents/antigravity/mengpo/SKILL.md`: same trigger edit.

## 5. Phantasos scope (6 platforms)

- [x] 5.1 `internal/scaffold/templates/agents/claude-code/phantasos.md`: add a responsibility covering agent-roster changes (new agent authoring or retirement, from a direct ask or a `zhougong` recommendation) — describe the agent's role/rationale in `proposal.md`/`design.md`, scope `tasks.md` so Janus can dispatch to `hypnos` (creation) or `mengpo` (retirement). No new fixed hand-off — Phantasos still reports to Janus, same as for every other change.
- [x] 5.2 `internal/scaffold/templates/agents/cursor/phantasos.mdc`: same scope edit.
- [x] 5.3 `internal/scaffold/templates/agents/codex/phantasos.toml`: same scope edit.
- [x] 5.4 `internal/scaffold/templates/agents/kiro/phantasos.md`: same scope edit.
- [x] 5.5 `internal/scaffold/templates/agents/github-copilot/phantasos.agent.md`: same scope edit. No frontmatter change — `agents: [janus]` stays as-is (no new direct edge).
- [x] 5.6 `internal/scaffold/templates/agents/antigravity/phantasos/SKILL.md`: same scope edit.

## 6. Verify

- [x] 6.1 Re-read all 30 edited files to confirm consistent phrasing per platform's existing style (numbered-list vs. condensed paragraph).
- [x] 6.2 Run `go build ./...` and `go test ./...` to confirm the (unaffected) scaffold installer still passes — template content isn't asserted verbatim in Go tests, but installer paths/wiring must stay intact.
- [x] 6.3 Grep all 30 files for stray old phrasing ("direct request routed via Janus, or a", "hand off directly to \`hypnos\`" in zhougong files, "archive/delete an unneeded agent" in janus files) to confirm no file was missed.
