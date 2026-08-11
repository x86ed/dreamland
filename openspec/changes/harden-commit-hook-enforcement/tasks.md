## 1. Blocking-vs-advisory exit code mechanism

- [ ] 1.1 Create `cmd/hookexit.go` (package `cmd`): a `blockingError` type wrapping an `error`, a `Blocking(err error) error` constructor (returns `nil` for a `nil` input), an `(*blockingError).Error()`/`Unwrap()` pair, and `IsBlocking(err error) bool` implemented via `errors.As`
- [ ] 1.2 Update `cmd/root.go`'s `Execute()`: on a non-nil error from `rootCmd.Execute()`, call `os.Exit(2)` if `IsBlocking(err)`, else `os.Exit(1)` exactly as today
- [ ] 1.3 Add unit tests (`cmd/hookexit_test.go`): `Blocking(nil)` returns `nil`; `IsBlocking` returns `true` for a `Blocking`-wrapped error and `false` for a plain `errors.New(...)`; `errors.Unwrap` on a `Blocking`-wrapped error returns the original error

## 2. `no git repository` becomes a named, shared skip condition

- [ ] 2.1 In `internal/config/config.go`, add `var ErrNoGitRepo = errors.New("no git repository found in any parent directory")` and change `FindRepoRoot` to `return "", ErrNoGitRepo` in place of its current ad hoc `errors.New(...)` call
- [ ] 2.2 Add/extend a test in `internal/config/config_test.go` asserting `errors.Is(err, config.ErrNoGitRepo)` is true when `FindRepoRoot` is called outside any git repository

## 3. Hook-payload reads: replace the timeout race with deterministic interactive-stdin detection

- [ ] 3.1 In `cmd/coauthor.go`, delete the `hookPayloadReadTimeout` constant and the goroutine/`select`/`time.After` machinery in `agentNameFromHookPayload`. Refactor it to `func agentNameFromHookPayloadFrom(r io.Reader) string`: if `r` is an `*os.File` and `Stat().Mode()&os.ModeCharDevice != 0` (a real terminal, not a pipe), return `""` immediately with no read attempted; otherwise do a plain synchronous `io.ReadAll(io.LimitReader(r, 1<<16))` → `json.Unmarshal` → `agentidentity.FromPayload`, exactly as today's non-timeout-branch logic. Keep a zero-arg `agentNameFromHookPayload() string { return agentNameFromHookPayloadFrom(os.Stdin) }` wrapper so existing call sites are unchanged
- [ ] 3.2 In `cmd/version_bump.go`, delete the `bashHookStdinReadTimeout` constant and the equivalent goroutine/`select`/`time.After` machinery in `changeSlugFromBashHookStdin`, applying the same `*os.File` + `ModeCharDevice` check before its existing synchronous `io.ReadAll` (the function already takes `r io.Reader`, so its signature is unchanged)
- [ ] 3.3 Rewrite `cmd/coauthor_test.go`'s `TestAgentNameFromHookPayload_NeverBlocksWhenNoDataArrives`: its old `os.Pipe()`-based simulation modeled the timeout's hang scenario, which no longer exists as a code path to test. Replace it with a test asserting `agentNameFromHookPayloadFrom` returns `""` immediately, with no read attempted, when given an `*os.File` whose `Stat()` reports `ModeCharDevice` set (e.g. an opened character-device file appropriate to the test's platform, or a small seam/interface if a real one isn't reliably available in CI) — this now tests the actual detection mechanism instead of a timing race
- [ ] 3.4 Add/extend tests confirming the piped case (a non-`*os.File` reader, e.g. `strings.NewReader`, or an `*os.File` without `ModeCharDevice` set) still parses a legitimately-arriving payload correctly, for both `agentNameFromHookPayloadFrom` and `changeSlugFromBashHookStdin` — no timeout-related behavior remains to test
- [ ] 3.5 Confirm `cmd/version_bump.go`'s `--if-agent` path (which calls `agentNameFromHookPayload`) picks up the new detection logic automatically — no code change expected there, but extend its existing test coverage to match

## 4. Shared, correct identity resolution between `coauthor` and `commit`

- [ ] 4.1 In `cmd/coauthor.go`, extract the existing inline block in `runCoauthor` (lines resolving `agentName` via `resolveAgentName`, the hook-payload override, and the Claude-Code-to-`janus` correction) into `func resolveEnforcedAgentName(cfg *config.Config) string`; update `runCoauthor` to call it
- [ ] 4.2 Add `func currentGitIdentityName(cfg *config.Config) string` in `cmd/coauthor.go`: runs `gitExec("config", "--local", "--get", "user.name")`, returns the trimmed result if non-empty, else falls back to `resolveEnforcedAgentName(cfg)`
- [ ] 4.3 In `cmd/commit.go`'s `runCommit`, replace `agentName := resolveAgentName(cfg.CodingTool)` with `agentName := currentGitIdentityName(cfg)`
- [ ] 4.4 Add a test in `cmd/commit_test.go` (or extend an existing one): with `git config --local user.name` pre-set to `"phobetor"` and no hook payload on stdin (simulating a `Stop`-triggered invocation, which carries no sub-agent identity), `dreamland commit --reason turn-complete`'s resulting commit subject reads `chore: turn-complete checkpoint (phobetor)`, not a value derived from `resolveAgentName`/coding-tool fallback
- [ ] 4.5 Add a regression test confirming `runCommit` still resolves a sensible name (via `resolveEnforcedAgentName`'s fallback path) when `git config --local user.name` is unset entirely

## 5. `coauthor`/`commit` exit codes: skip vs blocking

- [ ] 5.1 In `cmd/coauthor.go`'s `runCoauthor`, after `config.Load(cwd)`: if `errors.Is(err, config.ErrNoGitRepo)`, `return nil` (skip); else if `err != nil`, `return Blocking(err)`. Note `cfg == nil` (no `.dreamland.json`, config-load itself succeeded) keeps its existing behavior of proceeding with an empty `Config`, unchanged
- [ ] 5.2 Wrap both `git config --local user.name`/`user.email` write-failure return paths, and the `installPrepareCommitMsgHook` error return path, in `Blocking(...)`
- [ ] 5.3 In `cmd/commit.go`'s `runCommit`, apply the same `config.Load`/`ErrNoGitRepo` handling as 5.1 (skip on no-repo, `Blocking` on any other load error)
- [ ] 5.4 Wrap the `git add -A` and `git commit` error-return paths in `runCommit` with `Blocking(...)` (the existing clean-tree no-op path, `return nil`, is unchanged)
- [ ] 5.5 Add/extend tests in `cmd/coauthor_test.go` and `cmd/commit_test.go`: a stubbed `gitExec` failure on the config-write / add / commit path causes `runCoauthor`/`runCommit` to return an error satisfying `IsBlocking(err)`; running either command outside a git repo (via `config.ErrNoGitRepo`) returns `nil`

## 6. `dreamland test`: real failures block, output is visible, outcome is recorded

- [ ] 6.1 Create `cmd/testresult.go` with a shared type (e.g. `type testResult struct { Status string; HeadSHA string; WrittenAt string }`), `func writeLastTestResult(repoRoot, status string) error` (computes `HeadSHA` via `git rev-parse HEAD`, writes `.dreamland/last-test-result.json`, creating `.dreamland/` if absent), and `func readLastTestResult(repoRoot string) (*testResult, error)` (returns `nil, nil` if the file doesn't exist)
- [ ] 6.2 In `cmd/test.go`'s `runTest`, change `c.Stdout = nil; c.Stderr = nil` to `c.Stdout = os.Stdout; c.Stderr = os.Stderr` so the underlying test command's real output is visible
- [ ] 6.3 In `runTest`, after `c.Run()` returns: on success, call `writeLastTestResult(repoRoot, "pass")` and `return nil`; on failure (nonzero exit or exec failure), call `writeLastTestResult(repoRoot, "fail")` and `return Blocking(err)` instead of the current bare `return c.Run()`/passthrough. The existing skip branches (`cfg == nil`, unknown language, git unavailable, no matching files) are unchanged and do not write a result record
- [ ] 6.4 Update/replace the existing "Test failure propagates exit code" test in `cmd/test_test.go` to assert the new behavior: `runTest` returns an error satisfying `IsBlocking(err)` on a real test-command failure, and `.dreamland/last-test-result.json` records `"status": "fail"` with the current `HEAD` sha
- [ ] 6.5 Add a test asserting a successful test run writes `"status": "pass"` to `.dreamland/last-test-result.json`
- [ ] 6.6 Add a test asserting a skipped run (no matching source files changed) does not create or modify `.dreamland/last-test-result.json`

## 7. `dreamland commit` gates on the last test result

- [ ] 7.1 In `cmd/commit.go`'s `runCommit`, before the `git status --porcelain` clean-tree check (or immediately after, but before any `git add`/`git commit` call), call `readLastTestResult(repoRoot)`; if it returns a non-nil result with `Status == "fail"` and `HeadSHA` equal to the current `git rev-parse HEAD`, return `Blocking(...)` with a message naming the failing HEAD sha and pointing at `.dreamland/last-test-result.json`, without running `git add` or `git commit`
- [ ] 7.2 Confirm (via test) that a missing result file, or one whose `HeadSHA` does not match current `HEAD`, does not gate the commit — `runCommit` proceeds to its normal staging/commit behavior
- [ ] 7.3 Add a test: with a `.dreamland/last-test-result.json` recording `"status": "fail"` at the current `HEAD` and pending changes present, `dreamland commit --reason turn-complete` creates no commit, leaves the working tree's changes staged/unstaged as before, and returns a blocking error
- [ ] 7.4 Add a test: with the same failing record but pending changes staged, running `dreamland commit --reason turn-complete` again after `.dreamland/last-test-result.json` is updated to `"status": "pass"` (still at the same `HEAD`, simulating a fix-and-rerun) succeeds and creates the commit

## 8. Git hook symmetry: `commit-msg` fails closed on a missing `dreamland` binary

- [ ] 8.1 Rewrite `internal/scaffold/templates/hooks/commit-msg`: replace the `if command -v dreamland >/dev/null 2>&1; then ... fi; exit 0` guard with a fail-closed form — `if ! command -v dreamland >/dev/null 2>&1; then echo "dreamland: not found on PATH — commit blocked (bypass with 'git commit --no-verify')" >&2; exit 1; fi` followed by the existing trailer-append logic and a final `exit 0`
- [ ] 8.2 Update `internal/scaffold/hook_test.go` (and any other test asserting the old `commit-msg` template content) to match the new script body
- [ ] 8.3 Add a test (shell-invocation-based, consistent with how `internal/scaffold/hook_test.go` already tests installed hook behavior, if it does — otherwise a new integration-style test) confirming the installed `commit-msg` hook exits nonzero when `PATH` is set to a directory without a `dreamland` binary, and exits 0 (with trailers appended) when it is present
- [ ] 8.4 Confirm `.git/hooks/prepare-commit-msg` (`installPrepareCommitMsgHook` in `cmd/coauthor.go`) needs no code change — verify its existing content still fails closed as documented, and add a code comment there cross-referencing this change's decision so a future edit doesn't "fix" it into matching `commit-msg`'s old fail-open behavior

## 9. Binary freshness: build SHA embedding and drift warning

- [ ] 9.1 Create `cmd/version.go`: package-level `var buildCommit = "unknown"`; a `versionCmd` (`Use: "version"`) whose `RunE` prints `dreamland build <buildCommit>` (or similar) to `cmd.OutOrStdout()`; register it in `init()` via `rootCmd.AddCommand(versionCmd)`
- [ ] 9.2 Add `func isDreamlandSourceRepo(repoRoot string) bool` (reads `go.mod` at `repoRoot`, checks it declares `module dreamland`) and `func checkBinaryFreshness(cwd string)` (finds the repo root via `config.FindRepoRoot`, returns early — no output — if that fails or `isDreamlandSourceRepo` is false or `buildCommit == "unknown"`; otherwise runs `git rev-parse HEAD` in that repo and, on a mismatch with `buildCommit`, writes one diagnostic line to stderr naming both SHAs) — both in `cmd/version.go`
- [ ] 9.3 Call `checkBinaryFreshness(cwd)` from `loadConfig` (`cmd/root.go`'s `PersistentPreRunE`) after resolving `cwd`, before returning — it never returns an error and never affects `currentConfig`
- [ ] 9.4 Add tests: `dreamland version` prints `"unknown"` when `buildCommit` is unset; `checkBinaryFreshness` prints nothing when `isDreamlandSourceRepo` is false (a consumer-repo `go.mod` fixture); prints nothing when `buildCommit` matches a fixture repo's `HEAD`; prints a diagnostic naming both SHAs on a mismatch
- [ ] 9.5 Update `README.md`'s build instructions (currently `go build -o dreamland .`) to also show the ldflags-embedded form, e.g. `go build -o dreamland -ldflags "-X dreamland/cmd.buildCommit=$(git rev-parse HEAD)" .`, and briefly explain why (so the freshness check in `dreamland version`/the drift warning has something to compare against)
- [ ] 9.6 Rebuild and reinstall this repo's own `~/.local/bin/dreamland` and repo-root `./dreamland` using the new ldflags-aware build command once tasks 1-8 are merged, so this repo's own dogfood installation reflects every fix in this change and isn't left freshly stale

## 10. Close out `claude-code-self-hosting` tasks 6.4 / 7.3 via this change's own real delegation

- [ ] 10.1 As this change's tasks are dispatched via real `Task`/`Agent`-tool delegation during implementation (not a synthetic/manual invocation crafted solely to close these tasks), capture the resulting commit(s)' `git log`/`git interpret-trailers` output showing a non-`janus` sub-agent's identity (author and, once task 4 lands, commit subject) resolved correctly end-to-end
- [ ] 10.2 Once confirmed, mark `openspec/changes/claude-code-self-hosting/tasks.md` items 6.4 and 7.3 as done, replacing their existing "BLOCKED"/"Partially checked" notes with a reference to the specific commit sha(s) from 10.1 as evidence
- [ ] 10.3 If, in practice, this change's own implementation completes without any real sub-agent delegation having occurred, leave 6.4/7.3 open and note in this change's own closure report that they remain blocked, rather than closing them without genuine evidence

## 11. Verification

- [ ] 11.1 Run `go build ./...` and `go vet ./...` — confirm no regressions
- [ ] 11.2 Run `go test ./...` — confirm all new and existing tests pass, including `cmd/coauthor_test.go`, `cmd/commit_test.go`, `cmd/test_test.go`, `cmd/version_bump_test.go`, `cmd/hookexit_test.go`, `cmd/version_test.go`, `internal/config/config_test.go`, and `internal/scaffold/hook_test.go`
- [ ] 11.3 Run `openspec validate --change harden-commit-hook-enforcement` (or equivalent) to confirm the delta specs apply cleanly against the base `dev-workflow-hooks` and `otel-commit-hook` specs
- [ ] 11.4 Manually confirm (once rebuilt per 9.6) that a deliberately-broken `test_command` in this repo's own `.dreamland.json` causes `dreamland test` to exit 2 and `dreamland commit --reason turn-complete` to refuse the commit — then restore the correct `test_command` before finishing
