## ADDED Requirements

### Requirement: Four lifecycle commands added to the dreamland binary

The `dreamland` binary SHALL expose four new cobra subcommands that implement agent lifecycle hook logic. These commands are the same across all platforms; only the binding files that invoke them differ.

```text
dreamland version-bump [--major | --minor | --patch] [--version <semver>] [--breaking]
dreamland coauthor
dreamland transition-log
dreamland test
```

Each command reads `.dreamland.json` for configuration and exits 0 on success, non-zero on failure. No shell scripts are installed; hook bindings reference these binary commands directly.

#### Scenario: Commands are available after installing the binary

- **WHEN** the `dreamland` binary is installed and `dreamland --help` is run
- **THEN** `version-bump`, `coauthor`, `transition-log`, and `test` appear in the command list

### Requirement: version-bump minor/major fires once per branch at session start

`dreamland version-bump` (without `--patch`) SHALL run at the session-start lifecycle event. It performs a minor or major version bump **at most once per branch**, enforced by a branch marker file at `.dreamland/branch-bumps`.

Algorithm:

1. If `git diff <last-tag>..HEAD` is empty (no commits since last tag), exit 0 silently.
2. Read `.dreamland/branch-bumps`. If the current branch name has an entry, exit 0 silently.
3. If the branch has no remote upstream (`@{u}` unset), treat it as a new uninitialized branch.
4. Bump `--minor` by default; bump `--major` if `--breaking` is passed.
5. Write `<branch-name>=<new-version>` to `.dreamland/branch-bumps`.
6. If no upstream was set in step 3, push: `git push --set-upstream origin <branch>`.

Version bump is dispatched to the language-appropriate tool from `.dreamland.json`:

| Language | Default `version_bump_command` | Invocation |
| --- | --- | --- |
| Go | _(empty)_ | dreamland creates an annotated git tag directly |
| Node/TypeScript | `npm version` | `npm version minor\|major` |
| Rust | `cargo bump` | `cargo bump minor\|major` |
| Python | `bump-my-version bump` | `bump-my-version bump minor\|major` |

#### Scenario: Minor bump on first session of a new branch

- **WHEN** `dreamland version-bump` runs, the current branch is not in `.dreamland/branch-bumps`, and commits exist since the last tag
- **THEN** the minor version is incremented and `<branch>=<new-version>` is written to `.dreamland/branch-bumps`

#### Scenario: Skipped on second and subsequent sessions on same branch

- **WHEN** `dreamland version-bump` runs and the current branch is already in `.dreamland/branch-bumps`
- **THEN** the command exits 0 silently without bumping

#### Scenario: Major bump when --breaking passed

- **WHEN** `dreamland version-bump --breaking` runs on a branch not in `.dreamland/branch-bumps`
- **THEN** the major version is incremented

#### Scenario: Upstream pushed when branch has no remote

- **WHEN** `dreamland version-bump` runs, the branch has no remote upstream, and a minor/major bump is performed
- **THEN** the branch is pushed with `git push --set-upstream origin <branch>` after bumping

#### Scenario: No-op when no commits since last tag

- **WHEN** `dreamland version-bump` runs and `git diff <last-tag>..HEAD` returns no output
- **THEN** the command exits 0 without modifying any file or creating any tag

#### Scenario: First-ever version bump on repo with no tags

- **WHEN** `dreamland version-bump` runs and no semver tags exist in the repo
- **THEN** the command treats the baseline as `v0.0.0` and creates the appropriate first tag (e.g., `v0.1.0` for a minor bump)

#### Scenario: Explicit version override updates marker

- **WHEN** `dreamland version-bump --version v2.0.0` runs
- **THEN** the version is set to `v2.0.0` and the branch marker is updated, regardless of prior bump history

#### Scenario: cargo bump absent

- **WHEN** `version_bump_command` is `cargo bump` and the `cargo-bump` plugin is not installed
- **THEN** the command prints a human-readable install hint and exits non-zero

### Requirement: version-bump --patch fires at end of turn when code changed

`dreamland version-bump --patch` SHALL run at the end-of-turn (`Stop`) lifecycle event. It checks `git diff <last-tag>..HEAD` for changes; if none are present it exits 0 silently. If changes are present, it bumps the patch version using the configured tool. It does not consult or update `.dreamland/branch-bumps`.

| Language | Default `version_bump_command` | Invocation |
| --- | --- | --- |
| Go | _(empty)_ | dreamland creates an annotated git tag directly |
| Node/TypeScript | `npm version` | `npm version patch` |
| Rust | `cargo bump` | `cargo bump patch` |
| Python | `bump-my-version bump` | `bump-my-version bump patch` |

#### Scenario: Patch bump when code changed at end of turn

- **WHEN** `dreamland version-bump --patch` runs and `git diff <last-tag>..HEAD` is non-empty
- **THEN** the patch version is incremented using the configured tool

#### Scenario: No-op at end of turn when no code changed

- **WHEN** `dreamland version-bump --patch` runs and `git diff <last-tag>..HEAD` is empty
- **THEN** the command exits 0 silently

### Requirement: coauthor sets agent identity and installs prepare-commit-msg hook

`dreamland coauthor` SHALL run at the session-start lifecycle event and perform two actions:

**a. Set agent git identity (repository-local scope):**

AgentName is read from the platform's current-agent env var at runtime (e.g., `CLAUDE_AGENT_ID`); falls back to the coding tool name in `.dreamland.json`. AgentEmail is derived by cleaning AgentName and appending `email_suffix` from `.dreamland.json` (default `@github.com`).

Email cleaning: lowercase → replace spaces and underscores with `-` → strip characters not in `[a-z0-9.\-]` → trim leading/trailing `-` and `.`.

`git config --local user.name` is set to AgentName. `git config --local user.email` is set to AgentEmail.

**b. Install a `prepare-commit-msg` git hook:**

Write (or update) `.git/hooks/prepare-commit-msg` as a minimal shell wrapper that delegates to `dreamland`:

```sh
#!/bin/sh
dreamland coauthor --trailer "$1" "$2" "$3"
```

When invoked with `--trailer`, `dreamland coauthor` reads `$1` (commit message file path), constructs model identity from `.dreamland.json`, and appends to the file if no matching trailer is already present:

```text
Co-authored-by: <model-name> <model-email>
```

`model-name` is the name portion of `model_id` (text before the first space). `model-email` is the cleaned model-name plus `email_suffix`. All logic in Go; no shell tools required.

The hook file is written with mode 0755. If `.git/hooks/prepare-commit-msg` already contains `dreamland coauthor --trailer`, the file is left unchanged.

#### Scenario: Agent git identity set from env var at session start

- **WHEN** `dreamland coauthor` runs and the platform env var for current agent is set (e.g., `CLAUDE_AGENT_ID=orchestrator`)
- **THEN** `git config --local user.name` is set to `"orchestrator"` and `git config --local user.email` to `"orchestrator@github.com"` (with configured suffix)

#### Scenario: Agent git identity falls back to coding tool name

- **WHEN** `dreamland coauthor` runs and no platform agent env var is set
- **THEN** `git config --local user.name` is set to the coding tool name from `.dreamland.json` (e.g., `"Claude Code"`) and email to `"claude-code@github.com"`

#### Scenario: Email cleaning applied to agent name

- **WHEN** AgentName is `"Spec Writer"` and `email_suffix` is `@github.com`
- **THEN** AgentEmail is `"spec-writer@github.com"`

#### Scenario: prepare-commit-msg hook installed

- **WHEN** `dreamland coauthor` runs and `.git/hooks/prepare-commit-msg` does not exist
- **THEN** the file is created with mode 0755 containing `#!/bin/sh` and `dreamland coauthor --trailer "$1" "$2" "$3"`

#### Scenario: prepare-commit-msg hook is idempotent

- **WHEN** `dreamland coauthor` runs and `.git/hooks/prepare-commit-msg` already contains the delegation line
- **THEN** the file is not modified

#### Scenario: Co-authored-by trailer appended by --trailer mode

- **WHEN** `dreamland coauthor --trailer <file>` runs and the commit message does not contain a matching `Co-authored-by:` line
- **THEN** `Co-authored-by: <model-name> <model-email>` is appended to the file

#### Scenario: Co-authored-by trailer not duplicated

- **WHEN** `dreamland coauthor --trailer <file>` runs and the commit message already contains `Co-authored-by: <model-name>`
- **THEN** the file is not modified

### Requirement: init accepts --email-suffix to configure the agent/model email domain

The `init` command SHALL accept an `--email-suffix` flag (default `@github.com`). The value is stored in `.dreamland.json` as `email_suffix` and used by `dreamland coauthor` to construct AgentEmail and ModelEmail.

#### Scenario: Default email suffix used when flag absent

- **WHEN** `dreamland init` runs without `--email-suffix`
- **THEN** `.dreamland.json` contains `"email_suffix": "@github.com"`

#### Scenario: Custom suffix stored when flag provided

- **WHEN** `dreamland init --email-suffix @myorg.com` runs
- **THEN** `.dreamland.json` contains `"email_suffix": "@myorg.com"`

### Requirement: transition-log appends a timestamped line at end of turn

`dreamland transition-log` SHALL append one line to `.dreamland/transition.log` on every invocation:

```text
<ISO-8601 timestamp> [<session-id>] turn complete
```

`session-id` is read from the `CLAUDE_SESSION_ID`, `CODEX_SESSION_ID`, or equivalent environment variable set by the platform. If no session variable is found, a random 8-character hex ID is generated for the line.

The `.dreamland/` directory is created if absent. Appending to the log is always a no-op failure-safe: a write error does not cause a non-zero exit.

#### Scenario: Log entry written on every turn

- **WHEN** `dreamland transition-log` runs
- **THEN** `.dreamland/transition.log` gains one new line with a valid ISO-8601 timestamp and session identifier

#### Scenario: Directory created when absent

- **WHEN** `dreamland transition-log` runs and `.dreamland/` does not exist
- **THEN** the directory is created and the log file is written

### Requirement: test only runs when executable source files changed

`dreamland test` SHALL inspect `git status --porcelain` for unstaged and staged changes to source files matching the configured language's extensions. If matching files are present, it runs `test_command` from `.dreamland.json` and exits with that command's exit code. If no matching files are found, it exits 0 silently.

Source file extensions by language:

| Language | Extensions |
| --- | --- |
| Go | `.go` |
| Node/TypeScript | `.ts`, `.tsx`, `.js`, `.jsx`, `.mts`, `.cts` |
| Rust | `.rs` |
| Python | `.py` |

#### Scenario: Tests run after source code changes

- **WHEN** `dreamland test` runs and `git status --porcelain` shows at least one file with a matching source extension
- **THEN** `test_command` from `.dreamland.json` is executed and its exit code is forwarded

#### Scenario: Tests skipped when no source changed

- **WHEN** `dreamland test` runs and no source files have changes
- **THEN** the command exits 0 without running anything

#### Scenario: Test failure propagates exit code

- **WHEN** `dreamland test` runs, source files changed, and `test_command` exits non-zero
- **THEN** `dreamland test` exits with the same non-zero code

### Requirement: Hook logic is platform-independent

The four lifecycle commands (`version-bump`, `coauthor`, `transition-log`, `test`) are implemented entirely in Go and are identical in behavior on every platform. Platform-specific variation is limited to the binding files that invoke them.

#### Scenario: Identical behavior across platforms

- **WHEN** `dreamland test` is run on a Cursor project and a Claude Code project with the same `.dreamland.json`
- **THEN** the behavior and output are identical in both cases

### Requirement: All session-start commands bind to the platform's session-start event

The scaffold installer SHALL register `dreamland version-bump` and `dreamland coauthor` under each platform's session-start event when writing hook binding files.

Session-start event names by platform (verified from platform docs):

| Platform | Session-start event | Scope |
| --- | --- | --- |
| Claude Code | `SessionStart` | All modes |
| Codex CLI | `SessionStart` | All modes |
| Cursor | `sessionStart` | Agent, ask, edit modes |
| Kiro CLI | `agentSpawn` | Agent mode only |
| Antigravity | stub | Session-start undocumented |
| GitHub Copilot | stub | No public hook API |

#### Scenario: Session-start commands registered for Claude Code

- **WHEN** `dreamland init` completes with "Claude Code" selected
- **THEN** `.claude/settings.json` contains `dreamland version-bump` and `dreamland coauthor` under the `SessionStart` event key

#### Scenario: Session-start commands registered for Kiro

- **WHEN** `dreamland init` completes with "Kiro" selected
- **THEN** `.kiro/agent.json` contains `dreamland version-bump` and `dreamland coauthor` under the `agentSpawn` event key

### Requirement: All end-of-turn commands bind to the platform's stop event

The scaffold installer SHALL register `dreamland version-bump --patch`, `dreamland transition-log`, and `dreamland test` under each platform's end-of-turn event.

End-of-turn event names by platform:

| Platform | End-of-turn event |
| --- | --- |
| Claude Code | `Stop` |
| Codex CLI | `Stop` |
| Cursor | `stop` |
| Kiro CLI | `stop` |
| Antigravity | `PostTurnHook` |
| GitHub Copilot | stub — no public hook API |

#### Scenario: End-of-turn commands registered for Claude Code

- **WHEN** `dreamland init` completes with "Claude Code" selected
- **THEN** `.claude/settings.json` contains `dreamland version-bump --patch`, `dreamland transition-log`, and `dreamland test` under the `Stop` event key

#### Scenario: End-of-turn commands registered for Cursor

- **WHEN** `dreamland init` completes with "Cursor" selected
- **THEN** `.cursor/hooks.json` contains `dreamland version-bump --patch`, `dreamland transition-log`, and `dreamland test` under the `stop` key within a `version: 1` envelope

### Requirement: Claude Code hook binding via settings.json merge

For "Claude Code", the scaffold installer SHALL merge the hook registrations into `.claude/settings.json` under `hooks.SessionStart` and `hooks.Stop`, creating the file if absent. The merge is atomic: write to a temp file in the same directory, then rename into place.

If the file already contains dreamland hook entries they are replaced, not duplicated.

#### Scenario: Claude Code settings.json created when absent

- **WHEN** `.claude/settings.json` does not exist and platform is "Claude Code"
- **THEN** the file is created with `SessionStart` and `Stop` hook registrations

#### Scenario: Claude Code settings.json merged when present

- **WHEN** `.claude/settings.json` already exists with other settings
- **THEN** existing non-hook settings are preserved and hook entries are added under `SessionStart` and `Stop`

### Requirement: Codex CLI hook binding via hooks.json merge

For "Codex CLI", the scaffold installer SHALL merge hook registrations into `.codex/hooks.json` under `SessionStart` and `Stop` keys, creating the file if absent.

#### Scenario: Codex hooks.json created when absent

- **WHEN** `.codex/hooks.json` does not exist and platform is "Codex CLI"
- **THEN** the file is created with `SessionStart` and `Stop` hook registrations

### Requirement: Cursor hook binding via hooks.json merge

For "Cursor", the scaffold installer SHALL merge hook registrations into `.cursor/hooks.json` using `version: 1` envelope with `sessionStart` and `stop` event keys.

#### Scenario: Cursor hooks.json created when absent

- **WHEN** `.cursor/hooks.json` does not exist and platform is "Cursor"
- **THEN** the file is created as `{"version": 1, "hooks": {"sessionStart": [...], "stop": [...]}}`

### Requirement: Kiro hook binding via agent.json merge

For "Kiro", the scaffold installer SHALL merge hook registrations into `.kiro/agent.json` with `agentSpawn` and `stop` keys.

#### Scenario: Kiro agent.json created when absent

- **WHEN** `.kiro/agent.json` does not exist and platform is "Kiro"
- **THEN** the file is created with `agentSpawn` and `stop` hook registrations

### Requirement: Antigravity hook binding via plugin hooks.json

For "Antigravity", the scaffold installer SHALL write `hooks.json` into the plugin bundle at `~/.gemini/antigravity-cli/plugins/dreamland/`. End-of-turn commands bind to `PostTurnHook` (the documented Antigravity equivalent of `Stop`). Session-start binding is stubbed with `"_note"` because Antigravity's session-start hook is not yet publicly documented. The file is marked `"_preview": true`.

#### Scenario: Antigravity plugin hooks.json written

- **WHEN** `dreamland init` completes with "Antigravity" selected
- **THEN** `~/.gemini/antigravity-cli/plugins/dreamland/hooks.json` exists with `PostTurnHook` end-of-turn registrations and a stub session-start note

### Requirement: GitHub Copilot hook binding is a documented stub

For "GitHub Copilot", the scaffold installer SHALL write `.github/copilot-hooks/hooks-manifest.json` as a stub with a `_note` field explaining the schema is not yet publicly documented.

#### Scenario: GitHub Copilot stub manifest written

- **WHEN** `dreamland init` completes with "GitHub Copilot" selected
- **THEN** `.github/copilot-hooks/hooks-manifest.json` exists and contains a `_note` field identifying it as a placeholder

### Requirement: JSON config merges are atomic

All platform hook binding files that require merging into existing JSON (Claude Code, Codex, Cursor, Kiro) SHALL use an atomic write strategy: write the merged content to a temp file in the same directory, then call `os.Rename(tempFile, targetFile)`. The original file is never modified directly.

#### Scenario: Original file preserved on write failure

- **WHEN** a merge write fails after the temp file is created but before rename completes
- **THEN** the original config file remains unchanged

### Requirement: init sets version_bump_command and model_id in config

The `init` wizard SHALL set `version_bump_command` and `model_id` in `.dreamland.json` based on the language and coding tool selections, so that `dreamland version-bump` and `dreamland coauthor` can operate without further configuration.

#### Scenario: version_bump_command set for Node project

- **WHEN** `dreamland init` completes with "Node/TypeScript" selected
- **THEN** `.dreamland.json` contains `"version_bump_command": "npm version"`

#### Scenario: model_id set for Claude Code project

- **WHEN** `dreamland init` completes with "Claude Code" selected
- **THEN** `.dreamland.json` contains `"model_id": "claude-sonnet-4-6"` (or the current default model)
