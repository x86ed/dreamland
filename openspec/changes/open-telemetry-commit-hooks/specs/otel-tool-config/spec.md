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
| GitHub Copilot | `.github/hooks/dreamland-telemetry.json`                  | `agentStop`               |

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

#### Scenario: Telemetry write command added to GitHub Copilot agentStop hook
- **WHEN** `dreamland init` completes with "GitHub Copilot" selected
- **THEN** `.github/hooks/dreamland-telemetry.json` contains `dreamland telemetry write --tool github-copilot` registered under the `agentStop` event

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
4. Read `model` from `message.model` on the most recent assistant turn in the transcript (the Anthropic API embeds the served model on every response). Fall back to `.dreamland.json` `model_id` if the transcript is absent or no assistant turn has a non-empty `model` field.
5. Write the accumulated `SnapshotResult` to `.dreamland-session.json`.

**Note on token accuracy**: Claude Code writes JSONL entries during streaming before the input token count is finalized. The summed counts from transcript JSONL are best-effort and may not match final API billing. This is documented behavior; the snapshot is for observability, not billing reconciliation.

#### Scenario: Claude Code Stop hook writes telemetry from transcript
- **WHEN** `dreamland telemetry write --tool claude-code` runs in a Claude Code `Stop` hook with `transcript_path` on stdin pointing to a JSONL with 3 assistant turns totaling 5000 input, 800 output, and 2000 cache_read tokens where the last turn has `"model": "claude-sonnet-4-6"`
- **THEN** `.dreamland-session.json` is written with `input_tokens: 5000`, `output_tokens: 800`, `cached_tokens: 2000`, `total_tokens: 5800`, `thinking_effort` from `effort.level`, and `model: "claude-sonnet-4-6"` from the transcript

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

### Requirement: Cursor telemetry data sourced from stop hook payload and transcript JSONL

The Cursor `stop` hook payload (confirmed from [cursor.com/docs/hooks](https://cursor.com/docs/hooks)) contains:
- `model` (string) — model name directly available
- `model_id` (string) — model identifier
- `model_params` (array)
- `conversation_id`, `generation_id`, `hook_event_name`, `cursor_version`, `workspace_roots`, `user_email` (strings)
- `transcript_path` (string | null) — path to session transcript when enabled
- `status` (`"completed"` | `"aborted"` | `"error"`)
- `loop_count` (integer)

Cursor also auto-sets `CURSOR_TRANSCRIPT_PATH` as an env var for all hooks.

When `dreamland telemetry write --tool cursor` runs in the Cursor `stop` hook, it SHALL:
1. Read `model` directly from the stop payload stdin.
2. If `transcript_path` is non-null, parse the transcript JSONL for token counts using the same parser as Claude Code and Codex. Fall back to `CURSOR_TRANSCRIPT_PATH` env var when `transcript_path` is null in the payload.
3. Wrap transcript parsing in error recovery: parse failure yields zero tokens + a stderr warning.
4. Write the `SnapshotResult` to `.dreamland-session.json` with `tool: "cursor"`, `model`, and token counts.

#### Scenario: Cursor stop hook extracts model and parses transcript for tokens
- **WHEN** `dreamland telemetry write --tool cursor` runs with stop payload containing `"model": "claude-sonnet-4-5"` and a valid `transcript_path`
- **THEN** `.dreamland-session.json` contains `model: "claude-sonnet-4-5"` and token counts summed from the transcript JSONL

#### Scenario: Cursor stop with null transcript_path falls back to env var
- **WHEN** `transcript_path` is null in the stop payload but `CURSOR_TRANSCRIPT_PATH` env var is set
- **THEN** `dreamland telemetry write` uses the env var path for transcript parsing

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

### Requirement: Antigravity telemetry data sourced from PostTurnHook transcript JSONL

The Antigravity `PostTurnHook` stdin payload contains confirmed fields (from [antigravity.google/docs/hooks](https://antigravity.google/docs/hooks)):
- `stepIdx` (integer)
- `conversationId` (string)
- `workspacePaths` (array of strings)
- `transcriptPath` (string) — path to `~/.gemini/antigravity/brain/<conversationId>/.system_generated/logs/transcript.jsonl`
- `artifactDirectoryPath` (string)

Each line of the transcript JSONL has the schema:
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

When `dreamland telemetry write --tool antigravity` runs in the Antigravity `PostTurnHook`, it SHALL:

1. Parse the PostTurnHook payload from stdin and extract `transcriptPath`.
2. Call `transcript.ParseAntigravityTranscript(path)` which sums `usageMetadata.promptTokenCount` → `InputTokens`, `usageMetadata.candidatesTokenCount` → `OutputTokens`, `usageMetadata.cachedContentTokenCount` → `CachedTokens`.
3. Extract `model` from the most recent transcript line where the `model` field is non-empty. Fall back to `cfg.ModelID` if absent.
4. Wrap all transcript parsing in error recovery: any parse failure yields zero tokens + stderr warning, not a fatal error.
5. Write the `SnapshotResult` with `tool: "antigravity"`, `model`, and token counts.

#### Scenario: Antigravity PostTurnHook parses transcript for real token counts
- **WHEN** `dreamland telemetry write --tool antigravity` runs with a valid `transcriptPath` pointing to 3 lines each having `usageMetadata.promptTokenCount: 500` and `candidatesTokenCount: 100`
- **THEN** `.dreamland-session.json` contains `input_tokens: 1500`, `output_tokens: 300`, `model` from the most recent transcript line

#### Scenario: Antigravity transcript parse failure falls back gracefully
- **WHEN** `transcriptPath` points to a malformed JSONL
- **THEN** zero tokens are written, `model` falls back to `.dreamland.json`, a warning is printed to stderr, exit code is 0

---

### Requirement: GitHub Copilot telemetry data sourced from agentStop hook transcript

GitHub Copilot has a full hooks system ([docs.github.com/en/copilot/reference/hooks-reference](https://docs.github.com/en/copilot/reference/hooks-reference)) configured via `.github/hooks/*.json`. The `agentStop` event fires when a Copilot agent session ends and provides:
- `sessionId` (string)
- `timestamp` (integer, epoch ms)
- `cwd` (string)
- `transcriptPath` (string) — path to session transcript
- `stopReason` (`"end_turn"` | other values)

When `dreamland telemetry write --tool github-copilot` runs in the Copilot `agentStop` hook, it SHALL:
1. Parse the `agentStop` payload from stdin and extract `transcriptPath`.
2. Attempt to parse `transcriptPath` for token counts and model name using best-effort JSONL parsing. The Copilot transcript format is not publicly documented — wrap all parsing in full error recovery.
3. Fall back to zero tokens and `cfg.ModelID` from `.dreamland.json` if transcript parsing fails.
4. Write the `SnapshotResult` with `tool: "github-copilot"`, `model`, and token counts.

The native OTel path (`.vscode/settings.json` keys) remains available in parallel for OTEL backend observability independent of the commit trailer path.

#### Scenario: GitHub Copilot agentStop hook writes telemetry snapshot
- **WHEN** `dreamland telemetry write --tool github-copilot` runs with `agentStop` stdin containing a valid `transcriptPath`
- **THEN** `.dreamland-session.json` is written with `tool: "github-copilot"` and any token counts parseable from the transcript; the command exits 0

#### Scenario: GitHub Copilot transcript parse failure falls back gracefully
- **WHEN** `transcriptPath` is unreadable or in an unrecognized format
- **THEN** zero tokens and `model` from `.dreamland.json` are written; a warning is printed to stderr; exit code is 0

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

#### Scenario: Codex native OTEL config written directly to user-level config with confirmation
- **WHEN** `dreamland init` completes with "Codex" selected and the user confirms the prompt `"Write OTEL config to ~/.codex/config.toml? [y/N]"`
- **THEN** `.codex/config.toml` does NOT contain any `[otel]` section (Codex ignores project-level OTEL config)
- **AND** `~/.codex/config.toml` is read (created if absent), the `[otel]` block is merged preserving all other keys, and the file is written atomically:
  ```toml
  [otel]
  environment = "dev"
  exporter = { otlp-http = { endpoint = "<otel_endpoint>", protocol = "binary" } }
  ```
- **AND** `dreamland init` prints confirmation: "OTEL config written to ~/.codex/config.toml"

#### Scenario: Codex OTEL config skipped when user declines
- **WHEN** the user answers `N` at the `~/.codex/config.toml` prompt
- **THEN** neither `~/.codex/config.toml` nor any project file is modified for OTEL, and `dreamland init` prints: "Skipped — set [otel] in ~/.codex/config.toml manually to enable Codex telemetry"

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
