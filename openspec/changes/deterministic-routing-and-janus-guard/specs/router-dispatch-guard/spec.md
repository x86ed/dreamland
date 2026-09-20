## ADDED Requirements

### Requirement: `dreamland guard-router` blocks Janus from doing specialist work

`dreamland guard-router [--agent-name <name>]` SHALL be a `PreToolUse` hook command that reads the hook JSON payload from stdin and blocks (writes a reason to stderr and exits 2, through the existing `Blocking` mechanism in `cmd/hookexit.go`) any tool call that would do specialist work while the acting identity is `janus`. It is a separate command from `guard-artifact`: `guard-artifact` answers "does this agent own this path", `guard-router` answers "is this identity permitted to act at all beyond routing".

**Acting identity** is resolved, in order, from: (1) the `--agent-name` flag, when present (used by agent-scoped hooks, where the identity is known statically); (2) the payload's top-level `agent_type`, which Claude Code sets both inside a dispatched subagent and for a session launched with `claude --agent <name>`; (3) otherwise *unattributed* (a plain main session with no `--agent`). Resolution SHALL NOT read `git config user.name`, `.dreamland-session.json`, or any other file: those are shared mutable state that parallel sessions in the same repository overwrite, and identity for an access decision must not depend on them.

For acting identity `janus`:

- Tools `Write`, `Edit`, `MultiEdit`, and `NotebookEdit` SHALL be blocked unconditionally.
- Tools `Bash` and `PowerShell` SHALL be blocked unless the command satisfies the allowlist in the next requirement.
- All other tools (`Read`, `Grep`, `Glob`, `Agent`, `Task`, `AskUserQuestion`, etc.) SHALL be allowed.

For any other acting identity, `guard-router` SHALL exit 0 (ownership of protected paths remains `guard-artifact`'s concern). A missing, empty, or unparseable payload SHALL exit 0 with a note on stderr: the guard fails open on malformed input, matching `guard-artifact`, because a hook that blocks on a payload it cannot read would brick the session.

The blocking message SHALL name what to do next: for a file-modifying tool whose `tool_input.file_path` matches an owned path in `guard-artifact`'s ownership table, it names that owner (e.g. "this path is owned by phantasos; dispatch phantasos"); otherwise it directs the caller to run `dreamland route` and dispatch the returned agent, naming `iktomi` as the free-form default.

#### Scenario: Janus's Edit is blocked and names the owner

- **WHEN** `guard-router` receives `{"agent_type":"janus","tool_name":"Edit","tool_input":{"file_path":"openspec/changes/x/proposal.md"}}`
- **THEN** it exits 2 and stderr names `phantasos` as the agent to dispatch

#### Scenario: Janus's Write to an unowned path is blocked with the generic message

- **WHEN** `guard-router` receives `{"agent_type":"janus","tool_name":"Write","tool_input":{"file_path":"cmd/serve.go"}}`
- **THEN** it exits 2 and stderr directs the caller to `dreamland route` and names `iktomi` as the free-form default

#### Scenario: Another agent's edit is not this guard's concern

- **WHEN** `guard-router` receives `{"agent_type":"morpheus","tool_name":"Edit","tool_input":{"file_path":"cmd/serve.go"}}`
- **THEN** it exits 0

#### Scenario: Janus can still dispatch

- **WHEN** `guard-router` receives `{"agent_type":"janus","tool_name":"Agent","tool_input":{"subagent_type":"morpheus"}}`
- **THEN** it exits 0

#### Scenario: Static flag takes precedence over the payload

- **WHEN** `guard-router --agent-name janus` receives a payload with no `agent_type` and `tool_name` `Write`
- **THEN** it exits 2

#### Scenario: Identity never comes from shared git config

- **WHEN** `git config --local user.name` reads `janus` but the payload has `agent_type` `morpheus` and `--agent-name` is absent
- **THEN** `guard-router` exits 0, because the acting identity is `morpheus`

#### Scenario: Malformed payload fails open

- **WHEN** `guard-router` receives an empty stdin or non-JSON text
- **THEN** it exits 0

### Requirement: Janus's shell access is a read-only allowlist, fail-closed

Under acting identity `janus`, a `Bash` or `PowerShell` call SHALL be allowed only when all of the following hold; otherwise it is blocked. This is an allowlist, not a parse of shell text for mutations: unlike a denylist it fails closed on any command it does not recognize.

1. The command, trimmed, contains none of the characters `;` `&` `|` `<` `>` `` ` `` `$` `(` `)` `{` `}` `\` or a newline or carriage return, with one exception (item 4). A `PowerShell` call is always blocked: Janus has no `PowerShell` tool, so a call under its identity is anomalous.
2. The first two whitespace-separated tokens match one of: `openspec status`, `openspec list`, `openspec show`, `openspec validate`, `openspec instructions`, `dreamland route`, `dreamland version`, `git status`, `git log`, `git diff`, `git show`, `git rev-parse`; or the first token is `ls` or `pwd`.
3. No argument begins with `--output`, `-o`, `--ext-diff`, `--exec`, or `--no-index`.
4. Exception, so the orchestrator can pass a user's free text (which routinely contains `(`, `$`, quotes) to `dreamland route` verbatim: a single quoted-heredoc form is allowed when the first line is `dreamland route ... --stdin <<'DREAMLAND_ROUTE_EOF'` (satisfying items 2 and 3, with the `<<'DREAMLAND_ROUTE_EOF'` suffix the only permitted use of `<`), the final line of the command is exactly `DREAMLAND_ROUTE_EOF`, that terminator appears exactly once, and nothing follows it. The body between is unrestricted: a quoted heredoc delimiter makes the shell treat the body as literal text, and `dreamland route` only reads it.

`openspec new`, `openspec archive`, `dreamland coauthor`, `dreamland commit`, and every other command are therefore blocked for Janus. This narrows the precedent set by `fixed-pipeline-enforcement` (which left Bash-mediated mutation out of scope because parsing shell text is unreliable) to the one identity where an allowlist is practical: Janus has no legitimate need for arbitrary shell.

#### Scenario: Read-only routing inputs are allowed

- **WHEN** `guard-router` receives a `Bash` call from `janus` with command `openspec status --change x --json`
- **THEN** it exits 0

#### Scenario: dreamland route with a quoted heredoc is allowed

- **WHEN** `guard-router` receives a `Bash` call from `janus` whose command is `dreamland route --stdin <<'DREAMLAND_ROUTE_EOF'`, then a body line `fix foo() and $HOME; rm -rf x`, then a final line `DREAMLAND_ROUTE_EOF`
- **THEN** it exits 0

#### Scenario: A heredoc that continues after the terminator is blocked

- **WHEN** the same command is followed, after the `DREAMLAND_ROUTE_EOF` line, by another line `rm -rf x`
- **THEN** it exits 2

#### Scenario: A heredoc on any other command is blocked

- **WHEN** `guard-router` receives a `Bash` call from `janus` with first line `cat <<'DREAMLAND_ROUTE_EOF' > notes.txt`
- **THEN** it exits 2

#### Scenario: A redirect is blocked

- **WHEN** `guard-router` receives a `Bash` call from `janus` with command `git log > notes.txt`
- **THEN** it exits 2

#### Scenario: A write masquerading as a permitted program is blocked

- **WHEN** `guard-router` receives a `Bash` call from `janus` with command `git diff --output=out.patch`
- **THEN** it exits 2

#### Scenario: An unrecognized command is blocked

- **WHEN** `guard-router` receives a `Bash` call from `janus` with command `sed -i s/a/b/ cmd/serve.go`
- **THEN** it exits 2

#### Scenario: Compound commands are blocked

- **WHEN** `guard-router` receives a `Bash` call from `janus` with command `openspec list --json && rm -rf x`
- **THEN** it exits 2

### Requirement: An unattributed main session is governed by a configurable policy, default allow

When the acting identity is unattributed (no `--agent-name`, no `agent_type`), `guard-router` SHALL consult `.dreamland.json` key `main_thread_guard`: value `"allow"` (the default when the key is absent) exits 0 for every call; value `"block"` applies the same rules as for `janus`. The default is `allow` because an unattributed main session is indistinguishable from a user's ordinary Claude Code session in a dreamland repository, and blocking all edits there would make the repository unusable outside the workflow. Teams that want every edit to go through a dispatched specialist set `"block"`.

`dreamland init` SHALL NOT add a top-level `"agent"` key to `.claude/settings.json`; making every session's main thread `janus` remains an explicit per-user choice (`claude --agent janus` or a personal settings file) and is not installed by default.

#### Scenario: Default policy leaves plain sessions unrestricted

- **WHEN** `.dreamland.json` has no `main_thread_guard` key and `guard-router` receives a `Write` payload with no `agent_type` and no flag
- **THEN** it exits 0

#### Scenario: Block policy treats an unattributed main session like Janus

- **WHEN** `.dreamland.json` has `"main_thread_guard": "block"` and `guard-router` receives a `Write` payload with no `agent_type` and no flag
- **THEN** it exits 2

#### Scenario: init does not force a default agent

- **WHEN** `dreamland init` completes with "Claude Code" selected
- **THEN** `.claude/settings.json` contains no top-level `"agent"` key

### Requirement: The guard is wired at agent scope and workspace scope on Claude Code

`.claude/agents/janus.md` SHALL declare an agent-scoped `hooks.PreToolUse` entry with matcher `Write|Edit|MultiEdit|NotebookEdit|Bash|PowerShell` running `dreamland guard-router --agent-name janus`. `.claude/settings.json` (via `internal/scaffold/templates/hooks/bindings/claude-code/settings-patch.json`) SHALL register the same matcher running `dreamland guard-router` (no flag) under `PreToolUse`, so identity comes from the payload's `agent_type` and covers a session launched as `janus` even if agent-scoped hooks do not fire, and covers the `main_thread_guard` policy. Both firing for one call is harmless: the command is read-only and idempotent. `.claude/settings.json` `permissions.allow` SHALL include `Bash(dreamland route *)` so routing runs without a permission prompt.

`janus.md` SHALL carry no `hooks.Stop` block. Every entry of the previous block is redundant or dead for a pure router: `coauthor --hook --agent-name janus` (identity already defaults to `janus` at `SessionStart`), `telemetry write --tool claude-code` and `version-bump --patch` (already run by the workspace `Stop` and `SubagentStop` chains, so they would run twice per turn under `claude --agent janus`), `version-bump --minor --if-agent janus` (Janus is no longer spawned as a subagent, and per-branch minor bumps already happen at `SessionStart`), and `commit --reason handoff --agent-name janus` (under `claude --agent janus` it double-commits alongside the workspace `Stop` chain's `test-and-commit`, and it is the mechanism that stamped specialist work with Janus's name). Janus's turns remain covered by the workspace-level `Stop` chain (`telemetry write`, `test-and-commit --reason turn-complete`) and its dispatches by the workspace-level `PreToolUse` `coauthor` and `SubagentStop` chain.

The other nine agents' `hooks.Stop` blocks are unchanged by this requirement.

#### Scenario: janus.md carries the guard and no Stop hooks

- **WHEN** `dreamland init` completes with "Claude Code" selected
- **THEN** `.claude/agents/janus.md` has `hooks.PreToolUse` with matcher `Write|Edit|MultiEdit|NotebookEdit|Bash|PowerShell` running `dreamland guard-router --agent-name janus`
- **AND** it has no `hooks.Stop` entry

#### Scenario: Workspace settings carry the payload-identity guard and the route permission

- **WHEN** `dreamland init` completes with "Claude Code" selected
- **THEN** `.claude/settings.json` `hooks.PreToolUse` contains an entry with the same matcher running `dreamland guard-router`
- **AND** `permissions.allow` contains `Bash(dreamland route *)`

#### Scenario: Other agents' Stop hooks are untouched

- **WHEN** `.claude/agents/nyx.md` is installed
- **THEN** its `hooks.Stop` block is byte-identical to the block installed before this change

#### Scenario: Merge preserves user hooks

- **WHEN** `dreamland init` merges `settings-patch.json` into a `.claude/settings.json` that already has user-authored `PreToolUse` entries
- **THEN** the user's entries are preserved and the `guard-router` entry is present exactly once

### Requirement: Guard scope is Claude Code only in this change

The guard, the `Agent(...)` grant, and the removal of the routing-subagent hop SHALL apply to Claude Code only. GitHub Copilot, Codex CLI, Cursor, Kiro, and Antigravity keep their existing Janus templates in this respect (tool restrictions per the `janus-router-agent` capability, prose-level "refuse to act" only); no `guard-router` binding is installed on them, because each has a different hook payload shape and event vocabulary that this change does not verify. Copilot is the natural next platform (its hook payload also carries a top-level `agent_type`).

#### Scenario: No guard binding on non-Claude platforms

- **WHEN** `dreamland init` completes with any of GitHub Copilot, Codex CLI, Cursor, Kiro, or Antigravity selected
- **THEN** none of the installed hook binding files references `guard-router`
