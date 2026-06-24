## Why

Kiro (AWS's AI coding tool) has no hook payload containing token counts or model names, but it runs every LLM call through Amazon Bedrock in the user's own AWS account. Bedrock model invocation logging captures `modelId`, `inputTokenCount`, and `outputTokenCount` for every call — this is the real telemetry source for Kiro. The main `open-telemetry-commit-hooks` change stubs Kiro out; this change implements the actual data collection path via CloudWatch Logs.

## What Changes

- Add `bedrock_log_group` field to `.dreamland.json` (default `aws/bedrock/modelinvocations`)
- Add `--phase start|stop` flag to `dreamland telemetry write --tool kiro` to support a two-phase collection pattern
- `agentSpawn` hook calls `dreamland telemetry write --tool kiro --phase start` to record `session_start_time`
- `stop` hook calls `dreamland telemetry write --tool kiro --phase stop` to query CloudWatch Logs for Bedrock invocations since session start, sum token counts, and normalize `modelId`
- `dreamland init` with Kiro selected prompts for `bedrock_log_group` and prints a one-time setup notice for enabling Bedrock logging
- No AWS SDK dependency — uses `aws logs filter-log-events` via the `aws` CLI (already required for Kiro)

## Capabilities

### New Capabilities
- `kiro-bedrock-collector`: Two-phase `dreamland telemetry write --tool kiro` implementation that records session start time and queries CloudWatch Logs for Bedrock invocation records to produce real token counts and model name at commit time

### Modified Capabilities
- `init-wizard`: Kiro selection adds `bedrock_log_group` prompt and Bedrock logging setup notice (delta spec)

## Impact

- `internal/config/config.go` — add `BedrockLogGroup string` field
- `internal/telemetry/tools/kiro.go` — replace stub with two-phase CloudWatch collector
- `cmd/telemetry.go` — add `--phase` flag to `telemetry write` subcommand
- `cmd/init.go` — Kiro path prompts for log group, prints setup instructions
- No new Go module dependencies (shells out to `aws` CLI)
- Depends on: `open-telemetry-commit-hooks` change (provides the `SnapshotResult` struct, `telemetry write` command skeleton, and `agentSpawn`/`stop` hook binding for Kiro)
