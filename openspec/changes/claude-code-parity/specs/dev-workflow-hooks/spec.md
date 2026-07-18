## MODIFIED Requirements

### Requirement: dreamland commit auto-commits pending changes on turn completion and agent handoff

A new lifecycle command, `dreamland commit --reason <turn-complete|handoff>`, SHALL run at end-of-turn and handoff-complete events so that every agent turn and every agent-to-agent transition is captured as its own commit, even if the agent itself never ran `git commit`.

Behavior:

1. Inspect `git status --porcelain`. If there are no staged or unstaged changes, exit 0 silently — no commit is created.
2. Otherwise, stage all changes (`git add -A`) and run `git commit -m "chore: <reason> checkpoint (<agent-name>)"`, where `<agent-name>` is resolved in this order: (a) an explicit `--agent-name <name>` flag, if provided, used directly with no further lookup; otherwise (b) the exact same precedence `dreamland coauthor`'s default mode uses: the platform's current-agent env var (e.g. `CLAUDE_AGENT_ID`) or the coding-tool-name fallback from `resolveAgentName`, overridden by `agent_type` read from a hook JSON payload on stdin if one arrives within the same short read timeout `coauthor` uses (`agentNameFromHookPayload`). `dreamland commit` and `dreamland coauthor` SHALL both accept `--agent-name` and SHALL use the identical (b) resolution function when it's absent, so `<agent-name>` in the commit subject always matches the `git config user.name` value the same hook invocation's `coauthor` call would set — the two SHALL NOT be allowed to diverge (e.g. one honoring the hook-payload `agent_type` while the other only reads env vars/coding-tool name).
3. Because this shells out to `git commit`, the already-installed `prepare-commit-msg` hook fires normally and appends the Co-authored-by trailer and token-usage report (see the modified `coauthor` requirement) to the commit message — `dreamland commit` does not duplicate that logic.

This resolution is identical on every platform: whichever hook binding delivers the `agent_type`-bearing payload (Claude Code's `SubagentStop`, GitHub Copilot's `SubagentStop`/`agentStop`, etc.) to `dreamland commit`'s stdin, `<agent-name>` reflects the specific dreamland agent (e.g. `nyx`, `morpheus`), not just the coding tool name — making commit subjects, git author identity, and the `Tokens:` trailer directly comparable across platforms for the same agent role.

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

#### Scenario: Commit subject uses the hook-payload agent_type over the generic coding-tool name

- **WHEN** `dreamland commit --reason handoff` runs with a hook JSON payload on stdin containing `"agent_type": "nyx"`
- **THEN** the commit subject is `chore: handoff checkpoint (nyx)`, not `chore: handoff checkpoint (Claude Code)` or any other coding-tool-name fallback

#### Scenario: Commit author and commit subject agree on agent identity for the same hand-off

- **WHEN** the same `SubagentStop` hook invocation runs both `dreamland coauthor` and `dreamland commit --reason handoff` with the same `agent_type` in their respective stdin payloads
- **THEN** the resulting commit's author name (set by `coauthor`) and its subject's `<agent-name>` (set by `commit`) are the same value

#### Scenario: An explicit --agent-name flag takes precedence over stdin/env resolution

- **WHEN** `dreamland commit --reason handoff --agent-name nyx` runs with no stdin payload provided at all
- **THEN** the commit subject is `chore: handoff checkpoint (nyx)`, proving the flag path doesn't depend on or attempt the stdin read

### Requirement: coauthor sets agent identity and installs prepare-commit-msg hook

`dreamland coauthor` SHALL run at the session-start lifecycle event and perform two actions:

**a. Set agent git identity (repository-local scope):**

AgentName is read from an explicit `--agent-name` flag if provided; otherwise from the platform's current-agent env var at runtime (e.g., `CLAUDE_AGENT_ID`); otherwise `janus` when `.dreamland.json` has a configured `coding_tool` — janus is the router, the implicit entry role for every dreamland session before any specialist has been explicitly dispatched, so it is the correct identity for the window between session start and the first hand-off, not the raw coding-tool string. (Falls back further to `dreamland` only when `.dreamland.json` itself is unconfigured — a degenerate case distinct from "no agent dispatched yet".) AgentEmail is derived by cleaning AgentName and appending `email_suffix` from `.dreamland.json` (default `@github.com`). The coding-tool name is not lost by this fallback — it is separately, always captured in the second `Co-authored-by:` trailer described below, independent of AgentName.

Email cleaning: lowercase → replace spaces and underscores with `-` → strip characters not in `[a-z0-9.\-]` → trim leading/trailing `-` and `.`.

`git config --local user.name` is set to AgentName. `git config --local user.email` is set to AgentEmail.

This identity logic is identical for every scaffolded agent (Janus, Phantasos, Nyx, Morpheus, Phobetor, Baku, Iktomi, Zhou Gong, Hypnos, Meng Po) — none of them get special-cased behavior; only the AgentName value differs per invocation.

**b. Install a `prepare-commit-msg` git hook:**

Write (or update) `.git/hooks/prepare-commit-msg` as a minimal shell wrapper that delegates to `dreamland`:

```sh
#!/bin/sh
dreamland coauthor --trailer "$1" "$2" "$3"
```

When invoked with `--trailer`, `dreamland coauthor` reads `$1` (commit message file path) and appends, in order, if not already present:

```text
Co-authored-by: <model-name> <model-email>
Co-authored-by: <coding-tool-name> <tool-email>
Tokens: input=<n> output=<n> cached=<n> total=<n>
```

`model-name` is the name portion of `model_id` from `.dreamland.json` (text before the first space); `model-email` is the cleaned model-name plus `email_suffix`. `coding-tool-name` is `.dreamland.json`'s `coding_tool` value verbatim (e.g. `Claude Code`, `GitHub Copilot`, `Cursor`) — not truncated at the first space, since coding-tool names routinely contain one; `tool-email` is the cleaned coding-tool-name plus `email_suffix`. Both `Co-authored-by:` lines are independently idempotent — each is appended only if a line matching its own name is not already present, so a partially-annotated commit message (e.g. one that already has the model trailer from a prior hook run) gains only the missing line. This second trailer is what makes the coding platform itself — not just the model — directly visible and greppable in `git log` output, closing the gap where platform identity previously lived only in `.dreamland-session.json`'s `Tool` field, invisible to plain git history. All logic in Go; no shell tools required.

Immediately after both Co-authored-by trailers, `dreamland coauthor --trailer` appends a token-usage report line sourced from the current turn's telemetry snapshot (the same data `dreamland telemetry write` collects — model name and input/output/cached/total token counts):

```text
Tokens: input=<n> output=<n> cached=<n> total=<n>
```

If telemetry data is unavailable for the current turn (e.g. the platform doesn't expose usage stats, or no turn has completed yet), the `Tokens:` line is omitted and only the Co-authored-by trailers are appended — this is not a failure condition.

The hook file is written with mode 0755. If `.git/hooks/prepare-commit-msg` already contains `dreamland coauthor --trailer`, the file is left unchanged.

#### Scenario: Agent git identity set from env var at session start

- **WHEN** `dreamland coauthor` runs and the platform env var for current agent is set (e.g., `CLAUDE_AGENT_ID=hypnos`)
- **THEN** `git config --local user.name` is set to `"hypnos"` and `git config --local user.email` to `"hypnos@github.com"` (with configured suffix)

#### Scenario: Agent git identity falls back to janus, not the coding tool name

- **WHEN** `dreamland coauthor` runs, no platform agent env var is set, and `.dreamland.json` has a configured `coding_tool` (e.g. `"Claude Code"` or `"GitHub Copilot"`)
- **THEN** `git config --local user.name` is set to `"janus"` and `git config --local user.email` to `"janus@github.com"` (with configured suffix), regardless of which coding tool is configured — the same fallback applies uniformly across platforms, since `resolveAgentName` is shared code

#### Scenario: Agent git identity falls back to dreamland only when unconfigured

- **WHEN** `dreamland coauthor` runs, no platform agent env var is set, and `.dreamland.json` is missing or has no `coding_tool` value
- **THEN** `git config --local user.name` is set to `"dreamland"`

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

#### Scenario: Coding-tool Co-authored-by trailer appended alongside the model trailer

- **WHEN** `dreamland coauthor --trailer <file>` runs with `.dreamland.json` `coding_tool` set to `"GitHub Copilot"` and the commit message contains neither `Co-authored-by:` line
- **THEN** the file gains both `Co-authored-by: claude-sonnet-5 <claude-sonnet-5@github.com>` (or whatever `model_id` resolves to) and `Co-authored-by: GitHub Copilot <github-copilot@github.com>`, in that order, with the coding-tool name not truncated at its internal space

#### Scenario: Coding-tool trailer not duplicated independently of the model trailer

- **WHEN** `dreamland coauthor --trailer <file>` runs and the commit message already contains `Co-authored-by: GitHub Copilot <github-copilot@github.com>` but not yet the model trailer
- **THEN** the model `Co-authored-by:` line is appended and the coding-tool line is left unchanged, not duplicated

#### Scenario: Token usage report appended alongside both trailers

- **WHEN** `dreamland coauthor --trailer <file>` runs and telemetry data is available for the current turn
- **THEN** the commit message file gains both `Co-authored-by:` trailers and a `Tokens:` report line

#### Scenario: Token usage report omitted when telemetry is unavailable

- **WHEN** `dreamland coauthor --trailer <file>` runs and no telemetry data is available for the current turn
- **THEN** both `Co-authored-by:` trailers are still appended and the `Tokens:` line is omitted
