## Context

dreamland is a Go CLI (module `dreamland`) that scaffolds a spec-driven agentic SDLC into a target repository. `dreamland init` runs an interactive wizard (coding tool, language, test/doc/version commands, OTel endpoint) and installs, per the chosen platform:

- Ten role agents (`janus`, `phantasos`, `nyx`, `morpheus`, `phobetor`, `baku`, `iktomi`, `zhougong`, `hypnos`, `mengpo`) as platform-native templates (Claude Code `.md`, Codex `.toml`, Cursor `.mdc`, Kiro `.md`, Antigravity `SKILL.md`, GitHub Copilot `.agent.md`).
- Per-agent slash commands and the `/opsx:propose` / `/opsx:apply` / `/opsx:archive` / `/opsx:explore` router commands, which shell out to the external `openspec` CLI (`@fission-ai/openspec`, installed separately via npm — not a Go dependency).
- Lifecycle hook bindings that call four `dreamland` subcommands directly (`version-bump`, `coauthor`, `transition-log`, `test`) at session-start / end-of-turn / commit time.
- A telemetry pipeline (`dreamland telemetry ...`, `dreamland serve` as an OTel-emitting MCP server) that writes per-turn token/model usage into commit trailers via `dreamland coauthor --trailer` and into `.dreamland-session.json`.

Feature work flows through OpenSpec: `phantasos` drafts `proposal.md`/`design.md`/`specs/`/`tasks.md`, `nyx`/`morpheus` implement against `tasks.md`, `phobetor` validates, `baku` archives and opens the PR. `janus` is a pure router (haiku model, `Read`+`Bash` tools only) that dispatches the *entry* into this pipeline; once inside, agents hand off directly to each other (`nyx`→`morpheus`→`phobetor`→(`baku`|`morpheus`|`phantasos`)) without returning through `janus`. `zhougong` closes the loop by mining git log / commit trailers / `.dreamland/transition.log` for per-agent token burn and hand-off timing, writing `.dreamland/reports/<date>-agent-report.md`, and — when a pattern is clear enough — handing off directly to `hypnos` to author a new specialized agent. `mengpo` retires agents that are no longer earning their keep.

None of this is documented outside `openspec/specs/*` and the template files themselves. A parallel change (`add-readme-documentation`, already apply-ready) covers similar ground with a generic five-role framing (orchestrator/spec-writer/implementer/tester/pr-closer) and classified-document visual theming; this change instead documents the actual ten named agents and their real hand-off graph, without the visual theming, so the user can choose which README direction to apply.

## Goals / Non-Goals

**Goals:**
- Requirements and install sections that are true today: Go toolchain, git, `openspec` npm CLI, `gh` CLI, choice of one of six coding tools — and a source-build install path, since no packaged release exists.
- A workflows section framed as five named paths off `janus`'s entry dispatch — **TDD/BDD** (`janus`→`nyx`→`morpheus`→`phobetor`→`baku`), **Standard SDD** (`janus`→`phantasos`→`morpheus`→`phobetor`→`baku`), **Walkabout** (`janus`→`iktomi`→context-dependent hop→`baku`), **Tuning** (`janus`→`zhougong` report, optionally feeding Agent Building), and **Agent Building** (creation: `janus`→`zhougong`→`phantasos`→`hypnos`→`phobetor`→`baku`; deletion: `janus`→`zhougong`→`phantasos`→`mengpo`) — so a reader understands why, for example, `morpheus` hands off to `phobetor` directly instead of reporting back to `janus`.
- A per-agent purpose section covering all ten agents by their actual name and responsibilities, sourced from `internal/scaffold/templates/agents/claude-code/*.md` frontmatter/bodies (the canonical description of each role).
- An analysis/improvement section that documents the real, already-shipped telemetry mechanism (`dreamland telemetry snapshot`, commit trailers, `zhougong`'s report file) as the concrete path to project-specific tuning — not aspirational auto-tuning.

**Non-Goals:**
- No changes to CLI behavior, templates, or specs — this change touches only `README.md`.
- No claim of a packaged release or `go install`-able path — that would require a separate change (goreleaser config, corrected module path).
- No exhaustive flag-by-flag command reference — that stays in `--help` output and `openspec/specs/`.
- No resolution of the overlap with `add-readme-documentation` — both changes stay apply-ready; the user decides which `README.md` version to apply (or archives the other change first).

## Decisions

**Document the real agent roster by name, not a generic five-role abstraction.** Alternative considered: keep the `add-readme-documentation` change's orchestrator/spec-writer/implementer/tester/pr-closer framing, since it's simpler for a first-time reader. Rejected per explicit user direction — the actual templates define ten named agents with a specific hand-off graph, and a reader (especially another agent) benefits more from the real routing table than a simplified abstraction that doesn't match `janus.md`'s routing rules.

**Source each agent's description from its own template file rather than paraphrasing from memory.** Ensures the README stays accurate to `internal/scaffold/templates/agents/claude-code/*.md`, which is the canonical definition dreamland actually installs. If an agent's role changes, the template changes first and the README section can be regenerated from it directly.

**Requirements section calls out `openspec` and `gh` as external prerequisites, not just the Go toolchain.** These are real, load-bearing dependencies discovered by reading `cmd/init.go` (wizard doesn't install them) and `baku.md` (`gh pr create`) — omitting them would leave a new user's `/opsx:propose` or `baku` hand-off failing with a missing-binary error the README never warned about.

**Installation section documents source builds only, same reasoning as `add-readme-documentation`'s design.md.** `go.mod`'s `module dreamland` doesn't match the GitHub import path, so `go install github.com/x86ed/dreamland@latest` fails today. Documenting `git clone && go build` is the honest current path.

**Four named workflows, not one generic lifecycle description.** `janus`'s routing table dispatches to a materially different agent depending on task shape (new-behavior-needs-test vs. mechanical vs. no-OpenSpec-context vs. tuning question), and each of those four paths has its own downstream chain. Naming them (TDD/BDD, Standard SDD, Walkabout, Tuning) gives a reader a lookup table instead of one paragraph they'd have to mentally branch themselves.

**Walkabout's post-`iktomi` step is documented as context-dependent, not a fixed agent.** `iktomi.md` gives it the same broad routing capability as `janus` — it hands off to whichever specialist its freeform work turns out to need, or reports to `janus`, with no single fixed next step. The README must say so explicitly rather than naming one agent that would misrepresent `iktomi`'s actual routing freedom; the only fixed point in this path is the eventual `baku` hand-off to close out.

**Agent Building is its own workflow, not a detail of Tuning.** `zhougong`'s recommendation is only the trigger; the actual creation/deletion work routes through `phantasos` (drafting the proposal/design/tasks for the new or retired agent, same as any other change) before reaching `hypnos` (authors the agent, hands to `phobetor` to validate, closes via `baku`) or `mengpo` (executes the archival/deletion directly, no `baku` hand-off — retiring an agent doesn't need a PR-closure step the way shipping one does). Keeping Tuning and Agent Building separate avoids collapsing "here's a report" and "here's the five-agent execution path that report can trigger" into one paragraph.
**Analysis section frames the telemetry loop as a manual, maintainer-driven action, not automatic.** `zhougong` only runs when invoked (directly or via `janus` routing on "agent performance, token usage, tuning questions") and only recommends — it hands off to `hypnos` to actually author a new agent. The README must not imply the harness self-modifies without a human or agent explicitly running this loop.

## Risks / Trade-offs

- [Risk] Two apply-ready changes (`add-readme-documentation` and this one) both rewrite `README.md`; applying both in sequence means the second overwrites the first. → Mitigation: each change's task list ends with a full-file write, so whichever is applied last wins; no merge conflict at the openspec layer since both are full-file replacements of the same path. Flagged to the user at proposal time.
- [Risk] Per-agent descriptions drift from the template files over time as agents are added (`hypnos`) or archived (`mengpo`). → Mitigation: tasks.md instructs re-reading each `internal/scaffold/templates/agents/claude-code/*.md` file at write time rather than relying on this design doc's summary, which is a point-in-time snapshot.
- [Risk] Listing `openspec`/`gh` as hard requirements without version pins could go stale. → Mitigation: link the requirement to the package name (`@fission-ai/openspec`) rather than a pinned version, matching how the CLI is actually installed today.

## Migration Plan

Single-file replace of `README.md`. No rollback complexity beyond `git revert`.

## Open Questions

None outstanding — proceeding to specs/tasks.
