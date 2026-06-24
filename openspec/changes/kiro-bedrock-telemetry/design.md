## Context

Kiro is AWS's AI coding tool. Unlike Claude Code, Codex, and Cursor — which expose `transcript_path` in their stop hook stdin — Kiro's `stop` hook only delivers `{hook_event_name, cwd, session_id, assistant_response}`. There is no token data, no model name, and no transcript file reference in Kiro's hook payload.

Kiro makes all LLM calls through Amazon Bedrock using the user's own AWS account and credentials. AWS Bedrock model invocation logging ([docs](https://docs.aws.amazon.com/bedrock/latest/userguide/model-invocation-logging.html)) captures a `ModelInvocationLog` record for every `Converse`/`InvokeModel` call, including `modelId`, `input.inputTokenCount`, and `output.outputTokenCount`. This logging is account-level, enabled once, and persists across all Bedrock usage in that region.

The `open-telemetry-commit-hooks` change provides the `SnapshotResult` struct, the `telemetry write` CLI skeleton, and the `agentSpawn`/`stop` hook binding files for Kiro. This change replaces the stub `kiro.go` collector with a two-phase implementation that uses CloudWatch Logs to recover real session telemetry.

## Goals / Non-Goals

**Goals:**
- Produce real `input_tokens`, `output_tokens`, and `model` for Kiro in the git commit trailer
- Zero new Go module dependencies (shell out to `aws` CLI)
- Graceful fallback to zero tokens when AWS prerequisites are not met
- Guide the user through Bedrock logging setup at `dreamland init` time

**Non-Goals:**
- Real-time token streaming during a Kiro session
- Per-tool or per-feature breakdown of token usage within a session
- Supporting other Bedrock-hosted tools (this collector is Kiro-specific)
- CloudWatch metrics (only CloudWatch Logs is used)

## Decisions

### D1: Two-phase collection via `--phase start|stop`

**Decision:** Add a `--phase` flag to `dreamland telemetry write --tool kiro`. The `agentSpawn` hook calls `--phase start` to stamp `session_start_time` into `.dreamland-session.json`. The `stop` hook calls `--phase stop` to query CloudWatch Logs for Bedrock invocations since that timestamp.

**Why not query at commit time instead?** The `commit-msg` hook would need to know the last Kiro session window without a start timestamp. Without `session_start_time`, we'd have to guess (e.g., last 4 hours), which risks including invocations from other sessions or unrelated Bedrock usage. The start timestamp gives us a precise window.

**Alternatives considered:**
- *Single hook at `stop` time using a fixed lookback window*: Imprecise — picks up unrelated Bedrock calls. Rejected.
- *Write start time from `agentSpawn` via a separate CLI command*: Same as `--phase start` but requires a second binary invocation. Same cost; cleaner to reuse `telemetry write`.

---

### D2: AWS CLI instead of Go SDK

**Decision:** Shell out to `aws logs filter-log-events` rather than importing `github.com/aws/aws-sdk-go-v2`.

**Rationale:** The AWS CLI is a hard prerequisite for using Kiro at all (Kiro itself requires configured AWS credentials and the CLI for authentication flows). Adding the Go SDK would add ~10 transitive dependencies to `go.mod` for functionality that's available free via a subprocess call.

**Alternatives considered:**
- *AWS SDK for Go v2*: Cleaner Go code, no subprocess. Rejected because the dependency cost outweighs the benefit for a dev tool.
- *Direct CloudWatch Logs HTTP API*: Would require manual Signature V4 signing. Rejected.

The subprocess call uses `exec.LookPath("aws")` to detect CLI absence and fall back gracefully.

---

### D3: Log group name stored in `.dreamland.json`

**Decision:** `bedrock_log_group` stored in `.dreamland.json` alongside `otel_endpoint`. Default: `aws/bedrock/modelinvocations` (the AWS-assigned name when CloudWatch Logs destination is selected). User can override if they use a custom log group.

**Why store it?** The log group name is set at Bedrock logging configuration time, not at session time. It doesn't change between sessions. Storing it avoids querying the Bedrock settings API on every hook invocation.

---

### D4: `modelId` normalization

**Decision:** Strip the provider prefix (`anthropic.`, `amazon.`, `meta.`, etc.) and the version/release suffix (`-v1:0`, `-v2:1`, etc.) from the raw Bedrock `modelId`. Example: `anthropic.claude-sonnet-4-20250514-v1:0` → `claude-sonnet-4`.

**Why normalize?** The raw Bedrock model ARN is unreadable in a git commit trailer. The normalized form matches what other tools (Claude Code, Codex, Cursor) report.

**Risk:** New model IDs with unexpected formats may not strip cleanly. Mitigation: the raw `modelId` is preserved in the stderr log; if normalization produces an empty string, the raw ID is used as-is.

---

### D5: Scope window — only current Kiro session

**Decision:** The CloudWatch query uses `--start-time <session_start_epoch_ms>` with no `--end-time`. This returns all Bedrock invocations from session start to the time the `stop` hook fires.

**Risk:** If the user has other Bedrock usage (non-Kiro) concurrently in the same AWS account and region, those invocations will be included. Mitigation: this is a known limitation noted in the commit trailer via the `AI-Tool: kiro` field — readers can correlate with the CloudWatch log for details.

## Risks / Trade-offs

**[Bedrock logging off by default]** → Users must manually enable it once. Mitigation: `dreamland init` prints the exact console navigation path and required IAM permission.

**[AWS credentials not configured]** → The collector falls back to zero tokens and exits 0. No commit is blocked.

**[CloudWatch log group missing or wrong name]** → Same fallback. `dreamland init` stores the correct name at setup time.

**[Cross-session token bleed]** → Other Bedrock usage in the same account/region since `session_start_time` is included. Acceptable for developer-level observability; not a billing system.

**[`aws` CLI version variation]** → `filter-log-events` and `--filter-pattern` with JMESPath have been stable since AWS CLI v2. No known compatibility issue. Minimum version check not enforced; errors fall back gracefully.

## Migration Plan

1. Implement `open-telemetry-commit-hooks` first — provides the stub `kiro.go` and the hook binding
2. Apply this change after — replaces the stub with the two-phase CloudWatch collector
3. Existing repos that ran `dreamland init` with Kiro get the new collector on next `go install` / binary update; no re-init required (`.dreamland.json` gets `bedrock_log_group` with default value added on next `dreamland init` run with `--force` or via a migration that writes the default if the key is absent)

## Open Questions

1. **Regional log group**: Bedrock logging is per-region. If the user's Kiro calls span multiple regions, should dreamland query all regions or just one? For now: query the configured region via `AWS_REGION` / `aws configure get region`. Multi-region support is deferred.
2. **IAM permission for `logs:FilterLogEvents`**: Should `dreamland init` run a preflight check (`aws logs describe-log-groups`) to validate credentials and log group access before writing the config? Recommend yes — fail fast at init time, not at commit time.
