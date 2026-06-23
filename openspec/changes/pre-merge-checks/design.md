## Context

Dreamland is a Go CLI/MCP server built with Cobra and the MCP Go SDK. It currently has no automated quality enforcement before merging to main. The project uses Claude Code as the primary development environment, with `.claude/settings.json` controlling hook behavior. The hook lifecycle (`PreToolUse`, `PostToolUse`, `Stop`) allows scripts to be executed automatically around Claude's actions.

The goal is to wire shell-based checks into Claude Code's `Stop` hook (fires when Claude finishes a task), acting as a merge gate before any push to main.

## Goals / Non-Goals

**Goals:**
- Run `go test ./...` and fail if any test fails
- Assert coverage ≥ 90% across all packages; emit a warning if below 95%
- Detect public functions/methods/types missing godoc comments and fail
- Detect if the `VERSION` file has had its minor or major segment incremented relative to `main` (patch-only increment is rejected)
- If coverage is below 90%, auto-generate `_test.go` stubs and prompt Claude to fill them in until the threshold is met
- All checks run as a single shell script invoked by the Claude Code `Stop` hook

**Non-Goals:**
- Replacing a CI system (GitHub Actions, etc.) — this is a local developer gate, not a server-side enforcement
- Enforcing formatting or linting beyond godoc
- Checking patch-only version bumps (minor or major bump required; patch-only is explicitly rejected)
- Modifying the hook for every tool call (only `Stop`, not `PreToolUse`/`PostToolUse`)

## Decisions

### 1. Single shell script, invoked by the `Stop` hook

**Decision:** All checks live in `scripts/pre-merge-check.sh`; `.claude/settings.json` adds a `Stop` hook that calls it.

**Rationale:** Keeps logic in one place, easy to run manually (`bash scripts/pre-merge-check.sh`), and avoids duplicating config across multiple hook entries. The `Stop` hook fires after Claude completes a response, making it the natural "task done" signal.

**Alternative considered:** Separate hook entries per check — rejected because ordering and failure aggregation become harder to manage.

### 2. Coverage via `go test -coverprofile` + `go tool cover`

**Decision:** Run `go test -coverprofile=coverage.out ./...` then parse total coverage from `go tool cover -func=coverage.out`.

**Rationale:** Standard Go toolchain, no extra dependencies. The `total:` line in `go tool cover` output gives an aggregate percentage.

**Alternative considered:** `go-coverage-report` or third-party tools — rejected to keep zero new dependencies.

### 3. Godoc check via `grep` on exported symbols

**Decision:** Use `grep` to find exported functions/types without a preceding comment line and report them.

**Rationale:** `go doc` and `golint` are not always available; a grep-based check is portable and requires no install. Pattern: any line matching `^func [A-Z]` or `^type [A-Z]` not preceded by a `//` comment.

**Alternative considered:** `golint` or `staticcheck` — valid but adds a tool dependency; can be upgraded later.

### 4. Version bump check via `git diff main -- go.mod`

**Decision:** Check a `VERSION` file at the repo root against `git show main:VERSION`. Pass if the major or minor segment has increased. Fail if only the patch segment changed or if no version was bumped at all.

**Rationale:** Go module paths encode major versions; minor/patch are tracked separately. A `VERSION` file is the simplest convention for semver tracking in a single-module repo.

**Alternative considered:** Git tags — requires the tag to already exist before the merge gate runs, which is backwards.

### 5. Auto-remediation: generate test stubs, then re-run

**Decision:** When coverage < 90%, the script identifies uncovered packages and generates empty `_test.go` stubs with `TODO` markers. It then exits with a non-zero code and prints instructions for Claude to fill in the tests.

**Rationale:** Fully auto-generating meaningful tests requires understanding business logic — that's Claude's job. The script's role is to scaffold the file and signal Claude to act.

## Risks / Trade-offs

- **`Stop` hook fires on every Claude stop, not just merges** → Mitigation: script checks `git rev-parse --abbrev-ref HEAD` and exits 0 (no-op) if not on a branch targeting main, or gate with a `MERGE_CHECK=1` env var set by the user when ready.
- **Coverage aggregation across packages can mask a 0%-covered package** → Mitigation: script also checks per-package coverage and fails if any individual package is below 80%.
- **Grep-based godoc check has false positives** (e.g., build-tag lines before exports) → Mitigation: check for `//` comment within 3 lines above the export, not just 1.
- **`go.mod` doesn't track semver minor for v0/v1** → Mitigation: rely on `VERSION` file as the canonical semver source; document this convention.
