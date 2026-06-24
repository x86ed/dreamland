## 1. Dependencies & Module Setup

- [ ] 1.1 Add `go.opentelemetry.io/otel`, `go.opentelemetry.io/otel/sdk`, `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc`, and `go.opentelemetry.io/otel/exporters/stdout/stdouttrace` to `go.mod` via `go get`
- [ ] 1.2 Run `go mod tidy` and verify the build passes with `go build ./...`

## 2. SnapshotResult Data Model

- [ ] 2.1 Create `internal/telemetry/snapshot.go` defining the `SnapshotResult` struct: `Tool`, `Model`, `ThinkingEffort`, `InputTokens`, `OutputTokens`, `CachedTokens`, `TotalTokens`, `CapturedAt` (no `ContextSize` — not exposed by any tool's hook payload)
- [ ] 2.2 Add `ComputeTotals()` method that sets `TotalTokens = InputTokens + OutputTokens` when `TotalTokens` is zero
- [ ] 2.3 Add `Read(repoRoot string) (*SnapshotResult, error)` and `Write(repoRoot string, s *SnapshotResult) error` in `internal/telemetry/snapshot.go`; `Write` accumulates token counts rather than overwriting
- [ ] 2.4 Write unit tests for `Read`, `Write` (accumulation), and `ComputeTotals` in `internal/telemetry/snapshot_test.go`

## 3. Transcript JSONL Parser

- [ ] 3.1 Create `internal/telemetry/transcript.go` with `ParseTranscript(path string) (TranscriptUsage, error)` that reads a JSONL file, filters lines where `type == "assistant"`, and sums `message.usage.input_tokens`, `output_tokens`, `cache_creation_input_tokens`, `cache_read_input_tokens` across all lines
- [ ] 3.2 Ensure the parser returns zero counts (not an error) when the file does not exist; returns a wrapped error only on file read failures other than not-found
- [ ] 3.3 Write unit tests with a synthetic JSONL fixture covering: multi-turn accumulation, missing usage fields, empty file, non-existent file
- [ ] 3.4 Create `ParseAntigravityTranscript(path string) (TranscriptUsage, string, error)` in `internal/telemetry/transcript.go` — reads the same JSONL path but maps Antigravity field names: `usageMetadata.promptTokenCount` → `InputTokens`, `usageMetadata.candidatesTokenCount` → `OutputTokens`, `usageMetadata.cachedContentTokenCount` → `CachedTokens`; second return value is the `model` field from the most recent line where it is non-empty
- [ ] 3.5 Write unit tests for `ParseAntigravityTranscript` covering: multi-turn accumulation, model extracted from last non-empty line, `thoughtsTokenCount` present but not surfaced in `SnapshotResult`, missing `usageMetadata`, empty file, non-existent file

## 4. Per-Tool Collectors

- [ ] 4.1 Create `internal/telemetry/collector.go` defining a `Collector` interface with `Collect(stdin io.Reader, cfg *config.Config) (*SnapshotResult, error)` and a `Registry` map keyed by tool name string; register all collectors in an `init()` block
- [ ] 4.2 Create `internal/telemetry/tools/claude.go` — Claude Code collector: parse Stop hook stdin JSON for `transcript_path` and `effort.level`; fall back to `CLAUDE_EFFORT` env for thinking effort; call `transcript.ParseTranscript` for token counts; read `model` from `cfg.ModelID`
- [ ] 4.3 Create `internal/telemetry/tools/codex.go` — Codex CLI collector: parse Stop hook stdin JSON for `model` (directly present), `transcript_path`; call `transcript.ParseTranscript` wrapped in error recovery (parse failure → zero tokens + stderr warning, not fatal); no thinking effort field
- [ ] 4.4 Create `internal/telemetry/tools/cursor.go` — Cursor collector: parse stop hook stdin JSON for `model` (directly present), `transcript_path` (null if transcript not enabled), `status`, `loop_count`; call `transcript.ParseTranscript` for token counts; fall back to `CURSOR_TRANSCRIPT_PATH` env var when `transcript_path` is absent from stdin; model falls back to `cfg.ModelID` only when stdin field is empty
- [ ] 4.5 Create `internal/telemetry/tools/kiro.go` — Kiro collector with two phases via `--phase start|stop` flag:
  - `start` phase: write `session_start_time` (RFC 3339 UTC) to `.dreamland-session.json`; called by Kiro `agentSpawn` hook
  - `stop` phase: read `session_start_time` from `.dreamland-session.json`; run `aws logs filter-log-events --log-group-name <cfg.BedrockLogGroup> --start-time <epoch_ms> --filter-pattern '{ $.schemaType = "ModelInvocationLog" }' --query 'events[*].message' --output json` via `exec.Command`; parse each returned JSON string to sum `input.inputTokenCount` + `output.outputTokenCount`; extract `modelId` from the most recent entry and normalize it (strip `anthropic.` prefix and `-v1:0` suffix); fall back to zero tokens + `cfg.ModelID` if `aws` not on PATH, credentials absent, non-zero exit code, or no events returned; write stderr warning on fallback
- [ ] 4.6 Create `internal/telemetry/tools/antigravity.go` — Antigravity collector: parse PostTurnHook stdin JSON for `transcriptPath`, `conversationId`, `stepIdx`; call `transcript.ParseAntigravityTranscript` for token counts; extract model from most recent transcript line with non-empty `model` field; fall back to `cfg.ModelID` if no model found in transcript; wrap all transcript reads in error recovery
- [ ] 4.7 Create `internal/telemetry/tools/copilot.go` — GitHub Copilot collector: write guidance message to stderr about OTel (`OTEL_EXPORTER_OTLP_ENDPOINT`) and `token-usage.jsonl`; return nil result (no-op write)
- [ ] 4.8 Write table-driven unit tests for each collector in `internal/telemetry/tools/*_test.go` covering: normal payloads, empty stdin, malformed JSON, missing fields

## 5. CLI Commands — dreamland telemetry

- [ ] 5.1 Create `cmd/telemetry.go` with a `telemetryCmd` cobra command group (`dreamland telemetry`) registered on `rootCmd`
- [ ] 5.2 Implement `dreamland telemetry write` subcommand: validate `--tool` flag against registry, call the matching collector with stdin, call `telemetry.Write()` with the returned result; skip write if collector returns nil (copilot no-op)
- [ ] 5.3 Implement `dreamland telemetry snapshot` subcommand: `--format json|trailers` flag; call `telemetry.Read()`; output formatted result; warn to stderr if `captured_at` older than `--max-age` (default `4h`); exit 0 with no output when file absent
- [ ] 5.4 Implement `dreamland telemetry reset` subcommand: delete `.dreamland-session.json`; no-op if absent
- [ ] 5.5 Implement `dreamland telemetry install` subcommand: install `commit-msg` hook using the same guarded-block logic as `dreamland init`
- [ ] 5.6 Implement `dreamland telemetry uninstall` subcommand: remove `# BEGIN dreamland-telemetry` / `# END dreamland-telemetry` block from `.git/hooks/commit-msg`; delete the file if only whitespace remains
- [ ] 5.7 Write unit tests for all subcommands in `cmd/telemetry_test.go` using temp directories

## 6. Commit-msg Hook Template & Installer

- [ ] 6.1 Create `internal/scaffold/embed.go` with `//go:embed templates/**` and an exported `FS` variable
- [ ] 6.2 Write the hook script template at `internal/scaffold/templates/hooks/commit-msg`: reads `$1`, runs `dreamland telemetry snapshot --format trailers`, appends output to `$1` with a preceding blank line if non-empty, always exits 0; degrades gracefully if `dreamland` is not on PATH
- [ ] 6.3 Create `internal/scaffold/hook.go` with `InstallCommitMsgHook(repoRoot string) error` that appends the guarded block to `.git/hooks/commit-msg` (or creates it), sets mode 0755, and is idempotent (skips if block already present)
- [ ] 6.4 Write unit tests for `InstallCommitMsgHook` in `internal/scaffold/hook_test.go`: fresh install, append-to-existing, idempotent re-run

## 7. Per-Tool Hook Binding Additions

These tasks add `dreamland telemetry write --tool <name>` into the same binding files already defined by `dev-workflow-hooks`. All merges use atomic write (temp file + rename).

- [ ] 7.1 Update the Claude Code binding template (`internal/scaffold/templates/hooks/bindings/claude-code/settings.json`) to include `dreamland telemetry write --tool claude-code` in the `Stop` hook array alongside the existing lifecycle commands
- [ ] 7.2 Update the Codex binding template (`internal/scaffold/templates/hooks/bindings/codex/hooks.json`) to include `dreamland telemetry write --tool codex` in the `Stop` array
- [ ] 7.3 Update the Cursor binding template (`internal/scaffold/templates/hooks/bindings/cursor/hooks.json`) to include `dreamland telemetry write --tool cursor` under `hooks.stop` in the `version: 1` envelope
- [ ] 7.4 Update the Kiro binding template (`internal/scaffold/templates/hooks/bindings/kiro/agent.json`) to include `dreamland telemetry write --tool kiro` under `stop`
- [ ] 7.5 Write the Antigravity plugin template (`internal/scaffold/templates/hooks/bindings/antigravity/hooks.json`) with `dreamland telemetry write --tool antigravity` under `PostTurnHook` and `"_preview": true`
- [ ] 7.6 Write the GitHub Copilot stub template (`internal/scaffold/templates/hooks/bindings/github-copilot/copilot-hooks-manifest.json`) with `_note` and `_telemetry_paths` (OTel and token-usage.jsonl documentation)

## 8. Scaffolding Logic

- [ ] 8.1 Create `internal/scaffold/toolconfig.go` with `ScaffoldTelemetry(repoRoot, tool string) error` that merges the telemetry write command into the correct binding file for the given tool (using the templates from task 7)
- [ ] 8.2 Create `internal/scaffold/gitignore.go` with `EnsureGitignoreEntry(repoRoot, entry string) error` that appends the entry only if not already present
- [ ] 8.3 Write unit tests for `ScaffoldTelemetry` (per-tool) and `EnsureGitignoreEntry` in `internal/scaffold/`

## 9. Per-Tool OTEL Environment Setup Scripts

Each platform needs a dedicated shell script or config file that injects the three core OTEL env vars (`OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_PROTOCOL`, `OTEL_SERVICE_NAME`) into the agent session using that platform's native mechanism. The endpoint value is read from `cfg.OtelEndpoint` (defaulting to `http://localhost:4317`).

- [ ] 9.1 Add `OtelEndpoint string` and `BedrockLogGroup string` fields to `internal/config/config.go` `Config` struct; `OtelEndpoint` defaults to `http://localhost:4317`; `BedrockLogGroup` defaults to `aws/bedrock/modelinvocations`
- [ ] 9.2 Add OTEL endpoint prompt to `cmd/init.go` wizard (optional, after version command step); store in `.dreamland.json` as `otel_endpoint`
- [ ] 9.3 Write `internal/scaffold/templates/hooks/bindings/claude-code/dreamland-otel-env.sh`: appends `KEY=VALUE` lines (no `export`) to `$CLAUDE_ENV_FILE`; exits 0 immediately if `CLAUDE_ENV_FILE` is unset; endpoint baked in from template parameter at scaffold time
- [ ] 9.4 Create `internal/scaffold/otelenv.go` with `RenderOtelEnvScript(platform, endpoint string) ([]byte, error)` that selects the correct template for the given platform and substitutes the endpoint; returns the rendered script bytes
- [ ] 9.5 Write `internal/scaffold/templates/hooks/bindings/cursor/dreamland-otel-env.sh`: prints `{"env":{"OTEL_EXPORTER_OTLP_ENDPOINT":"<endpoint>","OTEL_EXPORTER_OTLP_PROTOCOL":"grpc","OTEL_SERVICE_NAME":"dreamland"}}` to stdout; this is returned to Cursor as the sessionStart hook output
- [ ] 9.6 Write `internal/scaffold/templates/hooks/bindings/kiro/dreamland-otel-env.sh`: exports the three vars as shell `export KEY=VALUE` statements; note that Kiro's `agentSpawn` env propagation to subsequent hooks is unconfirmed
- [ ] 9.7 Write `.codex/otel-config.example.toml` template with the correct `[otel]` TOML block and a comment directing the user to merge it into `~/.codex/config.toml`; do NOT include `[otel]` in the main `.codex/config.toml` template (Codex ignores project-level OTEL config)
- [ ] 9.8 Create `internal/scaffold/vscodesettings.go` with `MergeVscodeSettings(repoRoot string, patch map[string]any) error` that reads `.vscode/settings.json` (creates empty `{}` if absent), deep-merges the patch, and atomically writes the result
- [ ] 9.9 Write the Copilot OTEL settings patch: `{"github.copilot.chat.otel.enabled": true, "github.copilot.chat.otel.exporterType": "otlp-http", "github.copilot.chat.otel.otlpEndpoint": "<http-endpoint>"}` where `<http-endpoint>` is derived from `cfg.OtelEndpoint` (replace port 4317→4318; store as separate `otel_endpoint_http` in config if needed)
- [ ] 9.10 Write `.agents/hooks.json` Antigravity project-scope template with `SessionStart` hook that exports `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_PROTOCOL`, `OTEL_SERVICE_NAME`, and `IDE_OTEL_IDE_NAME=antigravity`
- [ ] 9.11 Create `internal/scaffold/otelenv_installer.go` with `InstallOtelEnv(repoRoot string, cfg *config.Config) error` that orchestrates all per-tool steps: renders and writes the hook script, registers it in the appropriate session-start hook array, and writes any companion config files; per-file failures are logged but do not return an error
- [ ] 9.12 Write unit tests in `internal/scaffold/otelenv_installer_test.go` covering each of the six tools: verify correct files are created, hook script content matches expected env vars, and idempotent re-run skips existing files (without `--force`)

## 10. Init Wizard Updates

- [ ] 10.1 Add `"Cursor"` and `"Codex"` to the Step 1 tool select in `cmd/init.go` (expand from 4 to 6 options), consistent with the updated `init-wizard` spec
- [ ] 10.2 Add a post-save scaffolding step in `runInit` after `config.Save` that calls `scaffold.InstallOtelEnv`, `scaffold.ScaffoldTelemetry`, `scaffold.InstallCommitMsgHook`, and `scaffold.EnsureGitignoreEntry`; failures print a per-file warning but do not change the exit code if `.dreamland.json` was written
- [ ] 10.3 Print the Codex user-level OTEL warning when "Codex" is selected: `"Codex OTEL config must be set in ~/.codex/config.toml — see .codex/otel-config.example.toml"`
- [ ] 10.5 Print the Kiro Bedrock logging notice when "Kiro" is selected: `"Kiro telemetry uses AWS Bedrock model invocation logging. Enable it at: AWS Console → Bedrock → Settings → Model invocation logging → CloudWatch Logs. Required IAM permission: logs:FilterLogEvents on the log group."` Also prompt for `bedrock_log_group` (default `aws/bedrock/modelinvocations`) and store in `.dreamland.json`
- [ ] 10.4 Update `cmd/init_test.go` to cover Cursor and Codex options and verify the OTEL env scaffolding step is invoked for each

## 11. OTEL Instrumentation in MCP Server

- [ ] 10.1 Create `internal/telemetry/otel.go` with `NewTracerProvider(ctx context.Context) (*sdktrace.TracerProvider, error)` that initializes an OTLP exporter when `OTEL_EXPORTER_OTLP_ENDPOINT` is set, otherwise a stdout JSON exporter
- [ ] 10.2 Add a `telemetry_write` MCP tool handler in `cmd/serve.go` that calls `telemetry.Write` — this is the endpoint that MCP-capable tools (e.g. future Cursor MCP integration) can call
- [ ] 10.3 Wrap MCP tool-call dispatch in a `mcp.tool_call` span with `ai.tool` and `ai.model` attributes from `.dreamland.json`
- [ ] 10.4 Initialize the `TracerProvider` in `cmd/serve.go` `RunE` and defer `Shutdown`
- [ ] 10.5 Write a smoke test verifying the `telemetry_write` MCP tool is registered and returns a valid response

## 12. Coverage & Quality Gate

- [ ] 12.1 Run `go test ./...` and confirm all new packages meet the ≥80% per-package floor from `scripts/pre-merge-check.sh`
- [ ] 12.2 Run `go vet ./...` and resolve any issues
- [ ] 12.3 Validate Claude Code OTEL env: in a scratch repo, run `dreamland init` with "Claude Code", inspect `.claude/settings.json` for the `SessionStart` hook entry, and confirm `.claude/scripts/dreamland-otel-env.sh` appends the three OTEL vars to a mock `CLAUDE_ENV_FILE`
- [ ] 12.4 Validate Cursor OTEL env: confirm `.cursor/hooks/dreamland-otel-env.sh` outputs valid `{"env": {...}}` JSON with the configured endpoint, and that `.cursor/hooks.json` `sessionStart` array references it
- [ ] 12.5 Validate Codex OTEL env: confirm `.codex/config.toml` has NO `[otel]` section, `.codex/otel-config.example.toml` exists with correct TOML, and the init warning message is printed
- [ ] 12.6 Validate GitHub Copilot OTEL env: confirm `.vscode/settings.json` contains all three `github.copilot.chat.otel.*` keys with correct values; verify atomic merge preserves any pre-existing keys
- [ ] 12.7 Validate Kiro OTEL env: confirm `.kiro/agent.json` `agentSpawn` array references `dreamland-otel-env.sh` and that `.kiro/dreamland-otel-setup.md` exists with shell profile instructions
- [ ] 12.7b Validate Kiro Bedrock telemetry path: run `dreamland telemetry write --tool kiro --phase start`, then simulate an `aws logs filter-log-events` response via a mock (or real call if credentials available), run `--phase stop`, and confirm `.dreamland-session.json` contains non-zero token counts and a normalized `modelId`
- [ ] 12.8 Validate Antigravity OTEL env: confirm `.agents/hooks.json` contains `SessionStart` with the four OTEL vars including `IDE_OTEL_IDE_NAME=antigravity`
- [ ] 12.8b Validate Antigravity telemetry collection: synthesize a `transcript.jsonl` file with the Antigravity schema (`usageMetadata.promptTokenCount`, `candidatesTokenCount`, `cachedContentTokenCount`, `model`), call `dreamland telemetry write --tool antigravity` with a stdin payload containing the transcript path, and confirm `.dreamland-session.json` contains correct summed token counts and the model name from the transcript
- [ ] 12.9 Verify end-to-end commit hook: in a scratch git repo, run `dreamland init` with "Claude Code", simulate a Claude Code Stop hook writing `.dreamland-session.json`, make a commit, and confirm `git log --format=%B -1` contains `AI-Model:` and `AI-InputTokens:` trailer lines
- [ ] 12.10 Verify `git interpret-trailers --parse` correctly parses the `AI-*` trailers on a commit produced by the hook
