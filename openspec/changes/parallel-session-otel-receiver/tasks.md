## 0. Empirical verification (blocks 3.x and 5.x)

- [ ] 0.1 With a real Copilot session, capture one full export from the current receiver's `.dreamland/otel-receiver.log` (span names are already logged) plus a raw body dump. Record in `design.md` "Risks": (a) span names and `gen_ai.operation.name` values present; (b) which span type carries `gen_ai.usage.*` (agent-level `invoke_agent` vs child `chat`); (c) whether `invoke_agent` usage is per-turn or cumulative-to-date, and whether nested subagent `invoke_agent` spans include their children; (d) whether `gen_ai.conversation.id` equals the hook payload `session_id`; (e) whether any resource attribute identifies the workspace (informational only, not depended on). If (b) or (c) contradict Decision 3, update the filter constant / switch sum to max before proceeding.

## 1. State directory and cursor helpers

- [ ] 1.1 Add `internal/telemetry/otelreceiver/state.go`: `StateDir() string` (`$DREAMLAND_STATE_DIR` else `filepath.Join(os.UserCacheDir(), "dreamland", "otel")`, falling back to `os.TempDir()/dreamland-otel` if `UserCacheDir` errors), `ValidConversationID(id string) bool` (`^[A-Za-z0-9._-]{1,128}$`, not `.`/`..`), `SessionsDir`, `LogPath`, `PidFilePath(port)`, `LockPath(port)`.
- [ ] 1.2 Add `internal/telemetry/cursor` package: `type Counts struct{Input, Output, Cached int64}`, `Load(repoRoot, id string) (Counts, bool, error)`, `Store(repoRoot, id string, c Counts) error` (atomic temp+rename with the 3x/20 ms retry), `Prune(repoRoot string, olderThan time.Duration)`, paths under `<repo>/.dreamland/otel-cursors/`.
- [ ] 1.3 Add `.dreamland/otel-cursors/` next to the existing `.dreamland/otel-sessions/` and `.dreamland/otel-receiver.log` ignore entries wherever init/scaffold writes them (start from `grep -rn "otel-sessions" internal cmd .gitignore`); keep the old entries so existing checkouts stay clean.
- [ ] 1.4 Unit tests for 1.1 (env override, invalid ids including `../x`, `.`, empty, 129 chars, `a:b`) and 1.2 (missing file, round trip, prune by mtime, concurrent Store).

## 2. Receiver package (`internal/telemetry/otelreceiver/receiver.go`)

- [ ] 2.1 Add `const ReceiverRevision = 2` with a comment stating when to bump it.
- [ ] 2.2 Change `Handler(repoRoot string)` to `Handler(stateDir string)`; thread `stateDir` through `handleTraces`, `logRequest`, `processSpan`, `writeSessionUsage`, `ReadSessionUsage(stateDir, cid)`. Update `sessionPath` to `<stateDir>/sessions/<cid>.json`, `logRequest` to `<stateDir>/receiver.log`.
- [ ] 2.3 In `processSpan`: skip spans that are not `invoke_agent` (per 0.1 outcome), skip when `!ValidConversationID(cid)` (log note `invalid conversation id`), keep the zero-usage skip.
- [ ] 2.4 Make `writeSessionUsage` cumulative: process-wide `sync.Mutex`, read existing mailbox, add tokens, `span_count++`, `version: 2`, latest non-empty model, temp+rename with retry. Add a bounded per-conversation span-id set (4096, evict oldest) and skip spans already seen. Update `SessionUsage` with `Version` and `SpanCount` fields (JSON `version`, `span_count`).
- [ ] 2.5 Add `GET /.dreamland/health` handler returning `{service, revision, build, pid, state_dir}`. `build` is injected via a package-level `Build string` variable that `cmd` sets from `buildCommit`.
- [ ] 2.6 Log rotation: before append, if `receiver.log` > 5 MiB rename to `receiver.log.1`.
- [ ] 2.7 `GC(stateDir string, now time.Time)`: delete `sessions/*.json` older than 7 days and `*.tmp` older than 1 h; export `StartGC(ctx, stateDir)` that runs it once immediately and hourly.
- [ ] 2.8 Tests (`receiver_test.go`; update every existing test to the `stateDir` signature): two spans sum; duplicate span id ignored; `chat` span ignored; invalid ids write nothing (assert directory listing empty); interleaved conversations A/B; health JSON shape; log rotation; GC by mtime (use `os.Chtimes`); rename-retry using an injectable rename seam; `-race` with 50 concurrent posts across 5 ids.

## 3. Command: version-aware start (`cmd/otel_receiver.go`)

- [ ] 3.1 Split `runOtelReceiver`: in non-foreground mode the repo is optional for anything but `cfg.OtelEndpoint` (if no repo/config, use the default endpoint; do not error). Add flags `--addr` (foreground only, required there) and `--replace`.
- [ ] 3.2 Foreground: `net.Listen("tcp", addr)`; on `EADDRINUSE` return nil; write `receiver-<port>.json` after bind; `signal.NotifyContext(SIGTERM, Interrupt)` -> `srv.Shutdown(2s)` -> remove pid file; call `otelreceiver.StartGC`; set `otelreceiver.Build = buildCommit`; `os.Chdir(stateDir)` at start.
- [ ] 3.3 Non-foreground: implement the start algorithm from the "Receiver start is version-aware" requirement: lock file (`O_EXCL`, stale after 15 s), health probe (500 ms), refused/older/newer/unidentified branches, terminate then poll up to 3 s, spawn with `child.Dir = otelreceiver.StateDir()` (never `repoRoot`) and args `otel-receiver --foreground --addr <addr>`. Always return nil. Keep the `osExecutable` seam; add seams for health probe, `terminateProcess`, `processAlive` so tests never signal real processes.
- [ ] 3.4 `--replace`: resolve pid from health, else `lookupListenerPID(port)`; verify command line via `processCommandLine(pid)` contains `dreamland` and `otel-receiver`; terminate, wait, spawn. Print manual instructions and return nil when lookup tooling is missing.
- [ ] 3.5 Tests in `cmd/otel_receiver_test.go` (fake health servers via `httptest`, injected seams): refused -> spawns; newer running -> no-op; equal -> no-op; older -> terminate called with reported pid then spawn; unidentified 200 -> no-op + stderr line containing `--replace`; stale pid file removed; fresh lock -> exit 0 no spawn; stale lock removed; concurrent starts spawn once (`-race`); `--replace` refuses unrelated command line; child `Dir` equals state dir and is not the repo; running outside a git repo does not error.

## 4. Platform process control

- [ ] 4.1 `cmd/otel_receiver_unix.go`: `processAlive(pid)` (`syscall.Kill(pid, 0)`, treat `EPERM` as alive), `terminateProcess(pid)` (SIGTERM), `lookupListenerPID(port)` (`lsof -nP -iTCP:<port> -sTCP:LISTEN -t`), `processCommandLine(pid)` (`ps -o command= -p`).
- [ ] 4.2 `cmd/otel_receiver_windows.go`: same four functions using `OpenProcess`/`GetExitCodeProcess`, `Process.Kill`, `netstat -ano` parsing, and PowerShell `Get-CimInstance Win32_Process`; `detachProcess` flags become `0x00000008 | 0x00000200`.
- [ ] 4.3 Tests: `otel_receiver_unix_test.go` for `processAlive` (self alive, exited child dead) and `terminateProcess` on a spawned `sleep`; pure-Go tests for the `netstat`/`lsof` output parsers (table-driven, run on all platforms); CI/`make` step `GOOS=windows go vet ./...` (add to the existing check target).

## 5. Collector (`internal/telemetry/tools/copilot.go`)

- [ ] 5.1 Replace `otelreceiver.ReadSessionUsage(cfg.RepoRoot, sessionID)` with `ReadSessionUsage(otelreceiver.StateDir(), sessionID)`; gate on `ValidConversationID(sessionID)`.
- [ ] 5.2 Compute the delta against `cursor.Load(cfg.RepoRoot, sessionID)` (`max(T-C, 0)` per field), return the delta, then `cursor.Store(..., T)`; call `cursor.Prune(cfg.RepoRoot, 7*24*time.Hour)`. On cursor failures follow the spec (warn, never fail). Model still comes from the mailbox. Update the `CopilotCollector` doc comment: source 2 is now incremental.
- [ ] 5.3 Tests in `collectors_test.go`: two writes with no new span add once; new span adds only the delta; cursor per session; unsafe id skips source and creates no cursor; corrupt cursor file warns and reports zero; `telemetry.Write` end-to-end (call `Collect` then `Write` twice and read `.dreamland-session.json`) proves no double count; source 1 (chat-session log) behavior unchanged.

## 6. Integration and docs

- [ ] 6.1 End-to-end test in `cmd` (build-tagged or skipped without loopback): start a foreground receiver on an ephemeral port with a temp `DREAMLAND_STATE_DIR`; post spans for sessions in two temp git repos; run the collector for each; assert each repo's `.dreamland-session.json` has only its own totals and neither repo has `.dreamland/otel-sessions` or `otel-receiver.log`.
- [ ] 6.2 Update the README/docs section about the receiver (state directory, `--replace`, one-time migration note that a pre-handshake receiver must be evicted once with `dreamland otel-receiver --replace`). Do not edit `openspec/specs/**` directly; the deltas in this change are merged on archive.
- [ ] 6.3 Update the comment in `otelreceiver/receiver.go` package doc and `internal/scaffold/otelenv.go` if they still say the receiver is per-repo.

## 7. Live verification (before archive)

- [ ] 7.1 Rebuild/reinstall the binary; run `dreamland otel-receiver --replace` once to evict the running pre-handshake receiver; confirm `curl localhost:4318/.dreamland/health` reports revision 2.
- [ ] 7.2 Open two different repos and two worktrees of one repo in VS Code with Copilot, run a turn in each in parallel; confirm one receiver process, four distinct `<state-dir>/sessions/*.json`, and each checkout's next commit carrying its own tokens.
- [ ] 7.3 Trigger two Stops without a new turn; confirm `.dreamland-session.json` does not grow. Bump `ReceiverRevision` in a scratch build and confirm SessionStart replaces the running receiver.
- [ ] 7.4 Run `openspec validate parallel-session-otel-receiver` and the repo's full test suite with `-race`.
