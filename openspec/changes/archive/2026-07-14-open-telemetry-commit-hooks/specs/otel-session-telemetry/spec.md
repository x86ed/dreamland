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

The collector SHALL attempt to iterate every line in `transcript_path` whose `type` is `"assistant"` and accumulate:
- `message.usage.input_tokens` → `InputTokens`
- `message.usage.output_tokens` → `OutputTokens`
- `message.usage.cache_creation_input_tokens` + `message.usage.cache_read_input_tokens` → `CachedTokens`

**Stability caveat**: The Claude Code transcript JSONL schema is not documented as a stable interface in official docs ([code.claude.com/docs/en/hooks](https://code.claude.com/docs/en/hooks)). All transcript parsing MUST be wrapped in full error recovery: any parse failure (missing fields, changed schema, unreadable file) SHALL result in zero token counts + a stderr warning, never a fatal error or blocked commit. Token data is best-effort observability, not billing-accurate.

`model` SHALL be read from `message.model` on the most recent assistant turn in the transcript JSONL (confirmed present via `jq 'select(.type=="assistant").message.model'` on Claude Code session files — the Anthropic API embeds the served model on every response). Fall back to `.dreamland.json` `model_id` if the transcript is absent or no assistant turn has a non-empty `model` field.
`thinking_effort` SHALL be read from `effort.level` in the Stop hook stdin, with `CLAUDE_EFFORT` environment variable as a fallback.

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

The collector SHALL extract `model` directly from stdin. It SHALL attempt to read `transcript_path` for token counts using the same JSONL parsing as the Claude Code collector.

**Stability caveat**: The Codex docs ([developers.openai.com/codex/hooks](https://developers.openai.com/codex/hooks)) explicitly state: *"transcript_path points to a conversation transcript for convenience, but the transcript format is not a stable interface for hooks and may change over time."* All transcript reads SHALL be wrapped in full error recovery: on any parse failure, token counts default to zero and a warning is written to stderr. This is the same policy as Claude Code.

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

**Stability caveat on transcript**: Cursor's docs ([cursor.com/docs/hooks](https://cursor.com/docs/hooks)) confirm `transcript_path` in the stop payload but do not document the transcript JSONL schema as a stable interface. Apply the same full error-recovery policy as Claude Code and Codex: parse failure → zero tokens + stderr warning, never fatal.

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

### Requirement: Kiro collector is a stub in this change

Kiro's `stop` hook payload only contains `{hook_event_name, cwd, session_id, assistant_response}` — no model name, no tokens, no transcript path. Real token data via AWS Bedrock model invocation logging is implemented in the separate `kiro-bedrock-telemetry` change, which depends on this one.

For `--tool kiro` in this change, `dreamland telemetry write` SHALL:
1. Attempt to parse stdin as JSON; proceed regardless of whether it is valid.
2. Write a `SnapshotResult` with `tool: "kiro"`, `model` from `.dreamland.json` `model_id`, and all token counts zero.
3. Exit 0.

#### Scenario: Kiro stop hook writes stub snapshot
- **WHEN** `dreamland telemetry write --tool kiro` runs in a Kiro `stop` hook
- **THEN** `.dreamland-session.json` contains `tool: "kiro"`, `model` from `.dreamland.json`, all token counts zero, and `captured_at` set to current timestamp

---

### Requirement: Antigravity collector reads transcriptPath from PostTurnHook and parses transcript JSONL

The Antigravity `PostTurnHook` stdin payload contains the following confirmed fields:
- `stepIdx` (integer) — current step index
- `conversationId` (string) — session identifier
- `workspacePaths` (array of strings)
- `transcriptPath` (string) — path to `~/.gemini/antigravity/brain/<conversationId>/.system_generated/logs/transcript.jsonl`
- `artifactDirectoryPath` (string)

Each line of `transcript.jsonl` is a JSON object with the following schema:

```json
{
  "type": "string",
  "model": "string",
  "sessionId": "string",
  "timestamp": "string",
  "usageMetadata": {
    "promptTokenCount": 0,
    "candidatesTokenCount": 0,
    "thoughtsTokenCount": 0,
    "cachedContentTokenCount": 0,
    "totalTokenCount": 0
  }
}
```

For `--tool antigravity`, `dreamland telemetry write` SHALL:

1. Parse the PostTurnHook JSON payload from stdin.
2. Extract `transcriptPath` from the payload.
3. Call `transcript.ParseAntigravityTranscript(path)` which reads the JSONL and sums:
   - `usageMetadata.promptTokenCount` → `InputTokens`
   - `usageMetadata.candidatesTokenCount` → `OutputTokens`
   - `usageMetadata.cachedContentTokenCount` → `CachedTokens`
   - `usageMetadata.totalTokenCount` is verified against `InputTokens + OutputTokens` (use computed value if mismatched)
4. Extract `model` from the most recent transcript line where the `model` field is non-empty. Fall back to `cfg.ModelID` if no model is found.
5. Wrap transcript parsing in error recovery: parse failure yields zero tokens and a stderr warning, not a fatal error.
6. Write the `SnapshotResult` with `tool: "antigravity"`, `model`, and token counts.

`ParseAntigravityTranscript` is a separate function from `ParseTranscript` (used for Claude Code/Codex/Cursor) because the field names differ (`promptTokenCount` vs `input_tokens`, `usageMetadata` nesting vs flat `message.usage`).

#### Scenario: Antigravity PostTurnHook parses transcript for real token counts
- **WHEN** `dreamland telemetry write --tool antigravity` runs with stdin containing a valid `transcriptPath` pointing to a transcript with 3 lines each having `usageMetadata.promptTokenCount: 500` and `candidatesTokenCount: 100`
- **THEN** `.dreamland-session.json` contains `input_tokens: 1500`, `output_tokens: 300`, `model` from the most recent transcript line with a non-empty model field

#### Scenario: Antigravity model extracted from transcript line
- **WHEN** transcript lines include `"model": "gemini-2.5-pro"` on the last assistant turn
- **THEN** `.dreamland-session.json` contains `model: "gemini-2.5-pro"`

#### Scenario: Antigravity transcript parse failure falls back gracefully
- **WHEN** `transcriptPath` points to a malformed JSONL file
- **THEN** zero token counts are written, `model` falls back to `.dreamland.json`, a warning is printed to stderr, and the command exits 0

#### Scenario: Antigravity missing transcriptPath falls back gracefully
- **WHEN** the PostTurnHook stdin has no `transcriptPath` field or the file does not exist
- **THEN** a `SnapshotResult` with zero tokens and `model` from `.dreamland.json` is written; exit code is 0

---

### Requirement: GitHub Copilot collector reads agentStop hook payload and parses transcript

GitHub Copilot's `agentStop` hook (confirmed from [docs.github.com/en/copilot/reference/hooks-reference](https://docs.github.com/en/copilot/reference/hooks-reference)) sends the following payload on stdin:
- `sessionId` (string)
- `timestamp` (integer, epoch ms)
- `cwd` (string)
- `transcriptPath` (string) — path to the session transcript
- `stopReason` (string, e.g. `"end_turn"`)

For `--tool github-copilot`, `dreamland telemetry write` SHALL:
1. Parse the `agentStop` payload from stdin and extract `transcriptPath`.
2. Attempt to parse `transcriptPath` as JSONL for token counts and model name. The Copilot transcript format is not publicly documented — all parsing is best-effort and MUST be wrapped in full error recovery.
3. Fall back to zero tokens and `cfg.ModelID` if transcript parsing fails or the file is absent.
4. Write the `SnapshotResult` with `tool: "github-copilot"`, `model`, and any available token counts.
5. Exit 0 in all cases.

#### Scenario: GitHub Copilot agentStop hook writes snapshot with transcript data
- **WHEN** `dreamland telemetry write --tool github-copilot` runs with a valid `transcriptPath` in the agentStop payload
- **THEN** `.dreamland-session.json` is written with `tool: "github-copilot"` and token counts from the transcript where parseable

#### Scenario: GitHub Copilot transcript unreadable falls back gracefully
- **WHEN** `transcriptPath` does not exist or is in an unrecognized format
- **THEN** zero tokens and `model` from `.dreamland.json` are written, a stderr warning is printed, and the command exits 0

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
