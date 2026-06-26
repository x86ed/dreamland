## Context

The dreamland MCP server (`cmd/serve.go`) currently registers two tools: a `hello` placeholder and `telemetry_write`. Meanwhile, the CLI commands that power dreamland's hook scripts (`transition-log`, `version-bump`, `test`, `coauthor`) each contain their run logic in unexported `run*` functions. The MCP server has no access to these functions, and the `hello` tool is vestigial.

The goal is to delete both existing tools and replace them with four tools that delegate directly to the existing CLI command logic, making the MCP server a first-class invocation path for the hook scripts alongside the shell bindings.

## Goals / Non-Goals

**Goals:**
- Remove `hello` and `telemetry_write` MCP tools
- Register `transition_log`, `version_bump`, `run_tests`, and `coauthor` as MCP tools
- Reuse existing CLI command logic; no duplication of business logic
- Each MCP tool calls the same code path as its CLI counterpart

**Non-Goals:**
- Changing the behavior of the CLI commands themselves
- Modifying shell hook binding scripts
- Adding new flags or options beyond what the CLI already supports
- Telemetry changes to the new tools (the `makeSpanHandler` wrapper handles that)

## Decisions

### Refactor `run*` functions into shared internal functions

The CLI command handlers (`runTransitionLog`, `runVersionBump`, `runTest`, `runCoauthor`) are currently wired to `cobra.Command`. To allow the MCP server to call the same logic, extract the core logic of each into a package-internal function that accepts explicit inputs rather than reading from `cobra.Command`.

**Alternative considered:** Call the CLI via `exec.Command("dreamland", "transition-log", ...)`. Rejected — subprocess invocation is fragile, adds overhead, and breaks in environments where the binary is not on PATH.

### MCP tool input schemas map to CLI flags

Each MCP tool defines an input struct mirroring the flags of the corresponding CLI command:
- `transition_log`: no inputs required (session ID resolved from env)
- `version_bump`: fields for `major`, `minor`, `patch`, `breaking` (bool flags), `version` (string)
- `run_tests`: no inputs required (reads config from repo)
- `coauthor`: optional `trailer` string (for prepare-commit-msg delegation mode)

### Keep `makeSpanHandler` wrapper

The existing OTEL span wrapper is retained and applied to all four new tools, ensuring consistent telemetry across both CLI and MCP invocation paths.

### Remove `telemetry_write` entirely

`telemetry_write` was a direct telemetry ingestion tool used by hook scripts that cannot call back into the MCP server (e.g., shell hooks). With the hook scripts now exposed as MCP tools directly, this indirection is no longer needed.

## Risks / Trade-offs

- **Breaking change for any client using `hello` or `telemetry_write`** → These tools are not part of any shipped contract (the server is new), so removal is safe. Document in the commit message.
- **Logic duplication risk during refactor** → Mitigated by the extraction pattern: CLI `RunE` functions become thin wrappers that call the shared function; MCP handlers call the same shared function.
- **`coauthor --trailer` mode in MCP context** → The trailer mode is a git hook delegation pattern. It's still exposed via MCP for completeness, but typical MCP callers will use the default (install hook) mode.

## Migration Plan

1. Extract shared logic functions from each `run*` CLI handler
2. Update CLI `RunE` functions to delegate to the shared functions
3. Remove `hello` and `telemetry_write` tool registrations and handler functions
4. Register four new MCP tools using the shared logic functions
5. Update `serve_test.go` to reflect new tool set
