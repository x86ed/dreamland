## Why

`README.md` is currently two lines ("dreamland — AI driven SDLC framework"). dreamland actually ships ten named agents (Janus, Phantasos, Nyx, Morpheus, Phobetor, Baku, Iktomi, Zhou Gong, Hypnos, Meng Po), a six-platform scaffolder, an OpenSpec-driven change lifecycle, and a telemetry pipeline built specifically so a project can analyze its own agent usage and improve over time. None of that is discoverable without reading `openspec/specs/` and every template file individually. The README should be the one document a new user (or a new agent joining the project) reads to understand requirements, install the tool, run the propose → apply → archive workflow, know what each agent does, and use `zhougong`'s reports to tune the harness to their own project.

## What Changes

- Rewrite `README.md` to walk through, in order:
  - **Requirements**: Go toolchain (module declares `go 1.26.4`), git, the `openspec` CLI (`@fission-ai/openspec` via npm — required for `dreamland init`'s propose/apply/archive workflow), `gh` CLI (required by the `baku` agent to open PRs), and one of the six supported coding tools (Claude Code, Codex CLI, Cursor, Kiro, Antigravity, GitHub Copilot).
  - **Install**: building the `dreamland` binary from source (`git clone` + `go build`) and running `dreamland init`'s interactive wizard, since there is no packaged release today (`go.mod`'s `module dreamland` does not match the GitHub import path, so `go install github.com/x86ed/dreamland@latest` does not work).
  - **Workflows**: the OpenSpec change lifecycle (`/opsx:propose` → `/opsx:apply` → `/opsx:archive`) and how Janus routes a request into that lifecycle or off to `iktomi` for freeform work, including the fixed downstream hand-off chain (`nyx`/`morpheus` → `phobetor` → `baku`) agents follow without returning to Janus.
  - **Agent purposes**: a table or per-agent section explaining what each of the ten agents does and when it's invoked — `janus` (router), `phantasos` (proposal/design/spec drafting), `nyx` (TDD red-phase test authoring), `morpheus` (implementation), `phobetor` (validation), `baku` (finalize/PR), `iktomi` (freeform fallback), `zhou gong` (usage analysis/tuning reports), `hypnos` (new-agent authoring), `meng po` (agent archival).
  - **Improving your own results using analysis**: how to run `dreamland telemetry snapshot` and read `.dreamland/reports/<date>-agent-report.md` (written by `zhou gong` from git log, commit-trailer token totals, and `.dreamland/transition.log`), and how a recurring pattern in that report becomes a `hypnos`-authored specialized agent — i.e. how a project distills the generic agent-heavy workflow toward project-specific agents/hooks over time.
- No behavior, code, or spec changes to the CLI itself — documentation only.

## Capabilities

### New Capabilities
- `project-readme`: Defines the required sections and content contract for the repository's top-level `README.md` — requirements, install, workflows, per-agent purpose, and the telemetry-driven improvement loop.

### Modified Capabilities
(none — no existing runtime capability's requirements change)

## Impact

- `README.md` — full rewrite.
- No source, config, or CI changes.
