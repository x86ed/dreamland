## MODIFIED Requirements

### Requirement: Kiro tool selection triggers Bedrock logging setup

When the user selects **Kiro** in the `dreamland init` tool selection step, the wizard SHALL:

1. Print a one-time setup notice:
   ```
   Kiro telemetry uses AWS Bedrock model invocation logging.
   To enable it:
     1. Open AWS Console → Amazon Bedrock → Settings → Model invocation logging
     2. Enable logging, select "CloudWatch Logs" as destination
     3. Note the log group name (default: aws/bedrock/modelinvocations)
   Required IAM permission: logs:FilterLogEvents on the log group.
   ```
2. Prompt for `Bedrock log group name` with default `aws/bedrock/modelinvocations`.
3. (Optional preflight) Run `aws logs describe-log-groups --log-group-name-prefix <group>` to verify the log group exists and credentials are valid. If the check fails, print a warning but do not abort the wizard.
4. Store the log group name in `.dreamland.json` as `bedrock_log_group`.

#### Scenario: Kiro selected — wizard prompts for log group and stores it
- **WHEN** the user selects "Kiro" in Step 1 of the init wizard and accepts the default log group name
- **THEN** `.dreamland.json` contains `"bedrock_log_group": "aws/bedrock/modelinvocations"` and the setup notice is printed to stdout

#### Scenario: Kiro selected — user enters custom log group
- **WHEN** the user enters `"my-team-bedrock-logs"` at the log group prompt
- **THEN** `.dreamland.json` contains `"bedrock_log_group": "my-team-bedrock-logs"`

#### Scenario: Preflight check fails — wizard continues with warning
- **WHEN** `aws logs describe-log-groups` exits non-zero (e.g., log group not yet created, credentials issue)
- **THEN** a warning is printed (`"Could not verify log group — proceeding anyway"`) and the wizard completes normally; `bedrock_log_group` is still written to `.dreamland.json`
