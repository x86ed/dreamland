## 1. guard-artifact: tasks.md checkbox exception

- [x] 1.1 In `cmd/guard_artifact.go`, extend `hookToolInputPayload`/`checkArtifactOwnership` to also read `tool_input.old_string` and `tool_input.new_string` (present on `Edit` tool calls) alongside `file_path`.
- [x] 1.2 Add a helper `isCheckboxOnlyEdit(oldString, newString string) bool` that reports whether the only difference between `oldString` and `newString` is a `- [ ]` → `- [x]` (or `- [x]` → `- [ ]`) flip on a single line, with the rest of the line and surrounding whitespace identical after normalization. Return `false` (fail closed) for anything ambiguous: multi-line diffs beyond the checkbox marker, or a `Write` call (no `old_string`/`new_string` at all).
- [x] 1.3 In `checkArtifactOwnership`, when the matched pattern is the `tasks.md` entry (`^openspec/changes/[^/]+/tasks\.md$` — split this out as its own table entry distinct from `proposal|design`, which keep the existing exact-owner check) and `isCheckboxOnlyEdit` reports `true`, skip the ownership mismatch check regardless of `agent_type`.
- [x] 1.4 Unit tests: `morpheus` flipping one `- [ ]` to `- [x]` in `tasks.md` → allowed; `morpheus` changing task prose in the same `Edit` call → blocked (exit 2); `morpheus` calling `Write` on `tasks.md` → blocked; `phantasos` doing either → allowed (existing owner, unaffected); `proposal.md`/`design.md`/spec files unaffected by the new checkbox logic (still exact-owner-only).
- [x] 1.5 Run `go test ./cmd/...` and confirm all new and existing `guard_artifact` tests pass.

## 2. Remove duplicate workspace-level SubagentStop identity/handoff hooks (Claude Code)

- [x] 2.1 In `internal/scaffold/templates/hooks/bindings/claude-code/settings-patch.json`, remove `dreamland coauthor --hook` and `dreamland commit --reason handoff` from the `SubagentStop` array, keeping `dreamland telemetry write --tool claude-code` and `dreamland version-bump --patch` (and the `--if-agent janus` minor-bump entry, if present there) in place. Leave the `PreToolUse(Task|Agent)` `dreamland coauthor --hook` entry untouched.
- [x] 2.2 Add/update a scaffold test asserting the generated `.claude/settings.json`'s `SubagentStop` command list no longer contains `coauthor` or `commit --reason handoff`, and still contains `telemetry write --tool claude-code` and `version-bump --patch`.
- [x] 2.3 Update the existing scaffold test (from `claude-code-parity`, task 3.3) that diffs each agent's frontmatter `hooks.Stop` command list against the workspace `SubagentStop` command list — since the two are no longer expected to match one-for-one, replace it with a test asserting the workspace list is a strict subset (telemetry + version-bump only) of the per-agent list's non-identity commands.
- [x] 2.4 Run `go test ./internal/scaffold/...` and confirm all new and existing tests pass.

## 3. commit --reason handoff: non-blocking git-mechanics failures

- [x] 3.1 In `cmd/commit.go`'s `runCommit`, for the `--reason handoff` path only, change the `git add -A` and `git commit` failure returns from `Blocking(fmt.Errorf(...))` to a plain `fmt.Errorf(...)` (non-blocking, exit 1). Leave the `--reason turn-complete` test-failure gate (`testResult.Status == "fail"` block) and the missing-test-result error using `Blocking(...)` unchanged.
- [x] 3.2 Unit tests: `--reason handoff` with a simulated `git commit` failure (not "nothing to commit") returns a non-`Blocking` error; `--reason turn-complete` with the same simulated failure still returns a `Blocking` error (regression check that the two reasons diverge correctly); the existing "nothing to commit" no-op path is unaffected for either reason.
- [x] 3.3 Run `go test ./cmd/...` and confirm all new and existing `commit` tests pass.

## 4. Spec corrections

- [x] 4.1 Confirm `openspec/changes/fix-claude-code-handoff-integrity/specs/dev-workflow-hooks/spec.md`'s three `MODIFIED Requirements` blocks accurately reflect the implementation from tasks 2 and 3 above once those land (adjust wording only if implementation details diverge from the delta as written).
- [x] 4.2 Add a one-line note to `openspec/changes/claude-code-parity/tasks.md` under task 7.1 pointing to this change, since it corrects that task's `guard-artifact` ownership table before `claude-code-parity` is archived.

## 5. Self-hosting dogfood

- [x] 5.1 Regenerate this repo's own `.claude/settings.json` (or hand-edit to match) removing the duplicated `SubagentStop` `coauthor`/`commit --reason handoff` entries, consistent with task 2.1's template change.
- [x] 5.2 Manually verify, by inspecting `.claude/agents/morpheus.md`'s frontmatter and this repo's `.claude/settings.json` side by side, that identity/commit commands now appear in exactly one place (the per-agent file) and telemetry/version-bump appear in both (workspace-level, unscoped, as intended for built-in-agent coverage).
- [x] 5.3 Run `go build ./...` and `go test ./...` for the whole repo and confirm everything passes after all edits.
