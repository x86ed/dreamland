## MODIFIED Requirements

### Requirement: coauthor sets agent identity and installs prepare-commit-msg hook

`dreamland coauthor` SHALL run at the session-start lifecycle event and perform two actions:

**a. Set agent git identity (repository-local scope):**

AgentName resolution tries, in order:

1. A hook stdin payload for the current invocation, checked for an agent identifier in whichever shape the platform actually emits:
   - GitHub Copilot: top-level `agent_type` (e.g. `"morpheus"`) on `SubagentStart`/`SubagentStop` payloads.
   - Claude Code: `tool_input.subagent_type` (e.g. `"morpheus"`) on the `PreToolUse`/`PostToolUse` payload for the `Task`/`Agent` tool call — Claude Code does not emit a top-level `agent_type` field, and `SessionStart`/`Stop`/`SubagentStop` payloads on Claude Code do not carry a sub-agent identifier at all (only `session_id`/`transcript_path`/`hook_event_name`), so this path only resolves anything on the `PreToolUse`/`PostToolUse` hook for that tool.
2. The platform's current-agent env var, if the platform sets one at runtime (no currently-supported platform does; this path exists for forward compatibility and is not exercised by Claude Code or GitHub Copilot).
3. The coding tool name in `.dreamland.json`.
4. If a hook payload resolved a candidate value (step 1) that is not one of the ten registered dreamland agent names (`janus`, `phantasos`, `nyx`, `morpheus`, `phobetor`, `baku`, `iktomi`, `zhougong`, `hypnos`, `mengpo`), that candidate is discarded — treated the same as if step 1 had resolved nothing — rather than used verbatim.
5. On Claude Code specifically, if steps 1-2 resolve nothing, AgentName is `janus`, not the coding-tool name — see the `session-agent-identity` capability for the full "no default, no unknown agent" requirement this satisfies. On every other platform, step 3 (coding-tool name) remains the fallback when steps 1-2 resolve nothing, unchanged from prior behavior.

AgentEmail is derived by cleaning AgentName and appending `email_suffix` from `.dreamland.json` (default `@github.com`).

Email cleaning: lowercase → replace spaces and underscores with `-` → strip characters not in `[a-z0-9.\-]` → trim leading/trailing `-` and `.`.

`git config --local user.name` is set to AgentName. `git config --local user.email` is set to AgentEmail.

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

#### Scenario: Claude Code identity resolved from tool_input.subagent_type

- **WHEN** `dreamland coauthor` runs via Claude Code's `PreToolUse` hook for a `Task` tool call whose payload is `{"tool_input": {"subagent_type": "phobetor", ...}}`
- **THEN** `git config --local user.name` is set to `"phobetor"`, not the generic coding-tool fallback

#### Scenario: Unrecognized resolved identity falls back to janus, not the raw value

- **WHEN** `dreamland coauthor` resolves a candidate AgentName that is not one of the ten registered dreamland agent names (e.g. a malformed or unexpected payload value)
- **THEN** AgentName falls back to `"janus"` rather than using the unrecognized value verbatim

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
