## Why

On Claude Code, Janus dispatches specialist agents via the `Agent`/`Task` tool, and dispatch defaults to running in the background: the tool call happens, then a disconnected notification arrives later with no persistent indicator of who's currently acting in between. Janus deliberately has no `agent` tool of its own (see `agent-scaffolding`) — dispatch is a visible tool call in the transcript, but nothing survives past that single line once the call scrolls out of view or runs in the background. There's no glanceable, always-on answer to "which agent is active right now."

Claude Code's statusline is the native, always-visible surface for exactly this kind of session state — it's how a user currently sees model, cwd, cost, git branch. Its JSON payload has no field for a currently-dispatched subagent (confirmed against current docs), so the state has to be captured and stored by Dreamland itself, via hooks that already fire around the `Agent`/`Task` tool call.

## What Changes

- New `dreamland statusline` command: prints a status line segment showing the currently active agent (or an idle/janus indicator when no dispatch is in flight), reading agent-dispatch state Dreamland itself writes.
- New `dreamland agent-status` command (`--start` / `--stop` modes): invoked by `PreToolUse`/`PostToolUse` hooks matched on `Task|Agent`, extracts the dispatched `subagent_type` from the hook payload, and records active/idle state, keyed by session, to a single shared state file.
- `settings-patch.json` (Claude Code binding only): add the `PreToolUse`/`PostToolUse` `agent-status` hook entries (alongside the existing `Task|Agent` `coauthor --hook` entry), and add the `statusLine` key pointing at `dreamland statusline`.
- Scoped to Claude Code only — GitHub Copilot's VS Code UI already surfaces active-participant state natively and has no `statusLine`-equivalent gap to fill; Cursor/Codex/Kiro/Antigravity are untouched, consistent with how `claude-code-parity` scoped its own deltas.

## Capabilities

### New Capabilities
- `agent-dispatch-visibility`: tracks and surfaces which Dreamland agent is currently dispatched on Claude Code — the hook wiring that populates the state (`PreToolUse`/`PostToolUse` on `Task|Agent`), the state file itself, and the `statusLine` command that reads it. Purely additive: no existing capability's requirements change (the `Task|Agent` hooks gain a second command alongside the existing `coauthor --hook` entry; nothing about `coauthor`'s own behavior changes).

### Modified Capabilities
_(none — purely additive)_

## Impact

- `cmd/statusline.go` (new), `cmd/agent_status.go` (new) — plus tests.
- `internal/scaffold/templates/hooks/bindings/claude-code/settings-patch.json` — add `PreToolUse`/`PostToolUse` (`Task|Agent`) `agent-status` entries and a `statusLine` key.
- `.dreamland/agent-status.json` (new, gitignored) — single shared runtime state file, keyed by session ID, pruned of stale entries on every write.
- `.gitignore` — add `.dreamland/agent-status.json`; unlike `.dreamland-session.json`/`.dreamland/last-test-result.json` (tracked as part of this repo's self-hosting), this file churns per-dispatch with no history value.
- No changes to GitHub Copilot, Cursor, Codex, Kiro, or Antigravity templates or bindings.
- Repo self-hosting: `.claude/settings.json` in this repo gains the same `statusLine` + hook entries once re-scaffolded.
