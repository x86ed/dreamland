## Status and recorded decisions

Design approved by the user. Decisions recorded from the review:

1. Approved: one shared repo-agnostic receiver, per-user state directory, cursor-based incremental Copilot usage, version-aware replacement. Per-repo ports (option B below) stay rejected. Implementation proceeds after task 0.1.
2. Old-receiver eviction is **automatic**, not manual. A new binary evicts a running pre-handshake receiver on its own (Decision 2, step 6), under strict safety rules. `--replace` remains as an explicit manual override.
3. State directory is `os.UserCacheDir()/dreamland/otel`, override `DREAMLAND_STATE_DIR`.
4. Defaults adopted for items the user left unanswered (stated plainly so they can be overturned):
   - **No legacy mailbox read.** The new collector never reads repo-local `<repo>/.dreamland/otel-sessions/<id>.json`. Sessions in flight at upgrade lose only their OTel-fallback tokens.
   - **Equal revision means no replacement.** Two receivers with the same `ReceiverRevision` but different `build` values are not replaced; dreamland developers bump `ReceiverRevision` or use `--replace`.
5. Evidence status: the multi-repo problem was **derived from reading the code**, not from a live incident; nobody has observed a failure, and it is not known whether one would hit the OTel path or the chat-session-log path. Task 0.1 stays as the empirical gate before implementation.

## Context (verified in code)

- `cmd/otel_receiver.go` `runOtelReceiver`: loads config, resolves `repoRoot` via `config.FindRepoRoot(cwd)`, computes `addr := otelReceiverAddr(cfg.OtelEndpoint)`. `--foreground` runs `http.Server{Handler: otelreceiver.Handler(repoRoot)}`. Without it, `net.DialTimeout` probes `addr`; if anything answers it returns nil; otherwise it spawns `dreamland otel-receiver --foreground` with `child.Dir = repoRoot`, detached (`Setsid` on unix, `DETACHED_PROCESS` on Windows).
- `internal/telemetry/otelreceiver/receiver.go`: `Handler(repoRoot)` serves `POST /v1/traces` and a catch-all `/` that returns 200. `processSpan` reads only span-level attributes: `gen_ai.conversation.id`, `gen_ai.usage.input_tokens`, `gen_ai.usage.output_tokens`, `gen_ai.usage.cache_read.input_tokens`, `gen_ai.response.model`/`gen_ai.request.model`. **Resource attributes are never read**, and every existing test builds `Resource: &resourcepb.Resource{}` (empty). So there is no evidence in code or tests that Copilot sends a workspace/repo attribute, and this design does not depend on one. `writeSessionUsage` overwrites `<repo>/.dreamland/otel-sessions/<cid>.json`; `<cid>` is joined into the path unvalidated.
- `internal/telemetry/tools/copilot.go` `CopilotCollector.Collect`: source 1 is the VS Code chat-session log (`findChatSessionFile`/`parseChatSessionTokens`, whole-file sum); source 2, only if source 1 found nothing, is `otelreceiver.ReadSessionUsage(cfg.RepoRoot, sessionID)`; source 3 is `ParseTranscript`. The comment there records that OTel export was empirically observed to deliver nothing in one environment; the receiver is a fallback, which bounds the blast radius of both the bug and this fix. It is still the only source that works when the chat-session file is not found, so it must be correct.
- `internal/telemetry/snapshot.go` `Write`: adds the returned token counts to whatever `.dreamland-session.json` already holds. Collectors must therefore return *deltas*.
- `config.FindRepoRoot` walks up looking for `.git` (file or directory), so each git worktree is its own repo root with its own `.dreamland-session.json`, `.dreamland.json`, `.vscode/settings.json`.
- Copilot's endpoint (`github.copilot.chat.otel.otlpEndpoint`) is written by `dreamland init` into workspace `.vscode/settings.json`.
- Nothing in this section was observed live; it is all read from code (see "Status and recorded decisions", item 5).

## Decision 1: one shared receiver, no routing, user-level mailboxes (recommended)

Options considered:

| | A. Shared receiver, routing table registered at SessionStart | B. Per-repo port (hash of repo root) in `otlpEndpoint` | **C. Shared receiver, repo-agnostic mailboxes keyed by conversation id (chosen)** |
| --- | --- | --- | --- |
| Span to repo attribution | receiver looks up `cid -> repo` | implicit (port = repo) | done by the *consumer*, which knows its repo and its `session_id` |
| Needs SessionStart to carry the session id | yes (and it must arrive before the first span) | no | no |
| Stale state | registrations, must be GC'd | dead per-repo receivers, one process per repo | old mailboxes only (TTL GC) |
| Worktrees | ok | **broken by default**: `.vscode/settings.json` is a copied/committed file, so every worktree inherits the port hashed from the *main* checkout; each worktree would need its own `init` | ok |
| Port collisions / firewalls | one port | hash collisions with other software, ephemeral-range conflicts, N ports | one port |
| Changes user-visible settings | no | yes: re-run `dreamland init` in every checkout | no |
| Failure mode | span arrives before registration is lost/misfiled | wrong port after moving/cloning a repo | none new |

Why C: `gen_ai.conversation.id` is already the join key between the span and the hook payload's `session_id` (spec: "which matches the hook payload's `session_id`"), and it is a UUID, globally unique. The receiver needs no repo knowledge at all; attribution happens where the repo is already known. That removes the registration step, its ordering race, and all stale-registration cleanup. B is rejected mainly because of worktrees and because it requires touching every checkout's Copilot settings.

Trade-offs accepted with C:

- Mailboxes for all repos live in one per-user directory, so any local process of the same OS user can read token counts of any repo. Token counts are not sensitive at that level, and the same user already owns every repo. Different OS users on one machine are out of scope (they would collide on the port anyway).
- If Copilot's conversation id ever stops matching the hook `session_id`, attribution fails (same as today). Falling back to a workspace/resource attribute is not possible until one is observed; task 0.1 captures a real export to check.

State directory: `$DREAMLAND_STATE_DIR` if set, else `os.UserCacheDir()/dreamland/otel` (`~/Library/Caches`, `~/.cache`, `%LocalAppData%`). Transient data only, so a cache dir is appropriate. The receiver and the collector must resolve the same directory; `DREAMLAND_STATE_DIR` exists for tests and is reported in the health response so a mismatch is diagnosable.

```
<state>/sessions/<cid>.json        receiver-owned cumulative totals (mailbox v2)
<state>/receiver-<port>.json       advisory pid file {pid, addr, revision, build, started_at}
<state>/receiver-<port>.lock       O_EXCL start lock (stale after 15 s)
<state>/receiver.log               request log (rotated at 5 MiB to receiver.log.1)
<repo>/.dreamland/otel-cursors/<session_id>.json   consumer-owned cursor
```

Files are keyed by port in the pid/lock names so two repos configured with different `otel_endpoint` ports each get their own receiver without interfering (each receiver is still repo-agnostic).

## Decision 2: version/health handshake and replacement

Problem: the port is the only identity today, so a stale receiver of any age blocks the new binary.

- `otelreceiver.ReceiverRevision` is an integer constant, bumped whenever handler behavior or mailbox format changes. This change sets it to 2 (pre-handshake receivers are "revision 1" by definition). `buildCommit` (already in `cmd/version.go`) is reported for diagnostics only.
- `GET /.dreamland/health` -> `200 {"service":"dreamland-otel-receiver","revision":2,"build":"<sha|unknown>","pid":1234,"state_dir":"..."}`.
- Start algorithm (`dreamland otel-receiver`, non-foreground), all steps best-effort; the command always exits 0 so the SessionStart hook chain never fails:
  1. Take `<state>/receiver-<port>.lock` (`O_EXCL`; if it exists and is older than 15 s, remove and retry once; if it exists and is fresh, another start is in progress, exit 0).
  2. Probe `GET http://<addr>/.dreamland/health` with a 500 ms timeout.
  3. Connection refused: remove any stale `receiver-<port>.json` (pid dead), spawn.
  4. Health OK and `revision >= ours`: no-op. (Never replace newer with older: two worktrees on different binaries would otherwise flap. Equal revision with a different `build` is also a no-op; developers of dreamland itself use `--replace`.)
  5. Health OK and `revision < ours`: `terminateProcess(pid)`, poll dial for the port to free (up to 3 s), spawn. If the port does not free, log to stderr and exit 0.
  6. Something answers but health is 404, non-JSON, or has the wrong `service`: an unidentified listener (this includes every pre-handshake dreamland receiver, whose catch-all `/` handler answers 200 without a health document). Run the **eviction check**, which requires all of the following, in order:
     1. `lookupListenerPID(port)` (`lsof -nP -iTCP:<port> -sTCP:LISTEN -t` on unix; `netstat -ano` parsing on Windows) returns exactly one distinct pid (the same pid listed for IPv4 and IPv6 is deduplicated; two or more distinct pids, or none, is a failure to identify).
     2. `processCommandLine(pid)` (`ps -o comm=` plus `ps -o args=` on unix; PowerShell `Get-CimInstance Win32_Process` on Windows) returns the executable path and arguments, and `isDreamlandReceiver(exe, args)` is true: the executable's basename (case-insensitive, `.exe` stripped) is exactly `dreamland` and `otel-receiver` is one of the arguments as a whole argument. `dreamland-foo`, `vim otel-receiver.md`, `node ... dreamland otel-receiver` (executable is `node`) and `go run` wrappers do not match.
     If both hold: `terminateProcess(pid)`, poll dial for the port to free (up to 3 s), spawn the current binary (same as step 5), and write one stderr line noting the eviction (`dreamland: evicted pre-handshake dreamland receiver (pid <n>) on port <p>`). If the port does not free, one stderr line and exit 0.
     If either fails: **leave the listener alone**, write exactly one stderr line (`dreamland: port <p> is held by a process that is not a dreamland receiver or could not be identified; leaving it alone (use 'dreamland otel-receiver --replace' to override for a dreamland receiver)`), and exit 0. A missing `lsof`, a permission failure (a listener owned by another user is invisible to `lsof`), a timeout, unparseable output, or an unreadable command line are all this branch, never an error and never a kill.
  Safety invariant for steps 5 and 6 and `--replace`: a pid is signalled only when `isDreamlandReceiver` has just accepted that pid's command line. The same check applies to a pid reported by a health response (step 5), so a stale or spoofed health `pid` cannot cause an unrelated process to be killed.
  7. Release the lock.
- `--replace`: explicit manual override. Skips the revision comparison of step 4 (replaces an equal or newer receiver too) and, for an unidentified listener, runs the same eviction check as step 6 (the safety invariant is not bypassed: `--replace` never signals a process that fails `isDreamlandReceiver`). Since step 6 now evicts pre-handshake receivers automatically, `--replace` is needed only to replace an equal-or-newer receiver, for example while developing dreamland itself. When lookup tooling is missing it prints manual instructions and exits 0.
- Liveness: `processAlive(pid)` is `syscall.Kill(pid, 0)` on unix and `OpenProcess`+`GetExitCodeProcess == STILL_ACTIVE` on Windows. A pid file whose pid is dead or whose port is closed is deleted by the next start. The pid file is never trusted over the health response.
- Foreground receiver: binds with `net.Listen` (a bind failure with `EADDRINUSE` exits 0 silently, which resolves a race between two starters that both passed the probe), writes the pid file only after a successful bind, removes it on graceful exit, handles SIGTERM/Interrupt (`srv.Shutdown` with a 2 s timeout). On Windows only `os.Interrupt` exists; `terminateProcess` there is `Process.Kill`, so the pid file may be left behind, which is why staleness is checked by liveness.

Rejected: an HTTP `POST /shutdown` endpoint. A loopback HTTP shutdown is reachable by any web page via `fetch('http://localhost:4318/...')`, i.e. cross-site request forgery against a local process. Pid-based termination avoids adding an unauthenticated control surface.

## Decision 3: cumulative mailbox plus consumer cursor (covers the related double-count bug)

Current behavior: `writeSessionUsage` overwrites; `Write` accumulates. Two failure modes: (a) a session with N spans keeps only the last; (b) if no new span arrived between two Stops, the same mailbox is added twice.

Chosen design (in scope, because the mailbox is being moved and reformatted anyway and correct multi-session attribution is the stated goal):

- Receiver (single owner of `<state>/sessions/<cid>.json`): read-modify-write under a process-wide mutex; add each qualifying span's `input`, `output`, `cache_read` to running totals; keep `model` = latest non-empty; `span_count++`; write via temp file + rename (retry up to 3x with 20 ms backoff on rename failure, for Windows sharing violations). Deduplicate by span id with an in-memory bounded set per conversation (4096 ids), which absorbs exporter retries; a receiver restart forgets the set, an accepted small risk.
- Only spans with `gen_ai.operation.name == "invoke_agent"` (fallback: span name has prefix `invoke_agent`) are counted. Rationale: GenAI semconv can put usage on both an agent-level span and its child `chat` spans; summing both would double count. **This filter is an assumption to be confirmed by task 0.1** against a real capture (span names are already logged by `logRequest`); if usage turns out to live only on `chat` spans, only the filter constant changes.
- Consumer (`CopilotCollector`, source 2): read totals `T` from the state-dir mailbox and cursor `C` from `<repo>/.dreamland/otel-cursors/<session_id>.json` (missing = zero). Return `delta = max(T - C, 0)` per field. Advance the cursor to `T` inside `Collect` (before `telemetry.Write`; if `Write` later fails that delta is lost, acceptable for best-effort observability and documented). Cursor lives in the repo (consumer-owned, gitignored) so the receiver and collector never write the same file and there is no cross-process locking.
- Prune: collector deletes cursors older than 7 days in its repo on each call; receiver GC deletes `sessions/*.json` older than 7 days at start and hourly, and `*.tmp` older than 1 h.
- Two sessions in the *same* repo still accumulate into one `.dreamland-session.json` (existing behavior: it is the running total for the next commit). With deltas this is now correct rather than accidentally correct.

Alternative rejected: making the receiver track "consumed" state and reset on read. It couples the receiver to consumers, needs an RPC or shared write access, and breaks when two consumers (SubagentStop and Stop) read the same session.

The cursor package (`internal/telemetry/cursor`) is generic (`Load(repo, id) (Counts, error)`, `Store(repo, id, Counts) error`) so the follow-up change can apply it to the other cumulative sources.

## Scope: related bugs

- **In scope**: mailbox double-count and last-span-only (Decision 3); conversation-id path validation; log/receiver cwd pinning a worktree.
- **Out of scope (default, unchanged)**: `ClaudeCollector` re-sums the entire transcript on every Stop and `Write` accumulates, inflating totals. It shares the accumulation contract with this change but not a code path or a file. Fixing it needs a Claude-side cursor (transcript path + last line/byte offset) and a decision on backfilling existing inflated `.dreamland-session.json` files. Recommended as a separate change (`telemetry-delta-accounting`) that also covers the Copilot chat-session-log source (`parseChatSessionTokens` has the identical whole-file re-sum) using the cursor package from this change. This change does not touch either.

## Windows parity checklist

- `os.UserCacheDir()` yields `%LocalAppData%`; use `filepath` everywhere; conversation-id validation removes `:`/`\` characters, which matters on Windows.
- `detachProcess` already uses `DETACHED_PROCESS`; add `CREATE_NEW_PROCESS_GROUP` (0x200) so a Ctrl+C in the parent console does not reach the receiver. Verify with `GOOS=windows go vet ./...` in CI and one manual run.
- `processAlive`, `terminateProcess` implemented in `otel_receiver_windows.go` (`golang.org/x/sys/windows` if already a dependency, else `syscall.OpenProcess`); no `lsof`, no signals.
- The eviction check (Decision 2 step 6) and `--replace` use `netstat -ano` and PowerShell `Get-CimInstance`. If either is unavailable it is an identification failure: one stderr line, nothing killed, exit 0.
- Rename-over-existing and open-file sharing: retry loop (Decision 3).

## Risks

- **Span semantics unverified**: whether `invoke_agent` spans are per-turn deltas, cumulative, or include child agent spans is not established by the code or tests; a cumulative-to-date span would over-count when summed. Task 0.1 must settle it before implementation; if spans are cumulative the receiver should keep `max` instead of `sum`, an isolated change in `processSpan`.
- Auto-eviction kills a process. Mitigations: pid must come from an OS listener lookup that yields exactly one pid, and its executable basename must be `dreamland` with `otel-receiver` as an argument; anything else is left alone (Decision 2 step 6). Residual risk: a human-started `dreamland otel-receiver --foreground` on the same port for another purpose would be evicted (and replaced by an equivalent current receiver, so functionally harmless). A dreamland receiver owned by another OS user is invisible to `lsof`, so it is left alone with the stderr line.
- Cost: every SessionStart that finds an unidentified listener runs `lsof`/`netstat` and `ps` (tens of ms); only in that branch, never on the healthy path.
- Equal-revision different-build no-op (recorded default) means a dreamland developer must bump `ReceiverRevision` (or `--replace`) to test receiver changes.
- No legacy mailbox read (recorded default): sessions in flight at upgrade lose OTel-fallback tokens for that session only.
- Same-repo multi-session `.dreamland-session.json` contention (two writers doing read-modify-write in `telemetry.Write`) is pre-existing and untouched.
