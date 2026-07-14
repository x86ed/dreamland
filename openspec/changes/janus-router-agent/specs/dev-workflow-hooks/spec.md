## ADDED Requirements

### Requirement: Claude Code binds identity, telemetry, and patch-version commands to the sub-agent handoff lifecycle

In addition to the existing `SessionStart`/`Stop` bindings, the scaffold installer SHALL register `dreamland coauthor` under Claude Code's `PreToolUse` event (matcher: `Task`) and `dreamland coauthor`, `dreamland telemetry write --tool claude-code`, and `dreamland version-bump --patch` under Claude Code's `SubagentStop` event, so that git identity, telemetry, and the patch version are refreshed on every hand-off to a sub-agent — every agent turn — not only once per session.

#### Scenario: Identity refreshed before a sub-agent is dispatched

- **WHEN** Janus invokes the `Task` tool to delegate to another agent
- **THEN** `dreamland coauthor` runs via the `PreToolUse` hook before the sub-agent's turn begins, updating `git config user.name`/`user.email` to the dispatched agent's identity

#### Scenario: Identity, telemetry, and patch version refreshed after a sub-agent's turn completes

- **WHEN** a sub-agent invoked by Janus finishes its turn
- **THEN** `dreamland coauthor`, `dreamland telemetry write --tool claude-code`, and `dreamland version-bump --patch` all run via the `SubagentStop` hook

#### Scenario: Claude Code settings.json contains handoff hook entries

- **WHEN** `dreamland init` completes with "Claude Code" selected
- **THEN** `.claude/settings.json` contains a `PreToolUse` entry matching `Task` that runs `dreamland coauthor`
- **AND** contains a `SubagentStop` entry that runs `dreamland coauthor`, `dreamland telemetry write --tool claude-code`, and `dreamland version-bump --patch`

### Requirement: Platforms without a sub-agent lifecycle hook rely on agent-driven invocation

For platforms where no `PreToolUse`/`SubagentStop`-equivalent hook event is documented (Codex CLI, Cursor, Kiro, Antigravity), the scaffold installer SHALL NOT add a handoff-lifecycle hook binding. Instead, this is documented as a known limitation: the Janus agent's own instructions direct it to invoke `dreamland coauthor`, `dreamland telemetry write`, and `dreamland version-bump --patch` immediately before and after each delegation (see the `janus-router-agent` capability), and the existing `SessionStart`/`Stop` bindings continue to provide a session-level fallback.

GitHub Copilot is the one exception to this limitation: rather than a global session-level binding file, it structurally declares the equivalent guarantee — `coauthor`, `telemetry-write`, `commit`, and `version-bump` — in each agent's own frontmatter (`hooks:`, see the `agent-scaffolding` capability's "GitHub Copilot frontmatter declares its subagent routing graph and hooks in the header" requirement), which is a stronger mechanism than the prose-only convention the other four platforms rely on.

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

## MODIFIED Requirements

### Requirement: coauthor sets agent identity and installs prepare-commit-msg hook

`dreamland coauthor` SHALL run at the session-start lifecycle event and perform two actions:

**a. Set agent git identity (repository-local scope):**

AgentName is read from the platform's current-agent env var at runtime (e.g., `CLAUDE_AGENT_ID`); falls back to the coding tool name in `.dreamland.json`. AgentEmail is derived by cleaning AgentName and appending `email_suffix` from `.dreamland.json` (default `@github.com`).

Email cleaning: lowercase → replace spaces and underscores with `-` → strip characters not in `[a-z0-9.\-]` → trim leading/trailing `-` and `.`.

`git config --local user.name` is set to AgentName. `git config --local user.email` is set to AgentEmail.

This identity logic is identical for every scaffolded agent (Janus, Phantasos, Nyx, Morpheus, Phobetor, Baku, Iktomi, Zhou Gong, Hypnos, Meng Po) — none of them get special-cased behavior; only the AgentName value read from the env var differs per invocation.

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

- **WHEN** `dreamland coauthor --trailer <file>` runs and the commit message does not contain a matching `Co-authored-by:` line
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
