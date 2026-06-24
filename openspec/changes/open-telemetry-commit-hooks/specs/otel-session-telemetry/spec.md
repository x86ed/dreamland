## ADDED Requirements

### Requirement: dreamland telemetry write command
The CLI SHALL expose a `dreamland telemetry write` subcommand that accepts a JSON payload on stdin (or via flags) and writes/accumulates a normalized `SnapshotResult` to `.dreamland-session.json` at the repository root.

The command SHALL accept a `--tool <name>` flag identifying the source tool. Recognized tool names: `claude-code`, `github-copilot`, `cursor`, `codex`, `kiro`, `antigravity`.

If `.dreamland-session.json` already exists for the current session, token counts SHALL be accumulated (summed) rather than overwritten, so multi-turn sessions produce a running total.

#### Scenario: Claude Code Stop hook writes telemetry
- **WHEN** `dreamland telemetry write --tool claude-code --stdin` receives `{"usage":{"inputTokens":1000,"outputTokens":200,"cacheReadInputTokens":500,"cacheCreationInputTokens":0},"model":"claude-sonnet-4-6"}` on stdin
- **THEN** `.dreamland-session.json` is created/updated with `input_tokens: 1000`, `output_tokens: 200`, `cached_tokens: 500`, `total_tokens: 1700`, `model: "claude-sonnet-4-6"`, `tool: "claude-code"`, and `captured_at` set to the current UTC timestamp

#### Scenario: Token counts accumulate across turns
- **WHEN** `dreamland telemetry write` is called twice in the same session with 1000 and 500 input tokens respectively
- **THEN** `.dreamland-session.json` contains `input_tokens: 1500` after the second call

#### Scenario: Unknown tool name rejected
- **WHEN** `dreamland telemetry write --tool unknown-tool` is called
- **THEN** the command exits with code 1 and prints an error listing valid tool names

#### Scenario: Malformed stdin JSON rejected gracefully
- **WHEN** `dreamland telemetry write --stdin` receives invalid JSON
- **THEN** the command exits with code 1 and prints the parse error; `.dreamland-session.json` is not modified

### Requirement: dreamland telemetry snapshot command
The CLI SHALL expose a `dreamland telemetry snapshot` subcommand that reads `.dreamland-session.json` and outputs a normalized JSON object to stdout.

The command SHALL support a `--format` flag with values `json` (default) and `trailers` (git-trailer format).

#### Scenario: JSON snapshot output
- **WHEN** `dreamland telemetry snapshot` is run and `.dreamland-session.json` exists
- **THEN** stdout contains a single JSON object matching the `SnapshotResult` schema with all available fields populated

#### Scenario: Trailer format output
- **WHEN** `dreamland telemetry snapshot --format trailers` is run
- **THEN** stdout contains one `Key: value` line per non-empty field, prefixed with `AI-`, suitable for appending as git trailers (e.g., `AI-Model: claude-sonnet-4-6`)

#### Scenario: Missing session file exits cleanly
- **WHEN** `dreamland telemetry snapshot` is run and no `.dreamland-session.json` exists
- **THEN** the command exits with code 0 and writes nothing to stdout (so the commit-msg hook can safely discard empty output)

#### Scenario: Stale snapshot warning
- **WHEN** `dreamland telemetry snapshot` is run and `captured_at` is older than the configured staleness threshold (default 4 hours)
- **THEN** the command writes a warning to stderr but still outputs the snapshot to stdout

### Requirement: SnapshotResult data model
The normalized snapshot object SHALL conform to the following schema:

| Field | Type | Source | Notes |
|-------|------|--------|-------|
| `tool` | string | `--tool` flag | Tool identifier |
| `model` | string | Tool payload | Model name/version |
| `thinking_effort` | string | Tool payload | e.g., `"low"`, `"medium"`, `"high"` — omitted if not available |
| `context_size` | int64 | Tool payload | Context window tokens — omitted if not available |
| `input_tokens` | int64 | Tool payload | Prompt tokens this session |
| `output_tokens` | int64 | Tool payload | Completion tokens this session |
| `cached_tokens` | int64 | Tool payload | Cache-read tokens — omitted if not available |
| `total_tokens` | int64 | Computed | `input_tokens + output_tokens` always |
| `captured_at` | string | System | RFC 3339 UTC timestamp of last write |

#### Scenario: total_tokens computed when not provided
- **WHEN** the tool payload does not include a `total_tokens` field
- **THEN** `SnapshotResult.TotalTokens` equals `InputTokens + OutputTokens`

### Requirement: dreamland telemetry reset command
The CLI SHALL expose a `dreamland telemetry reset` subcommand that deletes `.dreamland-session.json` to start a fresh session accumulator.

#### Scenario: Session file deleted
- **WHEN** `dreamland telemetry reset` is run and `.dreamland-session.json` exists
- **THEN** the file is deleted and the command exits with code 0

#### Scenario: No session file is a no-op
- **WHEN** `dreamland telemetry reset` is run and no `.dreamland-session.json` exists
- **THEN** the command exits with code 0 without error

### Requirement: dreamland telemetry uninstall command
The CLI SHALL expose a `dreamland telemetry uninstall` subcommand that removes the dreamland-managed block from `.git/hooks/commit-msg`.

#### Scenario: Hook block removed
- **WHEN** `dreamland telemetry uninstall` is run and the `commit-msg` hook contains a `# BEGIN dreamland-telemetry` / `# END dreamland-telemetry` block
- **THEN** that block is removed; if the file is otherwise empty or only whitespace, the file is also deleted

### Requirement: OTEL span emitted for each MCP tool call
The `dreamland serve` MCP server SHALL initialize an OpenTelemetry `TracerProvider` at startup and wrap each tool-call handler in a span named `mcp.tool_call`.

The span SHALL carry the following attributes when available: `ai.tool`, `ai.model`, `ai.input_tokens`, `ai.output_tokens`, `ai.cached_tokens`, `ai.total_tokens`.

The OTEL exporter SHALL be configured via standard environment variables: `OTEL_EXPORTER_OTLP_ENDPOINT` (OTLP), defaulting to a stdout JSON exporter when no endpoint is configured.

#### Scenario: OTEL span emitted on tool call
- **WHEN** an MCP client calls a dreamland tool via `dreamland serve`
- **THEN** a span with name `mcp.tool_call` is created, populated with tool and token attributes, and exported to the configured exporter

#### Scenario: No exporter configured falls back to stdout
- **WHEN** `OTEL_EXPORTER_OTLP_ENDPOINT` is not set
- **THEN** spans are written as JSON to stdout and the server does not crash
