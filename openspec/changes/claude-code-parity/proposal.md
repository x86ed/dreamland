## Why

Claude Code and GitHub Copilot scaffold the same ten agents from the same role definitions, but Claude Code's frontmatter/hooks were never brought up to Copilot's bar. Some gaps are real platform differences (no structural `agents:` graph, no OTel token export) and stay as-is. The rest are closed here: agent-scoped hooks, a commit-attribution bug, telemetry coverage for built-in agents, fixed-pipeline write enforcement, and coding-tool co-author trailers. Full technical rationale for each (docs citations, rejected alternatives) is in design.md — this file states the decisions.

**Scope: Claude Code, plus whatever GitHub Copilot backfill the parity work above requires** (the `--agent-name` flag and the new coauthor trailer, both shared code paths applied to both platforms' templates). Cursor, Codex, Kiro, and Antigravity are out of scope — their platform-specific gaps (e.g. Antigravity's undocumented session-start, the missing `SubagentStop`-equivalent on all four) are pre-existing, already documented elsewhere, and not touched here.

## Frontmatter Parity Matrix

| Field | GitHub Copilot | Claude Code | Decision |
| --- | --- | --- | --- |
| `name`, `description`, `role`, `model` | Present | Present | At parity. No change. |
| `agents:` (hand-off allowlist) | Present, three-tier graph | Absent | Not mirrored. Copilot's `agent` tool is scoped by an explicit list; Claude's `Agent` tool reaches any target by name, so the graph lives in prose. Existing, correct asymmetry — not touched. |
| `tools:` includes `agent` | Present on all ten | Absent on all ten, `janus.md` included | Not mirrored, no exceptions. Granting it would let agents dispatch directly instead of through the main thread, or (for Janus) require making it the main-thread agent with its tool/model bundle changed. Both rejected — see design.md. |
| `hooks:` (agent-scoped) | Present on all ten | **Was absent — added by this change** | Adapted, not copied: `Stop:` (auto-converts to `SubagentStop`) instead of `SubagentStart`/`SubagentStop`; reads tokens from the transcript directly instead of Copilot's OTel receiver. Same commands, same coverage, different pipeline. |

## What Changes

- **`--agent-name <name>` flag** on `coauthor`, `telemetry write`, and `commit` — takes precedence over the existing env/stdin `agent_type` lookup. Applied to both platforms' per-agent hook templates, since each block already knows its own agent statically and doesn't need the 200ms stdin sniff. Not a Copilot-parity item — Copilot doesn't do this either today.
- **Agent-scoped `hooks.Stop` block on all ten `.claude/agents/*.md`**: `coauthor`, `telemetry write --tool claude-code`, `version-bump --patch`, `version-bump --minor --if-agent janus`, `commit --reason handoff` — all with `--agent-name`, mirroring the workspace `SubagentStop` array. Uniform across all ten (including janus) rather than Copilot's 4-vs-5 per-file split.
- **Same `--agent-name` patch applied to Copilot's existing `.github/agents/*.agent.md` hooks** — template-only change, no new Go code beyond the flag.
- **Fix `cmd/commit.go`'s agent-name resolution**: `runCommit` currently ignores `agentNameFromHookPayload()`, unlike `coauthor`, so commit subjects don't match git author identity. One code path, applies to both platforms and any agent type (dreamland or built-in).
- **Second `Co-authored-by:` trailer for the coding tool itself** (`Co-authored-by: <coding-tool> <tool-email>`, e.g. `GitHub Copilot <github-copilot@github.com>`), appended by `coauthor --trailer` alongside the existing model trailer. Shared code (`cmd/coauthor.go`), applies everywhere the hook already runs — no template changes needed. Makes platform identity greppable directly from `git log`, not just `.dreamland-session.json`'s `Tool` field.
- **`coauthor`'s pre-dispatch identity fallback changes from the raw coding-tool name to `janus`**: now that the coding-tool trailer exists independently, the fallback used before any agent has been dispatched can be agent-specific instead. Shared code — Claude Code and Copilot both get it via their existing `SessionStart` bindings.
- **New capability `agent-telemetry-coverage`**: locks in that Claude Code's `SubagentStop`/`PreToolUse` matchers stay unscoped (`""` / `Task|Agent`), so built-in agents (`general-purpose`, `Explore`, `Plan`) get the same hook coverage dreamland agents do. Replaces the earlier `permissions.deny` denylist plan — that solved a coverage gap Claude Code doesn't actually have.
- **New capability `fixed-pipeline-enforcement`**: `dreamland guard-artifact`, a `PreToolUse(Write|Edit)` hook enforcing `phantasos`→specs, `hypnos`→agent templates, `zhougong`→reports. Janus is untouched (`Read`, `Bash`, no exceptions). Two accepted, unsolved gaps: Bash-mediated writes (`mengpo`'s `rm`) and main-thread-direct edits with no subagent active.
- **New capability `janus-dispatch-guardrails`** (Claude Code only, instruction-level not code-enforced): `janus.md` gains explicit language to validate a hand-off suggestion against the known agent graph before dispatching, at the points it's already invoked (entry, ambiguous/terminal reports, broad-routing agents' reports) — doesn't touch the deterministic direct hops (`nyx`→`morpheus`→`phobetor`→`baku`), which stay exactly as they are.
- **Iktomi routing gap, fixed on both platforms**: `iktomi.md`/`iktomi.agent.md` currently report completed work straight to Janus with no distinction for code changes — confirmed identical gap on Claude Code and Copilot. Both now route to `phobetor` for validation first when the work touched files (mirroring `morpheus.agent.md`'s existing phrasing for its own fixed hand-off), reporting directly to Janus only for non-code work. This is a `janus-router-agent` delta (router architecture, not platform frontmatter), not a new capability. Cursor/Codex/Kiro/Antigravity have the same gap, not fixed here — out of scope, noted in the spec.
- Tests for all of the above; dogfood `dreamland init` (Claude Code) against this repo; live-verify with a real Claude Code session (routing, built-in-agent telemetry, guard block/allow, cross-platform commit comparability).

## Capabilities

### New Capabilities

- `agent-telemetry-coverage` — unscoped hook matchers guarantee telemetry fires for every subagent, built-in or dreamland.
- `fixed-pipeline-enforcement` — `PreToolUse` write-gate enforces fixed artifact ownership, no permission changes to any agent.
- `janus-dispatch-guardrails` — Janus validates hand-offs it's already involved in against the known graph. Claude Code only, instruction-level.

### Modified Capabilities

- `agent-scaffolding` — adds Claude Code agent-scoped `hooks:`; adds `--agent-name` to Copilot's existing agent-scoped hooks.
- `dev-workflow-hooks` — `dreamland commit` resolves agent name the same way `coauthor` does (plus the new `--agent-name` flag); `coauthor` gains a second `Co-authored-by:` trailer for the coding tool. Both apply on every platform (shared code).
- `janus-router-agent` — Iktomi routes completed code changes through Phobetor before reporting done, on Claude Code and Copilot; Cursor/Codex/Kiro/Antigravity's identical gap is unfixed, noted as follow-up.

## Impact

- `internal/scaffold/templates/agents/claude-code/*.md` (all ten) — add `hooks.Stop` block. No `tools:` changes. `janus.md`, `iktomi.md` additionally get instruction-body edits (guardrail language, Phobetor routing).
- `internal/scaffold/templates/agents/github-copilot/*.agent.md` (all ten) — add `--agent-name` to existing hook commands; `iktomi.agent.md` additionally gets the Phobetor-routing instruction fix.
- `internal/scaffold/templates/hooks/bindings/claude-code/settings-patch.json` — add `PreToolUse(Write|Edit)` → `guard-artifact`. No `"agent"` key.
- `cmd/coauthor.go`, `cmd/telemetry.go`, `cmd/commit.go` — add `--agent-name` flag; `commit.go` also gets the `agentNameFromHookPayload()` fallback; `coauthor.go` gains the second coding-tool `Co-authored-by:` trailer.
- `cmd/guard_artifact.go` (new) — ownership-table enforcement, exit 2 on violation, fail-open on unknown identity.
- Tests across `internal/scaffold`, `cmd/coauthor`, `cmd/telemetry`, `cmd/commit`, `cmd/guard_artifact`.
- `openspec/specs/agent-telemetry-coverage/spec.md`, `openspec/specs/fixed-pipeline-enforcement/spec.md`, `openspec/specs/janus-dispatch-guardrails/spec.md` — new. `agent-scaffolding`, `dev-workflow-hooks`, `janus-router-agent` — deltas.
- Repo self-hosting: `.claude/agents/`, `.claude/settings.json` in this repo.
- No changes to `internal/telemetry/tools/claude.go`, the OTEL env installer, or `SnapshotResult`. The commit-trailer format does change (second `Co-authored-by:` line) — deliberately, per this proposal, not a side effect.
- Scoped to Claude Code plus required GitHub Copilot backfill. Cursor, Codex, Kiro, Antigravity untouched.
