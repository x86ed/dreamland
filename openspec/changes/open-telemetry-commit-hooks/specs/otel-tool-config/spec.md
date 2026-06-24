## ADDED Requirements

### Requirement: dreamland telemetry write registered in each tool's end-of-turn hook

`dreamland init` SHALL add `dreamland telemetry write --tool <name>` to the end-of-turn hook array for each platform, alongside the existing `version-bump --patch`, `transition-log`, and `test` commands. It SHALL merge into the same binding files already defined by the `dev-workflow-hooks` spec — no separate telemetry config file is created.

The exact end-of-turn event key and binding file per platform:

| Platform       | Binding file                                              | End-of-turn event key     |
| -------------- | --------------------------------------------------------- | ------------------------- |
| Claude Code    | `.claude/settings.json`                                   | `Stop`                    |
| Codex CLI      | `.codex/hooks.json`                                       | `Stop`                    |
| Cursor         | `.cursor/hooks.json`                                      | `stop`                    |
| Kiro           | `.kiro/agent.json`                                        | `stop`                    |
| Antigravity    | `~/.gemini/antigravity-cli/plugins/dreamland/hooks.json`  | `PostTurnHook`            |
| GitHub Copilot | `.github/copilot-hooks/hooks-manifest.json`               | stub — no public hook API |

All file merges use the atomic write strategy (temp file + rename) defined in `dev-workflow-hooks`.

#### Scenario: Telemetry write command added to Claude Code Stop array
- **WHEN** `dreamland init` completes with "Claude Code" selected
- **THEN** `.claude/settings.json` contains `dreamland telemetry write --tool claude-code` as a command entry under `hooks.Stop`, alongside the existing lifecycle commands

#### Scenario: Telemetry write command added to Codex Stop array
- **WHEN** `dreamland init` completes with "Codex" selected
- **THEN** `.codex/hooks.json` contains `dreamland telemetry write --tool codex` under the `Stop` key

#### Scenario: Telemetry write command added to Cursor stop array
- **WHEN** `dreamland init` completes with "Cursor" selected
- **THEN** `.cursor/hooks.json` contains `dreamland telemetry write --tool cursor` as a command under `hooks.stop` in the `version: 1` envelope

#### Scenario: Telemetry write command added to Kiro stop array
- **WHEN** `dreamland init` completes with "Kiro" selected
- **THEN** `.kiro/agent.json` contains `dreamland telemetry write --tool kiro` under the `stop` key

#### Scenario: Telemetry write command added to Antigravity plugin hooks
- **WHEN** `dreamland init` completes with "Antigravity" selected
- **THEN** `~/.gemini/antigravity-cli/plugins/dreamland/hooks.json` contains `dreamland telemetry write --tool antigravity` under `PostTurnHook`

#### Scenario: GitHub Copilot telemetry remains a stub
- **WHEN** `dreamland init` completes with "GitHub Copilot" selected
- **THEN** `.github/copilot-hooks/hooks-manifest.json` contains a `_note` field referencing the OTel/token-usage.jsonl approach described below, with no executable hook command registered

---

### Requirement: Claude Code telemetry data sourced from Stop hook stdin and transcript JSONL

When `dreamland telemetry write --tool claude-code` runs inside the Claude Code `Stop` hook, it SHALL:

1. Read the Stop hook JSON payload from stdin. The payload contains:
   - `transcript_path` (string): path to the session JSONL file
   - `effort` (object): `{"level": "low"|"medium"|"high"|"xhigh"|"max"}` — maps to `thinking_effort`
   - `session_id` (string)
   - `cwd`, `permission_mode`, `hook_event_name`, `agent_id`, `agent_type`, `stop_hook_active`
2. Read the `CLAUDE_EFFORT` environment variable as a fallback for thinking effort.
3. Open `transcript_path` and iterate every JSONL line whose `type` is `"assistant"`. Sum across all lines:
   - `message.usage.input_tokens` → `InputTokens`
   - `message.usage.output_tokens` → `OutputTokens`
   - `message.usage.cache_creation_input_tokens` → accumulated into `CachedTokens` (creation cost)
   - `message.usage.cache_read_input_tokens` → accumulated into `CachedTokens` (read savings)
4. Read `model` from `.dreamland.json` `model_id` field (the Stop hook payload does not include model name).
5. Write the accumulated `SnapshotResult` to `.dreamland-session.json`.

**Note on token accuracy**: Claude Code writes JSONL entries during streaming before the input token count is finalized. The summed counts from transcript JSONL are best-effort and may not match final API billing. This is documented behavior; the snapshot is for observability, not billing reconciliation.

#### Scenario: Claude Code Stop hook writes telemetry from transcript
- **WHEN** `dreamland telemetry write --tool claude-code` runs in a Claude Code `Stop` hook with `transcript_path` on stdin pointing to a JSONL with 3 assistant turns totaling 5000 input, 800 output, and 2000 cache_read tokens
- **THEN** `.dreamland-session.json` is written with `input_tokens: 5000`, `output_tokens: 800`, `cached_tokens: 2000`, `total_tokens: 5800`, `thinking_effort` from `effort.level`, and `model` from `.dreamland.json`

#### Scenario: Missing transcript file handled gracefully
- **WHEN** `transcript_path` in Stop hook stdin points to a file that does not exist
- **THEN** `dreamland telemetry write` writes a `SnapshotResult` with zero token counts and exits 0

---

### Requirement: Codex CLI telemetry data sourced from Stop hook stdin

When `dreamland telemetry write --tool codex` runs inside the Codex CLI `Stop` hook, it SHALL read the Stop hook JSON payload from stdin. The Codex Stop payload contains:
- `model` (string): model name — directly available
- `session_id`, `turn_id`, `transcript_path`, `cwd`, `hook_event_name`, `permission_mode`, `stop_hook_active`, `last_assistant_message`

`dreamland telemetry write` SHALL extract `model` directly from stdin. It SHALL attempt to read `transcript_path` for token counts using the same JSONL parsing logic as Claude Code. Because the Codex transcript format is documented as unstable, the transcript read SHALL be wrapped in error recovery — on any parse failure, token counts default to zero and a warning is written to stderr.

#### Scenario: Codex Stop hook provides model name directly
- **WHEN** `dreamland telemetry write --tool codex` runs with Stop hook stdin containing `"model": "o4-mini"`
- **THEN** `.dreamland-session.json` contains `model: "o4-mini"`

#### Scenario: Codex transcript parse failure falls back gracefully
- **WHEN** `transcript_path` points to a file with an unrecognized format
- **THEN** `dreamland telemetry write` writes `model` from stdin, sets token counts to zero, prints a warning to stderr, and exits 0

---

### Requirement: Cursor telemetry data is minimal due to limited stop hook payload

The Cursor `stop` hook payload is minimal: `{"status": "completed"|"aborted"|"error", "loop_count": N}`. It does not include model name, token counts, session ID, or transcript path.

When `dreamland telemetry write --tool cursor` runs in the Cursor `stop` hook, it SHALL:
1. Read the stop payload from stdin and record `loop_count` in an extended field.
2. Read `model` from `.dreamland.json` `model_id`.
3. Set all token count fields to zero (unavailable from hook).
4. Write the partial `SnapshotResult` to `.dreamland-session.json` with `tool: "cursor"` and `model` populated, tokens as zero.

If the `AGENT_TELEMETRY_URL` environment variable is set by Cursor (its internal run-summary endpoint), `dreamland telemetry write` MAY attempt an HTTP GET to that URL and parse any token fields from the response. This is a best-effort enhancement; failure does not block the write.

#### Scenario: Cursor stop hook writes partial snapshot
- **WHEN** `dreamland telemetry write --tool cursor` runs in a Cursor `stop` hook with `{"status": "completed", "loop_count": 4}`
- **THEN** `.dreamland-session.json` contains `tool: "cursor"`, `model` from `.dreamland.json`, all token counts zero, and `captured_at` set to current timestamp

#### Scenario: Cursor stop with aborted status still writes snapshot
- **WHEN** the Cursor stop payload has `"status": "aborted"`
- **THEN** `dreamland telemetry write` still writes a snapshot; the aborted status is not an error

---

### Requirement: Kiro telemetry data is stubbed due to undocumented hook payload

Kiro's hook system (`stop` event in `.kiro/agent.json`) does not have publicly documented stdin JSON for shell command hooks. Shell command stdout is fed back to the agent context rather than to the hook invoker.

When `dreamland telemetry write --tool kiro` runs in a Kiro `stop` hook, it SHALL:
1. Attempt to read JSON from stdin; if no valid JSON is present, treat all fields as absent.
2. Read `model` from `.dreamland.json` `model_id`.
3. Set all token count fields to zero.
4. Write the partial `SnapshotResult` to `.dreamland-session.json`.

#### Scenario: Kiro stop hook writes stub snapshot
- **WHEN** `dreamland telemetry write --tool kiro` runs in a Kiro `stop` hook
- **THEN** `.dreamland-session.json` contains `tool: "kiro"`, `model` from `.dreamland.json`, all token counts zero, and `captured_at` set to current timestamp

---

### Requirement: Antigravity telemetry data sourced from PostTurnHook payload

Antigravity's `PostTurnHook` fires after each agent turn. The exact payload schema is not publicly documented; the hook is classified as an "Inspect" (read-only, non-blocking) hook in the Antigravity SDK.

When `dreamland telemetry write --tool antigravity` runs in the Antigravity `PostTurnHook`, it SHALL:
1. Attempt to read JSON from stdin and extract any recognized token or model fields using best-effort field matching.
2. Read `model` from `.dreamland.json` `model_id` as a fallback.
3. Write whatever partial `SnapshotResult` can be constructed.
4. Token fields for which no value is found SHALL be set to zero.

The binding file (`~/.gemini/antigravity-cli/plugins/dreamland/hooks.json`) SHALL be marked `"_preview": true` to signal that Antigravity hook payload documentation is not yet finalized.

#### Scenario: Antigravity PostTurnHook writes best-effort snapshot
- **WHEN** `dreamland telemetry write --tool antigravity` runs in a `PostTurnHook` context
- **THEN** `.dreamland-session.json` is written with available fields populated and unknown fields zeroed; the command exits 0

---

### Requirement: GitHub Copilot telemetry documented via OTel and token-usage.jsonl

GitHub Copilot does not expose a public lifecycle hook API. The hook binding file (`.github/copilot-hooks/hooks-manifest.json`) is a stub per `dev-workflow-hooks`.

The stub manifest SHALL document the two available telemetry paths that users can configure manually:
1. **OTel traces**: GitHub Copilot CLI SDK exports traces, metrics, and events via OpenTelemetry. Token counts, model name, and duration appear as span attributes. Configure via `OTEL_EXPORTER_OTLP_ENDPOINT`.
2. **token-usage.jsonl**: Every Copilot workflow outputs a `token-usage.jsonl` artifact with one record per API call containing `input_tokens`, `output_tokens`, `cache_read_tokens`, `cache_write_tokens`, `model`, `provider`, and timestamps.

The stub SHALL include a `_note` field and a `_telemetry_paths` object describing both options and referencing the `dreamland telemetry write` command for manual invocation.

#### Scenario: GitHub Copilot stub manifest created
- **WHEN** `dreamland init` completes with "GitHub Copilot" selected
- **THEN** `.github/copilot-hooks/hooks-manifest.json` exists with `_note` and `_telemetry_paths` fields, and no executable hook command

---

### Requirement: otel_endpoint stored in .dreamland.json

`dreamland init` SHALL prompt the user for an OTEL collector endpoint (Step 6 of the wizard, optional). The value SHALL be written to `.dreamland.json` as `"otel_endpoint"`. When the field is absent or empty, the default value `http://localhost:4317` SHALL be used by all scaffold scripts.

#### Scenario: Custom OTEL endpoint persisted
- **WHEN** the user enters `http://otel-collector.internal:4317` during `dreamland init`
- **THEN** `.dreamland.json` contains `"otel_endpoint": "http://otel-collector.internal:4317"`

#### Scenario: Default endpoint used when field absent
- **WHEN** the wizard is completed without entering an endpoint
- **THEN** all generated hook scripts default to `http://localhost:4317`

---

### Requirement: OTEL environment variables injected via platform-native session hooks

`dreamland init` SHALL configure OTEL environment variables for each platform using that platform's session-initialization mechanism. These env vars enable any OTEL-instrumented subprocess (including the dreamland MCP server) to export spans for the project. The three core vars set on every platform are:

- `OTEL_EXPORTER_OTLP_ENDPOINT` — value from `.dreamland.json` `otel_endpoint`
- `OTEL_EXPORTER_OTLP_PROTOCOL` — `grpc` (default)
- `OTEL_SERVICE_NAME` — `dreamland`

Platform-specific injection mechanisms:

| Platform       | Mechanism                                                     | Files written                                  |
| -------------- | ------------------------------------------------------------- | ---------------------------------------------- |
| Claude Code    | `SessionStart` hook writes to `CLAUDE_ENV_FILE`               | `.claude/scripts/dreamland-otel-env.sh`        |
| Codex CLI      | Project hooks run a setup script; native OTEL config blocked at project level | `.codex/otel-config.example.toml`   |
| Cursor         | `sessionStart` hook outputs `{"env": {...}}` JSON; vars persist to all subsequent hooks | `.cursor/hooks/dreamland-otel-env.sh` |
| Kiro           | `agentSpawn` hook script (env propagation undocumented)       | `.kiro/hooks/dreamland-otel-env.sh`            |
| Antigravity    | `.agents/hooks.json` `SessionStart` hook sets env vars        | `.agents/hooks.json` (merged)                  |
| GitHub Copilot | `.vscode/settings.json` native OTel config keys               | `.vscode/settings.json` (merged)               |

#### Scenario: Claude Code SessionStart hook injects OTEL env via CLAUDE_ENV_FILE
- **WHEN** `dreamland init` completes with "Claude Code" selected
- **THEN** `.claude/scripts/dreamland-otel-env.sh` is created (mode 0755) containing:
  ```sh
  #!/bin/sh
  [ -z "$CLAUDE_ENV_FILE" ] && exit 0
  printf 'OTEL_EXPORTER_OTLP_ENDPOINT=%s\n' "<otel_endpoint>" >> "$CLAUDE_ENV_FILE"
  printf 'OTEL_EXPORTER_OTLP_PROTOCOL=grpc\n' >> "$CLAUDE_ENV_FILE"
  printf 'OTEL_SERVICE_NAME=dreamland\n' >> "$CLAUDE_ENV_FILE"
  ```
- **AND** `.claude/settings.json` contains `dreamland-otel-env.sh` in the `SessionStart` hook array

#### Scenario: Codex native OTEL config is user-level; project-level writes an example only
- **WHEN** `dreamland init` completes with "Codex" selected
- **THEN** `.codex/config.toml` does NOT contain any `[otel]` section (Codex ignores project-level OTEL config)
- **AND** `.codex/otel-config.example.toml` is written with the correct `[otel]` TOML snippet and a comment instructing the user to merge it into `~/.codex/config.toml`:
  ```toml
  # Add this to ~/.codex/config.toml (project-level config.toml cannot set OTEL)
  [otel]
  environment = "dev"
  exporter = { otlp-http = { endpoint = "<otel_endpoint>", protocol = "binary" } }
  ```
- **AND** `dreamland init` prints a warning: "Codex OTEL config must be set in ~/.codex/config.toml — see .codex/otel-config.example.toml"

#### Scenario: Cursor sessionStart hook returns env JSON that persists to all hooks
- **WHEN** `dreamland init` completes with "Cursor" selected
- **THEN** `.cursor/hooks/dreamland-otel-env.sh` is created (mode 0755) containing:
  ```sh
  #!/bin/sh
  printf '{"env":{"OTEL_EXPORTER_OTLP_ENDPOINT":"%s","OTEL_EXPORTER_OTLP_PROTOCOL":"grpc","OTEL_SERVICE_NAME":"dreamland"}}\n' "<otel_endpoint>"
  ```
- **AND** `.cursor/hooks.json` contains `dreamland-otel-env.sh` in the `sessionStart` hook array
- **AND** the returned `env` block from the hook is available in all subsequent hooks including `stop`

#### Scenario: Kiro OTEL env requires system-level setup; dreamland writes a README only
- **WHEN** `dreamland init` completes with "Kiro" selected
- **THEN** `.kiro/dreamland-otel-setup.md` is written documenting the three OTEL env vars that must be exported in the user's shell profile before launching Kiro, because Kiro's hook system provides no mechanism for hooks to inject env vars into the Kiro agent session
- **AND** `dreamland init` prints: "Kiro has no hook-based env injection — set OTEL vars in your shell profile (see .kiro/dreamland-otel-setup.md)"

#### Scenario: Antigravity .agents/hooks.json SessionStart sets OTEL env
- **WHEN** `dreamland init` completes with "Antigravity" selected
- **THEN** `.agents/hooks.json` is created or merged, with a `SessionStart` entry that calls a hook setting `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_PROTOCOL`, `OTEL_SERVICE_NAME`, and `IDE_OTEL_IDE_NAME=antigravity`
- **AND** the plugin bundle at `~/.gemini/antigravity-cli/plugins/dreamland/hooks.json` contains the same `PostTurnHook` entry for the telemetry write command (per the lifecycle hook requirement above)

#### Scenario: GitHub Copilot .vscode/settings.json enables native OTEL
- **WHEN** `dreamland init` completes with "GitHub Copilot" selected
- **THEN** `.vscode/settings.json` is created or merged (atomically) with:
  ```json
  {
    "github.copilot.chat.otel.enabled": true,
    "github.copilot.chat.otel.exporterType": "otlp-http",
    "github.copilot.chat.otel.otlpEndpoint": "<otel_endpoint_http>"
  }
  ```
  where `otel_endpoint_http` uses port 4318 (OTLP HTTP default) when the configured endpoint uses port 4317 (OTLP gRPC default), since Copilot's native OTEL uses HTTP/protobuf

#### Scenario: OTEL env scripts not duplicated on re-init
- **WHEN** `dreamland init` is run a second time with the same tool selected
- **THEN** existing hook scripts are overwritten only if `--force` is passed; otherwise they are skipped with a "skipped (already exists):" notice

---

### Requirement: .dreamland-session.json added to .gitignore
`dreamland init` SHALL append `.dreamland-session.json` to `.gitignore` (creating the file if absent) to prevent accidental commits of the ephemeral session cache.

#### Scenario: .gitignore updated on init
- **WHEN** `dreamland init` completes successfully
- **THEN** `.gitignore` contains `.dreamland-session.json`

#### Scenario: Duplicate .gitignore entry avoided
- **WHEN** `.dreamland-session.json` is already present in `.gitignore`
- **THEN** `init` does not add a duplicate entry

### Requirement: Tool config templates embedded in the binary
All hook binding additions and stub files SHALL be generated from templates embedded in the dreamland binary using `go:embed`. No external template files are required at runtime.

#### Scenario: Binary generates files without on-disk templates
- **WHEN** `dreamland init` is run in an environment without the `internal/scaffold/templates/` directory on disk
- **THEN** all hook binding files and stubs are created correctly from compiled-in templates
