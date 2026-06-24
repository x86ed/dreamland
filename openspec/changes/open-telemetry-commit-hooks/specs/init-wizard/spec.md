## MODIFIED Requirements

### Requirement: Step 1 — coding tool selection
The `init` wizard SHALL present a numbered list of supported AI coding tools and require the user to select exactly one before proceeding.

Supported options: Claude Code, GitHub Copilot, Antigravity, Kiro, Cursor, Codex.

#### Scenario: User selects a coding tool
- **WHEN** the user navigates the list and confirms a selection
- **THEN** the wizard records the chosen tool and advances to step 2

#### Scenario: No selection made
- **WHEN** the user presses Ctrl-C or closes stdin without selecting
- **THEN** the wizard exits with a non-zero code and writes no config file

## ADDED Requirements

### Requirement: Post-save scaffolding step
After writing `.dreamland.json`, `dreamland init` SHALL execute a scaffolding step that:
1. Creates per-tool OTEL configuration files (as defined in the `otel-tool-config` spec)
2. Installs the `commit-msg` git hook (as defined in the `otel-commit-hook` spec)
3. Adds `.dreamland-session.json` to `.gitignore`

The scaffolding step SHALL print one line per file written/skipped/updated.

#### Scenario: Scaffolding runs after config write
- **WHEN** all five wizard steps complete and `.dreamland.json` is written
- **THEN** the tool-specific OTEL config files and the `commit-msg` hook are created before the success message is printed

#### Scenario: Scaffolding failures do not abort init
- **WHEN** a scaffolding file write fails (e.g., permission denied on `.git/hooks/`)
- **THEN** `init` prints the error for that file and continues scaffolding remaining files; the final exit code is 0 if `.dreamland.json` was written successfully
