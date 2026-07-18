## Context

Copilot's `.github/agents/*.agent.md` frontmatter (`name`/`description`/`role`/`tools`/`model` + `agents:` graph + agent-scoped `hooks:`) is more complete than Claude Code's (`name`/`description`/`role`/`tools`/`model` only). Ground truth, confirmed against `code.claude.com/docs/en/sub-agents`, `.../hooks`, and this repo's templates:

- **No dreamland agent's `tools:` includes `Agent`/`Task`.** Every hand-off is executed by the main thread re-invoking `Agent` after reading a subagent's returned instructions — no agent dispatches directly.
- **`SubagentStop`/`PreToolUse` matchers are unscoped by design, and this repo's templates already use that form** (`matcher: ""`, `matcher: "Task|Agent"`). Docs confirm empty/omitted matches everything, and list `general-purpose`/`Explore`/`Plan` as valid `SubagentStop` matcher values — built-in agents already get the same hook chain dreamland agents do, no code change needed.
- **Claude Code supports agent-scoped frontmatter hooks** (`Stop:`, auto-converts to `SubagentStop`). No frontmatter `SubagentStart` exists — only `PreToolUse`/`PostToolUse`/`Stop`.
- **No OTel receiver needed on Claude Code** — `telemetry write --tool claude-code` reads tokens from the transcript JSONL directly. Copilot's hook payload has no token data at all, hence `dreamland otel-receiver`.
- **`iktomi` is a Copilot-only necessity.** Copilot has no unscoped hook mechanism — its hooks are either a fixed workspace event list or per-agent frontmatter each agent must declare individually. No Copilot built-in gets telemetry coverage for free, so dreamland built its own. Claude Code's built-ins already get it via the unscoped matchers above.
- **`cmd/commit.go` doesn't call `agentNameFromHookPayload()`; `cmd/coauthor.go` does.** Both platforms deliver `agent_type` on `SubagentStop` — this is a code/spec drift bug, not a platform gap.
- **Copilot's `janus.agent.md` alone carries `version-bump --minor --if-agent janus`**; Claude's workspace `SubagentStop` array already runs it unconditionally for every agent (self-filtering, harmless).
- This repo has never run `dreamland init` against itself for Claude Code — no `.claude/agents/`, no dreamland hooks in `.claude/settings.json`.

**Dispatch-graph enforcement (Copilot's `agents:` restriction) has no Claude Code equivalent — confirmed, not assumed.** Walked the exact scenario: session launched `--agent janus`; janus dispatches `nyx`; `nyx`'s own tool calls report `agent_type: nyx` correctly; `nyx`'s turn ends; the main thread relays the hand-off to `morpheus` — that call's `agent_type` is `janus` again (session-level value), not `nyx`. Every relayed hop is indistinguishable from Janus's own first dispatch, and Janus can legitimately reach all nine, so a dispatch-time check would never block anything. The only place identity narrows correctly is *inside* a subagent's own tool call (`nyx` calling `Write` reports `agent_type: nyx` unconditionally) — that's the enforcement point this change uses, and it checks a different, adjacent property: not "who was allowed to dispatch here" but "who actually produced this file."

Also confirmed: running as the main-thread agent (`--agent`/`agent` setting) inherits that agent's tools **and model**, not just tools, and the `Agent` tool must be explicitly granted via `tools:` even for main-thread mode — omitted entirely means it can't spawn anyone. Janus's `tools:` is `Read, Bash`; making it the main-thread agent without adding `Agent` would leave the main thread unable to dispatch at all.

## Goals / Non-Goals

**Goals:**
- Every Claude Code/Copilot frontmatter difference is a stated decision (matched, adapted, or omitted with a reason) — no unexamined gaps.
- Agent-scoped `hooks:` on all ten Claude Code agents, same coverage Copilot's have, wired to Claude's own telemetry pipeline.
- Locked-in, tested guarantee that built-in Claude Code agents get the same hook chain as dreamland agents.
- `dreamland commit` and `dreamland coauthor` agree on agent identity, on every platform, for every agent type.
- A dispatched agent can't produce an artifact outside its role (specs, agent templates, reports), without touching any agent's permissions.
- Prove all of the above against this repo with the real CLI.

**Non-Goals:**
- Granting `Agent`/`Task` to any dreamland agent, Janus included. Real platform-capability difference (Copilot's `agent` tool is allowlist-scoped; Claude's isn't) — not an oversight. Would also let agents nest-spawn directly, a different architecture this change doesn't adopt.
- `permissions.deny`-blocking Claude Code's built-ins. The telemetry gap it would prevent doesn't exist here — rejected, not deferred.
- Making Janus the main-thread agent (`"agent": "janus"`). Would change its model and require granting it `Agent` — explicitly rejected; Janus stays `Read, Bash`, no exceptions. Consequence: main-thread-direct edits with no subagent active aren't guardable.
- A new telemetry schema. `SnapshotResult`/`.dreamland-session.json` unchanged — every fix here feeds the existing pipeline.
- Cursor, Codex, Kiro, Antigravity. `--agent-name` and the coauthor trailer land there too since they're shared code, but no platform-specific work targets them — their gaps (no `SubagentStop`-equivalent, Antigravity's undocumented session-start) are pre-existing and untouched.

## Decisions

**`--agent-name <name>` flag on `coauthor`, `telemetry write`, `commit`.** Precedence: flag → env var → stdin `agent_type` sniff → coding-tool fallback. Used by the new per-agent-scoped hook blocks (identity is statically known there — no reason to re-derive it via a 200ms-timeout stdin read). The workspace-level shared array has no static value to use and keeps the runtime chain unchanged — this adds a second path, doesn't replace the first. Not a Copilot-parity item (checked — Copilot's own blocks don't do this either); applied to both platforms' templates anyway since it's strictly safer wherever usable.

**Per-agent `hooks.Stop` block, uniform across all ten Claude Code files, including janus's `--if-agent janus` line — not Copilot's 4-vs-5 split.** Same five commands as the workspace `SubagentStop` array, plus `--agent-name`. Keeping the janus-only command on all ten is simpler than a per-file split and behaviorally identical (self-filtering).

**No frontmatter `SubagentStart` analog.** Doesn't exist for Claude Code frontmatter hooks. The workspace `PreToolUse(Task|Agent)` hook already covers that role.

**Coverage guarantee (`agent-telemetry-coverage`), not a denylist.** Requires `SubagentStop`/`PreToolUse` matchers stay unscoped, with a regression test. Rejected: denylist (removes functionality for a gap that doesn't exist); allowlist via `Agent(agent_type)` (only applies when that agent is the main thread, which dreamland doesn't control); doing nothing (an unrequired invariant is one refactor away from silent regression). Kept as its own capability rather than folded into `dev-workflow-hooks` — it's a correctness property of existing bindings, not a new one.

**Fix `commit.go`'s workspace-level resolution AND add `--agent-name` for the per-agent path — both.** The shared array still needs runtime lookup (`agentNameFromHookPayload()`, matching `coauthor`); the per-agent blocks use the flag instead. An earlier draft rejected the flag outright on the reasoning that it "duplicates" the stdin convention — that held only before a per-agent-scoped path existed to use it. Rejected alternative: reading `git config user.name` back at commit time (races on hook-array execution order).

**Per-agent token burn: subtraction over git log, not a precomputed delta.** `Tokens:` is a cumulative session total (`snapshot.go`'s `Write` always sums). Adding a "since last commit" checkpoint was considered and rejected — real schema surface for no need: coverage (every completion telemetered) + attribution (every commit named to a specific agent) already produce an ordered `(agent, cumulative_total)` sequence `zhougong` or a script can subtract over. Neither alone makes the subtraction reliable.

**Second `Co-authored-by:` trailer for the coding tool.** `appendCoauthorTrailer`'s existing name-parsing truncates at the first space (correct for model IDs, wrong for `"Claude Code"`/`"GitHub Copilot"`). Extracted a non-truncating append-and-dedupe helper, called twice: once for the model, once for `cfg.CodingTool` verbatim. Shared code — no template changes anywhere. Platform identity was previously visible only in `.dreamland-session.json`'s `Tool` field (working-tree JSON, not commit text); now it's a second greppable trailer line.

**Pre-dispatch identity fallback: `janus`, not the raw coding-tool name.** `resolveAgentName`'s fallback (reached only with no env var and no `agent_type` payload — true session start) no longer needs to double as the tool-name carrier now that the trailer above exists independently. Janus is the implicit entry role for every session regardless of whether a formal dispatch to it has happened yet. One shared function — Copilot's identical `SessionStart` binding gets this automatically.

**Verification is a tasks.md checklist, not a spec requirement.** Run `dreamland init` against this repo, diff against existing specs, fix drift, then drive a real Claude Code session (including a built-in-agent turn) to confirm routing/coverage/attribution together.

**Janus dispatch guardrails: instruction-level, at existing touchpoints only — not a reversal of `janus-router-agent`'s direct-hand-off architecture.** `janus-router-agent` deliberately routes `nyx`→`morpheus`→`phobetor`→`baku` directly, bypassing Janus, on all six platforms — that's a named, cross-platform requirement with its own scenario coverage, not something this change touches. Considered and rejected: making every hop re-enter Janus (reverses that requirement, touches 7 agent files, adds a Janus round-trip to every hand-off for no telemetry/coverage benefit — coverage and attribution are already solved elsewhere in this change without it). What ships instead: explicit language in `janus.md` telling it to validate a hand-off against the known graph at the points it's *already* invoked (entry, ambiguous/terminal reports, broad-routing agents' reports). No hook can check this — there's no structured, inspectable record of "what target did Janus choose and why," unlike a file path in `guard_artifact`'s payload — so this is prompt content, not a code gate, and stated as such rather than implied to be enforced.

**Iktomi routes completed code changes through Phobetor, not straight to Janus — fixed on both platforms, lives in `janus-router-agent`, not `janus-dispatch-guardrails`.** Real gap, not a hardening: both `iktomi.md` (Claude Code) and `iktomi.agent.md` (Copilot) currently say "report completion... to Janus" with no distinction between code changes and pure investigation — checked both, identical gap, not a platform difference — so this belongs in `janus-router-agent` (router architecture, applies wherever Iktomi exists) rather than the Claude-only guardrails capability. Phrasing mirrors `morpheus.agent.md`'s existing fixed hand-off ("hand off directly to `phobetor` for validation... a fixed next step, do not report to Janus first") rather than inventing new wording. Fix is scoped narrowly: file changes → `phobetor` first (existing, unmodified pass/fail routing); no file changes → report to Janus as before. Cursor/Codex/Kiro/Antigravity have the same gap; not fixed here, stated as follow-up in the spec itself, not silently left inconsistent.

## Risks / Trade-offs

- **[Risk] Unscoped-matcher requirement regresses silently** if a future edit scopes `SubagentStop` to the ten dreamland names. → `agent-telemetry-coverage`'s regression test exists specifically to catch this.
- **[Risk] Built-in `agent_type` coverage is doc-sourced, not live-captured** (unlike the Copilot case, which cites a live payload). → Live-verification task drives a real built-in-agent completion before this is considered closed.
- **[Risk] 200ms stdin timeout still gates `commit`'s workspace-level path.** → Unchanged risk for that path only; the per-agent path bypasses it entirely via `--agent-name`, a net improvement.
- **[Trade-off] Ten files' hook blocks must stay in sync with `settings-patch.json`'s array.** → A test diffs them, allowing exactly the expected `--agent-name` difference per file.
- **[Risk] Janus's dispatch guardrail has no test that can actually verify it fires correctly** — it's prompt compliance, not code, so "does Janus really refuse an out-of-graph suggestion" can only be checked by live observation, not `go test`. → Live-verification task includes deliberately trying to provoke this.

## Open Questions

None. Janus's `--if-agent` asymmetry and the built-in-coverage question are both resolved above, not deferred.
