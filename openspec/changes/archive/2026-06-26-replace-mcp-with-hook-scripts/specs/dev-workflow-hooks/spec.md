## ADDED Requirements

### Requirement: Hook command logic is reusable via shared functions
Each hook command (`transition-log`, `version-bump`, `test`, `coauthor`) SHALL expose its core logic through a package-internal function decoupled from `cobra.Command`, so it can be called from both the CLI runner and the MCP handler without duplication.

The CLI `RunE` functions SHALL delegate to these shared functions. The MCP tool handlers SHALL call the same shared functions.

#### Scenario: CLI and MCP invoke identical code path
- **WHEN** `dreamland transition-log` is run from the shell AND the `transition_log` MCP tool is invoked
- **THEN** both code paths call the same underlying function and produce identical side effects
