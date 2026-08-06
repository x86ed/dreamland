## 1. Claude Code agent-identity resolution

- [x] 1.1 In `cmd/coauthor.go`, extend `agentNameFromHookPayload()` to also check `payload["tool_input"].(map[string]any)["subagent_type"]`, after the existing top-level `agent_type` check, returning that value if present and non-empty
- [x] 1.2 Add an allow-list of the ten registered dreamland agent names and change `resolveAgentName()`/`agentNameFromHookPayload()` call sites in `runCoauthor` so any resolved candidate not in the allow-list falls back to `"janus"` instead of being used verbatim
- [x] 1.3 Update unit tests in a new/existing `cmd/coauthor_test.go` case: a payload shaped like `{"tool_input": {"subagent_type": "phobetor"}}` resolves to `"phobetor"`; a payload with an unrecognized value falls back to `"janus"`
- [x] 1.4 Update `openspec/specs/dev-workflow-hooks/spec.md` language references to `CLAUDE_AGENT_ID` are already superseded by this change's delta — confirm no other doc (README, agent templates) still asserts `CLAUDE_AGENT_ID` is set by Claude Code

## 2. AI-Agent telemetry trailer

- [x] 2.1 Add an `Agent` field to `telemetry.SnapshotResult` (`internal/telemetry/snapshot.go`)
- [x] 2.2 Populate `Agent` in `internal/telemetry/tools/claude.go`'s `ClaudeCollector.Collect` using the same resolution path as task 1.1/1.2 (extract a shared helper if reasonable rather than duplicating the allow-list logic)
- [x] 2.3 Add `AI-Agent` to the trailer-rendering code path (wherever `AI-Tool`/`AI-Model` etc. are currently rendered from `SnapshotResult`), honoring the existing zero/empty-value omission rule
- [x] 2.4 Add/extend tests asserting `AI-Agent: phobetor` appears in a rendered trailer set when `Agent` is populated, and is omitted when empty

## 3. Deterministic change-scoped version bump

- [x] 3.1 Add a `PostToolUse` hook entry (matcher: `Bash`) to `internal/scaffold/templates/hooks/bindings/claude-code/settings-patch.json` that runs a small detection step for the `openspec new change <slug>` / `openspec change create <slug>` command shape
- [x] 3.2 Decide and implement the detection mechanism: either (a) a new small `dreamland version-bump --detect-change-from-bash-command` mode that reads the hook's own `tool_input.command` from stdin and extracts the slug itself, or (b) a wrapper shell one-liner in the hook `command` field — prefer (a) to keep all logic in Go per the "no shell scripts" principle in `dev-workflow-hooks`
- [x] 3.3 Wire the extracted slug into the existing `runChangeBump`/`--change` code path in `cmd/version_bump.go` (no changes needed there if task 3.2 lands as a thin new flag/mode that ultimately calls the same `runChangeBump`)
- [x] 3.4 Update `internal/scaffold/templates/agents/claude-code/phantasos.md` to note the hook now performs this automatically, keeping the manual instruction only as a documented fallback note (per the modified `dev-workflow-hooks` spec's platform-without-post-tool-hook scenario)
- [x] 3.5 Add a test exercising the new detection mode against a representative `openspec new change "foo-bar"` command string, asserting `dreamland version-bump --change foo-bar` semantics fire exactly once

## 4. Bare per-agent slash commands

- [x] 4.1 In `internal/scaffold/scaffold.go` (Claude Code install path), after writing each `.claude/commands/drmlnd/<agent>.md` file, also write a bare `.claude/commands/<agent>.md` from the same template content; additionally write `.claude/commands/dreamland.md` AND `.claude/commands/janus.md` from `route.md`'s content (janus has no per-agent template of its own — forcing the destination to janus is exactly what the generic route already does)
- [x] 4.2 Add a dreamland-authored marker (consistent with whatever marker convention, if any, other dreamland-managed files in this repo already use — introduce one if none exists) to both the namespaced and bare files so re-init can distinguish dreamland-owned files from user files
- [x] 4.3 Before writing a bare command file, check for an existing file at that path without the marker; if found, skip the write and print a stderr warning naming the path and the agent that would have been installed
- [x] 4.4 Add scaffold tests: fresh install produces both `.claude/commands/drmlnd/nyx.md` and `.claude/commands/nyx.md`; a pre-existing unmarked `.claude/commands/janus.md` is left untouched with a warning printed; a marked one is overwritten on re-init
- [x] 4.5 Repeat 4.1-4.4's intent for any other platform where bare aliases are in scope per the modified `router-slash-commands` spec (confirm with the spec's scenarios which platforms are covered — Claude Code is the only platform named in this change's scenarios; extend to others only if their existing per-agent command scaffolding makes it a trivial mirror of the Claude Code path)

## 5. Session-agent-identity default and telemetry safety

- [x] 5.1 Confirm (via test) that `dreamland coauthor` run with no hook payload at all (plain `SessionStart`, no `Task` call yet) resolves identity to `"janus"`, not the coding-tool name — adjust the fallback order in `runCoauthor`/`resolveAgentName` if the coding-tool-name fallback currently takes precedence over the "no agent yet ⇒ janus" default
- [x] 5.2 Add a regression test combining tasks 1.2 and 2.2: an unrecognized identity never reaches `git config user.name`, `git config user.email`, or the `AI-Agent` trailer — all three land on `"janus"`/its email/`"janus"` respectively

## 6. Self-host dreamland on its own repository

- [x] 6.1 Run the updated scaffolder (`dreamland init`, or the targeted agent/hook installation step) against this repository once tasks 1-5 are merged, so `.claude/agents/*.md` (real Task-tool sub-agent definitions) are installed for the first time
- [x] 6.2 Merge the full Claude Code hook set from `settings-patch.json` into this repo's `.claude/settings.json`, preserving the existing `Stop` → `bash scripts/pre-merge-check.sh` entry alongside the new dreamland-managed `Stop`/`SessionStart`/`PreToolUse`/`SubagentStop`/`PostToolUse` entries
- [x] 6.3 Verify the bare and `drmlnd`-prefixed command files (tasks 4.1-4.4) are both present under `.claude/commands/` in this repo after the scaffold run
- [ ] 6.4 Manually exercise one full delegation (e.g. `/drmlnd:nyx` or bare `/nyx`) in this repo and confirm via `git log` / `git interpret-trailers` that the resulting commit's author and `AI-Agent` trailer both read `"nyx"`, not a generic or unrelated identity
- [ ] 6.5 Confirm `openspec new change <slug>` in this repo triggers the automatic minor bump (task 3) without any manual `dreamland version-bump` invocation

## 7. Verification

- [x] 7.1 Run the full Go test suite (`go test ./...`) and confirm no regressions in existing `cmd/coauthor_test.go`, `cmd/version_bump_test.go`, `internal/scaffold/scaffold_test.go`, or `internal/telemetry` tests
- [x] 7.2 Run `openspec validate --change claude-code-self-hosting` (or equivalent) to confirm the delta specs apply cleanly against the base specs
- [ ] 7.3 Re-check `gitStatus`/`git log` after task 6's manual exercise to confirm no unexpected "mystery" identity appears in the new commits
