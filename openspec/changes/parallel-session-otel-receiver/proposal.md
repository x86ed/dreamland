## Why

`dreamland otel-receiver` is bound to exactly one repository. `runOtelReceiver` resolves `repoRoot` once, passes it into `otelreceiver.Handler(repoRoot)`, and every mailbox (`<repo>/.dreamland/otel-sessions/<gen_ai.conversation.id>.json`) and log line (`<repo>/.dreamland/otel-receiver.log`) is written under that root. The receiver listens on a single address derived from `.dreamland.json` `otel_endpoint` (default `:4317` translated to `:4318`), and `SessionStart` start-up is idempotent by *dialing the port*: if anything answers, it does nothing.

Consequences when more than one repo, or more than one worktree, is open:

- Repo B's `SessionStart` finds the port taken by repo A's receiver and does nothing. Copilot spans from repo B's sessions are written into repo A's `.dreamland/otel-sessions/`. Repo B's `telemetry write --tool github-copilot` looks in its own `.dreamland/otel-sessions/`, finds nothing, and its commits carry no tokens.
- A receiver started by an older `dreamland` binary keeps holding the port after the binary is upgraded. There is no way for the new binary to tell, or to replace it (this happened; the pid was killed by hand).
- Independent of multi-repo: the mailbox holds only the *latest* span (overwrite, not sum), and `telemetry.Write` *adds* whatever the collector returns to `.dreamland-session.json` on every Stop. Two Stops with no new span in between add the same numbers twice; a span sequence loses every span but the last.

The goal is: any number of Copilot sessions, across any number of repos and worktrees, running in parallel, each getting correct token attribution, with the receiver process self-healing across binary upgrades.

Only the GitHub Copilot path uses the receiver. Claude Code reads tokens from the transcript (see `claude-code-parity/design.md`), so none of this changes Claude Code behavior.

## What Changes

- **One shared, repo-agnostic receiver per listen address.** The receiver stops knowing about any repository. It writes mailboxes and its log into a per-user state directory (`os.UserCacheDir()/dreamland/otel`, override `DREAMLAND_STATE_DIR`), keyed by `gen_ai.conversation.id` (a UUID, globally unique). The consumer, `telemetry write --tool github-copilot`, already runs inside the right repo and already knows its `session_id`; it reads the mailbox by id from the state directory. No SessionStart registration, no routing table, and therefore no stale registrations to clean up.
- **Conversation id validation.** `gen_ai.conversation.id` is used as a file name today with no validation (a crafted id could escape the directory). Ids not matching `^[A-Za-z0-9._-]{1,128}$` (and not `.`/`..`) are ignored.
- **Version/health handshake.** New `GET /.dreamland/health` returns `{service, revision, build, pid, state_dir}`. `dreamland otel-receiver` (start) probes it: same-or-newer receiver, no-op; older `revision`, terminate by the reported pid, wait for the port to free, spawn the current binary; listener that is not a dreamland receiver, no-op plus a one-line stderr warning. A pid/state file (`receiver-<port>.json`) is advisory only, liveness-checked and deleted when stale. `--replace` forces replacement (needed once to evict pre-handshake receivers).
- **Cumulative mailbox plus consumer-side cursor (fixes the double-count).** Mailbox v2 holds running totals summed across spans (deduplicated by span id). `CopilotCollector` keeps a per-session cursor in `<repo>/.dreamland/otel-cursors/<session_id>.json` and returns only `totals - cursor`, then advances the cursor. Two Stops with no new span now yield a zero delta.
- **Receiver process hygiene.** Foreground child no longer needs a repo (address passed as `--addr`), runs with its working directory set to the state directory (it currently runs with cwd = the repo, which pins a worktree directory open, notably on Windows), handles SIGTERM/Interrupt with a graceful shutdown, garbage-collects mailboxes older than 7 days, and rotates its log at 5 MiB.
- **Windows parity.** `otel_receiver_windows.go` gains `processAlive` and `terminateProcess`; state paths use `%LocalAppData%`; mailbox rename retries on sharing violations; no reliance on POSIX signals or `lsof`.

Explicitly **not** changed here (see design.md "Scope"):

- `ClaudeCollector` re-summing the entire transcript on every Stop and inflating totals. Separate change.
- The same whole-file re-sum in `CopilotCollector`'s primary source (VS Code `chatSessions/<id>.jsonl`, `parseChatSessionTokens`). Same class of bug, separate change; the cursor helper introduced here is designed to be reused by it.
- Per-repo ports, `dreamland init` port allocation, changing the default `4318`.
- Multiple OS users on one machine sharing one port; remote/container/WSL setups where Copilot's `localhost` is not the receiver's `localhost`.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `dev-workflow-hooks` — the "A local OTLP/HTTP receiver captures GitHub Copilot's real token usage" requirement is rewritten (shared receiver, user-level state, cumulative mailbox, id validation); new requirements for the health/replace handshake, stale-state cleanup, and cross-platform process control.
- `otel-session-telemetry` — new requirement that the GitHub Copilot collector reports incremental (cursor-based) usage from the receiver mailbox, so repeated Stops do not double-count.

## Impact

- Code: `cmd/otel_receiver.go`, `cmd/otel_receiver_unix.go`, `cmd/otel_receiver_windows.go`, `internal/telemetry/otelreceiver/receiver.go`, new `internal/telemetry/otelreceiver/state.go` (state dir, pid file, GC) and `internal/telemetry/cursor` (reusable cursor store), `internal/telemetry/tools/copilot.go`, `.gitignore` template/scaffold entries for `.dreamland/otel-cursors/`.
- Tests: `cmd/otel_receiver_test.go`, `cmd/otel_receiver_unix_test.go`, `internal/telemetry/otelreceiver/receiver_test.go`, `internal/telemetry/tools/collectors_test.go`.
- Behavior/compat: `otelreceiver.Handler` and `ReadSessionUsage` change signature (`repoRoot` becomes `stateDir`). In-flight Copilot sessions at upgrade time lose only their OTel-fallback tokens (source 2 in the collector; the VS Code chat-session log is source 1).
- No new dependencies. No change to `.dreamland.json` schema, VS Code settings written by `dreamland init`, or hook bindings.
