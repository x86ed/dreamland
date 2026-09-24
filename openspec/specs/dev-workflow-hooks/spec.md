# dev-workflow-hooks
## Requirements
### Requirement: Four lifecycle commands added to the dreamland binary

The `dreamland` binary SHALL expose four new cobra subcommands that implement agent lifecycle hook logic. These commands are the same across all platforms; only the binding files that invoke them differ.

```text
dreamland version-bump [--major | --minor | --patch] [--version <semver>] [--breaking]
dreamland coauthor
dreamland transition-log
dreamland test
```

Each command reads `.dreamland.json` for configuration and exits 0 on success, non-zero on failure. No shell scripts are installed; hook bindings reference these binary commands directly.

#### Scenario: Commands are available after installing the binary

- **WHEN** the `dreamland` binary is installed and `dreamland --help` is run
- **THEN** `version-bump`, `coauthor`, `transition-log`, and `test` appear in the command list

### Requirement: version-bump minor/major fires once per branch at session start

`dreamland version-bump` (without `--patch`) SHALL run at the session-start lifecycle event. It performs a minor or major version bump **at most once per branch**, enforced by a branch marker file at `.dreamland/branch-bumps`.

Algorithm:

1. If `git diff <last-tag>..HEAD` is empty (no commits since last tag), exit 0 silently.
2. Read `.dreamland/branch-bumps`. If the current branch name has an entry, exit 0 silently.
3. If the branch has no remote upstream (`@{u}` unset), treat it as a new uninitialized branch.
4. Bump `--minor` by default; bump `--major` if `--breaking` is passed.
5. Write `<branch-name>=<new-version>` to `.dreamland/branch-bumps`.
6. If no upstream was set in step 3, push: `git push --set-upstream origin <branch>`.

Version bump is dispatched to the language-appropriate tool from `.dreamland.json`:

| Language | Default `version_bump_command` | Invocation |
| --- | --- | --- |
| Go | _(empty)_ | dreamland creates an annotated git tag directly |
| Node/TypeScript | `npm version` | `npm version minor\|major` |
| Rust | `cargo bump` | `cargo bump minor\|major` |
| Python | `bump-my-version bump` | `bump-my-version bump minor\|major` |

#### Scenario: Minor bump on first session of a new branch

- **WHEN** `dreamland version-bump` runs, the current branch is not in `.dreamland/branch-bumps`, and commits exist since the last tag
- **THEN** the minor version is incremented and `<branch>=<new-version>` is written to `.dreamland/branch-bumps`

#### Scenario: Skipped on second and subsequent sessions on same branch

- **WHEN** `dreamland version-bump` runs and the current branch is already in `.dreamland/branch-bumps`
- **THEN** the command exits 0 silently without bumping

#### Scenario: Major bump when --breaking passed

- **WHEN** `dreamland version-bump --breaking` runs on a branch not in `.dreamland/branch-bumps`
- **THEN** the major version is incremented

#### Scenario: Upstream pushed when branch has no remote

- **WHEN** `dreamland version-bump` runs, the branch has no remote upstream, and a minor/major bump is performed
- **THEN** the branch is pushed with `git push --set-upstream origin <branch>` after bumping

#### Scenario: No-op when no commits since last tag

- **WHEN** `dreamland version-bump` runs and `git diff <last-tag>..HEAD` returns no output
- **THEN** the command exits 0 without modifying any file or creating any tag

#### Scenario: First-ever version bump on repo with no tags

- **WHEN** `dreamland version-bump` runs and no semver tags exist in the repo
- **THEN** the command treats the baseline as `v0.0.0` and creates the appropriate first tag (e.g., `v0.1.0` for a minor bump)

#### Scenario: Explicit version override updates marker

- **WHEN** `dreamland version-bump --version v2.0.0` runs
- **THEN** the version is set to `v2.0.0` and the branch marker is updated, regardless of prior bump history

#### Scenario: cargo bump absent

- **WHEN** `version_bump_command` is `cargo bump` and the `cargo-bump` plugin is not installed
- **THEN** the command prints a human-readable install hint and exits non-zero

### Requirement: version-bump --patch fires at end of turn when code changed

`dreamland version-bump --patch` SHALL run at the end-of-turn (`Stop`) lifecycle event. It checks `git diff <last-tag>..HEAD` for changes; if none are present it exits 0 silently. If changes are present, it bumps the patch version using the configured tool. It does not consult or update `.dreamland/branch-bumps`.

| Language | Default `version_bump_command` | Invocation |
| --- | --- | --- |
| Go | _(empty)_ | dreamland creates an annotated git tag directly |
| Node/TypeScript | `npm version` | `npm version patch` |
| Rust | `cargo bump` | `cargo bump patch` |
| Python | `bump-my-version bump` | `bump-my-version bump patch` |

#### Scenario: Patch bump when code changed at end of turn

- **WHEN** `dreamland version-bump --patch` runs and `git diff <last-tag>..HEAD` is non-empty
- **THEN** the patch version is incremented using the configured tool

#### Scenario: No-op at end of turn when no code changed

- **WHEN** `dreamland version-bump --patch` runs and `git diff <last-tag>..HEAD` is empty
- **THEN** the command exits 0 silently

### Requirement: coauthor sets agent identity and installs prepare-commit-msg hook

`dreamland coauthor` SHALL run at the session-start lifecycle event and perform two actions:

**a. Set agent git identity (repository-local scope):**

AgentName resolution tries, in order:

1. A hook stdin payload for the current invocation, read only when `dreamland coauthor` is invoked with `--hook` — a flag set exclusively by dreamland's own scaffold-installed hook-binding templates (every platform: Claude Code, GitHub Copilot, Cursor, Codex, Kiro), never by a human running the command manually. Without `--hook`, stdin is never opened or read at all, so a manual invocation in any terminal returns immediately with no possibility of blocking, regardless of what kind of stdin is attached. With `--hook`, the payload is read synchronously to completion (no timeout) — correct because every hook-binding caller writes its payload and closes its end of the pipe promptly, so the read completes as soon as the real data arrives, however long that legitimately takes, checked for an agent identifier in whichever shape the platform actually emits:
   - GitHub Copilot: top-level `agent_type` (e.g. `"morpheus"`) on `SubagentStart`/`SubagentStop` payloads.
   - Claude Code: top-level `agent_type` (e.g. `"morpheus"`) on the `SubagentStop` payload (which also carries `agent_id`, `agent_transcript_path`, and `last_assistant_message`), and `tool_input.subagent_type` (e.g. `"morpheus"`) on the `PreToolUse`/`PostToolUse` payload for the `Task`/`Agent` tool call. `SessionStart`/`Stop` payloads on Claude Code do not carry a sub-agent identifier (only `session_id`/`transcript_path`/`hook_event_name`), so those events resolve nothing from the payload. See the "Claude Code sub-agent identity resolution reads the SubagentStop payload's agent_type field" requirement below.
2. The platform's current-agent env var, if the platform sets one at runtime (no currently-supported platform does; this path exists for forward compatibility and is not exercised by Claude Code or GitHub Copilot).
3. The coding tool name in `.dreamland.json`.
4. If a hook payload resolved a candidate value (step 1) that is not one of the ten registered dreamland agent names (`janus`, `phantasos`, `nyx`, `morpheus`, `phobetor`, `baku`, `iktomi`, `zhougong`, `hypnos`, `mengpo`), that candidate is discarded — treated the same as if step 1 had resolved nothing — rather than used verbatim.
5. On Claude Code specifically, if steps 1-2 resolve nothing, AgentName is `janus`, not the coding-tool name — see the `session-agent-identity` capability for the full "no default, no unknown agent" requirement this satisfies. On every other platform, step 3 (coding-tool name) remains the fallback when steps 1-2 resolve nothing, unchanged from prior behavior.

This full resolution sequence (steps 1-5) is exposed as a single internal function so no other command re-implements it independently; the `dreamland commit` requirement below, after an explicit `--agent-name` and (with `--hook`) the hook payload's own agent identity, reads the *result already persisted by this logic* (the git config value this requirement sets, per part **a** below) rather than re-running steps 1-5 itself, falling back to this function only when that value is unset, since `commit` may run at a different hook event with a different payload shape than the `coauthor` invocation that last set identity — see that requirement for why independent re-resolution at a different lifecycle event would be incorrect.

AgentEmail is derived by cleaning AgentName and appending `email_suffix` from `.dreamland.json` (default `@github.com`).

Email cleaning: lowercase → replace spaces and underscores with `-` → strip characters not in `[a-z0-9.\-]` → trim leading/trailing `-` and `.`.

`git config --local user.name` is set to AgentName. `git config --local user.email` is set to AgentEmail. A failure to write either value with `git config` is a genuine, blocking failure (exit code 2 — see the "Hook-invoked commands distinguish blocking failures from advisory skips" requirement), not an advisory one: identity enforcement is the entire purpose of this command.

If no git repository exists at or above the current working directory, `dreamland coauthor` SHALL exit 0 silently (a skip, not a failure) — there is no identity to set in a location `git config --local` cannot apply to. Any other failure while loading `.dreamland.json` (e.g. malformed JSON) is a genuine, blocking failure.

This identity logic is identical for every scaffolded agent (Janus, Phantasos, Nyx, Morpheus, Phobetor, Baku, Iktomi, Zhou Gong, Hypnos, Meng Po) — none of them get special-cased behavior; only the AgentName value resolved per invocation differs.

**b. Install a `prepare-commit-msg` git hook:**

Write (or update) `.git/hooks/prepare-commit-msg` as a minimal shell wrapper that delegates to `dreamland`:

```sh
#!/bin/sh
dreamland coauthor --trailer "$1" "$2" "$3"
```

When invoked with `--trailer`, `dreamland coauthor` reads `$1` (commit message file path), constructs model identity from `.dreamland.json`, and appends to the file if no matching trailer is already present:

```text
Co-authored-by: <model-name> <model-email>
```

`model-name` is the name portion of `model_id` (text before the first space). `model-email` is the cleaned model-name plus `email_suffix`. All logic in Go; no shell tools required.

Immediately after the Co-authored-by trailer, `dreamland coauthor --trailer` appends a token-usage report line sourced from the current turn's telemetry snapshot (the same data `dreamland telemetry write` collects — model name and input/output/cached/total token counts):

```text
Tokens: input=<n> output=<n> cached=<n> total=<n>
```

If telemetry data is unavailable for the current turn (e.g. the platform doesn't expose usage stats, or no turn has completed yet), the `Tokens:` line is omitted and only the Co-authored-by trailer is appended — this is not a failure condition.

The hook file is written with mode 0755. If `.git/hooks/prepare-commit-msg` already contains `dreamland coauthor --trailer`, the file is left unchanged.

This hook script intentionally has no `command -v dreamland` guard: if `dreamland` is not resolvable when git invokes it, the shell wrapper fails with a nonzero exit and git aborts the commit. This fail-closed behavior is a deliberate decision (not an accident of the script's simplicity) — see the `otel-commit-hook` capability's "Hook appends AI telemetry as git trailers" requirement, which makes the parallel `commit-msg` hook fail the same way for the same condition, for consistency. The standard `git commit --no-verify` bypass remains available for a developer who genuinely needs to commit without `dreamland` installed.

#### Scenario: Agent git identity set from env var at session start

- **WHEN** `dreamland coauthor` runs and the platform env var for current agent is set (e.g., `CLAUDE_AGENT_ID=hypnos`)
- **THEN** `git config --local user.name` is set to `"hypnos"` and `git config --local user.email` to `"hypnos@github.com"` (with configured suffix)

#### Scenario: Agent git identity falls back to coding tool name on non-Claude-Code platforms

- **WHEN** `dreamland coauthor` runs with `.dreamland.json` coding tool `"GitHub Copilot"`, no platform agent env var is set, and no hook payload resolves an identity
- **THEN** `git config --local user.name` is set to `"GitHub Copilot"` and email to `"github-copilot@github.com"`

#### Scenario: Claude Code falls back to janus, not the coding tool name

- **WHEN** `dreamland coauthor` runs with `.dreamland.json` coding tool `"Claude Code"`, no platform agent env var is set, and no hook payload resolves an identity
- **THEN** `git config --local user.name` is set to `"janus"`, not `"Claude Code"`

#### Scenario: Email cleaning applied to agent name

- **WHEN** AgentName is `"Spec Writer"` and `email_suffix` is `@github.com`
- **THEN** AgentEmail is `"spec-writer@github.com"`

#### Scenario: prepare-commit-msg hook installed

- **WHEN** `dreamland coauthor` runs and `.git/hooks/prepare-commit-msg` does not exist
- **THEN** the file is created with mode 0755 containing `#!/bin/sh` and `dreamland coauthor --trailer "$1" "$2" "$3"`

#### Scenario: Claude Code identity resolved from SubagentStop agent_type

- **WHEN** `dreamland coauthor --hook` runs via Claude Code's `SubagentStop` hook with payload `{"agent_type": "phobetor", ...}`
- **THEN** `git config --local user.name` is set to `"phobetor"`, not the generic coding-tool fallback

#### Scenario: Claude Code identity resolved from tool_input.subagent_type

- **WHEN** `dreamland coauthor` runs via Claude Code's `PreToolUse` hook for a `Task` tool call whose payload is `{"tool_input": {"subagent_type": "phobetor", ...}}`
- **THEN** `git config --local user.name` is set to `"phobetor"`, not the generic coding-tool fallback

#### Scenario: Unrecognized resolved identity falls back to janus, not the raw value

- **WHEN** `dreamland coauthor` resolves a candidate AgentName that is not one of the ten registered dreamland agent names (e.g. a malformed or unexpected payload value)
- **THEN** AgentName falls back to `"janus"` rather than using the unrecognized value verbatim

#### Scenario: coauthor without --hook never touches stdin

- **WHEN** `dreamland coauthor` runs without the `--hook` flag (a human invoking it directly, in any kind of terminal)
- **THEN** no read of stdin is attempted, AgentName resolution proceeds directly to step 2, and the command cannot block regardless of what is or isn't attached to stdin

#### Scenario: coauthor --hook flag added to every platform's hook binding

- **WHEN** `dreamland init` completes for Claude Code, GitHub Copilot, Cursor, Codex, or Kiro
- **THEN** every default-mode `dreamland coauthor` entry in that platform's installed hook-binding file includes `--hook`
- **AND** `--trailer`-mode invocations (the installed `prepare-commit-msg` git hook) do not include `--hook`, since that code path never reads stdin for an agent-identity payload

#### Scenario: GitHub Copilot identity resolution is unaffected by the --hook gating change

- **WHEN** `dreamland coauthor --hook` runs via GitHub Copilot's `SubagentStart`/`SubagentStop` hook with a payload of `{"agent_type": "iktomi", ...}`
- **THEN** `git config --local user.name` is set to `"iktomi"`, exactly as it would resolve before this change — only when the payload is read changed, not how it is parsed

#### Scenario: coauthor skips silently outside a git repository

- **WHEN** `dreamland coauthor` runs in a directory with no git repository at or above it
- **THEN** the command exits 0 without error and does not attempt to write git config

#### Scenario: A genuine git config write failure blocks the lifecycle event

- **WHEN** `dreamland coauthor` resolves an AgentName/AgentEmail but the `git config --local user.name` or `git config --local user.email` write itself fails
- **THEN** the command exits with the blocking code (2), not the advisory code

### Requirement: init accepts --email-suffix to configure the agent/model email domain

The `init` command SHALL accept an `--email-suffix` flag (default `@github.com`). The value is stored in `.dreamland.json` as `email_suffix` and used by `dreamland coauthor` to construct AgentEmail and ModelEmail.

#### Scenario: Default email suffix used when flag absent

- **WHEN** `dreamland init` runs without `--email-suffix`
- **THEN** `.dreamland.json` contains `"email_suffix": "@github.com"`

#### Scenario: Custom suffix stored when flag provided

- **WHEN** `dreamland init --email-suffix @myorg.com` runs
- **THEN** `.dreamland.json` contains `"email_suffix": "@myorg.com"`

### Requirement: transition-log appends a timestamped line at end of turn

`dreamland transition-log` SHALL append one line to `.dreamland/transition.log` on every invocation:

```text
<ISO-8601 timestamp> [<session-id>] turn complete
```

`session-id` is read from the `CLAUDE_SESSION_ID`, `CODEX_SESSION_ID`, or equivalent environment variable set by the platform. If no session variable is found, a random 8-character hex ID is generated for the line.

The `.dreamland/` directory is created if absent. Appending to the log is always a no-op failure-safe: a write error does not cause a non-zero exit.

#### Scenario: Log entry written on every turn

- **WHEN** `dreamland transition-log` runs
- **THEN** `.dreamland/transition.log` gains one new line with a valid ISO-8601 timestamp and session identifier

#### Scenario: Directory created when absent

- **WHEN** `dreamland transition-log` runs and `.dreamland/` does not exist
- **THEN** the directory is created and the log file is written

### Requirement: test only runs when executable source files changed

`dreamland test` SHALL inspect `git status --porcelain` for unstaged and staged changes to source files matching the configured language's extensions. If matching files are present, it runs `test_command` from `.dreamland.json`, connecting the child process's stdout/stderr to `dreamland test`'s own so the underlying test output is visible (not discarded), and records the outcome as described below. If no matching files are found, it exits 0 silently and writes no result record.

Source file extensions by language:

| Language | Extensions |
| --- | --- |
| Go | `.go` |
| Node/TypeScript | `.ts`, `.tsx`, `.js`, `.jsx`, `.mts`, `.cts` |
| Rust | `.rs` |
| Python | `.py` |

When the configured `test_command` is actually run, `dreamland test` SHALL write `.dreamland/last-test-result.json` (creating the `.dreamland/` directory if absent) recording the outcome:

```json
{"status": "pass" | "fail", "head_sha": "<git rev-parse HEAD>", "written_at": "<RFC3339 timestamp>"}
```

This record exists so `dreamland commit` (see the modified "dreamland commit" requirement below) can refuse to checkpoint a commit over a test failure at the same `HEAD`, without depending on any particular ordering guarantee between separate hook-bound command invocations.

If `test_command` exits nonzero (the configured tests actually failed) or fails to execute at all (e.g. the command isn't found), `dreamland test` SHALL exit with the blocking code (2) — not the raw exit code the underlying test command produced, and not the advisory code (1) — so Claude Code recognizes a real test failure as blocking. `git`, config-loading, and "nothing to test" conditions remain non-blocking skips exiting 0, unchanged from prior behavior.

#### Scenario: Tests run after source code changes

- **WHEN** `dreamland test` runs and `git status --porcelain` shows at least one file with a matching source extension
- **THEN** `test_command` from `.dreamland.json` is executed with its stdout/stderr connected to `dreamland test`'s own, and `.dreamland/last-test-result.json` is written recording the outcome and the current `HEAD` sha

#### Scenario: Tests skipped when no source changed

- **WHEN** `dreamland test` runs and no source files have changes
- **THEN** the command exits 0 without running anything and without writing or modifying `.dreamland/last-test-result.json`

#### Scenario: Test failure blocks the lifecycle event

- **WHEN** `dreamland test` runs, source files changed, and `test_command` exits non-zero
- **THEN** `.dreamland/last-test-result.json` records `"status": "fail"` for the current `HEAD`, and `dreamland test` exits with the blocking code (2), regardless of the underlying test command's own specific exit code

#### Scenario: Test success is recorded

- **WHEN** `dreamland test` runs, source files changed, and `test_command` exits 0
- **THEN** `.dreamland/last-test-result.json` records `"status": "pass"` for the current `HEAD`, and `dreamland test` exits 0

### Requirement: Hook logic is platform-independent

The four lifecycle commands (`version-bump`, `coauthor`, `transition-log`, `test`) are implemented entirely in Go and are identical in behavior on every platform. Platform-specific variation is limited to the binding files that invoke them.

#### Scenario: Identical behavior across platforms

- **WHEN** `dreamland test` is run on a Cursor project and a Claude Code project with the same `.dreamland.json`
- **THEN** the behavior and output are identical in both cases

### Requirement: All session-start commands bind to the platform's session-start event

The scaffold installer SHALL register `dreamland version-bump` and `dreamland coauthor` under each platform's session-start event when writing hook binding files.

Session-start event names by platform (verified from platform docs):

| Platform | Session-start event | Scope |
| --- | --- | --- |
| Claude Code | `SessionStart` | All modes |
| Codex CLI | `SessionStart` | All modes |
| Cursor | `sessionStart` | Agent, ask, edit modes |
| Kiro CLI | `agentSpawn` | Agent mode only |
| Antigravity | stub | Session-start undocumented |
| GitHub Copilot | stub | No public hook API |

#### Scenario: Session-start commands registered for Claude Code

- **WHEN** `dreamland init` completes with "Claude Code" selected
- **THEN** `.claude/settings.json` contains `dreamland version-bump` and `dreamland coauthor` under the `SessionStart` event key

#### Scenario: Session-start commands registered for Kiro

- **WHEN** `dreamland init` completes with "Kiro" selected
- **THEN** `.kiro/agent.json` contains `dreamland version-bump` and `dreamland coauthor` under the `agentSpawn` event key

### Requirement: All end-of-turn commands bind to the platform's stop event

The scaffold installer SHALL register `dreamland version-bump --patch`, `dreamland transition-log`, and `dreamland test` under each platform's end-of-turn event.

End-of-turn event names by platform:

| Platform | End-of-turn event |
| --- | --- |
| Claude Code | `Stop` |
| Codex CLI | `Stop` |
| Cursor | `stop` |
| Kiro CLI | `stop` |
| Antigravity | `PostTurnHook` |
| GitHub Copilot | stub — no public hook API |

#### Scenario: End-of-turn commands registered for Claude Code

- **WHEN** `dreamland init` completes with "Claude Code" selected
- **THEN** `.claude/settings.json` contains `dreamland version-bump --patch`, `dreamland transition-log`, and `dreamland test` under the `Stop` event key

#### Scenario: End-of-turn commands registered for Cursor

- **WHEN** `dreamland init` completes with "Cursor" selected
- **THEN** `.cursor/hooks.json` contains `dreamland version-bump --patch`, `dreamland transition-log`, and `dreamland test` under the `stop` key within a `version: 1` envelope

### Requirement: Claude Code hook binding via settings.json merge

For "Claude Code", the scaffold installer SHALL merge the hook registrations into `.claude/settings.json` under `hooks.SessionStart` and `hooks.Stop`, creating the file if absent. The merge is atomic: write to a temp file in the same directory, then rename into place.

If the file already contains dreamland hook entries they are replaced, not duplicated.

#### Scenario: Claude Code settings.json created when absent

- **WHEN** `.claude/settings.json` does not exist and platform is "Claude Code"
- **THEN** the file is created with `SessionStart` and `Stop` hook registrations

#### Scenario: Claude Code settings.json merged when present

- **WHEN** `.claude/settings.json` already exists with other settings
- **THEN** existing non-hook settings are preserved and hook entries are added under `SessionStart` and `Stop`

### Requirement: Codex CLI hook binding via hooks.json merge

For "Codex CLI", the scaffold installer SHALL merge hook registrations into `.codex/hooks.json` under `SessionStart` and `Stop` keys, creating the file if absent.

#### Scenario: Codex hooks.json created when absent

- **WHEN** `.codex/hooks.json` does not exist and platform is "Codex CLI"
- **THEN** the file is created with `SessionStart` and `Stop` hook registrations

### Requirement: Cursor hook binding via hooks.json merge

For "Cursor", the scaffold installer SHALL merge hook registrations into `.cursor/hooks.json` using `version: 1` envelope with `sessionStart` and `stop` event keys.

#### Scenario: Cursor hooks.json created when absent

- **WHEN** `.cursor/hooks.json` does not exist and platform is "Cursor"
- **THEN** the file is created as `{"version": 1, "hooks": {"sessionStart": [...], "stop": [...]}}`

### Requirement: Kiro hook binding via agent.json merge

For "Kiro", the scaffold installer SHALL merge hook registrations into `.kiro/agent.json` with `agentSpawn` and `stop` keys.

#### Scenario: Kiro agent.json created when absent

- **WHEN** `.kiro/agent.json` does not exist and platform is "Kiro"
- **THEN** the file is created with `agentSpawn` and `stop` hook registrations

### Requirement: Antigravity hook binding via plugin hooks.json

For "Antigravity", the scaffold installer SHALL write `hooks.json` into the plugin bundle at `~/.gemini/antigravity-cli/plugins/dreamland/`. End-of-turn commands bind to `PostTurnHook` (the documented Antigravity equivalent of `Stop`). Session-start binding is stubbed with `"_note"` because Antigravity's session-start hook is not yet publicly documented. The file is marked `"_preview": true`.

#### Scenario: Antigravity plugin hooks.json written

- **WHEN** `dreamland init` completes with "Antigravity" selected
- **THEN** `~/.gemini/antigravity-cli/plugins/dreamland/hooks.json` exists with `PostTurnHook` end-of-turn registrations and a stub session-start note

### Requirement: JSON config merges are atomic

All platform hook binding files that require merging into existing JSON (Claude Code, Codex, Cursor, Kiro) SHALL use an atomic write strategy: write the merged content to a temp file in the same directory, then call `os.Rename(tempFile, targetFile)`. The original file is never modified directly.

#### Scenario: Original file preserved on write failure

- **WHEN** a merge write fails after the temp file is created but before rename completes
- **THEN** the original config file remains unchanged

### Requirement: init sets version_bump_command and model_id in config

The `init` wizard SHALL set `version_bump_command` and `model_id` in `.dreamland.json` based on the language and coding tool selections, so that `dreamland version-bump` and `dreamland coauthor` can operate without further configuration.

#### Scenario: version_bump_command set for Node project

- **WHEN** `dreamland init` completes with "Node/TypeScript" selected
- **THEN** `.dreamland.json` contains `"version_bump_command": "npm version"`

#### Scenario: model_id set for Claude Code project

- **WHEN** `dreamland init` completes with "Claude Code" selected
- **THEN** `.dreamland.json` contains `"model_id": "claude-sonnet-4-6"` (or the current default model)

### Requirement: Claude Code binds identity, telemetry, and patch-version commands to the sub-agent handoff lifecycle

In addition to the existing `SessionStart`/`Stop` bindings, the scaffold installer SHALL register `dreamland coauthor` under Claude Code's `PreToolUse` event (matcher: `Task|Agent` — the subagent-dispatch tool was renamed from `Task` to `Agent` in Claude Code v2.1.63; matching both keeps the binding correct across the VS Code extension's bundled CLI version and the standalone CLI regardless of which has updated) and `dreamland telemetry write --tool claude-code` and `dreamland version-bump --patch` under Claude Code's `SubagentStop` event, so that git identity is refreshed before every hand-off to a sub-agent, and telemetry/patch-version are refreshed on every hand-off — every agent turn — not only once per session.

The workspace-level `SubagentStop` binding SHALL NOT also register `dreamland coauthor`: that command writes the shared, mutable `git config user.name`/`user.email`, and every dreamland agent's own `.claude/agents/*.md` frontmatter already registers a `hooks.Stop` block (converted to `SubagentStop` by Claude Code's subagent-hooks mechanism) carrying `coauthor --agent-name <self>`. Because Claude Code runs every matching hook for an event in parallel with no ordering guarantee, two `coauthor` writers raced on that shared value.

The workspace-level `SubagentStop` binding SHALL register `dreamland commit --reason handoff --hook`, which resolves the acting agent from the payload's `agent_type` (so it also covers agents whose frontmatter hooks did not run) and pins the commit's author and committer to that identity explicitly (see the `dreamland commit` requirement below). Because the commit's authorship no longer depends on the shared git config, it is safe for this command to run alongside the per-agent `commit --reason handoff --agent-name <self>`: both attribute the same agent, and whichever loses the race finds nothing left to commit (a benign no-op).

#### Scenario: PreToolUse identity refresh before dispatch, unchanged

- **WHEN** Claude Code's `PreToolUse` hook fires for a `Task`/`Agent` tool call dispatching to `phobetor`
- **THEN** `dreamland coauthor` runs via the `PreToolUse` hook before the sub-agent's turn begins, updating `git config user.name`/`user.email` to `phobetor`

#### Scenario: SubagentStop does not duplicate per-agent identity-writing commands

- **WHEN** `.claude/settings.json` is generated by the scaffold installer for Claude Code
- **THEN** its `SubagentStop` entry runs `dreamland telemetry write --tool claude-code`, `dreamland version-bump --patch`, `dreamland version-bump --minor --if-agent janus`, and `dreamland commit --reason handoff --hook`
- **AND** it does not run `dreamland coauthor`

#### Scenario: Handoff commit is attributed to the dispatched agent even if per-agent hooks did not run

- **WHEN** `morpheus`'s turn ends, the shared `git config user.name` currently reads `janus`, and only the workspace-level `SubagentStop` binding runs with payload `{"agent_type": "morpheus"}`
- **THEN** the resulting commit's author and committer are `morpheus <morpheus@github.com>`, not `janus`

### Requirement: GitHub Copilot binds identity, telemetry, and lifecycle commands via a real hooks file

GitHub Copilot's VS Code agent framework supports a documented hooks mechanism (workspace-scoped `.github/hooks/*.json`, shipped February 2026) with event names that closely mirror Claude Code's: `SessionStart`, `SubagentStart`, `SubagentStop`, and `Stop`, among others. GitHub Copilot is no longer a stub platform for hook binding.

The scaffold installer SHALL write `.github/hooks/dreamland-hooks.json` (merged with any existing content at that path, same atomic-merge behavior as the other platforms' hook bindings) containing:

- `SessionStart`: `dreamland version-bump`, `dreamland coauthor`, `dreamland otel-receiver`
- `SubagentStart`: `dreamland coauthor` (identity refresh before a sub-agent is dispatched — GitHub Copilot's `SubagentStart` event serves the same purpose Claude Code's `PreToolUse` matcher achieves indirectly)
- `SubagentStop`: `dreamland coauthor`, `dreamland telemetry write --tool github-copilot`, `dreamland version-bump --patch`, `dreamland commit --reason handoff`
- `Stop`: `dreamland version-bump --patch`, `dreamland transition-log`, `dreamland test`, `dreamland telemetry write --tool github-copilot`, `dreamland commit --reason turn-complete`

Each hook entry uses the real schema: `{"type": "command", "command": "<cmd>", "timeout": <seconds>}` — not a VS Code task (`.vscode/tasks.json`) and not an invented `bash`/`agentStop` shape.

This SHALL be declared redundantly via GitHub Copilot's agent-scoped `hooks:` frontmatter field too (see the `agent-scaffolding` capability) — the workspace file needs no settings flag and always fires; the agent-scoped copy fires once `chat.useCustomAgentHooks` is enabled. Both bind identical commands, so neither is a fallback for the other in a weaker sense — they're two independent, redundant triggers for the same effect.

#### Scenario: GitHub Copilot hooks file contains session-start entries

- **WHEN** `dreamland init` completes with "GitHub Copilot" selected
- **THEN** `.github/hooks/dreamland-hooks.json` contains a `SessionStart` entry running `dreamland version-bump` and `dreamland coauthor`

#### Scenario: GitHub Copilot hooks file refreshes identity and telemetry around subagent dispatch

- **WHEN** `dreamland init` completes with "GitHub Copilot" selected
- **THEN** `.github/hooks/dreamland-hooks.json` contains a `SubagentStart` entry running `dreamland coauthor`
- **AND** contains a `SubagentStop` entry running `dreamland coauthor`, `dreamland telemetry write --tool github-copilot`, `dreamland version-bump --patch`, and `dreamland commit --reason handoff`

#### Scenario: Existing hooks file content is preserved on merge

- **WHEN** `.github/hooks/dreamland-hooks.json` already contains a user-added hook entry and `dreamland init` runs again
- **THEN** the user's entry is preserved alongside dreamland's entries

### Requirement: A local OTLP/HTTP receiver captures GitHub Copilot's real token usage

Neither GitHub Copilot's `SubagentStop`/`Stop` hook payload nor its transcript file (referenced by `transcript_path`) expose token usage — confirmed empirically by parsing a complete, real captured transcript: no token or usage field exists anywhere in it. Copilot's only real source of token-usage data is its native OpenTelemetry export (`github.copilot.chat.otel.*`, already configured by `dreamland init`), which follows the OTel GenAI Semantic Conventions: an `invoke_agent` trace span carries `gen_ai.usage.input_tokens`, `gen_ai.usage.output_tokens`, `gen_ai.usage.cache_read.input_tokens`, `gen_ai.request.model`/`gen_ai.response.model`, and `gen_ai.conversation.id` (which matches the hook payload's `session_id`). `dreamland otel-receiver` SHALL implement a minimal OTLP/HTTP trace-export endpoint to capture that data locally.

`dreamland otel-receiver` SHALL implement a `POST /v1/traces` endpoint (accepting both `application/x-protobuf` and `application/json` bodies) at the same host:port `github.copilot.chat.otel.otlpEndpoint` is configured to use. For each span carrying a `gen_ai.conversation.id` and non-zero `gen_ai.usage.*` attributes, it SHALL write a per-session mailbox file at `.dreamland/otel-sessions/<conversation_id>.json` containing the extracted token counts and model.

The command SHALL be idempotent and non-blocking: invoked without `--foreground` (as bound to the `SessionStart` hook), it SHALL probe whether something is already listening at the target address and exit immediately if so; otherwise it SHALL spawn a detached child process (running with `--foreground`) that serves the receiver indefinitely, and the invoking process SHALL return without waiting for that child, so the `SessionStart` hook is never blocked by a long-running server.

`CopilotCollector` (the `dreamland telemetry write --tool github-copilot` collector) SHALL read `.dreamland/otel-sessions/<session_id>.json` (using the `session_id` from its own hook payload) and SHALL prefer that data over transcript parsing whenever present.

#### Scenario: Receiver captures token usage from a real trace export

- **WHEN** `dreamland otel-receiver` is running and receives a `POST /v1/traces` request containing a span with `gen_ai.conversation.id` and non-zero `gen_ai.usage.input_tokens`/`gen_ai.usage.output_tokens`
- **THEN** `.dreamland/otel-sessions/<conversation_id>.json` is written with those token counts and the resolved model

#### Scenario: Receiver is idempotent across repeated SessionStart invocations

- **WHEN** `dreamland otel-receiver` runs and a receiver is already listening at the configured address
- **THEN** it exits immediately without spawning a second instance

#### Scenario: Telemetry write prefers OTEL-captured usage over transcript parsing

- **WHEN** `dreamland telemetry write --tool github-copilot` runs and `.dreamland/otel-sessions/<session_id>.json` exists for the current hook payload's `session_id`
- **THEN** the resulting snapshot's token counts come from that file, not from parsing `transcript_path`

### Requirement: Platforms without a sub-agent lifecycle hook rely on agent-driven invocation

For platforms where no `PreToolUse`/`SubagentStop`-equivalent hook event is documented (Codex CLI, Cursor, Kiro, Antigravity), the scaffold installer SHALL NOT add a handoff-lifecycle hook binding. Instead, this is documented as a known limitation: the Janus agent's own instructions direct it to invoke `dreamland coauthor`, `dreamland telemetry write`, and `dreamland version-bump --patch` immediately before and after each delegation (see the `janus-router-agent` capability), and the existing `SessionStart`/`Stop` bindings continue to provide a session-level fallback.

GitHub Copilot is no longer in this category — see the "GitHub Copilot binds identity, telemetry, and lifecycle commands via a real hooks file" requirement above.

#### Scenario: No handoff hook binding added for Cursor

- **WHEN** `dreamland init` completes with "Cursor" selected
- **THEN** `.cursor/hooks.json` contains only the existing `sessionStart` and `stop` entries — no additional handoff-lifecycle entry is added

### Requirement: dreamland commit auto-commits pending changes on turn completion and agent handoff

A new lifecycle command, `dreamland commit --reason <turn-complete|handoff>`, SHALL run at end-of-turn and handoff-complete events so that every agent turn and every agent-to-agent transition is captured as its own commit, even if the agent itself never ran `git commit`.

Behavior:

1. If no git repository exists at or above the current working directory, exit 0 silently (a skip, not a failure) — there is nothing for this command to commit to.
2. Inspect `git status --porcelain`. If there are no staged or unstaged changes, exit 0 silently — no commit is created.
3. Before staging or committing anything, when `commitReason` is `turn-complete`, consult `.dreamland/last-test-result.json` (written by `dreamland test`, see the modified `test` requirement above), and `cfg.TestCommand` from `.dreamland.json`:
   - If the record exists and records `"status": "fail"` for a `head_sha` matching the repository's current `HEAD`, `dreamland commit` SHALL refuse to create the commit — no `git add`, no `git commit` — and SHALL exit with the blocking code (2). The pending changes remain staged/unstaged, to be picked up by the next successful `dreamland commit` invocation once tests pass.
   - If the record does not exist at all, and `cfg.TestCommand` is non-empty (a test command is configured for this project), `dreamland commit` SHALL treat this as a broken invariant, not a skip: it SHALL refuse to create the commit and SHALL exit with the blocking code (2), with a message stating that a test command is configured but no result was recorded for this turn and instructing that this should route to `iktomi` to investigate why `dreamland test` did not run or record a result before this commit attempt.
   - If the record does not exist and `cfg.TestCommand` is empty (no test command configured for this project), this gating step does not apply — there was never an expectation of a result — and `commit` proceeds normally.
   - If the record exists but its `head_sha` does not match the current `HEAD` (a stale record, not a missing one), this gating step does not apply and `commit` proceeds normally — staleness is treated differently from absence because a stale record at least proves tests ran successfully at some point, while absence (with a test command configured) proves they may not have run this turn at all.
4. Otherwise, stage all changes (`git add -A`) and run `git commit -m "chore: <reason> checkpoint (<agent-name>)"`, where `<agent-name>` is the resolved agent identity, resolved in this order (first match wins): (a) an explicit `--agent-name`; (b) when `--hook` is set, the hook payload's `agent_type`/`subagent_type` when it is a registered agent (a `SubagentStop` payload's `agent_type` is the subagent that just finished); (c) the configured `git config --local user.name`, i.e. the value `dreamland coauthor` most recently persisted, so that `commit` at a lifecycle event whose payload carries no sub-agent identity (`Stop`) does not re-derive a different value than `coauthor` set; (d) if `git config --local user.name` is unset, the same resolution `coauthor` uses (`resolveEnforcedAgentName`), ending in the `janus` default. The commit's author and committer are pinned to the resolved identity explicitly (`git -c user.name=<agent-name> -c user.email=<cleaned-name><suffix> commit ...`), independent of the shared `git config` value, so the subject and the author agree by construction even when a concurrent writer changes `git config --local user.name` between resolution and commit.
5. Because this shells out to `git commit`, the already-installed `prepare-commit-msg` hook fires normally and appends the Co-authored-by trailer and token-usage report (see the modified `coauthor` requirement) to the commit message — `dreamland commit` does not duplicate that logic.
6. A failure in `git add -A` or `git commit` itself (distinct from the test-gating refusal in step 3) is a genuine, blocking failure — exit code 2 — for `--reason turn-complete`, since the entire purpose of this command at end of turn is to guarantee a commit exists. For `--reason handoff` specifically, such a failure SHALL be reported to the user but SHALL NOT block the sub-agent's `SubagentStop` event: the command returns a non-blocking error (exit 1), so a transient git failure during hand-off cannot trap the fixed pipeline (`nyx`→`morpheus`→`phobetor`→`baku`) mid-transition. A `git commit` that reports "nothing to commit" because a concurrent session already committed the same staged changes is a benign no-op for both reasons (exit 0).

The scaffold installer SHALL bind, on Claude Code, `dreamland test-and-commit --reason turn-complete` to the `Stop` event (see the added `dreamland test-and-commit` requirement below) and `dreamland commit --reason handoff --hook` to the `SubagentStop` event, alongside `dreamland telemetry write --tool claude-code` and the version-bump commands; `dreamland coauthor` is not registered under `SubagentStop`. On any other platform whose `Stop`-equivalent binding chains `dreamland test` immediately before `dreamland commit --reason turn-complete`, the installer SHALL likewise bind `dreamland test-and-commit --reason turn-complete` in place of that pair, closing the race where the hook runner does not guarantee `test` finishes writing `.dreamland/last-test-result.json` before a separately-dispatched `commit` reads it. The test-gating step in behavior item 3 applies only to `--reason turn-complete`, so that race does not apply to handoff commits. On platforms without a `SubagentStop`-equivalent event, only the `Stop`-bound invocation applies; handoff commits there rely on the agent-driven convention described for `coauthor`/`telemetry write`.

#### Scenario: Commit created when a turn completes with pending changes

- **WHEN** `dreamland commit --reason turn-complete` runs via the `Stop` hook, `git status --porcelain` shows pending changes, and either no `.dreamland/last-test-result.json` record exists or it records a `"pass"` for the current `HEAD`
- **THEN** the changes are staged and committed with subject `chore: turn-complete checkpoint (<agent-name>)`, where `<agent-name>` is the identity resolved by behavior item 4

#### Scenario: No-op when a turn completes with a clean working tree

- **WHEN** `dreamland commit --reason turn-complete` runs and `git status --porcelain` is empty
- **THEN** the command exits 0 without creating a commit

#### Scenario: Commit refused when tests are failing at the current HEAD

- **WHEN** `dreamland commit --reason turn-complete` runs, `git status --porcelain` shows pending changes, and `.dreamland/last-test-result.json` records `"status": "fail"` for a `head_sha` matching the current `HEAD`
- **THEN** no `git add` or `git commit` is run, the pending changes remain uncommitted, and `dreamland commit` exits with the blocking code (2)

#### Scenario: Stale test-result record does not gate a commit

- **WHEN** `dreamland commit --reason turn-complete` runs and `.dreamland/last-test-result.json` records a `head_sha` that does not match the current `HEAD`
- **THEN** the gating step in behavior item 3 does not apply, and the commit proceeds normally per the other scenarios

#### Scenario: Commit refused and routed to iktomi when a configured test command recorded no result

- **WHEN** `dreamland commit --reason turn-complete` runs, `cfg.TestCommand` is non-empty, and no `.dreamland/last-test-result.json` file exists at all
- **THEN** no `git add` or `git commit` is run, `dreamland commit` exits with the blocking code (2), and the error message states that a test command is configured but no result was recorded, instructing that this routes to `iktomi` for investigation

#### Scenario: Missing test-result record does not gate a commit when no test command is configured

- **WHEN** `dreamland commit --reason turn-complete` runs, `cfg.TestCommand` is empty (or `.dreamland.json` is absent), and no `.dreamland/last-test-result.json` file exists
- **THEN** the gating step in behavior item 3 does not apply, and the commit proceeds normally

#### Scenario: Commit subject and git author never diverge

- **WHEN** `dreamland coauthor` most recently set `git config --local user.name` to `"phobetor"` and `dreamland commit --reason turn-complete` subsequently runs via the `Stop` hook, whose payload carries no sub-agent identity of its own
- **THEN** the resulting commit's subject reads `chore: turn-complete checkpoint (phobetor)` and its author and committer name are `phobetor`, not a value independently re-resolved from the `Stop` hook's own payload

#### Scenario: Commit author is pinned regardless of a concurrent git config change

- **WHEN** `dreamland commit` resolves agent identity `X` and `git config --local user.name` currently holds a different value
- **THEN** the commit's author and committer name/email are set explicitly to `X` (`git -c user.name=X -c user.email=...`), matching the `(X)` in the subject

#### Scenario: Handoff identity comes from the SubagentStop payload

- **WHEN** `dreamland commit --reason handoff --hook` runs with payload `{"agent_type": "morpheus"}` and `git config --local user.name` currently reads `janus`
- **THEN** the commit's subject is `chore: handoff checkpoint (morpheus)` and its author and committer are `morpheus`

#### Scenario: Handoff commit created when Janus hands off to another agent

- **WHEN** a sub-agent's turn ends via `SubagentStop` and `git status --porcelain` shows pending changes
- **THEN** `dreamland commit --reason handoff` stages and commits those changes with subject `chore: handoff checkpoint (<outgoing-agent-name>)` before Janus regains control

#### Scenario: Claude Code settings.json binds test-and-commit to Stop and commit --hook to SubagentStop

- **WHEN** `dreamland init` completes with "Claude Code" selected
- **THEN** `.claude/settings.json` contains `dreamland test-and-commit --reason turn-complete` under the `Stop` event key, and does not contain a separate `dreamland test` entry followed by `dreamland commit --reason turn-complete`
- **AND** contains `dreamland commit --reason handoff --hook` under the `SubagentStop` event key

#### Scenario: commit skips silently outside a git repository

- **WHEN** `dreamland commit` runs in a directory with no git repository at or above it
- **THEN** the command exits 0 without error and does not attempt `git status`, `git add`, or `git commit`

#### Scenario: A genuine git failure during a turn-complete commit blocks the lifecycle event

- **WHEN** `dreamland commit --reason turn-complete` has pending changes to commit (and is not refused by the test-gating step) but `git add -A` or `git commit` itself fails
- **THEN** the command exits with the blocking code (2), not the advisory code

#### Scenario: A transient git failure during handoff does not block the sub-agent from stopping

- **WHEN** `dreamland commit --reason handoff` is invoked and the underlying `git commit` call fails for a reason other than "nothing to commit"
- **THEN** the command exits with a non-blocking status (exit code 1, not 2) and the sub-agent's `SubagentStop` event completes, with the failure surfaced to the user

#### Scenario: A concurrent commit that already covered the changes is a benign no-op

- **WHEN** `git commit` reports "nothing to commit" because another session committed the same changes between the status check and the commit
- **THEN** the command exits 0 for both `turn-complete` and `handoff`

### Requirement: Claude Code sub-agent identity resolution reads the SubagentStop payload's agent_type field

`dreamland coauthor` and `dreamland commit` (the latter when invoked with `--hook`), when invoked without an explicit `--agent-name` flag, SHALL attempt to resolve the acting agent's identity from the hook payload's top-level `agent_type` field on Claude Code's `SubagentStop` event, in addition to `tool_input.subagent_type` on `PreToolUse`/`PostToolUse` payloads for the `Task`/`Agent` tool call. Claude Code's `SubagentStop` hook input documents `agent_type` as a standard field (alongside `agent_id`, `agent_transcript_path`, and `last_assistant_message`) — an earlier assumption in this codebase that `SubagentStop` carries no sub-agent identifier at all was incorrect and is corrected by this requirement, and by `agentidentity.FromPayload`'s existing implementation.

#### Scenario: SubagentStop payload's agent_type resolves identity without --agent-name

- **WHEN** `dreamland coauthor` runs without `--agent-name` and receives a `SubagentStop` payload of `{"agent_type": "nyx", ...}` on stdin (via `--hook`)
- **THEN** `git config user.name` is set to `"nyx"`, resolved from the payload's `agent_type` field, not the `janus` fallback

### Requirement: version-bump minor also fires once per new OpenSpec change, not only once per branch

`version-bump` (minor/major, no `--patch`) SHALL fire once per branch at session start (see the base `dev-workflow-hooks` capability) AND once per new OpenSpec change, independent of the branch trigger, so that starting a new change bumps the minor version even when it isn't also the first session on a new branch (multiple changes proposed on one long-lived branch each get their own minor bump). This SHALL use a change-scoped marker (`.dreamland/change-bumps`, keyed by change slug, analogous to the existing `.dreamland/branch-bumps`) so a given change is only minor-bumped once even across multiple sessions.

On Claude Code, this trigger SHALL be a real `PostToolUse` hook (matcher: `Bash`) that inspects the completed command for the shape `openspec new change <slug>` (or `openspec change create <slug>`) and, on a match, runs `dreamland version-bump --change <slug>` itself — the bump does not depend on the invoking agent remembering to run it. On platforms without an equivalent post-tool-completion hook, `phantasos`'s own instructions SHALL direct it to run `dreamland version-bump --change <slug>` immediately after successfully running `openspec new change`, as a documented fallback for that platform (same "agent-driven invocation" pattern already used elsewhere in this capability for platforms lacking a given hook event).

#### Scenario: New change on an existing branch bumps minor

- **WHEN** `openspec new change <slug>` succeeds on a branch that has already had its session-start minor bump for this session
- **THEN** `dreamland version-bump --change <slug>` still runs, and `.dreamland/change-bumps` gains an entry for `<slug>`

#### Scenario: Re-running propose on the same change does not double-bump

- **WHEN** `dreamland version-bump --change <slug>` runs again for a change slug already present in `.dreamland/change-bumps`
- **THEN** it does not bump the minor version again for that change

#### Scenario: Claude Code PostToolUse hook triggers the change-scoped bump without agent involvement

- **WHEN** a `Bash` tool call whose command matches `openspec new change <slug>` completes successfully under Claude Code
- **THEN** the `PostToolUse` hook runs `dreamland version-bump --change <slug>` itself, regardless of whether `phantasos`'s own instructions also mention doing so

#### Scenario: Platform without a post-tool-completion hook relies on phantasos's instructions

- **WHEN** `dreamland init` completes for a platform with no `PostToolUse`-equivalent event
- **THEN** no automatic change-scoped bump hook is installed, and `phantasos`'s instructions document running `dreamland version-bump --change <slug>` manually after `openspec new change` succeeds

### Requirement: Baku triggers a major version bump when the change is marked breaking

`baku`'s instructions SHALL direct it, before confirming closure with Janus, to check whether the change's `proposal.md` marks any item **BREAKING**, and if so, to run `dreamland version-bump --breaking` (major bump) prior to finalizing — rather than leaving the change to accumulate only patch/minor bumps despite being a breaking change.

#### Scenario: Breaking change triggers a major bump before closure

- **WHEN** `baku` finalizes a change whose `proposal.md` contains at least one **BREAKING**-marked item
- **THEN** it runs `dreamland version-bump --breaking` before confirming closure with Janus

#### Scenario: Non-breaking change does not trigger a major bump

- **WHEN** `baku` finalizes a change whose `proposal.md` contains no **BREAKING** marker
- **THEN** it does not run `dreamland version-bump --breaking`

### Requirement: dreamland test-and-commit runs test then commit in one process to remove the Stop-hook race

`dreamland` SHALL expose `dreamland test-and-commit --reason <turn-complete|handoff>`, which runs the same logic as `dreamland test` followed by the same logic as `dreamland commit --reason <reason>`, within a single process and a single invocation. If the test step returns an error, that error SHALL be written to stderr and execution SHALL still proceed to the commit step (which, per the modified `commit` requirement above, will itself observe the just-written `.dreamland/last-test-result.json` and refuse to commit when appropriate) — `test-and-commit`'s own exit code and final error SHALL be those of the commit step.

This exists because a `Stop`-equivalent hook binding that runs `dreamland test` and `dreamland commit --reason turn-complete` as two separately hook-bound commands does not have a documented guarantee that the first finishes writing `.dreamland/last-test-result.json` before the second reads it — confirmed empirically, not merely theoretically, when this exact pair produced a false "no result recorded" refusal on an ordinary turn where `test` had in fact run and was about to record a failure. Running both steps as one process removes the race structurally: there is no second process for a hook runner to schedule concurrently against.

#### Scenario: A failing test step still leads to commit's own refusal, not a separate ambiguous failure

- **WHEN** `dreamland test-and-commit --reason turn-complete` runs, source files changed, and the configured `test_command` fails
- **THEN** the test failure is written to `.dreamland/last-test-result.json` and to stderr, and the subsequent commit step observes that record for the current `HEAD` and refuses to commit, exactly as `dreamland commit --reason turn-complete` would given that same record
- **AND** `dreamland test-and-commit` exits with the blocking code (2)

#### Scenario: A passing test step allows the commit step to proceed

- **WHEN** `dreamland test-and-commit --reason turn-complete` runs, source files changed, and the configured `test_command` succeeds
- **THEN** `.dreamland/last-test-result.json` records `"status": "pass"` for the current `HEAD`, and the commit step stages and commits pending changes exactly as `dreamland commit --reason turn-complete` would given that record

### Requirement: Hook-invoked commands distinguish blocking failures from advisory skips via exit code

`dreamland` SHALL exit with code 2 — the only code Claude Code's hook lifecycle treats as blocking a `PreToolUse`/`Stop`/`SubagentStop` transition — when `coauthor`, `commit`, or `test` (the three commands the audit identified as having a genuine failure mode with real consequences) encounter a genuine, unrecoverable failure, as detailed in each command's own requirement above. Every other error from any other command, and every already-existing skip condition on these three commands, SHALL continue to exit 1 (advisory, unchanged default) or 0 (silent skip) exactly as before this change — this requirement narrows to the three named commands, not a blanket change to every command's error handling.

This is implemented as a single dispatch point in `Execute()` (the CLI's top-level error handler): a command's `RunE` marks a genuine failure by wrapping it before returning, and `Execute()` checks for that marker once, mapping it to exit code 2; an unmarked error keeps exiting 1 exactly as every command's errors do today.

#### Scenario: A blocking failure exits with code 2

- **WHEN** `dreamland coauthor`, `dreamland commit`, `dreamland test` (on an actual test-command failure), or `dreamland test-and-commit` (which inherits its exit code from the commit step, per its own requirement) encounters the genuine failure condition described in its own requirement
- **THEN** the process exits with code 2

#### Scenario: A skip condition still exits 0

- **WHEN** any of `coauthor`, `commit`, or `test` encounters one of its documented skip conditions (no git repository, clean working tree, nothing configured, no matching source files changed)
- **THEN** the process exits 0, unchanged from prior behavior

#### Scenario: Commands outside this change's scope are unaffected

- **WHEN** `dreamland version-bump`, `dreamland transition-log`, or `dreamland telemetry write` returns any error
- **THEN** the process exits with code 1, exactly as it did before this change

### Requirement: dreamland detects and reports when its own binary is stale relative to its build source

`dreamland` SHALL expose a `dreamland version` subcommand that prints the git commit SHA the running binary was built from, sourced from a `buildCommit` value embeddable at build time via `-ldflags "-X dreamland/cmd.buildCommit=<sha>"`. When built without this flag (e.g. a plain `go build -o dreamland .`), `buildCommit` SHALL default to `"unknown"`.

On every invocation, before dispatching to the requested subcommand, `dreamland` SHALL check whether the current working directory is inside dreamland's own source repository (a `go.mod` at the repository root declaring `module dreamland`) and, if so, whether `buildCommit` (when not `"unknown"`) matches that repository's current `git rev-parse HEAD`. On a mismatch, `dreamland` SHALL print a single diagnostic line to stderr naming both SHAs and directing the operator to rebuild and reinstall, and SHALL NOT alter the invoked command's exit code or behavior in any other way — this check is advisory, intended to surface exactly the kind of silent binary/source drift that let a prior identity-resolution bug persist unnoticed, not a new enforcement gate in its own right.

This check SHALL NOT run, and SHALL NOT print anything, when invoked from any repository other than dreamland's own source tree — comparing an embedded dreamland build SHA against an unrelated consumer project's `HEAD` would be meaningless.

#### Scenario: dreamland version reports the embedded build SHA

- **WHEN** `dreamland` was built with `-ldflags "-X dreamland/cmd.buildCommit=<sha>"` and `dreamland version` is run
- **THEN** the output includes `<sha>`

#### Scenario: dreamland version reports unknown when built without ldflags

- **WHEN** `dreamland` was built with a plain `go build -o dreamland .` (no `-ldflags`) and `dreamland version` is run
- **THEN** the output includes `"unknown"` in place of a build SHA

#### Scenario: Stale binary warning fires inside dreamland's own source repository

- **WHEN** any `dreamland` command is invoked with a working directory inside a git repository whose root `go.mod` declares `module dreamland`, the binary's embedded `buildCommit` is not `"unknown"`, and it does not match that repository's current `git rev-parse HEAD`
- **THEN** a diagnostic naming both SHAs is printed to stderr, and the invoked command's own exit code and behavior are otherwise unaffected

#### Scenario: No warning outside dreamland's own source repository

- **WHEN** any `dreamland` command is invoked with a working directory inside a consumer project (a repository whose `go.mod`, if any, does not declare `module dreamland`)
- **THEN** no build-freshness diagnostic is printed, regardless of the embedded `buildCommit` value
