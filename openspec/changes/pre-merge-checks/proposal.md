## Why

Merging untested, undocumented, or incorrectly versioned code to main introduces regressions and erodes code quality over time. Automating quality gates as Claude Code hooks enforces consistency without relying on manual discipline.

## What Changes

- Add a `pre-merge-gate` Claude Code hook that runs automatically when a task completes or a merge to main is triggered
- Hook validates: unit tests pass, test coverage ≥ 90% (warn at < 95%), all public methods have godoc comments, and the minor or major version has been bumped (patch-only bumps are not accepted)
- If Go files lack test coverage, the hook auto-generates missing `_test.go` files and iterates until coverage exceeds 90%

## Capabilities

### New Capabilities

- `pre-merge-gate`: Automated quality gate suite wired to Claude Code hooks — runs test execution, coverage analysis, godoc validation, and version bump verification before any merge to main; auto-remediates missing tests.

### Modified Capabilities

<!-- None -->

## Impact

- `.claude/settings.json` — new hook entries wired to the merge/task-stop lifecycle
- Go source files in `cmd/` — may receive generated `_test.go` files if coverage is below threshold
- `VERSION` file — minor or major version bump enforced before merge; patch-only bump rejected
