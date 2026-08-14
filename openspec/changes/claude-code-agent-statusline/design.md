## Context

Claude Code's `statusLine` is a user-configured shell command, re-invoked on session events (new message, `/compact`, permission-mode change) or an optional timer (min. 1s, 300ms-debounced), that reads a JSON payload on stdin (`session_id`, `cwd`, `model`, `cost`, git info, …) and writes plain/ANSI text to stdout for the terminal status bar. Confirmed against current docs: this payload has **no field for a currently-dispatched subagent** — that state doesn't exist anywhere Claude Code exposes today, so Dreamland has to originate and store it itself.

The only signal available for "a subagent dispatch is happening" is the existing `PreToolUse`/`PostToolUse` hook pair matched on `Task|Agent`, which Dreamland already relies on for `coauthor --hook` (`internal/scaffold/templates/hooks/bindings/claude-code/settings-patch.json`). `tool_input.subagent_type` is the exact field `agentidentity.FromPayload` already parses out of that payload today (`cmd/coauthor.go`), so extracting "which agent" is proven, not new.

## Goals / Non-Goals

**Goals:**
- A Claude Code statusline segment that shows the currently-dispatched agent, or an idle/no-dispatch state, updated automatically as dispatches start and end.
- Correct behavior with multiple concurrent Claude Code sessions in the same repo (separate terminals/worktrees) — no shared-state clobbering.
- Degrade safely if a stop signal is ever missed (background dispatch, crash, etc.) — never show a permanently stale agent name.

**Non-Goals:**
- Mirroring GitHub Copilot's VS Code UI. Copilot's chat surfaces active-participant state natively in its own UI; there is no equivalent gap to fill there, so this is Claude Code-only, matching how `claude-code-parity` scoped its own per-platform deltas.
- Tracking nested/concurrent multi-agent dispatch precisely (e.g. two subagents in flight from one Janus turn). Single "currently active agent" is the target; see Open Questions.
- A general-purpose event bus or pub/sub for hook state. This is one narrow read/write state file, consistent with the existing `.dreamland-session.json`/telemetry-snapshot pattern already in this codebase.

## Decisions

- **Start signal: `PreToolUse` on `Task|Agent`, not `SubagentStart`.** `SubagentStart` fires on the dispatched subagent's own session and (per current docs) carries `agent_type` — but Dreamland's own `agentNameFromHookPayloadFrom` doc comment (`cmd/coauthor.go`), backed by passing tests, states Claude Code's `SubagentStop`/`SessionStart` payloads carry **no** `agent_type` field at all on this platform, only `PreToolUse`/`PostToolUse`'s `tool_input.subagent_type` does. That comment reflects live-payload verification already in this codebase; general docs research is not enough to override it. Using `PreToolUse(Task|Agent)` reuses an already-proven extraction path instead of trusting an unverified `SubagentStart` field shape.
- **Stop signal: `PostToolUse` on `Task|Agent`, with a staleness fallback — not relying on exact timing.** Whether `PostToolUse` fires the instant a background dispatch is acknowledged or only once the subagent's work actually completes isn't clearly documented and isn't worth blocking this design on. Instead of depending on precise timing, the state file carries a `started_at` timestamp; `dreamland statusline` treats any entry older than a threshold (e.g. 10 minutes) as stale and falls back to the idle/janus display regardless of whether a stop signal ever arrived. This makes correctness independent of `PostToolUse`'s exact firing point.
- **One state file per session, not one shared file.** `.dreamland/agent-status/<session_id>.json`, keyed by the `session_id` every hook payload and the statusline payload both already carry. Avoids concurrent-write races between multiple Claude Code sessions open on the same repo — each session's hooks only ever touch their own file. `--stop` deletes the file (common case); `--start` overwrites it (idempotent, matches the existing pattern in `installPrepareCommitMsgHook`/config writers elsewhere in this codebase).
- **New small commands (`agent-status`, `statusline`), not reuse of `telemetry`/`coauthor`.** Distinct concern (ephemeral dispatch state vs. token telemetry vs. git identity) — bolting this onto an existing command would conflate unrelated lifecycles for no benefit; the codebase's own convention is one command per concern (see `commit`, `coauthor`, `telemetry`, `version-bump` as siblings, not variants of one command).
- **`.dreamland/agent-status/` must be added to `.gitignore`.** Unlike `.dreamland/last-test-result.json`, `.dreamland/transition.log`, or `.dreamland-session.json` — which this repo deliberately tracks as part of its own self-hosting — a per-session file directory has no history value and churns on every dispatch. Left untracked, it would get swept into every `dreamland commit --reason turn-complete`/`handoff` checkpoint (which runs `git add -A`), polluting history with meaningless per-session noise.

## Risks / Trade-offs

- [Risk] `PostToolUse` may never fire for a genuinely abandoned/crashed dispatch, leaving a stale file. → Mitigation: staleness threshold in `dreamland statusline` (see Decisions); stale files are cosmetically harmless and don't need active cleanup.
- [Risk] Two subagents dispatched in close succession within one Janus turn could overwrite each other's `--start` before the first one's `--stop` runs, showing the wrong name briefly. → Mitigation: accepted for v1 — single "currently active agent" is the stated goal, not a stack. Noted as an Open Question below in case real usage shows this matters.
- [Risk] `statusLine` payload's own `session_id` might not exactly match the dispatching session's `session_id` in edge cases (e.g. resumed sessions). → Mitigation: none built in for v1; if this surfaces, the fallback is simply an empty/idle statusline segment (fails safe, not misleading).
- [Trade-off] This adds two more Claude-Code-only hook invocations per dispatch (`PreToolUse`/`PostToolUse` on `Task|Agent` already fire today for `coauthor --hook`; this just adds a second command to the same matcher, not a new matcher). Negligible added latency — both are `dreamland` binary invocations, same class of cost as the existing hook.

## Open Questions

- Should nested/concurrent dispatch (Janus fanning out to more than one subagent) eventually track a stack instead of a single slot? Deferred until real usage shows it's needed — the immediate problem is "no indicator at all," not "indicator doesn't handle N-deep concurrency."
- Exact staleness threshold (10 minutes suggested) should be validated against real background-dispatch durations once this is in use, not fixed permanently at design time.
