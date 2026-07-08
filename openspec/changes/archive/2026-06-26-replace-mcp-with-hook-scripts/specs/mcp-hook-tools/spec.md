## ADDED Requirements

### Requirement: MCP server exposes transition_log tool
The MCP server SHALL register a `transition_log` tool that invokes the same logic as `dreamland transition-log`. It accepts no inputs and appends a timestamped turn-complete entry to `.dreamland/transition.log`.

#### Scenario: transition_log tool called via MCP
- **WHEN** an MCP client calls the `transition_log` tool
- **THEN** a timestamped line is appended to `.dreamland/transition.log` and the tool returns a success result

### Requirement: MCP server exposes version_bump tool
The MCP server SHALL register a `version_bump` tool that invokes the same logic as `dreamland version-bump`. It accepts optional boolean inputs `major`, `minor`, `patch`, `breaking` and an optional string input `version`.

#### Scenario: version_bump tool called with --patch semantics via MCP
- **WHEN** an MCP client calls `version_bump` with `{"patch": true}`
- **THEN** the patch version is incremented if source changes exist, identical to running `dreamland version-bump --patch`

#### Scenario: version_bump tool called with no inputs via MCP
- **WHEN** an MCP client calls `version_bump` with no inputs
- **THEN** the minor/major session-start bump logic executes, identical to running `dreamland version-bump`

### Requirement: MCP server exposes run_tests tool
The MCP server SHALL register a `run_tests` tool that invokes the same logic as `dreamland test`. It accepts no inputs and runs the project's configured test command if source files have changed.

#### Scenario: run_tests tool called via MCP when source changed
- **WHEN** an MCP client calls `run_tests` and source files have changed since the last commit
- **THEN** the configured `test_command` is executed and its output is returned in the tool result

#### Scenario: run_tests tool returns success when no source changed
- **WHEN** an MCP client calls `run_tests` and no source files have changed
- **THEN** the tool returns a success result without running any command

### Requirement: MCP server exposes coauthor tool
The MCP server SHALL register a `coauthor` tool that invokes the same logic as `dreamland coauthor`. It accepts an optional string input `trailer` (commit message file path for prepare-commit-msg delegation mode).

#### Scenario: coauthor tool called in default mode via MCP
- **WHEN** an MCP client calls `coauthor` with no inputs
- **THEN** agent git identity is set and the `prepare-commit-msg` hook is installed, identical to running `dreamland coauthor`

#### Scenario: coauthor tool called in trailer mode via MCP
- **WHEN** an MCP client calls `coauthor` with `{"trailer": "<path-to-commit-msg-file>"}`
- **THEN** the Co-authored-by trailer is appended to the commit message file, identical to running `dreamland coauthor --trailer <path>`

### Requirement: Removed hello and telemetry_write tools
The `hello` and `telemetry_write` MCP tools SHALL be removed from the server.

#### Scenario: hello tool no longer available
- **WHEN** an MCP client requests the tool list
- **THEN** `hello` is NOT present in the list

#### Scenario: telemetry_write tool no longer available
- **WHEN** an MCP client requests the tool list
- **THEN** `telemetry_write` is NOT present in the list

### Requirement: All MCP hook tools are wrapped with OTEL spans
Each of the four hook tools SHALL be wrapped with the `makeSpanHandler` OTEL span wrapper, recording the tool name and model ID as span attributes.

#### Scenario: Span recorded on tool invocation
- **WHEN** any hook MCP tool is called
- **THEN** a `mcp.tool_call` span is created with the `ai.tool` attribute set to the tool name
