## ADDED Requirements

### Requirement: Four lifecycle hook scripts are installed

After `dreamland init` completes, the scaffold installer SHALL write four hook scripts to `.dreamland/hooks/` in the repository:

- `version-bump.sh` — bumps the patch version if code has changed since the last commit
- `transition-log.sh` — appends a timestamped turn summary to `.dreamland/transition.log`
- `run-tests.sh` — runs the configured test command if executable code changed during the turn
- `coauthor.sh` — amends the most recent commit with a co-author trailer if a commit was made during the turn; otherwise updates `git config user.name` and `git config user.email` locally

#### Scenario: Hook scripts written to .dreamland/hooks/

- **WHEN** `dreamland init` completes successfully for any supported platform
- **THEN** the four `.sh` files exist under `.dreamland/hooks/` and are marked executable (mode 0755)

### Requirement: Hook logic is identical across platforms

The content of the four hook scripts SHALL be the same regardless of the selected coding tool; only the platform binding (the hook config file that registers them) differs.

#### Scenario: Script content is platform-independent

- **WHEN** `dreamland init` is run with "Claude Code" selected in one repo and "Cursor" selected in another
- **THEN** the content of each `.sh` file under `.dreamland/hooks/` is byte-for-byte identical in both repos

### Requirement: All four hooks bind to the end-of-turn event

The scaffold installer SHALL register all four hook scripts against the single end-of-turn lifecycle event on the selected platform. The end-of-turn event fires once per agent turn after all tool calls complete and before the agent hands control back to the user. Conditional logic inside each script determines whether to act.

End-of-turn event names by platform:

| Platform | Event name |
| --- | --- |
| Claude Code | `Stop` |
| Codex CLI | `Stop` |
| Cursor | `stop` |
| Kiro CLI | `stop` |
| Antigravity | `Stop` |
| GitHub Copilot | stub — schema not public |

#### Scenario: All four hooks registered to end-of-turn event for Claude Code

- **WHEN** `dreamland init` completes with "Claude Code" selected
- **THEN** `.claude/settings.json` contains all four hook script paths under the `Stop` event key

#### Scenario: All four hooks registered to end-of-turn event for Codex CLI

- **WHEN** `dreamland init` completes with "Codex CLI" selected
- **THEN** `.codex/hooks.json` contains all four hook script paths under the `Stop` event key

#### Scenario: All four hooks registered to end-of-turn event for Cursor

- **WHEN** `dreamland init` completes with "Cursor" selected
- **THEN** `.cursor/hooks.json` contains all four hook script paths under the `stop` event key with a `version: 1` envelope

#### Scenario: All four hooks registered to end-of-turn event for Kiro

- **WHEN** `dreamland init` completes with "Kiro" selected
- **THEN** `.kiro/agent.json` contains all four hook script paths under the `stop` event key

#### Scenario: All four hooks registered to end-of-turn event for Antigravity

- **WHEN** `dreamland init` completes with "Antigravity" selected
- **THEN** `~/.gemini/antigravity-cli/plugins/dreamland/hooks.json` contains all four hook script paths under the `Stop` event key

### Requirement: version-bump.sh only acts when code has changed

The `version-bump.sh` script SHALL check whether any tracked files have changed since the last commit before bumping the version. If no changes are detected it SHALL exit 0 without modifying any file.

#### Scenario: Version bumped when changes exist

- **WHEN** `version-bump.sh` runs and `git diff HEAD` returns non-empty output
- **THEN** the patch version in the project's version file is incremented and the change is committed

#### Scenario: Version not bumped when no changes

- **WHEN** `version-bump.sh` runs and `git diff HEAD` returns empty output
- **THEN** the script exits 0 without modifying any file

### Requirement: transition-log.sh always logs at end of turn

The `transition-log.sh` script SHALL append a timestamped entry to `.dreamland/transition.log` on every invocation, recording the session ID and a brief turn summary.

#### Scenario: Log entry written

- **WHEN** `transition-log.sh` runs
- **THEN** `.dreamland/transition.log` gains a new line with an ISO timestamp and session identifier

### Requirement: run-tests.sh only acts when executable code has changed

The `run-tests.sh` script SHALL check whether any source code files (determined by checking `git diff --name-only HEAD` against known executable file extensions for the configured language) changed during the turn. If no relevant files changed it SHALL exit 0 without running tests.

#### Scenario: Tests run after code change

- **WHEN** `run-tests.sh` runs and `git diff --name-only HEAD` includes at least one source file matching the project language's extensions
- **THEN** the script reads `test_command` from `.dreamland.json` and executes it, exiting with the same exit code

#### Scenario: Tests skipped when no code changed

- **WHEN** `run-tests.sh` runs and no source files have changed since the last commit
- **THEN** the script exits 0 without running tests

### Requirement: coauthor.sh amends commit if one was made during the turn, otherwise updates git config

The `coauthor.sh` script SHALL determine whether a commit was made during the current agent turn by comparing `git log` against a session-start timestamp written to `.dreamland/session-start` at session initialization. If a commit was made, it amends it with a `Co-authored-by:` trailer. If no commit was made, it updates `git config user.name` and `git config user.email` to the configured co-author identity.

#### Scenario: Co-author trailer added when commit was made during turn

- **WHEN** `coauthor.sh` runs and `git log --since` (using the session-start timestamp) returns at least one commit
- **THEN** the most recent commit is amended to include a `Co-authored-by: <name> <email>` trailer

#### Scenario: Git config updated when no commit was made during turn

- **WHEN** `coauthor.sh` runs and no commits have been made since the session-start timestamp
- **THEN** `git config user.name` and `git config user.email` are set to the configured co-author values

### Requirement: Claude Code hook binding via settings.json merge

For "Claude Code", the scaffold installer SHALL merge the four hook registrations into `.claude/settings.json` under the `hooks.Stop` key, creating the file if absent. The merge is atomic: read → merge in memory → write. If the file already contains dreamland hook entries they are replaced, not duplicated.

#### Scenario: Claude Code settings.json created when absent

- **WHEN** `.claude/settings.json` does not exist and platform is "Claude Code"
- **THEN** the file is created with a valid JSON object containing the four hook registrations under `Stop`

#### Scenario: Claude Code settings.json merged when present

- **WHEN** `.claude/settings.json` already exists with other settings and platform is "Claude Code"
- **THEN** existing non-hook settings are preserved and hook entries are added under `Stop`

### Requirement: Codex CLI hook binding via hooks.json merge

For "Codex CLI", the scaffold installer SHALL merge the four hook registrations into `.codex/hooks.json` under the `Stop` key, creating the file if absent.

#### Scenario: Codex hooks.json created when absent

- **WHEN** `.codex/hooks.json` does not exist and platform is "Codex CLI"
- **THEN** the file is created with a valid JSON object containing the four hook registrations under `Stop`

#### Scenario: Codex hooks.json merged when present

- **WHEN** `.codex/hooks.json` already exists with other hooks and platform is "Codex CLI"
- **THEN** existing hook entries are preserved and dreamland entries are added under `Stop`

### Requirement: Cursor hook binding via hooks.json merge

For "Cursor", the scaffold installer SHALL merge the four hook registrations into `.cursor/hooks.json` using the `version: 1` envelope and `stop` event key, creating the file if absent.

#### Scenario: Cursor hooks.json created when absent

- **WHEN** `.cursor/hooks.json` does not exist and platform is "Cursor"
- **THEN** the file is created containing `{"version": 1, "hooks": {"stop": [...]}}` with the four hook registrations

#### Scenario: Cursor hooks.json merged when present

- **WHEN** `.cursor/hooks.json` already exists and platform is "Cursor"
- **THEN** existing settings and hook entries are preserved and dreamland entries are appended under `stop`

### Requirement: Kiro hook binding via agent.json merge

For "Kiro", the scaffold installer SHALL merge the four hook registrations into `.kiro/agent.json` under the `hooks.stop` key using Kiro CLI's JSON agent config schema, creating the file if absent.

#### Scenario: Kiro agent.json created when absent

- **WHEN** `.kiro/agent.json` does not exist and platform is "Kiro"
- **THEN** the file is created with a valid JSON object containing the four hook registrations under `stop`

#### Scenario: Kiro agent.json merged when present

- **WHEN** `.kiro/agent.json` already exists and platform is "Kiro"
- **THEN** existing config is preserved and dreamland entries are appended under `stop`

### Requirement: Antigravity hook binding via plugin hooks.json

For "Antigravity", the scaffold installer SHALL write `hooks.json` into the plugin bundle at `~/.gemini/antigravity-cli/plugins/dreamland/` using the same `Stop` event schema as Claude Code.

#### Scenario: Antigravity plugin hooks.json written

- **WHEN** `dreamland init` completes with "Antigravity" selected
- **THEN** `~/.gemini/antigravity-cli/plugins/dreamland/hooks.json` exists and contains the four hook registrations under `Stop`

### Requirement: GitHub Copilot hook binding is a documented stub

For "GitHub Copilot", the scaffold installer SHALL write `.github/copilot-hooks/hooks-manifest.json` as a stub with a `_note` field explaining the schema is not yet publicly documented.

#### Scenario: GitHub Copilot stub manifest written

- **WHEN** `dreamland init` completes with "GitHub Copilot" selected
- **THEN** `.github/copilot-hooks/hooks-manifest.json` exists and contains a `_note` field identifying it as a placeholder

### Requirement: Existing hook scripts are not overwritten by default

If any hook script already exists at its target path, the installer SHALL skip that file and report it as skipped to stdout.

#### Scenario: Hook script already exists, no force flag

- **WHEN** `.dreamland/hooks/run-tests.sh` already exists and `dreamland init` is run without `--force`
- **THEN** the existing file is not modified and stdout includes `skipped (already exists): .dreamland/hooks/run-tests.sh`

#### Scenario: Hook script overwritten with force flag

- **WHEN** `.dreamland/hooks/run-tests.sh` already exists and `dreamland init` is run with `--force`
- **THEN** the file is overwritten with the current embedded template content
