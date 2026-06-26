## Why

The MCP server currently exposes a `hello` placeholder and a `telemetry_write` tool, but the real work of the dreamland workflow already lives in CLI commands (`transition-log`, `version-bump`, `test`, `coauthor`). Surfacing those commands as MCP tools lets AI coding agents invoke dreamland's hook scripts directly through the server instead of requiring shell bindings.

## What Changes

- Remove the `hello` placeholder tool from the MCP server
- Remove the existing `telemetry_write` tool from the MCP server
- Add `transition_log` MCP tool — delegates to the `transition-log` CLI command logic
- Add `version_bump` MCP tool — delegates to the `version-bump` CLI command logic
- Add `run_tests` MCP tool — delegates to the `test` CLI command logic
- Add `coauthor` MCP tool — delegates to the `coauthor` CLI command logic

## Capabilities

### New Capabilities

- `mcp-hook-tools`: Four MCP tools exposing the dreamland hook scripts (`transition_log`, `version_bump`, `run_tests`, `coauthor`) so AI agents can invoke them via the MCP server

### Modified Capabilities

- `dev-workflow-hooks`: The hook scripts now have an additional invocation path via MCP in addition to direct shell bindings

## Impact

- `cmd/serve.go`: All tool registrations replaced
- Existing hook binding scripts (`internal/scaffold/templates/hooks/bindings/`) are unchanged — MCP is an additive invocation path
- No new dependencies required; existing CLI command functions will be refactored for reuse
