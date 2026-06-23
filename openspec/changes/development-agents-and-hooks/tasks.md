## 1. Config — extend Config struct and persistence

- [ ] 1.1 Add `RepoRoot string` field with `json:"repo_root,omitempty"` to `internal/config/Config`
- [ ] 1.2 Add a unit test confirming `config.Load` returns zero-value `RepoRoot` when the field is absent in existing JSON

## 2. Scaffold package — embedded hook scripts (shared across all platforms)

- [ ] 2.1 Create `internal/scaffold/` directory and `embed.go` with `//go:embed all:templates` and an exported `TemplateFS fs.FS`
- [ ] 2.2 Create `internal/scaffold/templates/hooks/scripts/` directory
- [ ] 2.3 Write `version-bump.sh` — runs `git diff HEAD`; if output is non-empty, bumps the patch version in the project version file and commits; exits 0 with no action if no changes
- [ ] 2.4 Write `transition-log.sh` — appends an ISO-timestamp + session-ID line to `.dreamland/transition.log` on every invocation
- [ ] 2.5 Write `run-tests.sh` — checks `git diff --name-only HEAD` for source files matching the project language's extensions; if any found, reads `test_command` from `.dreamland.json` and runs it forwarding the exit code; exits 0 with no action if no relevant files changed
- [ ] 2.6 Write `coauthor.sh` — reads `.dreamland/session-start` timestamp; if `git log --since=<timestamp>` returns commits, amends the most recent one with a `Co-authored-by:` trailer; otherwise runs `git config user.name` and `git config user.email` to set the configured co-author identity locally
- [ ] 2.7 Write a `record-session-start.sh` helper — writes the current ISO timestamp to `.dreamland/session-start`; this is invoked by the session-start binding so `coauthor.sh` can bound its `git log` query

## 3. Scaffold package — agent templates per platform

- [ ] 3.1 Create `internal/scaffold/templates/agents/claude-code/` and write five `.md` files (`orchestrator.md`, `spec-writer.md`, `implementer.md`, `tester.md`, `pr-closer.md`) with Claude Code YAML frontmatter (`name`, `description`, `tools`) and role-specific prompts
- [ ] 3.2 Create `internal/scaffold/templates/agents/codex/` and write five `.toml` files with `name`, `description`, `developer_instructions` fields and role-specific prompts
- [ ] 3.3 Create `internal/scaffold/templates/agents/cursor/` and write five `.mdc` files with YAML frontmatter (`description`, `alwaysApply: false`) and role-specific instructions
- [ ] 3.4 Create `internal/scaffold/templates/agents/kiro/` and write five `.md` steering documents each with `---\ninclusion: always\n---` frontmatter followed by role-specific instructions
- [ ] 3.5 Create `internal/scaffold/templates/agents/antigravity/` and write five `.md` agent files plus a `plugin.json` manifest (`name`, `version`, `description`, `skills`, `rules` arrays)
- [ ] 3.6 Create `internal/scaffold/templates/agents/github-copilot/` and write five `.instructions.md` files with role-specific instructions

## 4. Scaffold package — hook binding files per platform

- [ ] 4.1 Write `internal/scaffold/templates/hooks/bindings/claude-code/settings-patch.json` — registers all four scripts plus `record-session-start.sh` under `Stop`; session-start helper also registered under `SessionStart`
- [ ] 4.2 Write `internal/scaffold/templates/hooks/bindings/codex/hooks.json` — same structure as Claude Code; `Stop` for the four hooks, `SessionStart` for `record-session-start.sh`
- [ ] 4.3 Write `internal/scaffold/templates/hooks/bindings/cursor/hooks.json` — `version: 1` envelope; `stop` key for the four hooks, `sessionStart` key for `record-session-start.sh`
- [ ] 4.4 Write `internal/scaffold/templates/hooks/bindings/kiro/agent-patch.json` — Kiro CLI JSON format; `stop` key for the four hooks, `agentSpawn` key for `record-session-start.sh`
- [ ] 4.5 Write `internal/scaffold/templates/hooks/bindings/antigravity/hooks.json` — mirrors Claude Code schema (`Stop`, `SessionStart`); mark as preview
- [ ] 4.6 Write `internal/scaffold/templates/hooks/bindings/github-copilot/hooks-manifest.json` — stub with `_note` field explaining schema is not yet publicly documented

## 5. Scaffold package — installer logic

- [ ] 5.1 Define `Config` struct (`RepoRoot`, `CodingTool string`, `Force bool`) and `Result` type (`Path`, `Action string`) in `internal/scaffold/scaffold.go`
- [ ] 5.2 Implement `Install(cfg Config) ([]Result, error)` as the single public entry point; calls `installAgents`, `installHookScripts`, `bindHooks` in sequence
- [ ] 5.3 Implement `installAgents` — reads platform-specific agent templates from embedded FS, writes to platform output path, respects `Force`, returns per-file `Result`
- [ ] 5.4 Implement platform output path resolver — maps `CodingTool` to target directory and file extension; for Antigravity returns `~/.gemini/antigravity-cli/plugins/dreamland/agents/`
- [ ] 5.5 Implement Antigravity extras — write `plugin.json` manifest alongside agent files; append post-install instruction to results telling user to run `antigravity plugin install ~/.gemini/antigravity-cli/plugins/dreamland`
- [ ] 5.6 Implement `installHookScripts` — writes `version-bump.sh`, `transition-log.sh`, `run-tests.sh`, `coauthor.sh`, and `record-session-start.sh` to `.dreamland/hooks/`, chmod 0755, respects `Force`
- [ ] 5.7 Implement `bindHooks` — reads the platform-specific binding template from embedded FS; dispatches to a platform-specific merge/write function
- [ ] 5.8 Implement JSON merge helper — reads existing JSON file (or starts with `{}`), merges the binding patch into the `hooks` key without overwriting unrelated keys, writes atomically
- [ ] 5.9 Implement Claude Code binding — use JSON merge helper on `.claude/settings.json`
- [ ] 5.10 Implement Codex CLI binding — use JSON merge helper on `.codex/hooks.json`
- [ ] 5.11 Implement Cursor binding — use JSON merge helper on `.cursor/hooks.json`; ensure `version: 1` field is present
- [ ] 5.12 Implement Kiro binding — use JSON merge helper on `.kiro/agent.json`
- [ ] 5.13 Implement Antigravity binding — write `hooks.json` directly into `~/.gemini/antigravity-cli/plugins/dreamland/` (no merge needed; new file)
- [ ] 5.14 Implement GitHub Copilot binding — write `.github/copilot-hooks/hooks-manifest.json` stub
- [ ] 5.15 Write unit tests for `installAgents` for each of the six platforms using a temp directory
- [ ] 5.16 Write unit tests for JSON merge helper: absent file, existing file with unrelated keys, existing file with existing hooks (no duplicates)
- [ ] 5.17 Write unit test confirming hook script content is byte-for-byte identical regardless of platform

## 6. Init wizard — repo root step and expanded tool list

- [ ] 6.1 Add `repoRoot string` field to `wizardResult` struct in `cmd/init.go`
- [ ] 6.2 Add step 1 to `defaultWizardRunner`: `huh.NewInput` pre-filled with `config.FindRepoRoot(cwd)` result, with path-exists validation
- [ ] 6.3 Expand the coding tool `huh.NewSelect` options to full list: Claude Code, Codex CLI, Cursor, GitHub Copilot, Antigravity, Kiro
- [ ] 6.4 Renumber displayed step titles from "Step N/5" to "Step N/6" for steps 2–6
- [ ] 6.5 Pass `res.repoRoot` into the `Config` struct written by `runInit`
- [ ] 6.6 Update `cmd/init_test.go` wizard stub to supply `repoRoot` and update step-count assertions

## 7. Init command — scaffolding integration and --force flag

- [ ] 7.1 Add `--force` boolean flag to `initCmd` (default false)
- [ ] 7.2 After `config.Save`, resolve effective repo root: use `res.repoRoot` if non-empty, else `config.FindRepoRoot(cwd)`
- [ ] 7.3 Call `scaffold.Install(scaffold.Config{RepoRoot: resolvedRoot, CodingTool: res.tool, Force: forceFlag})`
- [ ] 7.4 Print per-file scaffold results to `cmd.OutOrStdout()` before the final success message
- [ ] 7.5 On scaffold error, write to `cmd.ErrOrStderr()` and return non-zero; do not roll back the already-written config
- [ ] 7.6 Update `cmd/init_test.go` to assert scaffold output appears in stdout on the happy path
- [ ] 7.7 Update `cmd/init_test.go` to assert `--force` passes through to the scaffold installer

## 8. End-to-end validation

- [ ] 8.1 Run `go build ./...` and confirm zero errors
- [ ] 8.2 Run `go test ./...` and confirm all tests pass
- [ ] 8.3 Run `./dreamland init` (Claude Code) in a temp git repo; verify `.claude/agents/`, `.dreamland/hooks/`, and `.claude/settings.json` contain correct hook entries under `Stop`
- [ ] 8.4 Run `./dreamland init` (Codex CLI); verify `.codex/agents/` and `.codex/hooks.json` with `Stop` entries
- [ ] 8.5 Run `./dreamland init` (Cursor); verify `.cursor/rules/` and `.cursor/hooks.json` with `stop` entries and `version: 1`
- [ ] 8.6 Run `./dreamland init` (Kiro); verify `.kiro/steering/` (steering docs with `inclusion: always`) and `.kiro/agent.json` with `stop` entries
- [ ] 8.7 Run `./dreamland init` (Antigravity); verify plugin bundle at `~/.gemini/antigravity-cli/plugins/dreamland/` with `hooks.json` under `Stop`, and post-install instruction in stdout
- [ ] 8.8 Run `./dreamland init` (GitHub Copilot); verify `.github/copilot-agents/` and `.github/copilot-hooks/hooks-manifest.json` stub
- [ ] 8.9 Run `./dreamland init` (Claude Code) a second time without `--force`; verify existing files are skipped and hook config is not duplicated
- [ ] 8.10 Run `./dreamland init --force`; verify all agent and hook script files are overwritten
- [ ] 8.11 Manually trigger `.dreamland/hooks/version-bump.sh` with and without staged changes; verify conditional behavior
- [ ] 8.12 Manually trigger `.dreamland/hooks/run-tests.sh` with and without source file changes; verify conditional behavior
