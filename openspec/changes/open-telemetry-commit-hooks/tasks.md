## 1. Dependencies & Module Setup

- [ ] 1.1 Add `go.opentelemetry.io/otel`, `go.opentelemetry.io/otel/sdk`, `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc`, and `go.opentelemetry.io/otel/exporters/stdout/stdouttrace` to `go.mod` via `go get`
- [ ] 1.2 Run `go mod tidy` and verify the build passes with `go build ./...`

## 2. SnapshotResult Data Model

- [ ] 2.1 Create `internal/telemetry/snapshot.go` defining the `SnapshotResult` struct with all fields from the spec (`Tool`, `Model`, `ThinkingEffort`, `ContextSize`, `InputTokens`, `OutputTokens`, `CachedTokens`, `TotalTokens`, `CapturedAt`)
- [ ] 2.2 Add `ComputeTotals()` method on `SnapshotResult` that sets `TotalTokens = InputTokens + OutputTokens` when `TotalTokens` is zero
- [ ] 2.3 Add `Read(repoRoot string) (*SnapshotResult, error)` and `Write(repoRoot string, s *SnapshotResult) error` helpers in `internal/telemetry/snapshot.go` that read/write `.dreamland-session.json`; `Write` accumulates token counts instead of overwriting
- [ ] 2.4 Write unit tests for `Read`, `Write` (accumulation), and `ComputeTotals` in `internal/telemetry/snapshot_test.go`

## 3. Per-Tool Collectors

- [ ] 3.1 Create `internal/telemetry/collector.go` defining the `Collector` interface: `Parse(payload []byte) (*SnapshotResult, error)` and a `Register` map keyed by tool name string
- [ ] 3.2 Create `internal/telemetry/tools/claude.go` implementing the Claude Code collector — parse `usage.inputTokens`, `usage.outputTokens`, `usage.cacheReadInputTokens`, `usage.cacheCreationInputTokens`, `model`, and `thinking_effort` (if present) from the Stop hook JSON
- [ ] 3.3 Create `internal/telemetry/tools/copilot.go` implementing the GitHub Copilot collector — parse `prompt_tokens`, `completion_tokens`, and `model` from the `gh api /user/copilot/usage` JSON shape
- [ ] 3.4 Create `internal/telemetry/tools/cursor.go` implementing the Cursor collector — parse the MCP `telemetry_write` tool call arguments (`model`, `inputTokens`, `outputTokens`, `contextSize`, `thinkingEffort`)
- [ ] 3.5 Create `internal/telemetry/tools/codex.go` implementing the Codex CLI collector — parse the `~/.codex/sessions/<id>.json` `usage` block (`prompt_tokens`, `completion_tokens`, `model`)
- [ ] 3.6 Create `internal/telemetry/tools/kiro.go` implementing the Kiro collector — parse the `.kiro` hook payload (`usage.inputTokens`, `usage.outputTokens`, `model`)
- [ ] 3.7 Create `internal/telemetry/tools/antigravity.go` implementing the Antigravity collector — parse the `afterResponse` hook payload (`usage.inputTokens`, `usage.outputTokens`, `model`, `thinkingEffort`)
- [ ] 3.8 Register all six collectors in `internal/telemetry/collector.go` init block
- [ ] 3.9 Write table-driven unit tests for each collector in `internal/telemetry/tools/*_test.go` covering normal payloads and malformed JSON

## 4. CLI Commands — dreamland telemetry

- [ ] 4.1 Create `cmd/telemetry.go` with a `telemetryCmd` cobra command group (`dreamland telemetry`) registered on `rootCmd`
- [ ] 4.2 Implement `dreamland telemetry write` subcommand: reads `--tool` flag and `--stdin` flag, calls the matching collector, calls `telemetry.Write()` with accumulated result
- [ ] 4.3 Implement `dreamland telemetry snapshot` subcommand: reads `--format` flag (`json`|`trailers`), calls `telemetry.Read()`, outputs result; warns to stderr if `captured_at` is older than staleness threshold (default 4h, configurable via `--max-age` flag)
- [ ] 4.4 Implement `dreamland telemetry reset` subcommand: deletes `.dreamland-session.json` if it exists, no-op otherwise
- [ ] 4.5 Implement `dreamland telemetry install` subcommand: installs/updates the `commit-msg` hook using the same logic as the init scaffolding step
- [ ] 4.6 Implement `dreamland telemetry uninstall` subcommand: removes the `# BEGIN dreamland-telemetry` / `# END dreamland-telemetry` block from `.git/hooks/commit-msg`
- [ ] 4.7 Write unit tests for all subcommands in `cmd/telemetry_test.go` using table-driven tests with temp directories

## 5. Commit-msg Hook Template & Installer

- [ ] 5.1 Create `internal/scaffold/embed.go` with a `//go:embed templates/**` directive and an exported `FS` variable
- [ ] 5.2 Write the hook script template at `internal/scaffold/templates/hooks/commit-msg` (shell script that calls `dreamland telemetry snapshot --format trailers`, appends output to `$1` if non-empty, always exits 0)
- [ ] 5.3 Create `internal/scaffold/hook.go` with `InstallCommitMsgHook(repoRoot string) error` that reads the template, appends the guarded block to `.git/hooks/commit-msg` (or creates the file), sets executable bit, and skips if block already present
- [ ] 5.4 Write unit tests for `InstallCommitMsgHook` in `internal/scaffold/hook_test.go` covering: fresh install, append-to-existing, idempotent re-run

## 6. Per-Tool OTEL Config Templates & Scaffolder

- [ ] 6.1 Write Claude Code template at `internal/scaffold/templates/hooks/bindings/claude-code/settings-patch.json` — JSON fragment adding `Stop` and `PostToolUse` hooks calling `dreamland telemetry write --tool claude-code --stdin`
- [ ] 6.2 Write GitHub Copilot template at `internal/scaffold/templates/hooks/bindings/github-copilot/vscode-tasks.json` — VSCode tasks.json with `dreamland-telemetry` task
- [ ] 6.3 Write Cursor templates: `internal/scaffold/templates/hooks/bindings/cursor/mcp.json` (MCP server registration) and `internal/scaffold/templates/hooks/bindings/cursor/dreamland-telemetry.mdc` (Cursor rules file)
- [ ] 6.4 Write Codex templates: `internal/scaffold/templates/hooks/bindings/codex/instructions.md` (system prompt) and `internal/scaffold/templates/hooks/bindings/codex/codex.json` (provider config with OTEL env vars)
- [ ] 6.5 Write Kiro template at `internal/scaffold/templates/hooks/bindings/kiro/dreamland-telemetry.yaml` — Kiro `postToolUse` hook YAML
- [ ] 6.6 Write Antigravity template at `internal/scaffold/templates/hooks/bindings/antigravity/config-patch.json` — Antigravity `hooks.afterResponse` config fragment
- [ ] 6.7 Create `internal/scaffold/toolconfig.go` with `ScaffoldToolConfig(repoRoot, tool string) error` that selects and writes the correct templates for the given tool, skipping files that already exist, and prints one status line per file
- [ ] 6.8 Create `internal/scaffold/gitignore.go` with `EnsureGitignoreEntry(repoRoot, entry string) error` that appends the entry to `.gitignore` only if not already present
- [ ] 6.9 Write unit tests for `ScaffoldToolConfig` and `EnsureGitignoreEntry` in `internal/scaffold/`

## 7. Init Wizard Updates

- [ ] 7.1 Add `"Cursor"` and `"Codex"` options to the Step 1 tool select in `cmd/init.go` (expand from 4 to 6 options)
- [ ] 7.2 Add a `scaffold` step at the end of `runInit` that calls `scaffold.ScaffoldToolConfig`, `scaffold.InstallCommitMsgHook`, and `scaffold.EnsureGitignoreEntry` after `config.Save` succeeds
- [ ] 7.3 Update the success message to list each scaffolded file or print a summary count
- [ ] 7.4 Update `cmd/init_test.go` to cover the two new tool options and verify the scaffolding step is called (use a mock or temp dir)

## 8. OTEL Instrumentation in MCP Server

- [ ] 8.1 Create `internal/telemetry/otel.go` with `NewTracerProvider(ctx context.Context) (*sdktrace.TracerProvider, error)` that initializes OTLP exporter if `OTEL_EXPORTER_OTLP_ENDPOINT` is set, otherwise initializes stdout JSON exporter
- [ ] 8.2 Add a `telemetry_write` MCP tool handler in `cmd/serve.go` (or a new `internal/mcp/` package) that calls `telemetry.Write` — this is the endpoint Cursor calls
- [ ] 8.3 Wrap the existing MCP tool-call dispatch in a `mcp.tool_call` span that sets `ai.tool`, `ai.model`, and token attributes from the session snapshot
- [ ] 8.4 Initialize the `TracerProvider` in `cmd/serve.go`'s `RunE` and defer `Shutdown`
- [ ] 8.5 Write a smoke test in `cmd/serve_test.go` verifying the `telemetry_write` MCP tool is registered and returns a valid response

## 9. Coverage & Quality Gate

- [ ] 9.1 Run `go test ./...` and ensure all new packages meet the ≥80% per-package coverage floor enforced by `scripts/pre-merge-check.sh`
- [ ] 9.2 Run `go vet ./...` and resolve any issues
- [ ] 9.3 Verify `dreamland init` end-to-end in a scratch git repo: select each of the six tools, confirm the expected config files are created, and verify the `commit-msg` hook appends trailers
- [ ] 9.4 Verify `git interpret-trailers --parse` correctly parses a commit produced by the hook
