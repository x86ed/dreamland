## Context

dreamland is a Go CLI (module `dreamland`, repo `x86ed/dreamland`) that scaffolds a spec-driven AI SDLC workflow into a target repository: `dreamland init` installs five role agents (orchestrator, spec-writer, implementer, tester, pr-closer) into whichever coding tool the user picks (Claude Code, Codex CLI, Cursor, Kiro, Antigravity, GitHub Copilot), wires session-start/end-of-turn lifecycle hooks (`version-bump`, `coauthor`, `transition-log`, `test`), and runs an MCP server (`dreamland serve`) plus a telemetry pipeline (`dreamland telemetry ...`) that captures per-turn model/token usage into commit trailers and OTel traces. Feature work in this repo (and in repos dreamland is installed into) flows through OpenSpec: `openspec new change` → agent-authored `proposal.md`/`design.md`/`specs/`/`tasks.md` → `openspec apply` → `openspec archive`. None of this is currently documented outside the spec files under `openspec/specs/`; `README.md` is a two-line stub.

There is no packaged release (no goreleaser, no Homebrew tap, no `go install`-able module path — `go.mod` declares `module dreamland`, not the repo's import path). Installation today means building from source with the Go toolchain.

## Goals / Non-Goals

**Goals:**
- Give the repo a single `README.md` that explains dreamland's purpose, install path, day-to-day feature workflow, and the intended long-run harness-improvement loop, so a new human or agent reader is oriented without spelunking `openspec/specs/`.
- Describe installation truthfully for the project's actual current state (source build via Go toolchain on macOS/Linux/Windows), not aspirational package-manager commands that don't work yet.
- Explain the "distill agentic → deterministic" philosophy in concrete terms tied to what the tool already does (telemetry capture, hooks, scaffolded agents) rather than as abstract marketing language.
- Apply a light, consistent classified/redacted-document visual voice (headers, section framing, maybe a mock classification banner) without hurting scannability or technical accuracy.

**Non-Goals:**
- No changes to CLI behavior, templates, or specs — this change touches only `README.md`.
- No new install tooling (no goreleaser config, no Homebrew formula) — that's future work the README may gesture at but not claim exists.
- No exhaustive command reference — full flag-by-flag docs stay in `--help` output and `openspec/specs/`; the README links/points to the workflow, it doesn't restate every scenario.

## Decisions

**Single top-level README, no `docs/` split.** The project is small enough (one binary, one workflow) that fragmenting into a `docs/` tree would add navigation overhead for no benefit yet. Revisit if the README exceeds ~a few hundred lines.

**Installation section documents source builds only.** Alternative considered: write `go install github.com/x86ed/dreamland@latest` instructions. Rejected — `go.mod`'s `module dreamland` doesn't match the GitHub import path, so that command fails today (`module declares its path as: dreamland but was required as: github.com/x86ed/dreamland`). Documenting a broken command would be worse than documenting the honest `git clone && go build` path. The multi-platform requirement is satisfied by standard Go cross-platform build instructions (macOS, Linux, Windows), since the toolchain itself is what's portable, not a distributed binary.

**"Self-improving harness" framed around telemetry → specialization, not abstract AI claims.** Ground the pitch in real mechanisms already in the codebase: `dreamland telemetry snapshot`, OTel spans emitted from `dreamland serve`, and git trailers written by the `coauthor`/telemetry hooks. The README should describe the intended feedback loop (observe repeated agent behavior via telemetry → hand-author or generate a specialized agent/hook/script for that pattern → let the five generic agents step back as deterministic scripts take over) as the project's stated direction, clearly distinguishing "what dreamland does today" from "what the workflow is designed to grow into."

**Classified-document styling is presentational only.** Use it in headers, a redacted-banner intro, maybe file-stamp-style section dividers — but every code block, command, and instruction stays in plain, copy-pasteable form. No ASCII art or styling that would break markdown rendering on GitHub or obscure a command.

**One new capability spec (`project-readme`), no modified capabilities.** The README is a new deliverable, not a change to any existing runtime requirement, so there's nothing to express as a MODIFIED delta against `openspec/specs/*`.

## Risks / Trade-offs

- [Risk] Install instructions go stale if a real release pipeline (goreleaser/Homebrew) ships later. → Mitigation: keep the section short and explicit that source build is "current" method; the follow-up work to add packaged releases is a natural next change, not blocked by this one.
- [Risk] Classified-document tone could read as unprofessional to some audiences or drift into being cute at the expense of clarity. → Mitigation: keep the theming to headers/framing/light copy, never to command syntax or required steps; a reader skimming only code blocks and bold requirement text gets the same information as a plain README.
- [Risk] Describing the telemetry-to-specialization loop as intended direction (rather than a fully automated feature that exists today) could be misread as already-implemented. → Mitigation: explicitly phrase that section as guidance for maintainers ("how to use the data dreamland already collects to do this yourself"), not as a shipped auto-specialization feature.

## Migration Plan

Single-file replace of `README.md`. No rollback complexity beyond `git revert`.

## Open Questions

None outstanding — proceeding to specs/tasks.
