## ADDED Requirements

### Requirement: Per-tool OTEL configuration scaffolded on init
During `dreamland init`, after the config file is saved, the system SHALL scaffold OpenTelemetry configuration files specific to the selected AI coding tool into the repository.

Scaffolded files vary by tool as defined in the requirements below. All files are written relative to the repository root. If a file already exists, `init` SHALL NOT overwrite it; it SHALL print a notice that the file was skipped.

#### Scenario: Claude Code OTEL config created on init
- **WHEN** the user selects "Claude Code" during `dreamland init`
- **THEN** `init` writes `.claude/settings.json` additions that register a `Stop` hook calling `dreamland telemetry write --tool claude-code --stdin` (merging with any existing settings) and an `PostToolUse` hook that forwards `usage` JSON to the same command

#### Scenario: GitHub Copilot OTEL config created on init
- **WHEN** the user selects "GitHub Copilot" during `dreamland init`
- **THEN** `init` writes `.vscode/tasks.json` with a `dreamland-telemetry` task that calls `gh api /user/copilot/usage --jq '{model: .model, input_tokens: .prompt_tokens, output_tokens: .completion_tokens}' | dreamland telemetry write --tool github-copilot --stdin` and registers an `onSave`-trigger task in `tasks.json`

#### Scenario: Cursor OTEL config created on init
- **WHEN** the user selects "Cursor" during `dreamland init`
- **THEN** `init` writes `.cursor/mcp.json` registering the dreamland MCP server (if not already present) and writes `.cursor/rules/dreamland-telemetry.mdc` instructing Cursor to call the `telemetry_write` MCP tool after each response

#### Scenario: Codex OTEL config created on init
- **WHEN** the user selects "Codex" during `dreamland init`
- **THEN** `init` writes `.codex/instructions.md` with a system prompt instruction directing Codex to call `dreamland telemetry write --tool codex --stdin` at session end and writes a `codex.json` provider config setting `OTEL_EXPORTER_OTLP_ENDPOINT` if not already present

#### Scenario: Kiro OTEL config created on init
- **WHEN** the user selects "Kiro" during `dreamland init`
- **THEN** `init` writes `.kiro/hooks/dreamland-telemetry.yaml` defining a `postToolUse` hook that calls `dreamland telemetry write --tool kiro --stdin` with the usage JSON piped from Kiro's hook payload

#### Scenario: Antigravity OTEL config created on init
- **WHEN** the user selects "Antigravity" during `dreamland init`
- **THEN** `init` writes `.antigravity/config.json` (or merges into existing) adding a `hooks.afterResponse` entry that calls `dreamland telemetry write --tool antigravity --stdin`

#### Scenario: Existing config file skipped without error
- **WHEN** the target config file (e.g., `.claude/settings.json`) already exists and `init` cannot safely merge
- **THEN** `init` prints "Skipped <path>: already exists" and continues scaffolding other files without aborting

### Requirement: OTEL config templates are embedded in the binary
All per-tool configuration templates SHALL be embedded into the dreamland binary using `go:embed` so the tool functions without external template files.

#### Scenario: Binary works without a templates directory
- **WHEN** `dreamland init` is run without a `templates/` directory present on disk
- **THEN** scaffolded files are written correctly using templates compiled into the binary

### Requirement: Scaffolded files added to .gitignore exclusion
`dreamland init` SHALL add `.dreamland-session.json` to the repository's `.gitignore` (creating the file if absent) to prevent accidental commits of the session telemetry cache.

#### Scenario: .gitignore updated
- **WHEN** `dreamland init` completes successfully
- **THEN** `.dreamland-session.json` appears in `.gitignore` and is not tracked by git

#### Scenario: Duplicate .gitignore entry avoided
- **WHEN** `.dreamland-session.json` is already in `.gitignore`
- **THEN** `init` does not add a duplicate entry
