## ADDED Requirements

### Requirement: commit-msg hook installed by dreamland init
`dreamland init` SHALL install a `commit-msg` git hook at `.git/hooks/commit-msg` that appends AI session telemetry as git trailers to every commit message.

The hook script SHALL be guarded by `# BEGIN dreamland-telemetry` and `# END dreamland-telemetry` markers. If a `commit-msg` hook already exists, `init` SHALL append the dreamland block after the existing content; it SHALL NOT replace the existing hook.

The installed hook SHALL be executable (`chmod +x`).

#### Scenario: Hook installed on fresh repo
- **WHEN** `dreamland init` completes on a repo with no existing `commit-msg` hook
- **THEN** `.git/hooks/commit-msg` is created, is executable, and contains the dreamland block

#### Scenario: Hook appended to existing commit-msg
- **WHEN** `dreamland init` completes on a repo where `.git/hooks/commit-msg` already exists with custom content
- **THEN** the existing content is preserved and the dreamland block is appended after it

#### Scenario: Hook not duplicated on re-init
- **WHEN** `dreamland init` is run a second time on a repo that already has the dreamland block in `commit-msg`
- **THEN** the block is not added again; the file is unchanged

### Requirement: Hook appends AI telemetry as git trailers
The installed `commit-msg` hook SHALL call `dreamland telemetry snapshot --format trailers` and, if the output is non-empty, append it to the commit message file (passed as `$1`) with a preceding blank line separator.

The hook SHALL exit with code 0 regardless of whether telemetry data is available, so it never blocks a commit.

#### Scenario: Telemetry appended to commit message
- **WHEN** a developer runs `git commit -m "feat: add login"` and `.dreamland-session.json` exists with valid data
- **THEN** the resulting commit message contains the original text followed by a blank line and `AI-*` trailer lines (e.g., `AI-Model: claude-sonnet-4-6`, `AI-InputTokens: 15234`)

#### Scenario: No telemetry file — commit proceeds normally
- **WHEN** a developer runs `git commit` and no `.dreamland-session.json` exists
- **THEN** the commit message is unmodified and the commit succeeds with exit code 0

#### Scenario: dreamland binary not found — commit proceeds normally
- **WHEN** the `dreamland` binary is not on PATH
- **THEN** the hook exits with code 0 without modifying the commit message

### Requirement: Trailer key format
AI telemetry trailers SHALL use the prefix `AI-` followed by a PascalCase field name. The defined trailer keys are:

| Trailer Key | Maps to SnapshotResult field |
|-------------|------------------------------|
| `AI-Tool` | `tool` |
| `AI-Model` | `model` |
| `AI-ThinkingEffort` | `thinking_effort` |
| `AI-ContextSize` | `context_size` |
| `AI-InputTokens` | `input_tokens` |
| `AI-OutputTokens` | `output_tokens` |
| `AI-CachedTokens` | `cached_tokens` |
| `AI-TotalTokens` | `total_tokens` |
| `AI-CapturedAt` | `captured_at` |

Fields with zero or empty values SHALL be omitted from the trailer output.

#### Scenario: Only populated fields appear in trailers
- **WHEN** `thinking_effort` and `context_size` are not available for the active tool
- **THEN** the commit message contains no `AI-ThinkingEffort` or `AI-ContextSize` trailer lines

### Requirement: Trailers are parseable by git interpret-trailers
The appended trailer lines SHALL conform to the git trailer format so that `git interpret-trailers --parse <commit>` returns each `AI-*` key-value pair correctly.

#### Scenario: git interpret-trailers parses AI trailers
- **WHEN** a commit with AI trailers is parsed with `git interpret-trailers --parse`
- **THEN** each `AI-*` key appears as a separate trailer entry with the correct value

### Requirement: Hook installed by dreamland telemetry install command
The CLI SHALL expose a `dreamland telemetry install` subcommand that installs the `commit-msg` hook independently of `dreamland init`, for use in repos that have already been initialized.

#### Scenario: Hook installed standalone
- **WHEN** `dreamland telemetry install` is run in a git repo
- **THEN** the `commit-msg` hook is installed or updated following the same append/guard logic as `dreamland init`
