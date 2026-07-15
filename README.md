# dreamland

dreamland is a spec-driven agentic SDLC harness. `dreamland init` scaffolds a router agent (Janus) and nine specialist agents into your repository's AI coding tool of choice, wires them to an [OpenSpec](https://github.com/Fission-AI/OpenSpec)-driven change lifecycle (`/opsx:propose` → `/opsx:apply` → `/opsx:archive`), and captures per-turn telemetry so you can see which agents are earning their keep — and turn recurring patterns into project-specific agents over time.

This README covers:

- [Requirements](#requirements) and [Installation](#installation)
- [Workflows](#workflows) — the five paths a request can take through the agent roster
- [Agents](#agents) — what each of the ten agents does
- [Improving your results with analysis](#improving-your-results-with-analysis) — using the telemetry dreamland already collects to specialize your own project

## Requirements

- **Go toolchain** matching `go.mod`'s declared version (currently `go 1.26.4`) — dreamland has no packaged release yet, so you build it from source.
- **git**.
- **The `openspec` CLI** (`npm install -g @fission-ai/openspec`) — `dreamland init`'s propose/apply/archive workflow shells out to it.
- **The `gh` CLI** — the `baku` agent uses it to open pull requests when a change is closed.
- **One of six supported coding tools**: Claude Code, Codex CLI, Cursor, Kiro, Antigravity, or GitHub Copilot. `dreamland init` asks which one you're using and installs that platform's native agent format.

## Installation

There's no `go install`-able path yet — `go.mod` declares `module dreamland`, which doesn't match this repo's GitHub import path, so `go install github.com/x86ed/dreamland@latest` won't work. Build from source instead:

```sh
git clone <this-repo-url>
cd dreamland
go build -o dreamland .
```

Put the resulting binary on your `PATH`, then run the interactive wizard from the root of the repository you want to scaffold:

```sh
dreamland init
```

The wizard asks for: your repository root, which coding tool you're using, your primary language, your test command, an optional doc-generation command, your version command, and an OpenTelemetry endpoint (defaults to `http://localhost:4317`). It writes `.dreamland.json`, installs the ten agents and their slash commands in your coding tool's native format, wires session lifecycle hooks (`version-bump`, `coauthor`, `transition-log`, `test`), and installs a commit-msg telemetry hook.

## Workflows

Every request enters through **Janus**, a pure router (it never edits files, writes code, or writes specs — its only job is picking the right agent and dispatching to it). Once Janus makes its entry dispatch, downstream agents hand off directly to each other without routing back through Janus, except for genuinely ambiguous escalations. There are five paths:

### TDD/BDD

New behavior described by a spec scenario, with no covering test yet:

```text
janus → nyx → morpheus → phobetor → baku
```

`nyx` writes a failing acceptance test first (TDD red phase), then hands off directly to `morpheus` to implement it.

### Standard SDD

A mechanical/internal task, or a task whose test already exists:

```text
janus → phantasos → morpheus → phobetor → baku
```

`phantasos` drafts the proposal/design/tasks, then Janus dispatches the resulting task straight to `morpheus` — no `nyx` step needed.

### Walkabout

No OpenSpec context at all — no proposal, no task list, nothing that fits the spec-driven flow:

```text
janus → iktomi → (context-dependent) → baku
```

`iktomi` handles the request freeform. If it turns out to need a specialist mid-task (e.g. it needs a spec drafted, or code implemented against an existing one), it hands off directly there — the same broad routing capability Janus itself has. There's no single fixed agent after `iktomi`; the only fixed point in this path is the eventual close via `baku`.

### Tuning

A question about agent performance or token usage, not a feature change:

```text
janus → zhougong (report)
```

`zhougong` mines git log, commit trailers, and `.dreamland/transition.log` for per-agent commit counts, token totals, and hand-off timing, and writes a report to `.dreamland/reports/<date>-agent-report.md`. If the report's "recommended new agent" section names a specific, unambiguous next step, `zhougong` hands off directly to `phantasos` — feeding the Agent Building workflow below. `zhougong` never drafts the change itself; it only analyzes and recommends.

### Agent Building

Creating or retiring an agent — drafted the same way as any other change, because `phantasos` is involved in every spec:

```text
Creation: janus → zhougong → phantasos → hypnos → phobetor → baku
Deletion: janus → zhougong → phantasos → mengpo
```

`phantasos` drafts a proposal/design/tasks describing the agent's role (or the reason it's being retired) — it never authors or deletes agent files itself. Janus then dispatches the resulting task to `hypnos` (creation) or `mengpo` (deletion) the same way it dispatches code tasks to `morpheus`. `hypnos` hands off to `phobetor` to validate the new agent before closing via `baku`. `mengpo`'s retirement doesn't go through `baku` — archiving an agent doesn't need a PR-closure step the way shipping one does.

## Agents

| Agent | Purpose | Default hand-off |
| --- | --- | --- |
| **Janus** | Pure router — reads `openspec status`, picks the right agent, dispatches. Never edits files. | Entry point only; downstream agents hand off directly to each other. |
| **Phantasos** | Drafts and refines `proposal.md`/`design.md`/spec/`tasks.md` for a change — including agent-roster changes (new agent authoring or retirement). | Reports to Janus once drafted. |
| **Nyx** | Writes the failing acceptance test for a task's spec scenario before any implementation exists (TDD red phase). | `morpheus`, directly. |
| **Morpheus** | Implements tasks from `tasks.md`, marking each complete as it goes. | `phobetor`, directly, once implementation is done. |
| **Phobetor** | Runs tests and checks each completed task against its spec scenario. Never modifies code. | `baku` on pass, `morpheus` on implementation bug, `phantasos` on spec defect — all direct. |
| **Baku** | Verifies all tasks are complete, runs `/opsx:archive`, opens the PR with `gh pr create`. | Terminal — confirms closure with Janus. |
| **Iktomi** | General-purpose freeform agent for requests with no OpenSpec context. | Direct hand-off to any specialist its work turns out to need, or reports to Janus. |
| **Zhou Gong** | Analyzes git history, per-agent token burn, and turn duration; writes tuning reports. Never edits existing files. | `phantasos` when a report clearly recommends a new agent, otherwise reports to Janus. |
| **Hypnos** | Authors a new agent's template files across all six platforms and registers it in every `janus.*` routing table. | `phobetor`, directly, to validate the new agent. |
| **Meng Po** | Archives (default) or hard-deletes (on explicit instruction) an agent's template files across all six platforms. | Reports to Janus by default, or hands off directly if archival reveals a specific follow-up. |

## Improving your results with analysis

dreamland captures per-turn telemetry as a matter of course — it's not an opt-in feature you have to wire up separately:

- `dreamland coauthor --trailer` appends a `Tokens: input=<n> output=<n> cached=<n> total=<n>` line to every commit message, attributed to whichever agent made the handoff.
- `dreamland transition-log` appends a timestamped entry to `.dreamland/transition.log` on every turn.
- `dreamland telemetry snapshot` outputs the current session's telemetry as JSON (or commit trailers, with `--format trailers`).

Once you've run a few changes through the harness, invoke `zhougong` (directly, or via a request Janus recognizes as an agent-performance/tuning question) to turn that raw data into a report at `.dreamland/reports/<date>-agent-report.md`: a per-agent breakdown of commit counts and token totals, plus narrative tuning suggestions — for example, an agent whose commits show disproportionate token burn relative to commit count, or unusually long time-between-handoffs.

If the report identifies a recurring pattern not well served by the current roster — `iktomi` fielding many similar freeform requests that share a common shape is the canonical example — its "recommended new agent" section describes the gap. When that recommendation is specific and unambiguous, `zhougong` hands off to `phantasos` to draft the change, which `hypnos` then implements via the normal Agent Building workflow above. This is how the harness is meant to be used: start agent-heavy and general-purpose, then deliberately distill repeated agent behavior into project-specific agents (or, eventually, deterministic scripts and hooks) as your project's patterns stabilize.
