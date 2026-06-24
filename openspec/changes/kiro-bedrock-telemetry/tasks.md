## 1. Config Model

- [ ] 1.1 Add `BedrockLogGroup string` field to `internal/config/config.go` `Config` struct with JSON tag `bedrock_log_group`; default to `"aws/bedrock/modelinvocations"` when field is empty at read time
- [ ] 1.2 Add unit test for `BedrockLogGroup` default behavior in `internal/config/config_test.go`

## 2. CLI --phase Flag

- [ ] 2.1 Add `--phase` string flag to `dreamland telemetry write` in `cmd/telemetry.go`; valid values `"start"` and `"stop"`; when `--tool kiro` is used and `--phase` is absent, default to `"stop"` for backwards compatibility with existing hook bindings
- [ ] 2.2 Pass the resolved phase value to the `Collect` interface call; update `Collector` interface in `internal/telemetry/collector.go` to accept a `phase string` parameter (or a `CollectOptions` struct if that is cleaner)
- [ ] 2.3 Write unit tests confirming unknown phase values return a non-zero exit code

## 3. Kiro Collector — Start Phase

- [ ] 3.1 In `internal/telemetry/tools/kiro.go`, implement the `start` phase: read existing `.dreamland-session.json` if present (to avoid clobbering token counts), update `session_start_time` to `time.Now().UTC().Format(time.RFC3339)`, set `tool: "kiro"`, and write atomically
- [ ] 3.2 Write unit tests for the start phase: fresh file, existing file with token counts (counts preserved), and existing file with existing `session_start_time` (overwritten)

## 4. Kiro Collector — Stop Phase

- [ ] 4.1 Implement `session_start_time` → epoch milliseconds conversion; fall back to `time.Now().Add(-1 * time.Hour)` when field is absent and write a stderr warning
- [ ] 4.2 Implement `exec.LookPath("aws")` check; on not-found, write zero-token `SnapshotResult` with warning and return nil error
- [ ] 4.3 Build and run the `aws logs filter-log-events` subprocess with `--log-group-name`, `--start-time`, `--filter-pattern '{ $.schemaType = "ModelInvocationLog" }'`, `--query 'events[*].message'`, `--output json`
- [ ] 4.4 Parse the subprocess stdout as a JSON array of strings; parse each string as a `ModelInvocationLog` JSON object; sum `input.inputTokenCount` and `output.outputTokenCount`; identify the most recent record by `timestamp` field for `modelId` extraction
- [ ] 4.5 On subprocess non-zero exit or JSON parse failure: capture stderr, include in warning message, fall back to zero tokens; always exit 0
- [ ] 4.6 Write unit tests for the stop phase using a fake `aws` binary on PATH that returns controlled JSON; test: normal multi-record response, empty array, non-JSON output, non-zero exit

## 5. modelId Normalization

- [ ] 5.1 Implement `NormalizeModelID(raw string) string` in `internal/telemetry/tools/kiro.go`: strip provider prefix (everything up to and including the first `.`), strip date-version suffix matching `-\d{8}-v\d+:\d+$`; return raw string if result is empty
- [ ] 5.2 Write table-driven unit tests for: `anthropic.claude-sonnet-4-20250514-v1:0` → `claude-sonnet-4`, `amazon.titan-text-premier-v1:0` → `titan-text-premier`, `meta.llama3-70b-instruct-v1:0` → `llama3-70b-instruct`, unknown format → raw string unchanged

## 6. Init Wizard Updates

- [ ] 6.1 In `cmd/init.go`, detect when "Kiro" is selected and inject a Bedrock logging setup notice field into the wizard display before the log group prompt
- [ ] 6.2 Add a `huh` text-input field for `Bedrock log group name` with placeholder `aws/bedrock/modelinvocations`; store result in the wizard's output config
- [ ] 6.3 Run preflight check: `exec.Command("aws", "logs", "describe-log-groups", "--log-group-name-prefix", logGroup)`; on non-zero exit, print `"Could not verify log group — proceeding anyway"` to stderr; do not abort
- [ ] 6.4 Write `bedrock_log_group` to `.dreamland.json` in the post-wizard save step
- [ ] 6.5 Write unit tests for the Kiro wizard path: default log group, custom log group, preflight failure proceeds

## 7. Validation

- [ ] 7.1 In a scratch repo, run `dreamland init` selecting Kiro, verify `.dreamland.json` contains `bedrock_log_group`
- [ ] 7.2 Simulate `agentSpawn` by calling `dreamland telemetry write --tool kiro --phase start`; confirm `.dreamland-session.json` has `session_start_time` and `tool: "kiro"`
- [ ] 7.3 Simulate `stop` by calling `dreamland telemetry write --tool kiro --phase stop` with a mock `aws` binary that returns a `ModelInvocationLog` array; confirm token sums and normalized model in `.dreamland-session.json`
- [ ] 7.4 Simulate `stop` with no `aws` on PATH; confirm zero tokens, non-empty stderr warning, exit code 0
- [ ] 7.5 Make a git commit and confirm `AI-Tool: kiro` and token trailers appear in `git log --format=%B -1`
