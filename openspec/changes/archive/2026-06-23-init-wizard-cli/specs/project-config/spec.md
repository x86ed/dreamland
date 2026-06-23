## ADDED Requirements

### Requirement: Config file schema
The config file SHALL be stored as JSON at `.dreamland.json` in the repository root and SHALL contain the following top-level keys:

- `coding_tool` (string) — identifier of the selected AI coding tool
- `language` (string) — identifier of the selected primary language
- `test_command` (string) — the user-supplied test runner command
- `doc_command` (string) — the user-supplied documentation generation command
- `version_command` (string) — the version check command (defaults to a language-derived value, overridable)

#### Scenario: Valid config file is written
- **WHEN** the init wizard completes successfully
- **THEN** `.dreamland.json` exists at the repo root and contains non-empty values for all three keys

### Requirement: Config file location discovery
The `project-config` package SHALL locate the config file by walking parent directories from the current working directory until a `.git` directory is found; the config file lives alongside that `.git` directory.

#### Scenario: Invoked from repo root
- **WHEN** `dreamland` is run from the directory containing `.git`
- **THEN** config is read from / written to that directory

#### Scenario: Invoked from a subdirectory
- **WHEN** `dreamland` is run from a nested subdirectory of the repo
- **THEN** config is found by walking up and is read from the repo root

#### Scenario: No git repo found
- **WHEN** no `.git` directory exists in any ancestor directory
- **THEN** the package returns an error indicating the repo root could not be found

### Requirement: Config load on startup
The root cobra command SHALL attempt to load `.dreamland.json` during `PersistentPreRunE`. If the file does not exist, startup SHALL continue normally with a nil config (not an error).

#### Scenario: Config exists at startup
- **WHEN** `.dreamland.json` is present at the repo root
- **THEN** config values are available to all subcommands before their `RunE` executes

#### Scenario: Config absent at startup
- **WHEN** `.dreamland.json` does not exist
- **THEN** the CLI starts normally and subcommands receive a nil or zero-value config

### Requirement: Config write is atomic
Writing `.dreamland.json` SHALL be performed atomically (write to a temp file, then rename) to avoid leaving a partial file on failure.

#### Scenario: Write succeeds
- **WHEN** the config is written successfully
- **THEN** `.dreamland.json` contains complete, valid JSON

#### Scenario: Write fails mid-flight
- **WHEN** the filesystem operation fails after the temp file is created but before rename
- **THEN** the original `.dreamland.json` (if any) is unchanged and an error is returned
