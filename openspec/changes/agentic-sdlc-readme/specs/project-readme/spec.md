## ADDED Requirements

### Requirement: README documents requirements and installation

`README.md` SHALL include a requirements section listing every external prerequisite needed to use dreamland (Go toolchain matching `go.mod`'s declared version, git, the `openspec` CLI, the `gh` CLI, and one of the six supported coding tools), and an installation section describing the source-build path (`git clone` + `go build`) followed by running `dreamland init`.

#### Scenario: Reader follows install instructions on a clean machine

- **WHEN** a reader with none of the prerequisites installed follows the README's requirements and installation sections in order
- **THEN** they arrive at a working `dreamland` binary and a completed `dreamland init` run with no undocumented missing dependency

### Requirement: README documents the OpenSpec workflow lifecycle

`README.md` SHALL describe the `/opsx:propose` → `/opsx:apply` → `/opsx:archive` change lifecycle and the five distinct hand-off paths `janus` can start, naming each path and its agent sequence:

1. **TDD/BDD** — new behavior with a spec scenario and no covering test: `janus` → `nyx` → `morpheus` → `phobetor` → `baku`.
2. **Standard SDD** — mechanical/internal task, or a task whose test already exists: `janus` → `phantasos` → `morpheus` → `phobetor` → `baku`.
3. **Walkabout** — no OpenSpec context at all: `janus` → `iktomi` → *(context-dependent — `iktomi` routes freely to whichever agent its freeform work turns out to need, same broad routing capability as `janus`)* → `baku`.
4. **Tuning** — agent performance/token-usage questions, not a feature change: `janus` → `zhougong` produces a report; if it recommends a new agent, that recommendation feeds the Agent Building workflow.
5. **Agent Building** — creating or retiring an agent, with two sub-paths:
   - *Creation*: `janus` → `zhougong` → `phantasos` → `hypnos` → `phobetor` → `baku`.
   - *Deletion*: `janus` → `zhougong` → `phantasos` → `mengpo`.

For all five paths, once `janus` makes its entry dispatch, downstream agents hand off directly to each other and do not return through `janus` except for genuinely ambiguous escalations.

#### Scenario: Reader can trace a change from proposal to merged PR

- **WHEN** a reader follows the workflows section for a hypothetical new feature that needs a new test
- **THEN** they can name the TDD/BDD path in order (`janus` → `nyx` → `morpheus` → `phobetor` → `baku`) and identify it as distinct from the Standard SDD path used for mechanical tasks

#### Scenario: Reader distinguishes the five workflow paths

- **WHEN** a reader wants to know which path applies to a freeform request with no OpenSpec change, versus a request to retire an unneeded agent
- **THEN** the README identifies the former as Walkabout (`janus` → `iktomi` → context-dependent hop → `baku`) and the latter as Agent Building's deletion sub-path (`janus` → `zhougong` → `phantasos` → `mengpo`)

### Requirement: README documents the purpose of every agent

`README.md` SHALL list all ten agents (`janus`, `phantasos`, `nyx`, `morpheus`, `phobetor`, `baku`, `iktomi`, `zhougong`, `hypnos`, `mengpo`) with a one-to-a-few-sentence description of each agent's responsibility and its default hand-off target(s), consistent with the corresponding template file under `internal/scaffold/templates/agents/claude-code/`.

#### Scenario: Reader identifies the right agent for a task

- **WHEN** a reader wants to know which agent authors a new agent definition, or which agent validates a completed implementation
- **THEN** the README's agent section names `hypnos` and `phobetor` respectively, matching each agent's template file

### Requirement: README documents improving results via telemetry analysis

`README.md` SHALL include a section explaining how to use the telemetry dreamland already collects — `dreamland telemetry snapshot`, commit trailers written by `dreamland coauthor --trailer`, and `.dreamland/transition.log` — via the `zhougong` agent's report (`.dreamland/reports/<date>-agent-report.md`) to identify recurring agent behavior, and how such a finding leads to `hypnos` authoring a new project-specific agent to replace generic, token-heavy agent work.

#### Scenario: Reader uses analysis to specialize their own project

- **WHEN** a reader has been using dreamland on their project for multiple changes and wants to reduce repeated token-heavy agent work
- **THEN** the README tells them to invoke `zhougong` to generate a report and, if it recommends a new agent, to have `hypnos` author it
