## Context

Dreamland is a Go CLI (`cobra` + `huh`) that scaffolds and drives a spec-driven AI development workflow. The `init` wizard today collects tool/language/command settings and writes `.dreamland.json`. Once initialized, there is no automation: no agents to execute workflow steps, no hooks to enforce lifecycle events. Developers who want agents or hooks must configure them manually and differently per platform.

The four dreamland lifecycle commands bind to two event types: session-start (fires once when a new agent session begins) and end-of-turn (fires after all tool calls complete for a turn). Verified event names per platform:

| Platform | Session-start | End-of-turn |
| --- | --- | --- |
| Claude Code | `SessionStart` | `Stop` |
| Codex CLI | `SessionStart` | `Stop` |
| Cursor | `sessionStart` | `stop` |
| Kiro CLI | `agentSpawn` | `stop` |
| Antigravity | stub (undocumented) | `PostTurnHook` |
| GitHub Copilot | stub | stub |

Conditional logic (should this command act?) lives inside the Go command, not in platform-specific configuration.

**Kiro note**: Kiro has two hook systems. The IDE visual hook system (`.kiro/hooks/` YAML with `trigger`/`instructions`) targets agent-prompt automation. The CLI agent config hook system (JSON with `agentSpawn`/`userPromptSubmit`/`preToolUse`/`postToolUse`/`stop` events) targets shell commands and is what dreamland uses. The CLI schema is very close to Claude Code's.

## Goals / Non-Goals

**Goals:**

- Embed all agent templates in the binary (zero disk reads at install time)
- Install five agent definition files per coding tool after `dreamland init`, in the platform's native format
- Add four lifecycle subcommands to the `dreamland` binary; platform hook bindings invoke these commands directly — no OS-dependent shell scripts
- Add a repo-root step to the `init` wizard (new step 1, defaults to detected git root)
- Expand coding tool list to include Codex CLI and Cursor
- `init` writes language-appropriate defaults for version-bump dispatch into `.dreamland.json`

**Non-Goals:**

- Runtime execution of agents (dreamland only scaffolds them)
- Updating existing agent files when `dreamland init` is re-run without `--force`
- Generating agent prompts dynamically from project state
- Full Antigravity plugin lifecycle management
- Supporting Kiro IDE visual hooks (`.kiro/hooks/` YAML) — those are agent-prompt hooks, not shell-command hooks

## Decisions

### 1. Hook logic lives in Go; bindings call `dreamland <command>`

Rather than scaffolding shell scripts (which are OS-dependent and hard to test), the four lifecycle operations are implemented as cobra subcommands inside the `dreamland` binary. Platform hook binding files register these commands as the shell command to invoke at the end-of-turn event.

Example Claude Code binding entry:

```json
{
  "hooks": {
    "SessionStart": [
      {"command": "dreamland version-bump"},
      {"command": "dreamland coauthor"}
    ],
    "Stop": [
      {"command": "dreamland version-bump --patch"},
      {"command": "dreamland transition-log"},
      {"command": "dreamland test"}
    ]
  }
}
```

Because all platforms invoke a shell command at the end-of-turn event, the binding files differ only in the event key name — not in the command strings. This gives maximum consistency across platforms.

**Alternative considered**: Shell scripts in `.dreamland/hooks/`. Rejected — OS-dependent, harder to test in Go, require chmod handling, and create a second codebase to maintain alongside the Go binary.

### 2. Go `embed` FS for agent templates only

Agent templates live under `internal/scaffold/templates/agents/` and are embedded with `//go:embed all:templates`. Hook binding files are small JSON/YAML snippets also embedded under `internal/scaffold/templates/hooks/bindings/`. No hook scripts are embedded or written to disk.

```text
internal/scaffold/templates/
  agents/
    claude-code/       orchestrator.md, spec-writer.md, implementer.md, tester.md, pr-closer.md
    codex/             orchestrator.toml, spec-writer.toml, implementer.toml, tester.toml, pr-closer.toml
    cursor/            orchestrator.mdc, spec-writer.mdc, implementer.mdc, tester.mdc, pr-closer.mdc
    kiro/              orchestrator.md, spec-writer.md, implementer.md, tester.md, pr-closer.md
    antigravity/       orchestrator/SKILL.md, spec-writer/SKILL.md, implementer/SKILL.md, tester/SKILL.md, pr-closer/SKILL.md
    github-copilot/    orchestrator.agent.md, spec-writer.agent.md, implementer.agent.md, tester.agent.md, pr-closer.agent.md
  hooks/
    bindings/
      claude-code/settings-patch.json
      codex/hooks.json
      cursor/hooks.json
      kiro/agent-patch.json
      antigravity/hooks.json
      github-copilot/hooks-manifest.json
```

### 3. No-tag baseline: treat absent semver tags as v0.0.0

When `git describe --tags --abbrev=0 --match "v[0-9]*"` exits non-zero (no semver tags exist in the repo), `dreamland version-bump` treats the current state as `v0.0.0`:

- Minor bump → creates `v0.1.0`
- Major bump → creates `v1.0.0`
- Patch bump → creates `v0.0.1`

For Go, this creates the first annotated tag. For Node/Rust/Python, it passes the resolved target version string to the tool (e.g., `npm version v0.1.0`). This ensures `version-bump` is safe to run on a fresh repo.

### 4. Branch-bumps file format

`.dreamland/branch-bumps` is a JSON object keyed by branch name:

```json
{
  "feature-xyz": {
    "version": "v1.2.0",
    "initialized_at": "2026-06-23T12:00:00Z"
  }
}
```

Each entry records the version assigned when the branch was first initialized and the timestamp. The object is extensible — future fields (`tokens`, `model`, agent config) will be added to each entry value without a schema change. Reads and writes use the atomic temp-file rename strategy.

### 5. Four new cobra subcommands

```text
dreamland version-bump [--major | --minor | --patch] [--version <semver>] [--breaking]
dreamland coauthor
dreamland transition-log
dreamland test
```

`version-bump` and `coauthor` fire at **session start** (`SessionStart` / `agentSpawn`). `transition-log` and `test` fire at **session end** (`Stop` / `stop`).

Each reads `.dreamland.json` for context (language, test command, model info, etc.) and exits 0 on success, non-zero on failure.

### 6. `version-bump` has two modes: per-branch (minor/major) and per-change (patch)

Minor and major version bumps are a **per-branch** operation — they happen exactly once when a branch is first initialized, not on every session start. Patch bumps are **per-code-change**, triggered at the end of every turn.

**Binding layout:**

| Event | Command | Behavior |
| --- | --- | --- |
| `SessionStart` | `dreamland version-bump` | Checks branch marker; bumps minor/major on first session of new branch; skips on subsequent sessions |
| `Stop` | `dreamland version-bump --patch` | Bumps patch only if code changed since last tag; skips otherwise |

**Branch marker (idempotency for minor/major):**
When `dreamland version-bump` performs a minor or major bump, it writes an entry to `.dreamland/branch-bumps` as a JSON object keyed by branch name (see Decision 4 for the format). On subsequent `SessionStart` calls, if the current branch key already exists in this file, the minor/major bump is skipped. This ensures exactly one minor/major bump per branch regardless of how many sessions are opened.

**Bump level selection at `SessionStart`:**

- If current branch is NOT in `.dreamland/branch-bumps`: bump `--minor` by default, `--major` if `--breaking` is passed.
- If current branch IS already in `.dreamland/branch-bumps`: exit 0 silently.
- `--version v1.2.3`, `--major`, `--minor`, `--patch` flags override auto-detection entirely.

**No-upstream fallback:**
If the current branch has no remote upstream (`@{u}` is unset), treat the branch as new (not yet pushed). After bumping, push the branch with `git push --set-upstream origin <branch>`. The marker file prevents re-triggering on the next session.

**Language dispatch** (`version_bump_command` from `.dreamland.json`):

| Language | Default `version_bump_command` | How level is passed |
| --- | --- | --- |
| Go | _(empty)_ | dreamland creates annotated git tag directly |
| Node/TypeScript | `npm version` | `npm version patch\|minor\|major` |
| Rust | `cargo bump` | `cargo bump patch\|minor\|major` |
| Python | `bump-my-version bump` | `bump-my-version bump patch\|minor\|major` |

For the `--patch` mode at `Stop`: check `git diff <last-tag>..HEAD` first; if empty, exit 0 silently.

If `cargo bump` is not installed, print an install hint and exit non-zero rather than silencing the error.

### 7. `test` checks for source-file changes before running

`dreamland test` reads `test_command` from `.dreamland.json` and the configured language, then inspects `git status --porcelain` for uncommitted changes to source files with extensions matching the language:

| Language | Source extensions checked |
| --- | --- |
| Go | `.go` |
| Node/TypeScript | `.ts`, `.tsx`, `.js`, `.jsx`, `.mts`, `.cts` |
| Rust | `.rs` |
| Python | `.py` |

If matching files have changes, it runs `test_command` and forwards the exit code. If no matching files changed, it exits 0 silently.

### 8. `coauthor` sets agent git identity and wires a prepare-commit-msg hook

`dreamland coauthor` runs at session start and does two things:

**a. Set agent git identity (local scope):**

Agent identity is constructed at runtime:

- **AgentName**: read from the platform's current-agent env var (e.g., `CLAUDE_AGENT_ID` for Claude Code); falls back to the coding tool name from `.dreamland.json`.
- **AgentEmail**: `<cleaned-agent-name><email-suffix>`. Suffix defaults to `@github.com`, stored in `.dreamland.json` as `email_suffix`, configurable via `dreamland init --email-suffix`. Cleaning: lowercase → spaces and underscores to `-` → strip `[^a-z0-9.\-]` → trim leading/trailing `-` and `.`.

Example: agent `"spec-writer"`, suffix `@github.com` → `spec-writer@github.com`.

`git config --local user.name` = AgentName. `git config --local user.email` = AgentEmail.

**b. Install a `prepare-commit-msg` git hook:**

Writes (or updates) `.git/hooks/prepare-commit-msg` as a minimal shell wrapper:

```sh
#!/bin/sh
dreamland coauthor --trailer "$1" "$2" "$3"
```

The `--trailer` mode reads `$1` (commit message file path), constructs model identity from `.dreamland.json`, and appends to the file if no matching trailer is already present:

```text
Co-authored-by: <model-name> <model-email>
```

Where:

- **model-name**: the name portion of `model_id` (text before the first space); e.g., `"claude-sonnet-4-6"` from `"claude-sonnet-4-6 temperature=1.0"`.
- **model-email**: cleaned model-name + `email_suffix`; e.g., `"claude-sonnet-4-6@github.com"`.

All logic is in Go — no `jq` or other shell tools. The hook is idempotent: if `.git/hooks/prepare-commit-msg` already contains `dreamland coauthor --trailer`, the file is left unchanged.

### 9. `transition-log` appends a simple timestamped line

`dreamland transition-log` appends one line to `.dreamland/transition.log`:

```text
<ISO-8601 timestamp> [<session-id>] turn complete
```

`session-id` is sourced from the `SESSION_ID` environment variable if set by the platform (Claude Code, Codex, Cursor all pass session metadata to hook commands via stdin or env); otherwise a random short ID is generated.

### 10. Binding files map two events: session-start and end-of-turn

| Command | Session-start | End-of-turn |
| --- | --- | --- |
| `dreamland version-bump` | ✓ (minor/major, once per branch) | ✓ (`--patch`, per code change) |
| `dreamland coauthor` | ✓ | — |
| `dreamland transition-log` | — | ✓ |
| `dreamland test` | — | ✓ |

Platform event name mapping (verified against each platform's docs):

| Platform | Session-start event | End-of-turn event | Notes |
| --- | --- | --- | --- |
| Claude Code | `SessionStart` | `Stop` | Fires on session begin/resume; all modes |
| Codex CLI | `SessionStart` | `Stop` | PascalCase; all interaction modes |
| Cursor | `sessionStart` | `stop` | camelCase; fires in agent, ask, and edit modes |
| Kiro CLI | `agentSpawn` | `stop` | Agent mode only by platform design |
| Antigravity | stub | `PostTurnHook` | Session-start undocumented; end-of-turn confirmed |
| GitHub Copilot | stub | stub | No public shell-command hook API |

`SessionStart` / `sessionStart` / `agentSpawn` are real, documented events — not per-turn events. Kiro's `agentSpawn` only fires in agent mode (inherent to Kiro's design as an agent-first IDE).

For Antigravity, the session-start hook is not yet publicly documented; the binding file is marked `"_preview": true` and the session-start entry remains a stub. End-of-turn uses `PostTurnHook` (the closest documented equivalent to `Stop`).

GitHub Copilot has no public shell-command hook API for either agent or general session modes. The stub placeholder is the correct posture. Do not use prompt injection as a substitute for a real lifecycle binding.

Command strings (`"command": "dreamland ..."`) are identical across all platforms; only the JSON event key names differ.

### 11. JSON config merges are atomic via temp-file rename

For platforms that require merging into an existing config file (Claude Code's `settings.json`, Codex's `hooks.json`, Cursor's `hooks.json`, Kiro's `agent.json`): read the existing file → merge in memory → write the result to a temp file in the same directory → `os.Rename(tempFile, target)`. On POSIX, `rename(2)` is a single atomic syscall — either the swap completes or the original is untouched. The original file is never modified directly, so a mid-write crash cannot corrupt it.

### 12. Kiro agent files use `inclusion: always` frontmatter

Kiro steering documents with `inclusion: always` are loaded in every session, making the role context deterministic. Without frontmatter, Kiro defaults to `auto` (model-decided), which is non-deterministic for role-defining instructions.

### 13. Repo root step defaults to detected git root

The new step 1 pre-fills the detected git root (result of `config.FindRepoRoot(cwd)`), not `.` (the current directory). These differ when the user runs `dreamland init` from a subdirectory.

## Risks / Trade-offs

- **`dreamland` must be in PATH when hooks fire** → The binary is typically installed globally. If the user installed it only locally, hooks will fail. Mitigation: print a reminder at the end of `dreamland init` to ensure the binary is on PATH.
- **Antigravity API in preview** → Hook schema may change. Binding file is a separate embedded asset; update in a patch release.
- **Kiro CLI agent config filename** → Assumed `.kiro/agent.json`; confirm against official Kiro CLI docs before implementing task 4.4.
- **GitHub Copilot hook schema unknown** → Stub only; no commands wired until schema is published.
- **`cargo bump` is a third-party plugin** → Not installed by default with Rust. `dreamland version-bump` should detect absence and print a helpful message rather than erroring silently.
- **`prepare-commit-msg` hook idempotency** → `dreamland coauthor` must not write a duplicate trailer. The `--trailer` mode checks for `Co-authored-by: <model-name>` before appending.
- **Email suffix default** → `email_suffix` defaults to `"@github.com"` when absent from `.dreamland.json`. This produces addresses like `claude-sonnet-4-6@github.com` which GitHub recognises for attribution. Users who prefer a different domain set `--email-suffix` at `init` time.

## Migration Plan

1. `internal/config.Config` gains `RepoRoot`, `VersionBumpCommand`, `ModelID`, and `EmailSuffix` fields — backward compatible (zero values fall back to defaults). `AgentName` and `AgentEmail` are derived at runtime from env vars + config; they are not stored fields.
2. `dreamland init` re-run on an existing project: wizard shows new step 1 and expanded tool list; `scaffold.Install` skips agent files that already exist; hook binding merge is additive.
3. The four new subcommands are additive — no existing commands change.
4. `dreamland coauthor` installs `.git/hooks/prepare-commit-msg` idempotently — checks before writing.

## Open Questions

- Confirm the exact filename for Kiro CLI agent config (assumed `.kiro/agent.json`).
- Should `dreamland version-bump` default to `--patch` when no level flag is given, or require an explicit flag? Current assumption: `--patch`.
- Python version bump: `bump-my-version` vs `bumpuv` — both are widely used in 2026. Consider making the default configurable rather than hardcoding one tool.
