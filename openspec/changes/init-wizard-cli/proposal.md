## Why

The `dreamland` CLI has no way to know the project context (coding tool, language, test runner) for a given repository. An `init` wizard captures these choices interactively and persists them so every subsequent `dreamland` command can adapt its behavior without re-prompting the user.

## What Changes

- Add `dreamland init` subcommand with a 3-step interactive wizard.
- Step 1: select the AI coding tool in use (Claude Code, GitHub Copilot, Antigravity, Kiro).
- Step 2: select the primary language (Go, Node/TypeScript, Rust, Python).
- Step 3: enter the test command (free-text, e.g. `go test ./...`).
- Step 4: enter the doc generation command (free-text, e.g. `godoc`).
- Step 5: confirm or override the version command (pre-filled from the selected language, e.g. `go version`).
- Write selections to a `.dreamland.json` config file at the repository root.
- Load `.dreamland.json` automatically on every CLI invocation so downstream commands can read project context.

## Capabilities

### New Capabilities

- `init-wizard`: Interactive 5-step wizard that collects coding tool, language, test command, doc command, and version command selections.
- `project-config`: Config file reader/writer that persists and loads `.dreamland.json` from the repo root.

### Modified Capabilities

<!-- none -->

## Impact

- New file: `cmd/init.go` (wizard command) and `cmd/init_test.go`.
- New file: `internal/config/config.go` (config read/write) and `internal/config/config_test.go`.
- `cmd/root.go` updated to load config on startup and expose it to subcommands.
- New dependency: `github.com/charmbracelet/huh` for interactive prompts.
- `.dreamland.json` is committed to the repository (shared team config).
