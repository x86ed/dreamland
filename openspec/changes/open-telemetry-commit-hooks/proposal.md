## Why

AI coding tool sessions produce rich telemetry — model name, token counts, thinking effort, context size — that today disappears after each session. Capturing this data in commit messages creates a permanent, auditable record of every agent-assisted change, enabling cost attribution, quality analysis, and compliance reporting across all supported tools.

## What Changes

- `dreamland init` gains per-tool scaffolding that writes OpenTelemetry configuration files specific to each AI coding tool (Antigravity, Claude Code, Codex, Cursor, GitHub Copilot/VSCode, Kiro)
- New `dreamland telemetry snapshot` CLI command queries the active tool's API/SDK to collect model name, thinking effort, context size, input/output/cached/total token counts
- New git `commit-msg` hook template appended by `init` that runs `dreamland telemetry snapshot` and injects the JSON/text payload into the commit message footer
- OpenTelemetry SDK wired into dreamland's MCP server to emit spans for every tool call, completing the observability loop

## Capabilities

### New Capabilities

- `otel-tool-config`: Per-tool OpenTelemetry configuration scaffolding — what files are written, where they go, and what they enable for each of the six supported tools during `dreamland init`
- `otel-session-telemetry`: `dreamland telemetry snapshot` command — how it discovers the active tool, what fields it collects, and how it surfaces the data (stdout JSON + OTEL span)
- `otel-commit-hook`: Git `commit-msg` hook template — lifecycle (when it runs, what it appends, failure behavior) and the format of the telemetry footer block in commit messages

### Modified Capabilities

- `init-wizard`: `dreamland init` must scaffold the new per-tool OTEL config files and install the `commit-msg` hook in addition to its current outputs

## Impact

- **New files**: `internal/telemetry/`, `internal/scaffold/templates/hooks/otel/commit-msg`, per-tool config templates under `internal/scaffold/templates/hooks/bindings/<tool>/`
- **Modified files**: `cmd/init.go`, `internal/scaffold/` (new scaffolding step), `go.mod` (add `go.opentelemetry.io/otel` SDK)
- **Dependencies**: `go.opentelemetry.io/otel`, `go.opentelemetry.io/otel/sdk`, `go.opentelemetry.io/otel/exporters/otlp/otlptrace` (or stdout exporter for local dev)
- **External APIs**: Each tool's telemetry API/SDK (Claude Code usage events, Copilot API, Cursor metadata endpoint, Codex API, Kiro SDK, Antigravity SDK)
- **No breaking changes** to existing CLI commands or `.dreamland.json` schema (new fields are additive)
