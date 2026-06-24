## ADDED Requirements

### Requirement: dreamland telemetry write command

The CLI SHALL expose a `dreamland telemetry write` subcommand. It reads the hook payload from stdin, applies tool-specific parsing logic, and writes or accumulates a normalized `SnapshotResult` to `.dreamland-session.json` at the repository root.

Required flag: `--tool <name>`. Recognized values: `claude-code`, `codex`, `cursor`, `kiro`, `antigravity`, `github-copilot`.

On each invocation, token counts SHALL be accumulated (summed) into the existing `.dreamland-session.json` rather than replaced, so multi-turn sessions produce a running total.

#### Scenario: Unknown tool name rejected
- **WHEN** `dreamland telemetry write --tool unknown-tool` is called
- **THEN** the command exits with code 1 and prints an error listing valid tool names

#### Scenario: Token counts accumulate across turns
- **WHEN** `dreamland telemetry write --tool claude-code` is called twice, with 3000 and 2000 input tokens parsed from respective transcripts
- **THEN** `.dreamland-session.json` contains `input_tokens: 5000` after the second call

---

### Requirement: Claude Code collector reads Stop hook stdin and transcript JSONL

For `--tool claude-code`, `dreamland telemetry write` SHALL read the Claude Code Stop hook JSON payload from stdin, which contains:
- `transcript_path` (string) — path to the session JSONL file at `~/.claude/projects/<hash>/<session-id>.jsonl`
- `effort` (object) — `{"level": "low"|"medium"|"high"|"xhigh"|"max"}` — maps to `thinking_effort`
- `session_id` (string)
- `stop_hook_active` (boolean)

The collector SHALL iterate every line in `transcript_path` whose `type` is `"assistant"` and accumulate:
- `message.usage.input_tokens` → `InputTokens`
- `message.usage.output_tokens` → `OutputTokens`
- `message.usage.cache_creation_input_tokens` + `message.usage.cache_read_input_tokens` → `CachedTokens`

`model` SHALL be read from `.dreamland.json` `model_id` (the Stop hook payload does not include the model name).
`thinking_effort` SHALL be read from `effort.level` in the Stop hook stdin, with `CLAUDE_EFFORT` environment variable as a fallback.

Because Claude Code writes transcript entries during streaming before token counts are finalized, the summed values are best-effort observability data, not billing-accurate counts.

#### Scenario: Claude Code Stop hook parses transcript for token totals
- **WHEN** `dreamland telemetry write --tool claude-code` runs with Stop hook stdin containing a valid `transcript_path` pointing to a JSONL with 2 assistant turns (1000+500 input, 200+100 output, 300 cache_read tokens)
- **THEN** `.dreamland-session.json` contains `input_tokens: 1500`, `output_tokens: 300`, `cached_tokens: 300`, `total_tokens: 1800`, `thinking_effort` from `effort.level`, `model` from `.dreamland.json`

#### Scenario: Missing transcript path handled gracefully
- **WHEN** the Stop hook stdin has no `transcript_path` or the file does not exist
- **THEN** `dreamland telemetry write` writes a `SnapshotResult` with zero token counts, populates `thinking_effort` and `model` where available, and exits 0

---

### Requirement: Codex CLI collector reads model from Stop hook stdin

For `--tool codex`, `dreamland telemetry write` SHALL read the Codex Stop hook JSON payload from stdin, which contains:
- `model` (string) — model identifier, directly available in the payload
- `session_id`, `turn_id`, `transcript_path`, `cwd`, `hook_event_name`, `permission_mode`, `stop_hook_active`, `last_assistant_message`

The collector SHALL extract `model` directly from stdin. It SHALL attempt to read `transcript_path` for token counts using the same JSONL parsing as the Claude Code collector. Because the Codex transcript format is explicitly documented as unstable (not a stable interface), all transcript reads SHALL be wrapped in error recovery: on any parse failure, token counts default to zero and a warning is written to stderr.

#### Scenario: Codex Stop hook extracts model directly from payload
- **WHEN** `dreamland telemetry write --tool codex` runs with Stop hook stdin containing `"model": "o4-mini"`
- **THEN** `.dreamland-session.json` contains `model: "o4-mini"`

#### Scenario: Codex transcript parse failure falls back gracefully
- **WHEN** `transcript_path` points to a file in an unrecognized format
- **THEN** `model` is still written from stdin, token counts are zero, a warning is printed to stderr, and the command exits 0

---

### Requirement: Cursor collector reads model and transcript from stop hook payload

The Cursor `stop` hook payload (confirmed from [cursor.com/docs/hooks](https://cursor.com/docs/hooks)) includes:
- `conversation_id`, `generation_id` (strings)
- `model` (string) — model name directly available
- `model_id` (string) — model identifier
- `model_params` (array) — model parameter key-value pairs
- `hook_event_name`, `cursor_version`, `workspace_roots`, `user_email` (strings)
- `transcript_path` (string | null) — path to session transcript when enabled
- `status` (`"completed"` | `"aborted"` | `"error"`)
- `loop_count` (integer)

Cursor also auto-sets `CURSOR_TRANSCRIPT_PATH` as an environment variable for all hooks.

For `--tool cursor`, `dreamland telemetry write` SHALL:
1. Read the stop hook JSON payload from stdin.
2. Extract `model` (or `model_id`) directly from stdin.
3. If `transcript_path` is non-null, call `transcript.ParseTranscript` for token counts using the same JSONL parser as Claude Code and Codex. Wrap in error recovery — parse failure yields zero tokens and a stderr warning.
4. Fall back to `CURSOR_TRANSCRIPT_PATH` env var if `transcript_path` is absent from stdin.
5. Write the `SnapshotResult` with `tool: "cursor"`, `model`, and token counts.

#### Scenario: Cursor stop hook extracts model and parses transcript
- **WHEN** `dreamland telemetry write --tool cursor` runs with stop payload containing `"model": "claude-sonnet-4-5"` and a valid `transcript_path`
- **THEN** `.dreamland-session.json` contains `model: "claude-sonnet-4-5"` and token counts summed from the transcript JSONL

#### Scenario: Cursor stop with null transcript_path falls back to env var
- **WHEN** `transcript_path` is null in the stop payload but `CURSOR_TRANSCRIPT_PATH` is set
- **THEN** `dreamland telemetry write` uses the env var path for transcript parsing

#### Scenario: Cursor stop with aborted status still writes snapshot
- **WHEN** the Cursor stop payload has `"status": "aborted"`
- **THEN** `dreamland telemetry write` still writes a snapshot; the aborted status is not an error

---

### Requirement: Kiro collector queries AWS Bedrock model invocation logs

Kiro runs on Amazon Bedrock. AWS Bedrock model invocation logging ([docs.aws.amazon.com/bedrock/latest/userguide/model-invocation-logging.html](https://docs.aws.amazon.com/bedrock/latest/userguide/model-invocation-logging.html)) records every API call with `modelId`, `input.inputTokenCount`, and `output.outputTokenCount` to a CloudWatch Logs group (`aws/bedrock/modelinvocations` by default).

Two hooks collaborate:
1. **`agentSpawn` hook** — runs `dreamland telemetry write --tool kiro --phase start`: writes `session_start_time` (RFC 3339 UTC) into `.dreamland-session.json`.
2. **`stop` hook** — runs `dreamland telemetry write --tool kiro --phase stop`: reads `session_start_time` from `.dreamland-session.json`, queries CloudWatch Logs for `ModelInvocationLog` entries between that timestamp and now, sums `inputTokenCount` and `outputTokenCount`, and extracts `modelId` from the most recent entry.

The CloudWatch query uses the `aws` CLI (already required for Kiro usage) rather than importing the AWS Go SDK:

```bash
aws logs filter-log-events \
  --log-group-name <bedrock_log_group> \
  --start-time <session_start_epoch_ms> \
  --filter-pattern '{ $.schemaType = "ModelInvocationLog" }' \
  --query 'events[*].message' \
  --output json
```

`bedrock_log_group` defaults to `aws/bedrock/modelinvocations` and is stored in `.dreamland.json`. Each element of the returned array is a JSON string containing a `ModelInvocationLog` record; the collector parses each and sums `input.inputTokenCount` and `output.outputTokenCount`.

**Prerequisites** — `dreamland init` for Kiro SHALL print a one-time notice:
1. Bedrock model invocation logging must be enabled: AWS Console → Bedrock → Settings → Model invocation logging → select CloudWatch Logs
2. `aws` CLI must be configured with credentials that have `logs:FilterLogEvents` on the log group

If `aws` is not on PATH, credentials are unavailable, the log group does not exist, or the CLI returns an error, the collector falls back to zero token counts and reads model from `.dreamland.json`, printing a stderr warning.

`modelId` from Bedrock logs is a full ARN such as `anthropic.claude-sonnet-4-20250514-v1:0`. The collector SHALL strip the provider prefix and version suffix to produce a short model string (e.g. `claude-sonnet-4`).

#### Scenario: Kiro agentSpawn records session start time
- **WHEN** `dreamland telemetry write --tool kiro --phase start` is called by the Kiro `agentSpawn` hook
- **THEN** `.dreamland-session.json` contains `session_start_time` set to the current RFC 3339 UTC timestamp

#### Scenario: Kiro stop hook queries Bedrock logs and writes real token counts
- **WHEN** `dreamland telemetry write --tool kiro --phase stop` is called and CloudWatch contains Bedrock log entries since `session_start_time`
- **THEN** `.dreamland-session.json` contains `input_tokens` and `output_tokens` summed from all matching log entries, and `model` parsed from `modelId`

#### Scenario: Kiro falls back gracefully when AWS credentials absent
- **WHEN** no AWS credentials are configured
- **THEN** `dreamland telemetry write --tool kiro --phase stop` writes a snapshot with zero token counts, model from `.dreamland.json`, and a stderr warning; it exits 0

#### Scenario: Kiro falls back gracefully when log group not configured
- **WHEN** `bedrock_log_group` is empty in `.dreamland.json` and the default log group does not exist
- **THEN** zero tokens are written, a stderr warning is printed, exit code is 0

---

### Requirement: Antigravity collector uses best-effort PostTurnHook payload parsing

The Antigravity `PostTurnHook` payload schema is not publicly documented. For `--tool antigravity`, `dreamland telemetry write` SHALL attempt to read JSON from stdin and extract any fields matching known token or model keys using lenient parsing. Unrecognized or absent fields default to zero/empty. `model` falls back to `.dreamland.json` `model_id`.

#### Scenario: Antigravity PostTurnHook writes best-effort snapshot
- **WHEN** `dreamland telemetry write --tool antigravity` runs with any stdin content
- **THEN** `.dreamland-session.json` is written with all available fields populated and unknowns zeroed; the command exits 0

---

### Requirement: GitHub Copilot write command is a documented no-op

For `--tool github-copilot`, `dreamland telemetry write` SHALL print a message to stderr explaining that no lifecycle hook is available and directing the user to the two supported telemetry paths:
1. OTel traces via `OTEL_EXPORTER_OTLP_ENDPOINT` (Copilot CLI SDK exports token counts as span attributes)
2. `token-usage.jsonl` artifact (one record per API call: `input_tokens`, `output_tokens`, `cache_read_tokens`, `cache_write_tokens`, `model`, `provider`, timestamps)

The command SHALL then exit 0 without writing to `.dreamland-session.json`.

#### Scenario: GitHub Copilot write command exits cleanly with guidance
- **WHEN** `dreamland telemetry write --tool github-copilot` is called
- **THEN** stderr contains guidance about OTel and token-usage.jsonl, stdout is empty, exit code is 0

---

### Requirement: dreamland telemetry snapshot command

The CLI SHALL expose a `dreamland telemetry snapshot` subcommand that reads `.dreamland-session.json` and outputs a normalized snapshot.

`--format` flag: `json` (default) or `trailers` (git-trailer format, one `AI-Key: value` line per non-empty field).

#### Scenario: JSON snapshot output
- **WHEN** `dreamland telemetry snapshot` is run and `.dreamland-session.json` exists
- **THEN** stdout contains a single JSON object matching the `SnapshotResult` schema

#### Scenario: Trailer format output
- **WHEN** `dreamland telemetry snapshot --format trailers` is run
- **THEN** stdout contains `AI-Key: value` lines for each non-zero/non-empty field

#### Scenario: Missing session file exits cleanly with no output
- **WHEN** `dreamland telemetry snapshot` is run and `.dreamland-session.json` does not exist
- **THEN** the command exits 0 and writes nothing to stdout

#### Scenario: Stale snapshot warning on stderr
- **WHEN** `dreamland telemetry snapshot` is run and `captured_at` is older than the configured staleness threshold (default 4 hours, configurable via `--max-age` flag)
- **THEN** a warning is written to stderr but the snapshot is still written to stdout

---

### Requirement: SnapshotResult data model

The normalized snapshot object SHALL conform to the following schema. Fields with zero or empty values are omitted from trailer output but always present in JSON output.

| Field            | Type   | Source                                            |
| ---------------- | ------ | ------------------------------------------------- |
| `tool`           | string | `--tool` flag                                     |
| `model`          | string | Hook stdin or `.dreamland.json` `model_id`        |
| `thinking_effort`| string | `effort.level` from Claude Code stdin / `CLAUDE_EFFORT` env; empty for other tools |
| `context_size`   | int64  | Omitted — not exposed by any tool's hook payload  |
| `input_tokens`   | int64  | Summed from transcript JSONL (Claude Code, Codex) or zero |
| `output_tokens`  | int64  | Summed from transcript JSONL (Claude Code, Codex) or zero |
| `cached_tokens`  | int64  | Summed cache_creation + cache_read tokens (Claude Code); zero elsewhere |
| `total_tokens`   | int64  | Always computed: `input_tokens + output_tokens`   |
| `captured_at`    | string | RFC 3339 UTC timestamp of last write              |

**`context_size` is removed from the data model.** No supported tool exposes context window size in its hook payload or a stable programmatic interface at hook execution time.

#### Scenario: total_tokens always computed
- **WHEN** `input_tokens` is 1500 and `output_tokens` is 300
- **THEN** `total_tokens` is 1800 regardless of whether the source payload included it

---

### Requirement: dreamland telemetry reset command

The CLI SHALL expose `dreamland telemetry reset` to delete `.dreamland-session.json`, starting a fresh accumulation.

#### Scenario: Reset deletes session file
- **WHEN** `dreamland telemetry reset` runs and `.dreamland-session.json` exists
- **THEN** the file is deleted and the command exits 0

#### Scenario: Reset is a no-op when file absent
- **WHEN** `dreamland telemetry reset` runs and no session file exists
- **THEN** the command exits 0 without error

---

### Requirement: dreamland telemetry install and uninstall commands

The CLI SHALL expose `dreamland telemetry install` to add the `commit-msg` hook to an already-initialized repo, and `dreamland telemetry uninstall` to remove the dreamland-managed block from `.git/hooks/commit-msg`.

#### Scenario: Install adds commit-msg hook independently of init
- **WHEN** `dreamland telemetry install` is run in a git repo with `.dreamland.json` present
- **THEN** the `commit-msg` hook is installed or updated using the same guarded-block logic as `dreamland init`

#### Scenario: Uninstall removes dreamland block from commit-msg
- **WHEN** `dreamland telemetry uninstall` is run and `.git/hooks/commit-msg` contains the dreamland block
- **THEN** the block between `# BEGIN dreamland-telemetry` and `# END dreamland-telemetry` is removed; if the remaining file is empty or whitespace-only, it is deleted

---

### Requirement: OTEL span emitted for each MCP tool call

The `dreamland serve` MCP server SHALL initialize an OpenTelemetry `TracerProvider` at startup and wrap each tool-call handler in a span named `mcp.tool_call`.

The span SHALL carry attributes `ai.tool` and `ai.model` (from `.dreamland.json`) plus any token counts available in the session snapshot at the time of the call.

The exporter SHALL be configured via `OTEL_EXPORTER_OTLP_ENDPOINT`; when unset it SHALL default to a stdout JSON exporter. The server SHALL NOT crash when no exporter is configured.

#### Scenario: OTEL span emitted on MCP tool call
- **WHEN** a client calls a dreamland MCP tool via `dreamland serve`
- **THEN** a span named `mcp.tool_call` is created, carries `ai.tool` and `ai.model` attributes, and is exported

#### Scenario: No endpoint configured falls back to stdout
- **WHEN** `OTEL_EXPORTER_OTLP_ENDPOINT` is not set
- **THEN** spans are written as JSON to stdout and the MCP server continues running normally
