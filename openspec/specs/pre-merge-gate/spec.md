# pre-merge-gate

Automated quality gate suite wired to Claude Code hooks. Runs before any merge to main.

## Requirements

### Requirement: Unit tests pass

All Go unit tests in the repository SHALL pass before a merge to main is permitted.

#### Scenario: All tests pass

- **WHEN** `go test ./...` exits with code 0
- **THEN** the gate proceeds to the next check

#### Scenario: One or more tests fail

- **WHEN** `go test ./...` exits with a non-zero code
- **THEN** the gate MUST abort and print the failing test output

---

### Requirement: Test coverage threshold enforcement

Aggregate test coverage across all packages SHALL be at or above 90%. The gate MUST emit a warning (but not fail) if coverage is below 95%.

#### Scenario: Coverage meets the required threshold

- **WHEN** total coverage reported by `go tool cover` is ≥ 90%
- **THEN** the gate proceeds; if coverage is also ≥ 95% no warning is emitted

#### Scenario: Coverage is between 90% and 95%

- **WHEN** total coverage is ≥ 90% but < 95%
- **THEN** the gate proceeds AND prints a warning: "Coverage is X% — consider raising it above 95%"

#### Scenario: Coverage is below 90%

- **WHEN** total coverage is < 90%
- **THEN** the gate MUST abort with a non-zero exit code and list the under-covered packages

#### Scenario: Individual package coverage is below 80%

- **WHEN** any single package has coverage < 80% even if aggregate is ≥ 90%
- **THEN** the gate MUST abort and identify the specific package(s) below threshold

---

### Requirement: Auto-remediation of missing tests

When coverage is below 90%, the gate SHALL scaffold `_test.go` stub files for packages that lack test files and signal Claude to complete them.

#### Scenario: Package has no test file

- **WHEN** a Go package directory contains no `*_test.go` files AND aggregate coverage is < 90%
- **THEN** the gate MUST create a `<package>_test.go` stub with `TODO` markers for each exported symbol and exit non-zero with instructions for Claude to implement the tests

#### Scenario: Package has test file but still below threshold

- **WHEN** a package has an existing `*_test.go` file but its coverage is below 80%
- **THEN** the gate MUST exit non-zero with a message identifying the package and its current coverage percentage

---

### Requirement: Godoc on all exported symbols

Every exported function, method, and type in `.go` source files SHALL have a godoc comment immediately preceding its declaration.

#### Scenario: All exports are documented

- **WHEN** no exported symbol is missing a `//` comment on the line immediately before it
- **THEN** the gate proceeds

#### Scenario: One or more exports lack documentation

- **WHEN** an exported `func`, `type`, or `var` declaration has no preceding `//` comment within 3 lines
- **THEN** the gate MUST abort, list each undocumented symbol with its file and line number, and exit non-zero

---

### Requirement: Minor or major version bump before merge

The branch SHALL carry a Git semver tag (`v{major}.{minor}.{patch}`) whose major or minor segment is higher than the latest tag reachable from `main`. A patch-only increment SHALL NOT satisfy this requirement. For a major version bump, `go.mod` MUST also reflect the updated module path (e.g., `module dreamland/v2`).

#### Scenario: Tag major segment is incremented

- **WHEN** `git describe --tags --abbrev=0` on the current branch resolves a tag with a higher major segment than `git describe --tags --abbrev=0 main`
- **THEN** the gate verifies `go.mod` contains the updated major module path; if so, the gate proceeds

#### Scenario: Tag major segment is incremented but go.mod not updated

- **WHEN** the tag's major segment is higher than main's but the `module` line in `go.mod` still uses the old major path
- **THEN** the gate MUST abort with a message instructing the developer to update the module path in `go.mod`

#### Scenario: Tag minor segment is incremented

- **WHEN** the major segment matches `main` but the minor segment is higher
- **THEN** the gate proceeds

#### Scenario: Only patch segment is incremented

- **WHEN** the major and minor segments match `main`'s latest tag but the patch segment is higher
- **THEN** the gate MUST abort with a message explaining that a patch-only tag is not sufficient and exit non-zero

#### Scenario: No new tag exists on the branch

- **WHEN** `git describe --tags --abbrev=0` resolves the same tag as `main`, or no tag exists on the current branch
- **THEN** the gate MUST abort with a message instructing the developer to create a `v{major}.{minor}.{patch}` tag before merging

---

### Requirement: Claude Code Stop hook wiring

The gate script SHALL be invoked automatically by a Claude Code `Stop` hook so it runs whenever Claude finishes a task.

#### Scenario: Hook triggers gate script

- **WHEN** Claude Code fires the `Stop` lifecycle event
- **THEN** `scripts/pre-merge-check.sh` MUST be executed automatically

#### Scenario: Gate exits non-zero

- **WHEN** `scripts/pre-merge-check.sh` exits with a non-zero code
- **THEN** Claude Code MUST surface the failure output to the user

#### Scenario: Not targeting main

- **WHEN** the current branch is not intended for merging to main (no upstream or branch name indicates otherwise)
- **THEN** the gate MAY skip all checks and exit 0 to avoid blocking unrelated work
