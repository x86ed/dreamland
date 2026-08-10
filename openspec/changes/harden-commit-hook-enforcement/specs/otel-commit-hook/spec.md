## MODIFIED Requirements

### Requirement: Hook appends AI telemetry as git trailers

The installed `commit-msg` hook SHALL call `dreamland telemetry snapshot --format trailers` and, if the output is non-empty, append it to the commit message file (passed as `$1`) with a preceding blank line separator.

The hook SHALL exit with code 0 when telemetry data is unavailable — that remains a non-blocking, silent condition; a commit made outside any dreamland-driven session (e.g. a manual `git commit` with no `.dreamland-session.json` present) is expected and legitimate.

The hook SHALL exit with a nonzero code, aborting the commit, when the `dreamland` binary itself cannot be resolved on `PATH`. This is a deliberate change from this hook's prior fail-open behavior for that specific condition, made for consistency with `.git/hooks/prepare-commit-msg` (installed by `dreamland coauthor`, see the `dev-workflow-hooks` capability), which has always failed this way for the same condition. "No telemetry data" and "`dreamland` not found" are treated differently because they are different conditions: the former is a legitimate case where nothing to append is not a failure of anything; the latter means dreamland's enforcement machinery is not even reachable, which the project's stated intent is that these hooks must not silently tolerate. The standard `git commit --no-verify` bypass remains available for a developer who needs to commit without `dreamland` installed (e.g. debugging git internals) — no new dreamland-specific bypass mechanism is introduced.

**GitHub Copilot**: Commits made in Copilot repos will have `AI-*` trailers via the `agentStop` hook (which provides `transcriptPath`). Token counts are best-effort — the Copilot transcript format is undocumented, so parsing may yield zero tokens on format changes. Native OTel via `.vscode/settings.json` provides a parallel, more reliable signal in the OTEL backend.

#### Scenario: Telemetry appended to commit message

- **WHEN** a developer runs `git commit -m "feat: add login"` and `.dreamland-session.json` exists with valid data
- **THEN** the resulting commit message contains the original text followed by a blank line and `AI-*` trailer lines (e.g., `AI-Model: claude-sonnet-4-6`, `AI-InputTokens: 15234`)

#### Scenario: No telemetry file — commit proceeds normally

- **WHEN** a developer runs `git commit` and no `.dreamland-session.json` exists
- **THEN** the commit message is unmodified and the commit succeeds with exit code 0

#### Scenario: dreamland binary not found — commit is blocked

- **WHEN** the `dreamland` binary is not on `PATH` and the `commit-msg` hook runs
- **THEN** the hook exits with a nonzero code, printing a diagnostic to stderr naming the missing binary and the `git commit --no-verify` bypass, and the commit is aborted (unless the caller used `--no-verify`, in which case the hook never runs at all)
