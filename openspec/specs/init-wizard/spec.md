## ADDED Requirements

### Requirement: Init subcommand exists
The CLI SHALL expose an `init` subcommand registered on the root cobra command.

#### Scenario: Help text is visible
- **WHEN** user runs `dreamland init --help`
- **THEN** the command prints a short description and exits with code 0

### Requirement: Step 1 — coding tool selection
The `init` wizard SHALL present a numbered list of supported AI coding tools and require the user to select exactly one before proceeding.

Supported options: Claude Code, GitHub Copilot, Antigravity, Kiro.

#### Scenario: User selects a coding tool
- **WHEN** the user navigates the list and confirms a selection
- **THEN** the wizard records the chosen tool and advances to step 2

#### Scenario: No selection made
- **WHEN** the user presses Ctrl-C or closes stdin without selecting
- **THEN** the wizard exits with a non-zero code and writes no config file

### Requirement: Step 2 — language selection
After step 1, the wizard SHALL present a numbered list of supported languages and require the user to select exactly one.

Supported options (at minimum): Go, Node/TypeScript, Rust, Python.

#### Scenario: User selects a language
- **WHEN** the user confirms a language selection
- **THEN** the wizard records the chosen language and advances to step 3

### Requirement: Step 3 — test command
After step 2, the wizard SHALL prompt the user to enter a free-text test command string.

#### Scenario: User enters a test command
- **WHEN** the user types a non-empty string and confirms
- **THEN** the wizard records the test command and writes the config file

#### Scenario: User submits an empty test command
- **WHEN** the user confirms without typing anything
- **THEN** the wizard re-prompts until a non-empty value is provided

### Requirement: Step 4 — doc command (optional)
After step 3, the wizard SHALL prompt the user to enter a free-text documentation generation command string. This field is optional; submitting empty is allowed and stored as an empty string.

#### Scenario: User enters a doc command
- **WHEN** the user types a non-empty string and confirms
- **THEN** the wizard records the doc command and advances to step 5

#### Scenario: User skips the doc command
- **WHEN** the user confirms without typing anything
- **THEN** the wizard stores an empty string for `doc_command` and advances to step 5

### Requirement: Step 5 — version command with language-derived default
After step 4, the wizard SHALL present a pre-filled version command derived from the language selected in step 2, which the user may accept or override.

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

### Requirement: Re-initialization guard
If `.dreamland.json` already exists in the repository root, the wizard SHALL prompt the user to confirm before overwriting.

#### Scenario: User confirms re-init
- **WHEN** existing config is detected and the user confirms overwrite
- **THEN** the wizard runs all five steps and overwrites the config file

#### Scenario: User declines re-init
- **WHEN** existing config is detected and the user declines
- **THEN** the wizard exits with code 0 without modifying the config file

### Requirement: Success confirmation
After writing the config file, the wizard SHALL print the path of the written file and a success message to stdout.

#### Scenario: Successful init
- **WHEN** all five steps complete and the config file is written
- **THEN** the command prints the config file path and exits with code 0
