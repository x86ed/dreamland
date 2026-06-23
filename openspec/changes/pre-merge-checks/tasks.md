## 1. Project Scaffolding

- [x] 1.1 Create initial Git semver tag `v0.1.0` on the current commit (`git tag v0.1.0`) so the repo has a baseline tag
- [x] 1.2 Create `scripts/` directory and `scripts/pre-merge-check.sh` with a shebang and placeholder

## 2. Test Runner Check

- [x] 2.1 Implement `run_tests` function in the script that executes `go test ./...` and captures exit code
- [x] 2.2 Abort with failure output when tests fail

## 3. Coverage Enforcement

- [x] 3.1 Implement `check_coverage` function: run `go test -coverprofile=coverage.out ./...` and parse total from `go tool cover -func=coverage.out`
- [x] 3.2 Abort with package list when aggregate coverage < 90%
- [x] 3.3 Emit warning message when aggregate coverage is between 90% and 95%
- [x] 3.4 Check per-package coverage and abort if any package is below 80%

## 4. Auto-Remediation of Missing Tests

- [x] 4.1 Implement `find_untested_packages` function: identify packages with no `*_test.go` files
- [x] 4.2 For each untested package, generate a `<pkg>_test.go` stub with `package <name>_test` header and `TODO` comment per exported symbol
- [x] 4.3 Exit non-zero with instructions for Claude to implement the generated stubs

## 5. Godoc Validation

- [x] 5.1 Implement `check_godoc` function: use `grep` to find exported `func`/`type`/`var` declarations missing a `//` comment within the preceding 3 lines
- [x] 5.2 Print file path and line number for each undocumented symbol and abort

## 6. Version Bump Validation

- [x] 6.1 Implement `check_version_bump` function: resolve current tag via `git describe --tags --abbrev=0` and main's latest tag via `git describe --tags --abbrev=0 main`; abort with instructions if no tag exists on the current branch
- [x] 6.2 Parse `v{major}.{minor}.{patch}` from both tags; pass if major or minor increased, abort if only patch changed or tags are identical
- [x] 6.3 On a major version bump, verify the `module` line in `go.mod` has been updated to the new major path (e.g., `dreamland/v2`); abort if it hasn't

## 7. Branch Guard

- [x] 7.1 Add a branch check at the top of the script: if the current branch has no upstream tracking `main`, exit 0 (no-op) unless `MERGE_CHECK=1` is set

## 8. Hook Wiring

- [x] 8.1 Add execute permission to `scripts/pre-merge-check.sh` (`chmod +x`)
- [x] 8.2 Add a `Stop` hook entry to `.claude/settings.json` that invokes `bash scripts/pre-merge-check.sh`
- [x] 8.3 Add `Bash(bash scripts/pre-merge-check.sh*)` to the `permissions.allow` list in `.claude/settings.json`

## 9. Existing Code Compliance

- [ ] 9.1 Run the gate script against the current codebase; add missing `_test.go` files for any packages below threshold
- [ ] 9.2 Add godoc comments to any exported symbols in `cmd/` that are currently undocumented
- [ ] 9.3 Verify baseline `v0.1.0` tag exists and gate passes end-to-end with `MERGE_CHECK=1`
