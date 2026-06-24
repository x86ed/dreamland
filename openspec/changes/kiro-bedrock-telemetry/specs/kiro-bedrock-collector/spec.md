## ADDED Requirements

### Requirement: dreamland telemetry write --tool kiro accepts --phase flag

`dreamland telemetry write --tool kiro` SHALL accept a `--phase` flag with values `start` and `stop`. Both phases write to `.dreamland-session.json` at the repo root.

#### Scenario: Unknown phase value is rejected
- **WHEN** `dreamland telemetry write --tool kiro --phase invalid` is called
- **THEN** the command exits with code 1 and prints `"--phase must be 'start' or 'stop'"`

---

### Requirement: start phase records session_start_time

When called with `--phase start`, `dreamland telemetry write --tool kiro` SHALL write the current UTC timestamp in RFC 3339 format to `.dreamland-session.json` as `session_start_time`. It SHALL NOT overwrite existing token count fields if the file already exists.

This is called by the Kiro `agentSpawn` hook at the beginning of every Kiro session.

#### Scenario: start phase writes session_start_time
- **WHEN** `dreamland telemetry write --tool kiro --phase start` is invoked
- **THEN** `.dreamland-session.json` contains `"session_start_time"` set to the current UTC timestamp in RFC 3339 format and `"tool": "kiro"`

#### Scenario: start phase does not clobber existing token counts
- **WHEN** `.dreamland-session.json` already contains accumulated token counts and `--phase start` is called (e.g., a new nested session)
- **THEN** `session_start_time` is updated but `input_tokens`, `output_tokens`, `cached_tokens`, and `total_tokens` are preserved

---

### Requirement: stop phase queries CloudWatch Logs for Bedrock invocations

When called with `--phase stop`, `dreamland telemetry write --tool kiro` SHALL:

1. Read `session_start_time` from `.dreamland-session.json`. If absent, use a 1-hour lookback from now and print a stderr warning.
2. Convert `session_start_time` to epoch milliseconds for the CloudWatch Logs `--start-time` parameter.
3. Execute the following command via subprocess:
   ```
   aws logs filter-log-events \
     --log-group-name <bedrock_log_group> \
     --start-time <epoch_ms> \
     --filter-pattern '{ $.schemaType = "ModelInvocationLog" }' \
     --query 'events[*].message' \
     --output json
   ```
   where `<bedrock_log_group>` comes from `cfg.BedrockLogGroup` (defaulting to `aws/bedrock/modelinvocations`).
4. Parse the returned JSON array. Each element is a JSON string containing a `ModelInvocationLog` record.
5. Sum `input.inputTokenCount` and `output.outputTokenCount` across all records.
6. Extract `modelId` from the most recent record (highest `timestamp` field) and normalize it (see normalization requirement below).
7. Write the accumulated `SnapshotResult` (adding to any existing token counts already in the file).

#### Scenario: stop phase sums tokens from multiple Bedrock invocations
- **WHEN** `--phase stop` runs and CloudWatch returns 3 `ModelInvocationLog` records with inputTokenCounts 100, 200, 300 and outputTokenCounts 50, 75, 25
- **THEN** `.dreamland-session.json` contains `input_tokens: 600`, `output_tokens: 150`, `total_tokens: 750`

#### Scenario: stop phase uses session_start_time as query window
- **WHEN** `session_start_time` is `"2026-06-23T10:00:00Z"` in `.dreamland-session.json`
- **THEN** the `aws logs filter-log-events` call includes `--start-time 1750676400000` (the epoch ms equivalent)

#### Scenario: stop phase without session_start_time falls back to 1-hour lookback
- **WHEN** `.dreamland-session.json` has no `session_start_time` field
- **THEN** `--start-time` is set to `now - 1 hour`, a stderr warning is printed, and the command still exits 0

---

### Requirement: modelId normalization

The Bedrock `modelId` field (e.g., `anthropic.claude-sonnet-4-20250514-v1:0`) SHALL be normalized to a short model name for the git trailer:

1. Strip the provider prefix up to and including the first `.` (e.g., `anthropic.` → remove)
2. Strip the date-version suffix matching the pattern `-\d{8}-v\d+:\d+$` (e.g., `-20250514-v1:0` → remove)
3. If the resulting string is empty, use the raw `modelId` as-is

#### Scenario: Anthropic Claude model ID normalization
- **WHEN** `modelId` is `"anthropic.claude-sonnet-4-20250514-v1:0"`
- **THEN** normalized model is `"claude-sonnet-4"`

#### Scenario: Amazon Titan model ID normalization
- **WHEN** `modelId` is `"amazon.titan-text-premier-v1:0"`
- **THEN** normalized model is `"titan-text-premier"`

#### Scenario: Unknown model ID format preserved as-is
- **WHEN** `modelId` does not match either stripping pattern
- **THEN** the raw `modelId` is used unchanged

---

### Requirement: Graceful fallback when AWS CLI is unavailable

If `aws` is not found on PATH, the subprocess exits with a non-zero code, the output is not valid JSON, or the output is an empty array, `dreamland telemetry write --tool kiro --phase stop` SHALL:
- Write a `SnapshotResult` with `tool: "kiro"`, `model` from `cfg.ModelID`, and all token counts zero
- Print a descriptive warning to stderr (e.g., `"kiro: aws CLI unavailable — token counts will be zero"`)
- Exit with code 0

No commit SHALL be blocked by a Bedrock query failure.

#### Scenario: aws CLI not on PATH
- **WHEN** `aws` is not installed or not on PATH
- **THEN** `.dreamland-session.json` is written with zero tokens and `model` from `.dreamland.json`; stderr contains a warning; exit code is 0

#### Scenario: CloudWatch returns empty array (no Bedrock calls logged)
- **WHEN** CloudWatch returns `[]`
- **THEN** zero tokens are written, a stderr warning notes no Bedrock invocations found, exit code is 0

#### Scenario: aws CLI returns non-zero exit code
- **WHEN** `aws logs filter-log-events` fails (e.g., access denied, network error)
- **THEN** the stderr output of the subprocess is captured and included in the dreamland warning message; exit code is 0

---

### Requirement: BedrockLogGroup config field

The `.dreamland.json` config SHALL support a `bedrock_log_group` field (string, optional). When absent, the Kiro collector defaults to `aws/bedrock/modelinvocations`.

#### Scenario: Custom log group used when configured
- **WHEN** `.dreamland.json` contains `"bedrock_log_group": "my-custom-log-group"`
- **THEN** `aws logs filter-log-events` is called with `--log-group-name my-custom-log-group`

#### Scenario: Default log group used when field absent
- **WHEN** `.dreamland.json` has no `bedrock_log_group` field
- **THEN** `aws logs filter-log-events` is called with `--log-group-name aws/bedrock/modelinvocations`
