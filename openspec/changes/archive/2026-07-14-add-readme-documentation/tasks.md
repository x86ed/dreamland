## 1. Content research

- [ ] 1.1 Confirm current Go version (`go.mod`), module name, and repo import path to write accurate install commands
- [ ] 1.2 List the six supported coding tools and their `dreamland init` scaffold targets from `openspec/specs/agent-scaffolding/spec.md`
- [ ] 1.3 List the five agent roles and their responsibilities from `internal/scaffold/templates/agents/claude-code/*.md`
- [ ] 1.4 Confirm the OpenSpec CLI commands and skill names (`openspec new change`, `/opsx:propose`, `/opsx:apply`, `/opsx:archive`) from `.claude/skills/` and `.github/skills/`
- [ ] 1.5 Confirm telemetry commands and outputs (`dreamland telemetry write|snapshot|reset|install|uninstall`, `dreamland serve` OTel spans) from `cmd/telemetry.go` and `cmd/serve.go`

## 2. Write README sections

- [ ] 2.1 Write title, badge-free header, and mock classification banner (styling only, no functional content)
- [ ] 2.2 Write purpose statement: self-improving agentic-to-deterministic distillation philosophy
- [ ] 2.3 Write "Installation" section with macOS/Linux/Windows source-build instructions (git clone + go build), no unpublished `go install` remote path
- [ ] 2.4 Write "Building a Feature" section: `dreamland init`, the five scaffolded agents, and the OpenSpec propose → apply → archive lifecycle
- [ ] 2.5 Write "Evolving the Harness" section: using `dreamland telemetry snapshot` / OTel traces / commit trailers to spot repeated agent behavior and graduate it into specialized agents, hooks, or deterministic scripts — explicitly framed as maintainer-driven guidance, not automatic
- [ ] 2.6 Add a short "Status" or "Project Layout" pointer section linking to `openspec/specs/` for full behavioral detail (avoid duplicating spec content)

## 3. Style pass

- [ ] 3.1 Apply consistent classified/redacted-document voice to headers and section intros only
- [ ] 3.2 Verify every command/code block is plain, unstyled markdown and copy-pastable
- [ ] 3.3 Re-read for overclaiming (no implying automatic agent-specialization exists today)

## 4. Verification

- [ ] 4.1 Run every shell command in the Installation section from a clean clone to confirm it produces a working `dreamland` binary
- [ ] 4.2 Proofread full README render (e.g. `gh markdown` preview or GitHub PR diff view) for broken markdown/formatting
- [ ] 4.3 Confirm README satisfies all scenarios in `openspec/changes/add-readme-documentation/specs/project-readme/spec.md`
