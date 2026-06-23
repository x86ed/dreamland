## 1. Config — extend Config struct and persistence

- [ ] 1.1 Add `RepoRoot string` field (`json:"repo_root,omitempty"`) to `internal/config/Config`
- [ ] 1.2 Add `VersionBumpCommand string` field (`json:"version_bump_command,omitempty"`) — set by `init` based on language selection
- [ ] 1.3 Add `ModelID string` field (`json:"model_id,omitempty"`) — model name + optional settings string (e.g., `"claude-sonnet-4-6 temperature=1.0"`); set by `init`, overridable at runtime
- [ ] 1.4 Add `EmailSuffix string` field (`json:"email_suffix,omitempty"`) — defaults to `"@github.com"` when absent; used by `coauthor` for both agent and model emails
- [ ] 1.5 Implement `EmailClean(s string) string` helper in `internal/config`: lowercase → replace spaces/underscores with `-` → strip `[^a-z0-9.\-]` → trim leading/trailing `-` and `.`
- [ ] 1.6 Write unit tests confirming `config.Load` returns zero-value for all new fields when absent in existing JSON (backward compat)
- [ ] 1.7 Write unit tests for `EmailClean`: spaces, underscores, mixed case, leading/trailing hyphens, unicode stripped

## 2. dreamland version-bump command

- [ ] 2.1 Create `cmd/version_bump.go` with a `versionBumpCmd` cobra command registered as `version-bump`
- [ ] 2.2 Add flags: `--major` (bool), `--minor` (bool), `--patch` (bool), `--breaking` (bool), `--version` (string); validate that at most one of major/minor/patch/version is set
- [ ] 2.3 Resolve last tag: run `git describe --tags --abbrev=0 --match "v[0-9]*"`; if it exits non-zero (no tags), use `"v0.0.0"` as the baseline
- [ ] 2.4 Check for commits since last tag: run `git diff <last-tag>..HEAD`; if empty, exit 0 silently (covers both session-start and Stop modes)
- [ ] 2.5 Implement session-start mode (no `--patch`): read `.dreamland/branch-bumps` JSON; if current branch key exists, exit 0 silently; otherwise proceed to bump minor/major
- [ ] 2.6 No-upstream fallback: if `git rev-parse --abbrev-ref --symbolic-full-name @{u}` errors, set `noupstream=true`; treat branch as new
- [ ] 2.6 Apply bump level selection: `--breaking` → major; default → minor; explicit `--major`/`--minor` flags override; `--version` overrides entirely
- [ ] 2.7 Write branch-bumps JSON after successful minor/major bump: read existing `.dreamland/branch-bumps` (or start with `{}`), add/update entry `{"version": "<new>", "initialized_at": "<RFC3339>"}` for current branch, write atomically via temp-file rename; create `.dreamland/` if absent
- [ ] 2.8 If `noupstream=true`, run `git push --set-upstream origin <branch>` after bumping
- [ ] 2.9 Implement `--patch` mode (end-of-turn): skip branch-marker check; bump patch if changes exist since last tag
- [ ] 2.10 Implement Go language path (`version_bump_command` empty): parse latest semver tag, increment component, run `git tag -a <new-version> -m "<new-version>"`
- [ ] 2.11 Implement delegated path (`version_bump_command` set): construct `<version_bump_command> <level>` or `<version_bump_command> <explicit-version>` and exec; detect missing tool and print install hint before exiting non-zero
- [ ] 2.12 Write unit tests for no-tag baseline: `v0.0.0` → minor → `v0.1.0`, major → `v1.0.0`, patch → `v0.0.1`
- [ ] 2.13 Write unit tests for branch-bumps JSON: branch absent → bumped and written; branch present → exit 0; `--version` override updates entry
- [ ] 2.14 Write unit tests for flag override priority and no-upstream path

## 3. dreamland coauthor command

- [ ] 3.1 Create `cmd/coauthor.go` with a `coauthorCmd` cobra command registered as `coauthor`; add `--trailer` flag (string) for the `prepare-commit-msg` delegation path
- [ ] 3.2 Default mode (no `--trailer`): resolve AgentName by checking platform env vars in order: `CLAUDE_AGENT_ID`, `CODEX_AGENT_ID`, `CURSOR_AGENT_ID`, `KIRO_AGENT_ID`; fall back to `coding_tool` from `.dreamland.json`
- [ ] 3.3 Derive AgentEmail: `config.EmailClean(agentName) + cfg.EmailSuffix`; default suffix `"@github.com"` when `email_suffix` absent in config
- [ ] 3.4 Run `git config --local user.name "<agentName>"` and `git config --local user.email "<agentEmail>"`
- [ ] 3.5 Write `.git/hooks/prepare-commit-msg` as `#!/bin/sh\ndreamland coauthor --trailer "$1" "$2" "$3"\n`; mode 0755; skip write if file already contains `dreamland coauthor --trailer`
- [ ] 3.6 `--trailer` mode: read commit message file path from args[0]; extract model name as text before first space in `model_id`; derive model email via same `EmailClean` + `EmailSuffix`; if file does not contain `Co-authored-by: <model-name>`, append `Co-authored-by: <model-name> <model-email>` followed by newline
- [ ] 3.7 `--trailer` mode: all logic in Go — no shell utilities
- [ ] 3.8 Write unit tests: AgentName from env var, fallback to coding_tool, email cleaning applied
- [ ] 3.9 Write unit tests for `--trailer` mode: trailer appended when absent, not duplicated when present, model name correctly extracted from `model_id` with settings string

## 4. dreamland transition-log command

- [ ] 4.1 Create `cmd/transition_log.go` with a `transitionLogCmd` cobra command registered as `transition-log`
- [ ] 4.2 Resolve session ID: check `CLAUDE_SESSION_ID`, `CODEX_SESSION_ID`, `CURSOR_SESSION_ID`, `KIRO_SESSION_ID` env vars in order; if none found, generate a random 8-character hex string
- [ ] 4.3 Format the log line: `<RFC3339 timestamp> [<session-id>] turn complete`
- [ ] 4.4 Ensure `.dreamland/` directory exists (`os.MkdirAll`)
- [ ] 4.5 Append the line to `.dreamland/transition.log` (open with `O_APPEND|O_CREATE|O_WRONLY`); suppress write errors (exit 0 regardless)
- [ ] 4.6 Write unit tests: directory auto-created, line format, session ID fallback to random

## 5. dreamland test command

- [ ] 5.1 Create `cmd/test.go` with a `testCmd` cobra command registered as `test`
- [ ] 5.2 Read `test_command` and `language` from `.dreamland.json`
- [ ] 5.3 Define source extension map per language: Go → `{".go"}`, Node/TypeScript → `{".ts",".tsx",".js",".jsx",".mts",".cts"}`, Rust → `{".rs"}`, Python → `{".py"}`
- [ ] 5.4 Run `git status --porcelain` and parse output; filter for files whose extension matches the language set
- [ ] 5.5 If no matching files: exit 0 silently
- [ ] 5.6 If matching files found: exec `test_command` via shell and exit with its exit code
- [ ] 5.7 Write unit tests for extension matching (mock git output)
- [ ] 5.8 Write unit tests: skipped when no source files, exit code forwarded on test failure

## 6. Scaffold package — embedded agent templates per platform

- [ ] 6.1 Create `internal/scaffold/` directory and `embed.go` with `//go:embed all:templates` and an exported `TemplateFS fs.FS`
- [ ] 6.2 Create `internal/scaffold/templates/agents/claude-code/` and write five `.md` files (`orchestrator.md`, `spec-writer.md`, `implementer.md`, `tester.md`, `pr-closer.md`) with YAML frontmatter (`name`, `description`, `tools`) and role-specific prompts
- [ ] 6.3 Create `internal/scaffold/templates/agents/codex/` and write five `.toml` files with `name`, `description`, `developer_instructions` fields and role-specific prompts
- [ ] 6.4 Create `internal/scaffold/templates/agents/cursor/` and write five `.mdc` files with YAML frontmatter (`description`, `alwaysApply: false`) and role-specific instructions
- [ ] 6.5 Create `internal/scaffold/templates/agents/kiro/` and write five `.md` steering documents with `---\ninclusion: always\n---` frontmatter followed by role-specific instructions
- [ ] 6.6 Create `internal/scaffold/templates/agents/antigravity/` and write five `.md` agent files plus a `plugin.json` manifest (`name`, `version`, `description`, `skills`, `rules` arrays)
- [ ] 6.7 Create `internal/scaffold/templates/agents/github-copilot/` and write five `.instructions.md` files with role-specific instructions

## 7. Scaffold package — hook binding files per platform

Binding files call `dreamland <command>` directly. Session-start bindings include `version-bump` and `coauthor`; end-of-turn bindings include `transition-log` and `test`.

- [ ] 7.1 Write `internal/scaffold/templates/hooks/bindings/claude-code/settings-patch.json` — `hooks.SessionStart` array: `dreamland version-bump`, `dreamland coauthor`; `hooks.Stop` array: `dreamland version-bump --patch`, `dreamland transition-log`, `dreamland test`
- [ ] 7.2 Write `internal/scaffold/templates/hooks/bindings/codex/hooks.json` — identical command strings and event key names as Claude Code (`SessionStart`, `Stop`)
- [ ] 7.3 Write `internal/scaffold/templates/hooks/bindings/cursor/hooks.json` — `version: 1` envelope; `hooks.sessionStart`: same session-start commands; `hooks.stop`: same end-of-turn commands (lowercase event keys)
- [ ] 7.4 Write `internal/scaffold/templates/hooks/bindings/kiro/agent-patch.json` — `hooks.agentSpawn` for session-start commands; `hooks.stop` for end-of-turn commands
- [ ] 7.5 Write `internal/scaffold/templates/hooks/bindings/antigravity/hooks.json` — `PostTurnHook` for end-of-turn commands; stub `_note` for session-start (undocumented); include `"_preview": true` field
- [ ] 7.6 Write `internal/scaffold/templates/hooks/bindings/github-copilot/hooks-manifest.json` — stub with `_note` field; GitHub Copilot has no public shell-command hook API; do not attempt to fake lifecycle bindings via prompt instructions

## 8. Scaffold package — installer logic

- [ ] 8.1 Define `Config` struct (`RepoRoot`, `CodingTool string`, `Force bool`) and `Result` type (`Path`, `Action string`) in `internal/scaffold/scaffold.go`
- [ ] 8.2 Implement `Install(cfg Config) ([]Result, error)` — calls `installAgents` then `bindHooks` in sequence
- [ ] 8.3 Implement `installAgents` — reads platform-specific agent templates from embedded FS, writes to platform output path, respects `Force`, returns per-file `Result`
- [ ] 8.4 Implement platform output path resolver — maps `CodingTool` to target directory and file extension; Antigravity returns `~/.gemini/antigravity-cli/plugins/dreamland/agents/`
- [ ] 8.5 Implement Antigravity extras — write `plugin.json` manifest alongside agent files; append post-install instruction to results telling user to run `antigravity plugin install ~/.gemini/antigravity-cli/plugins/dreamland`
- [ ] 8.6 Implement `bindHooks` — reads the platform-specific binding template from embedded FS; dispatches to a platform-specific merge/write function
- [ ] 8.7 Implement atomic JSON merge helper — reads existing JSON (or starts with `{}`), deep-merges the binding patch without overwriting unrelated keys, writes to a temp file in the same directory, renames atomically (`os.Rename`)
- [ ] 8.8 Implement Claude Code binding — merge on `.claude/settings.json`
- [ ] 8.9 Implement Codex CLI binding — merge on `.codex/hooks.json`
- [ ] 8.10 Implement Cursor binding — merge on `.cursor/hooks.json`; ensure `"version": 1` field is present
- [ ] 8.11 Implement Kiro binding — merge on `.kiro/agent.json`
- [ ] 8.12 Implement Antigravity binding — write `hooks.json` directly into `~/.gemini/antigravity-cli/plugins/dreamland/` (no merge; new file)
- [ ] 8.13 Implement GitHub Copilot binding — write `.github/copilot-hooks/hooks-manifest.json` stub
- [ ] 8.14 Write unit tests for `installAgents` for each of the six platforms using a temp directory
- [ ] 8.15 Write unit tests for atomic JSON merge helper: absent file, existing file with unrelated keys, existing file with existing hooks (no duplicates)

## 9. Init wizard — repo root step, expanded tool list, language defaults

- [ ] 9.1 Add `repoRoot string` field to `wizardResult` struct in `cmd/init.go`
- [ ] 9.2 Add step 1 to `defaultWizardRunner`: `huh.NewInput` pre-filled with `config.FindRepoRoot(cwd)` result, with path-exists validation
- [ ] 9.3 Expand the coding tool `huh.NewSelect` options to full list: Claude Code, Codex CLI, Cursor, GitHub Copilot, Antigravity, Kiro
- [ ] 9.4 Renumber displayed step titles from "Step N/5" to "Step N/6" for steps 2–6
- [ ] 9.5 Set `VersionBumpCommand` in the config struct based on language selection (Node → `npm version`, Rust → `cargo bump`, Python → `bump-my-version bump`, Go → empty string)
- [ ] 9.6 Set `ModelID` in config based on coding tool selection (e.g., Claude Code → `"claude-sonnet-4-6"`, Codex CLI → `"codex-1"`, Cursor → `"cursor-default"`, Kiro → `"kiro-default"`, Antigravity → `"gemini-2.5-pro"`, GitHub Copilot → `"gpt-4o"`); document that users should update this field to match their actual model after init
- [ ] 9.7 Pass `res.repoRoot` into the `Config` struct written by `runInit`
- [ ] 9.8 Write `EmailSuffix` field: use value from `--email-suffix` flag; default to `"@github.com"` when flag is absent
- [ ] 9.9 Update `cmd/init_test.go` wizard stub to supply `repoRoot` and update step-count assertions

## 10. Init command — scaffolding integration, --force, and --email-suffix flags

- [ ] 10.1 Add `--force` boolean flag to `initCmd` (default false)
- [ ] 10.2 Add `--email-suffix` string flag to `initCmd` (default `"@github.com"`); pass value into config and write to `.dreamland.json`
- [ ] 10.2 After `config.Save`, call `scaffold.Install(scaffold.Config{RepoRoot: res.repoRoot, CodingTool: res.tool, Force: forceFlag})`
- [ ] 10.3 Print per-file scaffold results to `cmd.OutOrStdout()` before the final success message
- [ ] 10.4 Print PATH reminder after scaffold completes: `"Ensure 'dreamland' is in your PATH — hook bindings call it directly."`
- [ ] 10.5 On scaffold error, write to `cmd.ErrOrStderr()` and return non-zero; do not roll back the already-written config
- [ ] 10.6 Update `cmd/init_test.go` to assert scaffold output appears in stdout on the happy path
- [ ] 10.7 Update `cmd/init_test.go` to assert `--force` passes through to the scaffold installer

## 11. End-to-end validation

- [ ] 11.1 `go build ./...` — zero errors
- [ ] 11.2 `go test ./...` — all tests pass
- [ ] 11.3 Run `dreamland init` (Claude Code, Go) in a temp git repo; verify `.claude/agents/`, `.claude/settings.json` has `SessionStart` and `Stop` entries calling `dreamland` commands
- [ ] 11.4 Run `dreamland init` (Codex CLI, Node); verify `.codex/agents/` and `.codex/hooks.json` with `SessionStart`/`Stop` entries; verify `version_bump_command` = `npm version` in `.dreamland.json`
- [ ] 11.5 Run `dreamland init` (Cursor, Rust); verify `.cursor/rules/` and `.cursor/hooks.json` with `sessionStart`/`stop` keys and `"version": 1`
- [ ] 11.6 Run `dreamland init` (Kiro, Python); verify `.kiro/steering/` has `inclusion: always` frontmatter; `.kiro/agent.json` has `agentSpawn`/`stop` entries
- [ ] 11.7 Run `dreamland init` (Antigravity); verify plugin bundle at `~/.gemini/antigravity-cli/plugins/dreamland/`; verify `hooks.json` uses `PostTurnHook` and contains `_preview: true`; verify post-install instruction in stdout
- [ ] 11.8 Run `dreamland init` (GitHub Copilot); verify `.github/copilot-agents/` and `.github/copilot-hooks/hooks-manifest.json` stub
- [ ] 11.9 Re-run `dreamland init` (Claude Code) without `--force`; verify existing agent files skipped and hook entries not duplicated
- [ ] 11.10 Re-run `dreamland init --force`; verify all agent files overwritten
- [ ] 11.11 Run `dreamland version-bump` on a branch not in `.dreamland/branch-bumps`; verify minor bump and marker written
- [ ] 11.12 Run `dreamland version-bump` again on same branch; verify silent exit 0 (idempotent)
- [ ] 11.13 Run `dreamland version-bump --patch` with staged source changes; verify patch bump
- [ ] 11.14 Run `dreamland version-bump --patch` with no source changes; verify silent exit 0
- [ ] 11.15 Run `dreamland coauthor`; verify `git config --local user.name` and `user.email` are set; verify `.git/hooks/prepare-commit-msg` exists with mode 0755 and contains `dreamland coauthor --trailer`
- [ ] 11.16 Make a commit after `dreamland coauthor` runs; verify commit message contains `Co-authored-by: <model-id>` trailer
- [ ] 11.17 Make a second commit; verify trailer is not duplicated
- [ ] 11.18 Run `dreamland test` with a modified `.go` file present; verify test command executes
- [ ] 11.19 Run `dreamland test` with only non-source files changed; verify silent exit 0
- [ ] 11.20 Run `dreamland transition-log` twice; verify two lines appended to `.dreamland/transition.log`
- [ ] 11.21 Run `dreamland version-bump` in a repo with no semver tags; verify baseline treated as `v0.0.0` and first tag created (e.g., `v0.1.0`)
- [ ] 11.22 Run `dreamland init --email-suffix @myorg.com`; verify `.dreamland.json` contains `"email_suffix": "@myorg.com"`; run `dreamland coauthor` and verify email uses `@myorg.com`
