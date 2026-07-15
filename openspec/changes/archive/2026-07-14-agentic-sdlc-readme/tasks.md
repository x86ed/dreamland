## 1. Research current state

- [x] 1.1 Re-read each agent's canonical description in `internal/scaffold/templates/agents/claude-code/{janus,phantasos,nyx,morpheus,phobetor,baku,iktomi,zhougong,hypnos,mengpo}.md` to confirm responsibilities and hand-off targets are current before writing the README.
- [x] 1.2 Confirm `go.mod`'s declared Go version and the `openspec`/`gh` prerequisite claims in design.md still hold (`go version`, `openspec --version`, `gh --version`).

## 2. Write README sections

- [x] 2.1 Write the title/intro: what dreamland is (a spec-driven agentic SDLC harness) and how the README is organized.
- [x] 2.2 Write the Requirements section: Go toolchain, git, `openspec` CLI (`npm install -g @fission-ai/openspec`), `gh` CLI, and the six supported coding tools.
- [x] 2.3 Write the Installation section: `git clone` + `go build` source-build steps, then `dreamland init` and what the wizard asks for.
- [x] 2.4 Write the Workflows section covering `/opsx:propose` → `/opsx:apply` → `/opsx:archive` and the five paths `janus` can start:
  - [x] 2.4.1 **TDD/BDD**: `janus` → `nyx` → `morpheus` → `phobetor` → `baku` (new behavior, spec scenario, no covering test).
  - [x] 2.4.2 **Standard SDD**: `janus` → `phantasos` → `morpheus` → `phobetor` → `baku` (mechanical/internal task, or test already exists).
  - [x] 2.4.3 **Walkabout**: `janus` → `iktomi` → context-dependent hop (iktomi routes freely, same broad capability as janus) → `baku` (no OpenSpec context).
  - [x] 2.4.4 **Tuning**: `janus` → `zhougong` produces a report (agent performance/token-usage questions, not a feature change); a recommendation for a new agent feeds the Agent Building workflow.
  - [x] 2.4.5 **Agent Building**: creation is `janus` → `zhougong` → `phantasos` → `hypnos` → `phobetor` → `baku`; deletion is `janus` → `zhougong` → `phantasos` → `mengpo` (no `baku` — archival doesn't close via PR the way shipping an agent does).
  - [x] 2.4.6 Note the shared rule: once `janus` dispatches, agents hand off directly to each other without returning through `janus` except for ambiguous escalations.
- [x] 2.5 Write the Agents section: one entry per agent (`janus`, `phantasos`, `nyx`, `morpheus`, `phobetor`, `baku`, `iktomi`, `zhougong`, `hypnos`, `mengpo`) with purpose and default hand-off.
- [x] 2.6 Write the "Improving your results with analysis" section: `dreamland telemetry snapshot`, commit trailers, `zhougong`'s report at `.dreamland/reports/<date>-agent-report.md`, and the `hypnos` hand-off for authoring a new specialized agent.

## 3. Finalize

- [x] 3.1 Replace `README.md` with the new content, ensuring every command/path referenced is copy-pasteable and matches the actual CLI (`dreamland --help` command names, template file paths).
- [x] 3.2 Proofread for consistency with `specs/project-readme/spec.md` scenarios (requirements/install, workflow trace, agent lookup, analysis loop).
