## MODIFIED Requirements

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
