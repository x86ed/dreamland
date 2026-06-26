## 1. Refactor CLI command logic into shared functions

- [x] 1.1 Extract `runTransitionLog` core logic into a package-internal `execTransitionLog() error` function; update `RunE` to delegate to it
- [x] 1.2 Extract `runVersionBump` core logic into a package-internal `execVersionBump(major, minor, patch, breaking bool, version string) error` function; update `RunE` to delegate to it
- [x] 1.3 Extract `runTest` core logic into a package-internal `execTest() error` function; update `RunE` to delegate to it
- [x] 1.4 Extract `runCoauthor` core logic into a package-internal `execCoauthor(trailer string) error` function; update `RunE` to delegate to it

## 2. Replace MCP server tools

- [x] 2.1 Remove the `hello` tool handler, input/output structs, and `mcp.AddTool` registration from `cmd/serve.go`
- [x] 2.2 Remove the `telemetry_write` tool handler, input/output structs, and `mcp.AddTool` registration from `cmd/serve.go`
- [x] 2.3 Add `transition_log` MCP tool: define input/output structs (no required inputs), implement handler calling `execTransitionLog`, register with `makeSpanHandler`
- [x] 2.4 Add `version_bump` MCP tool: define input struct with `major`, `minor`, `patch`, `breaking` bool fields and `version` string field, implement handler calling `execVersionBump`, register with `makeSpanHandler`
- [x] 2.5 Add `run_tests` MCP tool: define input/output structs (no required inputs), implement handler calling `execTest`, register with `makeSpanHandler`
- [x] 2.6 Add `coauthor` MCP tool: define input struct with optional `trailer` string field, implement handler calling `execCoauthor`, register with `makeSpanHandler`

## 3. Update tests

- [x] 3.1 Update `cmd/serve_test.go` to remove tests for `hello` and `telemetry_write` tools
- [x] 3.2 Add tests to `cmd/serve_test.go` verifying the four new tools are registered and return expected results
- [x] 3.3 Verify existing CLI command tests still pass after the shared-function refactor
