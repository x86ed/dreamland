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

AgentName is read from the platform's current-agent env var at runtime (e.g., `CLAUDE_AGENT_ID`), a hook payload's identity field (see the `session-agent-identity` capability), or falls back to the coding tool name in `.dreamland.json`. AgentEmail is derived by cleaning AgentName and appending `email_suffix` from `.dreamland.json` (default `@github.com`).

Email cleaning: lowercase → replace spaces and underscores with `-` → strip characters not in `[a-z0-9.\-]` → trim leading/trailing `-` and `.`.

`git config --local user.name` is set to AgentName. `git config --local user.email` is set to AgentEmail.

This identity logic is identical for every scaffolded agent — every one of the ten built-in agents (Janus, Phantasos, Nyx, Morpheus, Phobetor, Baku, Iktomi, Zhou Gong, Hypnos, Meng Po) and every agent seeded via `dreamland oneiroi seed`/`revise`/`fork` (see the `oneiroi-seed-naming` capability) — none of them get special-cased behavior; only the AgentName value read from the env var/hook payload differs per invocation, and whether a candidate name is trusted is governed by `internal/agentidentity.IsRegistered`, which recognizes both the built-in ten and any name present in `.dreamland/oneiroi/registry.json`.

**b. Install a `prepare-commit-msg` git hook:**

Write (or update) `.git/hooks/prepare-commit-msg` as a minimal shell wrapper that delegates to `dreamland`:

```sh
#!/bin/sh
dreamland coauthor --trailer "$1" "$2" "$3"
```

When invoked with `--trailer`, `dreamland coauthor` reads `$1` (commit message file path) and first checks whether the message already contains a `Generated-By:` trailer. If it does, the message is treated as complete and self-describing — `dreamland coauthor --trailer` makes no changes to it at all (no `Co-authored-by:` append, no `Tokens:` append; see the `oneiroi-seed-naming` capability's self-authored-commit requirement, which is what produces this trailer). Otherwise, it constructs model identity from `.dreamland.json` and appends to the file, if no matching trailer is already present:

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

#### Scenario: Agent git identity set from env var at session start

- **WHEN** `dreamland coauthor` runs and the platform env var for current agent is set (e.g., `CLAUDE_AGENT_ID=hypnos`)
- **THEN** `git config --local user.name` is set to `"hypnos"` and `git config --local user.email` to `"hypnos@github.com"` (with configured suffix)

#### Scenario: Agent git identity falls back to coding tool name

- **WHEN** `dreamland coauthor` runs and no platform agent env var is set
- **THEN** `git config --local user.name` is set to the coding tool name from `.dreamland.json` (e.g., `"Claude Code"`) and email to `"claude-code@github.com"`

#### Scenario: Email cleaning applied to agent name

- **WHEN** AgentName is `"Spec Writer"` and `email_suffix` is `@github.com`
- **THEN** AgentEmail is `"spec-writer@github.com"`

#### Scenario: prepare-commit-msg hook installed

- **WHEN** `dreamland coauthor` runs and `.git/hooks/prepare-commit-msg` does not exist
- **THEN** the file is created with mode 0755 containing `#!/bin/sh` and `dreamland coauthor --trailer "$1" "$2" "$3"`

#### Scenario: prepare-commit-msg hook is idempotent

- **WHEN** `dreamland coauthor` runs and `.git/hooks/prepare-commit-msg` already contains the delegation line
- **THEN** the file is not modified

#### Scenario: Co-authored-by trailer appended by --trailer mode

- **WHEN** `dreamland coauthor --trailer <file>` runs and the commit message does not contain a matching `Co-authored-by:` line or a `Generated-By:` trailer
- **THEN** `Co-authored-by: <model-name> <model-email>` is appended to the file

#### Scenario: Co-authored-by trailer not duplicated

- **WHEN** `dreamland coauthor --trailer <file>` runs and the commit message already contains `Co-authored-by: <model-name>`
- **THEN** the file is not modified

#### Scenario: Token usage report appended alongside trailer

- **WHEN** `dreamland coauthor --trailer <file>` runs and telemetry data is available for the current turn
- **THEN** the commit message file gains both the `Co-authored-by:` trailer and a `Tokens:` report line

#### Scenario: Token usage report omitted when telemetry is unavailable

- **WHEN** `dreamland coauthor --trailer <file>` runs and no telemetry data is available for the current turn
- **THEN** the `Co-authored-by:` trailer is still appended and the `Tokens:` line is omitted

#### Scenario: Generated-By trailer suppresses both auto-appends

- **WHEN** `dreamland coauthor --trailer <file>` runs and the commit message already contains a `Generated-By:` trailer (e.g. `Generated-By: dreamland-oneiroi-seed`)
- **THEN** the file is left completely unchanged — no `Co-authored-by:` line and no `Tokens:` line are appended, even if telemetry data is available for the current turn

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

`dreamland test` SHALL inspect `git status --porcelain` for unstaged and staged changes to source files matching the configured language's extensions. If matching files are present, it runs `test_command` from `.dreamland.json` and exits with that command's exit code. If no matching files are found, it exits 0 silently.

Source file extensions by language:

| Language | Extensions |
| --- | --- |
| Go | `.go` |
| Node/TypeScript | `.ts`, `.tsx`, `.js`, `.jsx`, `.mts`, `.cts` |
| Rust | `.rs` |
| Python | `.py` |

#### Scenario: Tests run after source code changes

- **WHEN** `dreamland test` runs and `git status --porcelain` shows at least one file with a matching source extension
- **THEN** `test_command` from `.dreamland.json` is executed and its exit code is forwarded

#### Scenario: Tests skipped when no source changed

- **WHEN** `dreamland test` runs and no source files have changes
- **THEN** the command exits 0 without running anything

#### Scenario: Test failure propagates exit code

- **WHEN** `dreamland test` runs, source files changed, and `test_command` exits non-zero
- **THEN** `dreamland test` exits with the same non-zero code

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

In addition to the existing `SessionStart`/`Stop` bindings, the scaffold installer SHALL register `dreamland coauthor` under Claude Code's `PreToolUse` event (matcher: `Task|Agent` — the subagent-dispatch tool was renamed from `Task` to `Agent` in Claude Code v2.1.63; matching both keeps the binding correct across the VS Code extension's bundled CLI version and the standalone CLI regardless of which has updated) and `dreamland coauthor`, `dreamland telemetry write --tool claude-code`, and `dreamland version-bump --patch` under Claude Code's `SubagentStop` event, so that git identity, telemetry, and the patch version are refreshed on every hand-off to a sub-agent — every agent turn — not only once per session.

#### Scenario: Identity refreshed before a sub-agent is dispatched

- **WHEN** Janus invokes the `Task`/`Agent` tool to delegate to another agent
- **THEN** `dreamland coauthor` runs via the `PreToolUse` hook before the sub-agent's turn begins, updating `git config user.name`/`user.email` to the dispatched agent's identity

#### Scenario: Identity, telemetry, and patch version refreshed after a sub-agent's turn completes

- **WHEN** a sub-agent invoked by Janus finishes its turn
- **THEN** `dreamland coauthor`, `dreamland telemetry write --tool claude-code`, and `dreamland version-bump --patch` all run via the `SubagentStop` hook

#### Scenario: Claude Code settings.json contains handoff hook entries

- **WHEN** `dreamland init` completes with "Claude Code" selected
- **THEN** `.claude/settings.json` contains a `PreToolUse` entry matching `Task|Agent` that runs `dreamland coauthor`
- **AND** contains a `SubagentStop` entry that runs `dreamland coauthor`, `dreamland telemetry write --tool claude-code`, and `dreamland version-bump --patch`

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

1. Inspect `git status --porcelain`. If there are no staged or unstaged changes, exit 0 silently — no commit is created.
2. Otherwise, stage all changes (`git add -A`) and run `git commit -m "chore: <reason> checkpoint (<agent-name>)"`, where `<agent-name>` is the same value `dreamland coauthor` would set as `git config user.name`.
3. Because this shells out to `git commit`, the already-installed `prepare-commit-msg` hook fires normally and appends the Co-authored-by trailer and token-usage report (see the modified `coauthor` requirement) to the commit message — `dreamland commit` does not duplicate that logic.

The scaffold installer SHALL bind `dreamland commit --reason turn-complete` to Claude Code's `Stop` event (alongside the existing end-of-turn commands) and `dreamland commit --reason handoff` to Claude Code's `SubagentStop` event (alongside `dreamland coauthor` and `dreamland telemetry write --tool claude-code`, per the requirement above). On platforms without a `SubagentStop`-equivalent event, only the `Stop`-bound `--reason turn-complete` invocation applies; handoff commits on those platforms rely on the same agent-driven convention described in the requirement above for `coauthor`/`telemetry write`.

#### Scenario: Commit created when a turn completes with pending changes

- **WHEN** `dreamland commit --reason turn-complete` runs via the `Stop` hook and `git status --porcelain` shows pending changes
- **THEN** the changes are staged and committed with subject `chore: turn-complete checkpoint (<agent-name>)`

#### Scenario: No-op when a turn completes with a clean working tree

- **WHEN** `dreamland commit --reason turn-complete` runs and `git status --porcelain` is empty
- **THEN** the command exits 0 without creating a commit

#### Scenario: Handoff commit created when Janus hands off to another agent

- **WHEN** a sub-agent's turn ends via `SubagentStop` and `git status --porcelain` shows pending changes
- **THEN** `dreamland commit --reason handoff` stages and commits those changes with subject `chore: handoff checkpoint (<outgoing-agent-name>)` before Janus regains control

#### Scenario: Claude Code settings.json binds dreamland commit to Stop and SubagentStop

- **WHEN** `dreamland init` completes with "Claude Code" selected
- **THEN** `.claude/settings.json` contains `dreamland commit --reason turn-complete` under the `Stop` event key
- **AND** contains `dreamland commit --reason handoff` under the `SubagentStop` event key

### Requirement: version-bump minor also fires once per new OpenSpec change, not only once per branch

The existing `version-bump` (minor/major, no `--patch`) behavior fires once per branch at session start (see the base `dev-workflow-hooks` capability). This change adds a second, independent trigger: `phantasos`'s instructions SHALL direct it to run `dreamland version-bump` immediately after successfully running `openspec new change` (e.g. as part of `/opsx:propose`), so that starting a new change bumps the minor version even when it isn't also the first session on a new branch (multiple changes proposed on one long-lived branch each get their own minor bump). This uses a change-scoped marker (`.dreamland/change-bumps`, keyed by change slug, analogous to the existing `.dreamland/branch-bumps`) so a given change is only minor-bumped once even across multiple sessions.

#### Scenario: New change on an existing branch bumps minor

- **WHEN** `phantasos` runs `openspec new change <slug>` on a branch that has already had its session-start minor bump for this session
- **THEN** `dreamland version-bump` still runs for the new change, and `.dreamland/change-bumps` gains an entry for `<slug>`

#### Scenario: Re-running propose on the same change does not double-bump

- **WHEN** `phantasos` is invoked again for a change slug already present in `.dreamland/change-bumps`
- **THEN** it does not run `dreamland version-bump` again for that change

### Requirement: Baku triggers a major version bump when the change is marked breaking

`baku`'s instructions SHALL direct it, before confirming closure with Janus, to check whether the change's `proposal.md` marks any item **BREAKING**, and if so, to run `dreamland version-bump --breaking` (major bump) prior to finalizing — rather than leaving the change to accumulate only patch/minor bumps despite being a breaking change.

#### Scenario: Breaking change triggers a major bump before closure

- **WHEN** `baku` finalizes a change whose `proposal.md` contains at least one **BREAKING**-marked item
- **THEN** it runs `dreamland version-bump --breaking` before confirming closure with Janus

#### Scenario: Non-breaking change does not trigger a major bump

- **WHEN** `baku` finalizes a change whose `proposal.md` contains no **BREAKING** marker
- **THEN** it does not run `dreamland version-bump --breaking`

