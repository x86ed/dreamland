## Why

Dreamland has no mechanism to scaffold the AI coding agents and lifecycle hooks that drive its spec-driven workflow; developers must configure these manually per platform, which is error-prone and inconsistent. Adding agent and hook scaffolding as part of `init` gives every project a working, platform-correct setup from day one across all six supported coding tools.

## What Changes

- `init` wizard gains a new first step to capture the **repository root** (the directory where agent/hook files are installed), defaulting to the detected git root
- The coding tool list expands to six options: Claude Code, Codex CLI, Cursor, GitHub Copilot, Antigravity, Kiro
- After wizard completion, `init` invokes a new scaffolding stage that writes platform-specific agent definitions and hooks into the repo
- Five **agent definition files** are installed per platform in the platform's native format and directory:
  - Orchestrator/router
  - Spec writer
  - Code implementer
  - Test/validation runner
  - PR closer
- Four **lifecycle commands** are added to the `dreamland` binary; platform hook bindings invoke these commands directly:
  - `dreamland version-bump [--major|--minor|--patch|--version <semver>]`
  - `dreamland transition-log`
  - `dreamland test`
  - `dreamland coauthor`
- All agent and hook template files are **embedded in the binary** (Go `embed`) — nothing is read from disk at install time
- **BREAKING**: `init` wizard step numbering shifts; "Step 1" is now repository root selection, and step count increases to 6

## Capabilities

### New Capabilities

- `agent-scaffolding`: Install platform-specific AI agent definition files (orchestrator, spec-writer, implementer, tester, pr-closer) into the repo based on the selected coding tool, using each platform's native format and directory convention
- `dev-workflow-hooks`: Install four lifecycle hook scripts with platform-appropriate event bindings; hook business logic is identical across all platforms, only the binding layer differs

### Modified Capabilities

- `init-wizard`: Add repository root step (new step 1); expand coding tool list to include Codex CLI and Cursor; trigger agent and hook scaffolding after config is written

## Impact

- `cmd/init.go` — add repo-root step, expand tool options, add `--force` flag, call scaffolding after `config.Save`
- New `cmd/version_bump.go`, `cmd/coauthor.go`, `cmd/transition_log.go`, `cmd/test.go` — four cobra subcommands for lifecycle logic
- New `internal/scaffold/` package — embed templates, write agent files and hook binding files per platform
- Agent templates: `internal/scaffold/templates/agents/<platform>/` (one directory per tool)
- Hook bindings: `internal/scaffold/templates/hooks/bindings/<platform>/` (one JSON binding file per tool; commands call `dreamland <subcommand>` directly)
- `internal/config/` — extend `Config` struct with `RepoRoot`, `VersionBumpCommand`, `ModelID`, `AgentName`, `AgentEmail` fields
- No external dependencies added (uses stdlib `embed`, `os`, `path/filepath`)
