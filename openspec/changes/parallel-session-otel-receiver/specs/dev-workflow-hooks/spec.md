## MODIFIED Requirements

### Requirement: A local OTLP/HTTP receiver captures GitHub Copilot's real token usage

Neither GitHub Copilot's `SubagentStop`/`Stop` hook payload nor its transcript file (referenced by `transcript_path`) expose token usage — confirmed empirically by parsing a complete, real captured transcript: no token or usage field exists anywhere in it. Copilot's only OTel-derived source of token-usage data is its native OpenTelemetry export (`github.copilot.chat.otel.*`, already configured by `dreamland init`), which follows the OTel GenAI Semantic Conventions: an `invoke_agent` trace span carries `gen_ai.usage.input_tokens`, `gen_ai.usage.output_tokens`, `gen_ai.usage.cache_read.input_tokens`, `gen_ai.request.model`/`gen_ai.response.model`, and `gen_ai.conversation.id` (which matches the hook payload's `session_id`). `dreamland otel-receiver` SHALL implement a minimal OTLP/HTTP trace-export endpoint to capture that data locally.

The receiver SHALL be a single shared process per listen address that is independent of any repository: it SHALL NOT be bound to a repo root, SHALL NOT read or write any path under a repository, and SHALL serve spans from any number of concurrent sessions in any number of repositories and git worktrees.

`dreamland otel-receiver` SHALL implement a `POST /v1/traces` endpoint (accepting both `application/x-protobuf` and `application/json` bodies) at the same host:port `github.copilot.chat.otel.otlpEndpoint` is configured to use. For each span whose `gen_ai.operation.name` is `invoke_agent` (or, when that attribute is absent, whose span name begins with `invoke_agent`), carrying a valid `gen_ai.conversation.id` and non-zero `gen_ai.usage.*` attributes, it SHALL add the span's token counts to a cumulative per-session mailbox at `<state-dir>/sessions/<conversation_id>.json`, where `<state-dir>` is `$DREAMLAND_STATE_DIR` if set, otherwise `<os.UserCacheDir()>/dreamland/otel`. The mailbox SHALL contain `version` (2), `model` (most recent non-empty), `input_tokens`, `output_tokens`, `cached_tokens` (running totals across all counted spans), `span_count`, and `captured_at`. A span whose span id was already counted for that conversation by the running receiver SHALL NOT be counted again. A `gen_ai.conversation.id` is valid only if it matches `^[A-Za-z0-9._-]{1,128}$` and is not `.` or `..`; spans with any other id SHALL be ignored (and noted in the log) and SHALL NOT cause any file to be written.

The receiver SHALL write its request log to `<state-dir>/receiver.log`, rotating to `receiver.log.1` when it exceeds 5 MiB. It SHALL delete `sessions/*.json` files whose modification time is older than 7 days at startup and once per hour, and SHALL delete leftover `*.tmp` files older than one hour.

The command SHALL be idempotent and non-blocking: invoked without `--foreground` (as bound to the `SessionStart` hook), it SHALL follow the start algorithm in the "Receiver start is version-aware" requirement and return without waiting for the child, so the `SessionStart` hook is never blocked and never fails. The detached child (`--foreground --addr <host:port>`) SHALL NOT require to be inside a git repository and SHALL run with its working directory set to `<state-dir>`, never a repository or worktree directory.

`CopilotCollector` (the `dreamland telemetry write --tool github-copilot` collector) SHALL read `<state-dir>/sessions/<session_id>.json` (using the `session_id` from its own hook payload) and SHALL prefer that data over transcript parsing whenever present, subject to the incremental-reporting requirement in `otel-session-telemetry`.

#### Scenario: Receiver captures token usage from a real trace export

- **WHEN** `dreamland otel-receiver` is running and receives a `POST /v1/traces` request containing an `invoke_agent` span with `gen_ai.conversation.id` and non-zero `gen_ai.usage.input_tokens`/`gen_ai.usage.output_tokens`
- **THEN** `<state-dir>/sessions/<conversation_id>.json` is written with those token counts and the resolved model

#### Scenario: Multiple spans for one session are summed

- **WHEN** two `invoke_agent` spans with different span ids and the same `gen_ai.conversation.id` arrive, with 100/10 and 40/5 input/output tokens
- **THEN** the mailbox holds `input_tokens: 140`, `output_tokens: 15`, `span_count: 2`

#### Scenario: A re-delivered span is not double counted

- **WHEN** the same `POST /v1/traces` body (same span ids) is delivered twice
- **THEN** the mailbox totals equal those of a single delivery

#### Scenario: Non-agent spans are not counted

- **WHEN** a span with `gen_ai.operation.name` of `chat` carries `gen_ai.usage.*` attributes
- **THEN** no mailbox is created or changed by it

#### Scenario: Sessions in different repos share one receiver

- **WHEN** one receiver is running and spans for conversation `A` (from a session in repo X) and conversation `B` (from a session in repo Y) arrive interleaved
- **THEN** `<state-dir>/sessions/A.json` and `<state-dir>/sessions/B.json` each contain only their own conversation's totals, and no file is created under either repository

#### Scenario: Sessions in different worktrees of one repo share one receiver

- **WHEN** sessions run in two git worktrees of the same repository and both export spans to the same address
- **THEN** each conversation id gets its own mailbox and each worktree's `telemetry write` reads only its own session's mailbox

#### Scenario: Unsafe conversation id is rejected

- **WHEN** a span arrives with `gen_ai.conversation.id` of `../../evil` or an empty string
- **THEN** no file is written anywhere and the export still receives a valid OTLP success response

#### Scenario: Old mailboxes are garbage collected

- **WHEN** the receiver starts and `<state-dir>/sessions/old.json` has a modification time 8 days ago while `new.json` was modified 1 day ago
- **THEN** `old.json` is deleted and `new.json` is retained

#### Scenario: Foreground receiver works outside any repository and does not pin a worktree

- **WHEN** `dreamland otel-receiver --foreground --addr localhost:4318` runs with a cwd that is not inside a git repository
- **THEN** it starts serving and its process working directory is `<state-dir>`

#### Scenario: Telemetry write prefers OTEL-captured usage over transcript parsing

- **WHEN** `dreamland telemetry write --tool github-copilot` runs and `<state-dir>/sessions/<session_id>.json` exists for the current hook payload's `session_id`, and no VS Code chat-session log source supplied usage
- **THEN** the resulting snapshot's token counts come from that mailbox (as a delta, see `otel-session-telemetry`), not from parsing `transcript_path`

## ADDED Requirements

### Requirement: Receiver start is version-aware and replaces stale receivers

The receiver SHALL expose `GET /.dreamland/health` returning HTTP 200 with a JSON body containing `service` (`"dreamland-otel-receiver"`), `revision` (the integer `otelreceiver.ReceiverRevision`, incremented whenever handler behavior or mailbox format changes; this change sets it to 2), `build` (the build commit or `"unknown"`), `pid`, and `state_dir`.

`dreamland otel-receiver` (without `--foreground`) SHALL, under an exclusive start lock at `<state-dir>/receiver-<port>.lock` (a lock older than 15 seconds is stale and SHALL be removed; a fresh lock held by another starter SHALL cause an immediate exit 0), probe `GET /.dreamland/health` at the target address with a 500 ms timeout and then:

- if the connection is refused, remove any stale `<state-dir>/receiver-<port>.json` and spawn a detached foreground receiver;
- if the response identifies a dreamland receiver with `revision` greater than or equal to the running binary's `ReceiverRevision`, do nothing;
- if it identifies a dreamland receiver with a lower `revision`, terminate the reported `pid` (subject to the process-identification safety rule below), wait up to 3 seconds for the port to be released, then spawn a detached foreground receiver of the current binary;
- if something answers but the health probe does not identify a dreamland receiver (404, non-JSON body, or a `service` other than `dreamland-otel-receiver`; this includes every pre-handshake dreamland receiver), attempt automatic eviction: resolve the listener's pid with an operating-system lookup (`lsof` on Unix, `netstat -ano` on Windows) and read that process's executable and arguments; if exactly one distinct pid is found and the process is a dreamland receiver, terminate it, wait up to 3 seconds for the port to be released, and spawn a detached foreground receiver of the current binary, writing one stderr line stating that a pre-handshake receiver was evicted; otherwise leave the listener untouched and write exactly one stderr line naming the port and stating that the listener is not a dreamland receiver or could not be identified.

Process-identification safety rule: the command SHALL send a termination signal to a process only if that process's executable basename is exactly `dreamland` (`dreamland.exe` on Windows, case-insensitive on Windows) and `otel-receiver` is one of its arguments (a whole argument, not a substring), determined immediately before signalling from the operating system rather than from any value supplied by the listener. This rule applies to a pid reported in a health response, a pid resolved by listener lookup, and `--replace`. A process that fails the rule, an ambiguous lookup (zero or more than one distinct listener pid), and any failure of the lookup or command-line read (missing tool, permission denied, timeout, unparseable output) SHALL result in no signal being sent, never a non-zero exit, and never an error other than the single stderr line.

The command SHALL exit 0 in every case above, including failure to identify, terminate, or spawn, so that hook chains never fail. `dreamland otel-receiver --replace` is an explicit manual override: it SHALL terminate the dreamland receiver holding the port regardless of its `revision` (including an equal or newer one, resolving its pid from the health response or the operating-system listener lookup) and then spawn the current binary's receiver, and it remains subject to the process-identification safety rule.

The foreground receiver SHALL write `<state-dir>/receiver-<port>.json` (pid, addr, revision, build, started_at) only after successfully binding, SHALL remove it on graceful shutdown (SIGTERM or interrupt), and SHALL exit 0 silently when the bind fails with address-in-use. A pid file whose pid is not alive, or whose port refuses connections, SHALL be treated as stale and removed by the next start; liveness and the health response, never the pid file alone, decide whether a receiver is running. The receiver SHALL NOT expose any unauthenticated endpoint that terminates the process.

#### Scenario: Newer binary replaces older receiver

- **WHEN** a receiver reporting `revision: 1` (with a pid) holds the port and `dreamland otel-receiver` runs from a binary whose `ReceiverRevision` is 2
- **THEN** the old pid is terminated, the port is freed, and a new detached receiver reporting `revision: 2` is listening

#### Scenario: Older binary does not replace a newer receiver

- **WHEN** a receiver reporting `revision: 3` holds the port and `dreamland otel-receiver` runs from a binary whose `ReceiverRevision` is 2
- **THEN** nothing is terminated or spawned and the command exits 0

#### Scenario: Same revision is a no-op

- **WHEN** a receiver reporting the same `revision` (even with a different `build`) holds the port
- **THEN** the command exits immediately without spawning a second instance

#### Scenario: Receiver is idempotent across repeated SessionStart invocations

- **WHEN** `dreamland otel-receiver` runs and a current-revision receiver is already listening at the configured address
- **THEN** it exits immediately without spawning a second instance

#### Scenario: Two repos start concurrently

- **WHEN** `dreamland otel-receiver` runs at the same moment from repo X and repo Y with no receiver running
- **THEN** exactly one receiver process ends up listening, both commands exit 0, and neither repository contains an `otel-sessions` or `otel-receiver.log` file

#### Scenario: Pre-handshake dreamland receiver is evicted automatically

- **WHEN** a receiver started by an older binary (no `/.dreamland/health` document; the request returns 200 with a non-health body or 404) holds the port, the OS lookup returns exactly one pid for that port, and that process's executable is `.../dreamland` with the argument `otel-receiver`, and `dreamland otel-receiver` runs without `--replace`
- **THEN** that pid is terminated, the port is freed, a new detached receiver reporting the current `revision` is listening, one stderr line reports the eviction, and the command exits 0

#### Scenario: Non-dreamland listener on the port is left alone

- **WHEN** a process whose executable is not `dreamland` (for example `node`, `dreamland-helper`, or `python` with an argument `otel-receiver.py`) holds the port and returns no valid health document
- **THEN** no signal is sent to it, nothing is spawned, exactly one stderr line states that the listener is not a dreamland receiver or could not be identified, and the command exits 0

#### Scenario: Identification failure is non-fatal

- **WHEN** a listener with no valid health document holds the port and the listener lookup fails (tool missing, permission denied, timeout, unparseable output, no pid found, or two distinct pids found) or the command-line read fails
- **THEN** no signal is sent, exactly one stderr line is written, and the command exits 0 without spawning

#### Scenario: A health-reported pid is verified before it is signalled

- **WHEN** a health response reports `revision: 1` and a `pid` whose command line is not a dreamland `otel-receiver` process
- **THEN** no signal is sent, one stderr line is written, and the command exits 0

#### Scenario: Stale pid file is cleaned up

- **WHEN** `receiver-<port>.json` names a pid that is not alive and the port refuses connections
- **THEN** the next `dreamland otel-receiver` removes the file and starts a receiver

#### Scenario: Replace overrides the revision comparison

- **WHEN** a dreamland receiver reporting the same or a newer `revision` holds the port and `dreamland otel-receiver --replace` runs
- **THEN** that receiver's pid is terminated (after passing the process-identification safety rule), the port is freed, and a new detached receiver of the current binary is listening

#### Scenario: Replace refuses to signal an unrelated process

- **WHEN** `dreamland otel-receiver --replace` resolves the listener's pid and that process is not a dreamland `otel-receiver` process under the process-identification safety rule
- **THEN** no signal is sent, a message is written to stderr, and the command exits 0

### Requirement: Receiver process control works on Windows and Unix

Process liveness and termination used by the receiver start algorithm SHALL be implemented in platform files: `otel_receiver_unix.go` using signal 0 for liveness and SIGTERM for termination, and `otel_receiver_windows.go` using `OpenProcess`/`GetExitCodeProcess` for liveness and `Process.Kill` for termination. The detached child on Windows SHALL be created with `DETACHED_PROCESS | CREATE_NEW_PROCESS_GROUP`. State paths SHALL be built with `filepath` and resolve to `%LocalAppData%\dreamland\otel` on Windows by default. Mailbox writes SHALL retry a failed rename up to 3 times with 20 ms backoff. The command package SHALL compile for `GOOS=windows`.

#### Scenario: Windows build compiles

- **WHEN** `GOOS=windows go build ./...` runs
- **THEN** it succeeds, and `processAlive` and `terminateProcess` are defined for windows

#### Scenario: Liveness of a dead pid

- **WHEN** `processAlive` is called with a pid that has exited
- **THEN** it returns false on both Unix and Windows

#### Scenario: Windows rename contention is tolerated

- **WHEN** the first rename of a mailbox temp file fails and the second succeeds
- **THEN** the mailbox is updated and no error is surfaced to the exporter
