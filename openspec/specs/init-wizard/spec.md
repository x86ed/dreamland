# init-wizard

## Requirements

### Requirement: Init subcommand exists

The CLI SHALL expose an `init` subcommand registered on the root cobra command.

#### Scenario: Help text is visible

- **WHEN** user runs `dreamland init --help`
- **THEN** the command prints a short description and exits with code 0

### Requirement: Step 1 — repository root selection

The `init` wizard SHALL present an editable text input pre-filled with the detected git repository root as the first step, allowing the user to confirm or override the directory where agent and hook files will be installed. This is step 1 of 6.

#### Scenario: User accepts the detected repository root

- **WHEN** the wizard opens and a git root is detected
- **THEN** step 1 shows the detected absolute path pre-filled and the user may press Enter to accept it

#### Scenario: User overrides the repository root

- **WHEN** the user clears the pre-filled value and types a different path
- **THEN** the wizard records the custom path as `repo_root` and advances to step 2

#### Scenario: Non-existent path entered

- **WHEN** the user enters a path that does not exist on disk
- **THEN** the wizard re-prompts with an error: "Path does not exist"

### Requirement: Step 2 — coding tool selection

The `init` wizard SHALL present a numbered list of supported AI coding tools and require the user to select exactly one before proceeding. This is step 2 of 6.

Supported options: Claude Code, Codex CLI, Cursor, GitHub Copilot, Antigravity, Kiro.

#### Scenario: User selects a coding tool

- **WHEN** the user navigates the list and confirms a selection
- **THEN** the wizard records the chosen tool and advances to step 3

#### Scenario: No selection made

- **WHEN** the user presses Ctrl-C or closes stdin without selecting
- **THEN** the wizard exits with a non-zero code and writes no config file

### Requirement: Step 3 — language selection

After step 2, the wizard SHALL present a numbered list of supported languages and require the user to select exactly one. This is step 3 of 6.

Supported options (at minimum): Go, Node/TypeScript, Rust, Python.

#### Scenario: User selects a language

- **WHEN** the user confirms a language selection
- **THEN** the wizard records the chosen language and advances to step 4

### Requirement: Step 4 — test command

After step 3, the wizard SHALL prompt the user to enter a free-text test command string. This is step 4 of 6.

#### Scenario: User enters a test command

- **WHEN** the user types a non-empty string and confirms
- **THEN** the wizard records the test command and advances to step 5

#### Scenario: User submits an empty test command

- **WHEN** the user confirms without typing anything
- **THEN** the wizard re-prompts until a non-empty value is provided

### Requirement: Step 5 — doc command (optional)

After step 4, the wizard SHALL prompt the user to enter a free-text documentation generation command string. This field is optional; submitting empty is allowed and stored as an empty string. This is step 5 of 6.

#### Scenario: User enters a doc command

- **WHEN** the user types a non-empty string and confirms
- **THEN** the wizard records the doc command and advances to step 6

#### Scenario: User skips the doc command

- **WHEN** the user confirms without typing anything
- **THEN** the wizard stores an empty string for `doc_command` and advances to step 6

### Requirement: Step 6 — version command with language-derived default

After step 5, the wizard SHALL present a pre-filled version command derived from the language selected in step 3, which the user may accept or override. This is step 6 of 6.

Default values by language:

- Go → `go version`
- Node/TypeScript → `node --version`
- Rust → `rustc --version`
- Python → `python3 --version`

#### Scenario: User accepts the default version command

- **WHEN** the user confirms without modifying the pre-filled value
- **THEN** the wizard records the default version command and proceeds to write the config

#### Scenario: User overrides the version command

- **WHEN** the user clears the pre-filled value and types a custom command
- **THEN** the wizard records the custom command and proceeds to write the config

### Requirement: Scaffolding is triggered after config is written

After successfully writing `.dreamland.json`, the `init` command SHALL invoke the scaffold installer for the selected coding tool and repo root, installing agent and hook files and reporting results to stdout.

#### Scenario: Scaffold runs after successful init

- **WHEN** all six wizard steps complete and config is written
- **THEN** agent files and hook scripts are installed and a per-file summary is printed before the final success message

#### Scenario: Scaffold failure does not prevent config from being written

- **WHEN** the config file is written successfully but scaffolding encounters a filesystem error
- **THEN** the config file is preserved and the error is reported to stderr with a non-zero exit code

### Requirement: Init accepts --email-suffix flag for agent/model email domain

The `init` command SHALL accept an `--email-suffix` string flag (default `@github.com`). The value is stored in `.dreamland.json` as `email_suffix` and used by `dreamland coauthor` to construct AgentEmail and ModelEmail at runtime.

#### Scenario: Default suffix stored when flag absent

- **WHEN** `dreamland init` completes without `--email-suffix`
- **THEN** `.dreamland.json` contains `"email_suffix": "@github.com"`

#### Scenario: Custom suffix stored when flag provided

- **WHEN** `dreamland init --email-suffix @myorg.com` completes
- **THEN** `.dreamland.json` contains `"email_suffix": "@myorg.com"`

### Requirement: Init accepts --force flag to overwrite scaffold files

The `init` command SHALL accept a `--force` boolean flag. When set, the scaffold installer overwrites existing agent and hook files rather than skipping them.

#### Scenario: --force overwrites existing agent and hook files

- **WHEN** `dreamland init --force` is run and agent or hook files already exist
- **THEN** all files are overwritten with current template content and stdout shows `installed (forced):` for each

### Requirement: repo_root is persisted in config

The `repo_root` value collected in step 1 SHALL be written to `.dreamland.json` as `"repo_root"`.

#### Scenario: repo_root written to config

- **WHEN** the wizard completes with a non-default repo root entered
- **THEN** `.dreamland.json` contains `"repo_root": "<entered path>"`

#### Scenario: repo_root defaults when field absent in existing config

- **WHEN** `.dreamland.json` exists without a `repo_root` field and `config.Load` is called
- **THEN** `RepoRoot` on the returned struct is empty string (callers fall back to git root detection)

### Requirement: Re-initialization guard

If `.dreamland.json` already exists in the repository root, the wizard SHALL prompt the user to confirm before overwriting.

#### Scenario: User confirms re-init

- **WHEN** existing config is detected and the user confirms overwrite
- **THEN** the wizard runs all six steps and overwrites the config file

#### Scenario: User declines re-init

- **WHEN** existing config is detected and the user declines
- **THEN** the wizard exits with code 0 without modifying the config file

### Requirement: Success confirmation

After writing the config file, the wizard SHALL print the path of the written file and a success message to stdout.

#### Scenario: Successful init

- **WHEN** all six steps complete and the config file is written
- **THEN** the command prints the config file path and exits with code 0
