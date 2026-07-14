## Why

`README.md` is currently two lines ("dreamland — AI driven SDLC framework") while the project has grown a six-platform agent scaffolder, an OpenSpec-driven change workflow, a token/model telemetry pipeline, and a pre-merge quality gate. There is no single document explaining what dreamland is for, how to install it, how to build a feature with the shipped agents, or how the telemetry the tool already collects is meant to feed back into the harness itself. New contributors (and the agents dreamland installs) have no canonical entry point.

## What Changes

- Rewrite `README.md` as the project's front door, covering:
  - A mission/purpose statement: dreamland is a self-improving, spec-driven SDLC harness that starts agent-heavy and is meant to be deliberately distilled toward deterministic, code-generated scripts and hooks as a project's patterns stabilize.
  - Multi-platform installation instructions (macOS, Linux, Windows) for building the Go binary from source, since no packaged releases exist yet.
  - A guide to building features using `dreamland init`'s five scaffolded agents (orchestrator, spec-writer, implementer, tester, pr-closer) together with the OpenSpec change lifecycle (`propose` → `apply` → `archive`).
  - A section on closing the loop: using the session telemetry dreamland already captures (`dreamland telemetry snapshot`, OTel traces, commit trailers) to spot repeated agent behavior and promote it into project-specific agents, hooks, or deterministic scripts.
  - Presentation styled as a declassified/redacted field manual (visual wink at Area 51 / classified-document tropes) without sacrificing technical clarity.
- No behavior, code, or spec changes to the CLI itself — documentation only.

## Capabilities

### New Capabilities
- `project-readme`: Defines the required sections and content contract for the repository's top-level `README.md`.

### Modified Capabilities
(none — no existing runtime capability's requirements change)

## Impact

- `README.md` — full rewrite.
- No source, config, or CI changes.
