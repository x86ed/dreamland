## ADDED Requirements

### Requirement: `dreamland guard-router` blocks the orchestrator identity from doing specialist work

`dreamland guard-router [--agent-name <name>]` SHALL be a `PreToolUse` hook command that reads the hook JSON payload from stdin and blocks (writes a reason to stderr and exits 2, through the existing `Blocking` mechanism in `cmd/hookexit.go`) any tool call that would do specialist work while the acting identity is `janus`. It is a separate command from `guard-artifact`: `guard-artifact` answers "does this agent own this path", `guard-router` answers "is this identity permitted to act at all beyond routing".

**Acting identity** is resolved, in order, from: (1) the `--agent-name` flag, when present (used by agent-scoped hooks, where the identity is known statically); (2) the payload's top-level `agent_type`, which Claude Code sets both inside a dispatched subagent and for a session launched with `claude --agent <name>`; (3) a payload carrying a non-empty top-level `agent_id` but no `agent_type`, which is a call made inside some subagent of unknown type and resolves to the non-blocking identity `subagent`; (4) otherwise *unattributed* (a plain main session with no `--agent`), which is **treated as `janus` by default** (see the policy requirement below). Resolution SHALL NOT read `git config user.name`, `.dreamland-session.json`, the per-session identity files, or any other file for the identity itself: those are shared or mutable state that parallel sessions overwrite, and identity for an access decision must not depend on them. (`.dreamland.json` is read only for the policy in the next requirement, never to decide who the actor is.)

For acting identity `janus` (explicit or defaulted):

- Tools `Write`, `Edit`, `MultiEdit`, and `NotebookEdit` SHALL be blocked unconditionally.
- Tools `Bash` and `PowerShell` SHALL be blocked unless the command satisfies the allowlist in the next requirement.
- Tools `Agent` and `Task` SHALL be allowed only for a permitted dispatch target (see the requirement on `Agent` calls below).
- All other tools (`Read`, `Grep`, `Glob`, `AskUserQuestion`, etc.) SHALL be allowed.

For any other acting identity, `guard-router` SHALL exit 0 (ownership of protected paths remains `guard-artifact`'s concern). A missing, empty, or unparseable payload SHALL exit 0 with a note on stderr: the guard fails open on malformed input, matching `guard-artifact`, because a hook that blocks on a payload it cannot read would brick the session. If no git repository can be found from the working directory, `guard-router` SHALL exit 0 with a note (the hook is installed per repository, so this is a misconfiguration, not a session to police).

The blocking message SHALL name what to do next. For a file-modifying tool whose `tool_input.file_path` matches an owned path in `guard-artifact`'s ownership table, it names that owner (e.g. "this path is owned by phantasos; dispatch phantasos"). Otherwise it states that this session is acting as the janus orchestrator and must dispatch a specialist with the `Agent` tool, directs the caller to run `dreamland route` and dispatch the returned agent, and names `iktomi` as the free-form default and `morpheus` as the target for build, install, and git-mutation work. For an unattributed session the message additionally states that the guard can be turned off for plain sessions only by the user, via `.dreamland.json`.

#### Scenario: Janus's Edit is blocked and names the owner

- **WHEN** `guard-router` receives `{"agent_type":"janus","tool_name":"Edit","tool_input":{"file_path":"openspec/changes/x/proposal.md"}}`
- **THEN** it exits 2 and stderr names `phantasos` as the agent to dispatch

#### Scenario: Janus's Write to an unowned path is blocked with the generic message

- **WHEN** `guard-router` receives `{"agent_type":"janus","tool_name":"Write","tool_input":{"file_path":"cmd/serve.go"}}`
- **THEN** it exits 2 and stderr directs the caller to `dreamland route` and names `iktomi` as the free-form default

#### Scenario: A plain main session's Edit is blocked like Janus's

- **WHEN** `guard-router` receives `{"session_id":"S1","tool_name":"Edit","tool_input":{"file_path":"cmd/serve.go"}}` (no `agent_type`, no `agent_id`, no flag) and `.dreamland.json` has no `main_thread_guard` key
- **THEN** it exits 2 and stderr says the session must dispatch a specialist with the `Agent` tool

#### Scenario: Another agent's edit is not this guard's concern

- **WHEN** `guard-router` receives `{"agent_type":"morpheus","tool_name":"Edit","tool_input":{"file_path":"cmd/serve.go"}}`
- **THEN** it exits 0

#### Scenario: A call inside a subagent of unknown type is not treated as the main session

- **WHEN** `guard-router` receives `{"agent_id":"a1b2c3","tool_name":"Edit","tool_input":{"file_path":"cmd/serve.go"}}` (no `agent_type`)
- **THEN** it exits 0

#### Scenario: Static flag takes precedence over the payload

- **WHEN** `guard-router --agent-name janus` receives a payload with no `agent_type` and `tool_name` `Write`
- **THEN** it exits 2

#### Scenario: Identity never comes from shared git config

- **WHEN** `git config --local user.name` reads `janus` but the payload has `agent_type` `morpheus` and `--agent-name` is absent
- **THEN** `guard-router` exits 0, because the acting identity is `morpheus`

#### Scenario: Identity never comes from the per-session identity file

- **WHEN** the per-session identity file for the payload's `session_id` holds `morpheus` but the payload has no `agent_type`
- **THEN** `guard-router` treats the actor as unattributed (`janus`) and blocks a `Write`

#### Scenario: Malformed payload fails open

- **WHEN** `guard-router` receives an empty stdin or non-JSON text
- **THEN** it exits 0

### Requirement: The orchestrator's shell access is an allowlist, fail-closed, that permits read-only inspection and repository verification only

Under acting identity `janus` (explicit, or the default for an unattributed main session), a `Bash` or `PowerShell` call SHALL be allowed only when all of the following hold; otherwise it is blocked. This is an allowlist, not a parse of shell text for mutations: unlike a denylist it fails closed on any command it does not recognize.

1. The command, trimmed, contains none of the characters `;` `&` `|` `<` `>` `` ` `` `$` `(` `)` `{` `}` `\` `'` `"` or a newline or carriage return, with one exception (item 5). Quote characters are excluded so that a quoted argument cannot hide a blocked flag from the token checks below. A `PowerShell` call is always blocked: the orchestrator is expected to use `Bash`, so a `PowerShell` call under this identity is treated as anomalous and the message says to dispatch a specialist.
2. The first two whitespace-separated tokens match one of: `openspec status`, `openspec list`, `openspec show`, `openspec validate`, `openspec instructions`, `dreamland route`, `dreamland version`, `git status`, `git log`, `git diff`, `git show`, `git rev-parse`, `go vet`, `go test`; or the first token is `ls` or `pwd`.
3. For every command other than `go vet` and `go test`, no argument begins with `--output`, `-o`, `--ext-diff`, `--exec`, or `--no-index`.
4. For `go vet` and `go test`, every token after the second is either (a) a boolean flag from `-v`, `-race`, `-short`, `-cover`, `-failfast`, `-json` (`go test` only); (b) a value flag written in the single-token form `-name=value` where `name` is one of `run`, `skip`, `count`, `timeout`, `parallel`, `p`, `tags`, `covermode`, `shuffle`, `list` (`go test`) or `tags` (`go vet`) and `value` matches `^[A-Za-z0-9_.,:/^+-]+$`; or (c) a package pattern that begins with `./`, contains only `[A-Za-z0-9_./-]`, and has no `..` path segment (the wildcard `...` is allowed). Any other token, including `-o`, `-c`, `-exec`, `-toolexec`, `-vettool`, `-overlay`, `-modfile`, `-coverprofile`, `-cpuprofile`, `-memprofile`, `-blockprofile`, `-mutexprofile`, `-trace`, `-outputdir`, `-fuzz`, `-args`, `-C`, an absolute path, or a value given as a separate token, blocks the call. These two commands are permitted because they are how a session verifies the repository, they produce no artifact in the working tree, and dreamland already runs the project's tests at every `Stop` under the same trust; they do execute the repository's own code, and that is accepted, not overlooked.
5. Exception, so the orchestrator can pass a user's free text (which routinely contains `(`, `$`, quotes) to `dreamland route` verbatim: a single quoted-heredoc form is allowed when the first line is `dreamland route ... --stdin <<'DREAMLAND_ROUTE_EOF'` (satisfying items 2 and 3, with the `<<'DREAMLAND_ROUTE_EOF'` suffix the only permitted use of `<` and of `'`), the final line of the command is exactly `DREAMLAND_ROUTE_EOF`, that terminator appears exactly once, and nothing follows it. The body between is unrestricted: a quoted heredoc delimiter makes the shell treat the body as literal text, and `dreamland route` only reads it.

Consequently the following are blocked for this identity and must be delegated through the `Agent` tool: `go build`, `go install`, `go run`, `go generate`, `go mod`, `gofmt`; `mv`, `cp`, `rm`, `mkdir`, `chmod`, `touch`, `tee`, `sed`, `awk`; every `git` subcommand other than the five listed (`add`, `commit`, `checkout`, `switch`, `restore`, `stash`, `merge`, `rebase`, `reset`, `branch`, `tag`, `push`, `worktree`, ...); `openspec new`, `openspec archive`; `dreamland coauthor`, `commit`, `test`, `test-and-commit`, `version-bump`, `init`, `oneiroi`, `telemetry`; and every other command. Building and installing the `dreamland` binary (`go build -o`, then `mv`/`cp` to `~/.local/bin`) is therefore delegated to `morpheus`: its only effect is writing an artifact to disk, which is exactly the class of action this guard exists to route through an attributed specialist. This narrows the precedent set by `fixed-pipeline-enforcement` (which left Bash-mediated mutation out of scope because parsing shell text is unreliable) to the one identity where an allowlist is practical.

#### Scenario: Read-only routing inputs are allowed

- **WHEN** `guard-router` receives a `Bash` call from `janus` with command `openspec status --change x --json`
- **THEN** it exits 0

#### Scenario: A plain main session can run go vet and go test

- **WHEN** `guard-router` receives, from an unattributed session, `Bash` commands `go vet ./...` and `go test ./... -race -count=1 -run=TestFoo`
- **THEN** both exit 0

#### Scenario: go test flags that write files or run other programs are blocked

- **WHEN** `guard-router` receives `go test -coverprofile=c.out ./...`, `go test -exec=foo ./...`, `go test -o out ./...`, `go vet -vettool=/tmp/x ./...`, or `go test ../other`
- **THEN** each exits 2

#### Scenario: A value flag given as a separate token is blocked

- **WHEN** `guard-router` receives `go test -run TestFoo ./...`
- **THEN** it exits 2, and stderr shows the accepted `-run=TestFoo` form

#### Scenario: Building and installing the binary is delegated

- **WHEN** `guard-router` receives, from an unattributed session, each of `go build -o dreamland .`, `go build ./...`, and `mv dreamland /Users/x/.local/bin/dreamland`
- **THEN** each exits 2 and stderr names `morpheus` for build and install work

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

#### Scenario: A quoted flag cannot hide from the token checks

- **WHEN** `guard-router` receives a `Bash` call from `janus` with command `git diff "--output=out.patch"`
- **THEN** it exits 2

#### Scenario: An unrecognized command is blocked

- **WHEN** `guard-router` receives a `Bash` call from `janus` with command `sed -i s/a/b/ cmd/serve.go`
- **THEN** it exits 2

#### Scenario: Compound commands are blocked

- **WHEN** `guard-router` receives a `Bash` call from `janus` with command `openspec list --json && rm -rf x`
- **THEN** it exits 2

#### Scenario: PowerShell is always blocked for this identity

- **WHEN** `guard-router` receives a `PowerShell` call from an unattributed session with any command
- **THEN** it exits 2

### Requirement: Under the orchestrator identity, `Agent` calls may target only the registered roster or a read-only built-in

Under acting identity `janus`, a call to the `Agent` or `Task` tool SHALL be allowed only when `tool_input.subagent_type` is (a) a registered dispatch target, resolved through `agentidentity.IsRegistered` (the nine built-in non-router agents plus every agent in `.dreamland/oneiroi/registry.json`), excluding `janus` itself, or (b) `Explore` or `Plan` (Claude Code's read-only built-in subagents). A call with a missing or empty `subagent_type`, `general-purpose`, `janus`, or any other value SHALL exit 2 with a message that lists the registered roster and states why: `general-purpose` can edit files with no dreamland identity, so allowing it would launder specialist work past this guard and mislabel it.

This is the same restriction the `Agent(<registered roster>)` grant expresses in Janus's `tools` frontmatter (see the `janus-router-agent` capability), extended to a plain main session, which has no such frontmatter because it is not an `--agent` session. The `tools` grant governs `claude --agent janus`; this check governs both launch modes and is what closes the `general-purpose` bypass for the default-blocked plain session.

#### Scenario: Dispatching a registered specialist is allowed

- **WHEN** `guard-router` receives `{"tool_name":"Agent","tool_input":{"subagent_type":"morpheus"}}` from an unattributed session
- **THEN** it exits 0

#### Scenario: A seeded agent is dispatchable

- **WHEN** `amber-falcon` is in `.dreamland/oneiroi/registry.json` and `guard-router` receives an `Agent` call with `subagent_type` `amber-falcon`
- **THEN** it exits 0

#### Scenario: general-purpose is blocked

- **WHEN** `guard-router` receives `{"tool_name":"Agent","tool_input":{"subagent_type":"general-purpose"}}` from an unattributed session, and again with no `subagent_type` at all
- **THEN** each exits 2 and stderr lists the registered roster

#### Scenario: Read-only built-ins are allowed

- **WHEN** `guard-router` receives an `Agent` call with `subagent_type` `Explore`
- **THEN** it exits 0

#### Scenario: Spawning janus is blocked

- **WHEN** `guard-router` receives an `Agent` call with `subagent_type` `janus` from an unattributed session
- **THEN** it exits 2 and stderr says the session already is the janus orchestrator

#### Scenario: A specialist's own Agent call is not this guard's concern

- **WHEN** `guard-router` receives an `Agent` call with `agent_type` `morpheus` and `subagent_type` `general-purpose`
- **THEN** it exits 0

### Requirement: An unattributed main session is treated as `janus` by default; `.dreamland.json` can opt plain sessions out

When the acting identity is unattributed (no `--agent-name`, no `agent_type`, no `agent_id`), `guard-router` SHALL consult `.dreamland.json` key `main_thread_guard`: absent, `"block"`, or any other value SHALL apply the same rules as for `janus` (an unrecognized value additionally writes a note to stderr; the guard fails closed on it); the single value `"allow"` SHALL exit 0 for every call from an unattributed session. The opt-out never applies to an explicit `janus` identity (`--agent-name janus`, or `agent_type` `janus` from `claude --agent janus`), which is always guarded.

The default is `block` because the workflow's invariant is that no work happens without a dispatched, attributed specialist, and an unattributed main session doing the work itself is the source of unattributed commits. The opt-out exists, and is the only one, because: a repository in which dreamland is installed is also used by people running an ordinary Claude Code session and by scripted or SDK sessions (`claude -p`, CI) that have no dispatch step; and it is the operator's escape when the allowlist over-blocks a legitimate command. It is a `.dreamland.json` value, not an environment variable or a flag, so an agent that is itself blocked from writing files cannot flip it: only the user (or a specialist the user directs) can change it.

`dreamland init` SHALL NOT add a top-level `"agent"` key to `.claude/settings.json`; making every session's main thread `janus` remains an explicit per-user choice (`claude --agent janus` or a personal settings file) and is not installed by default. Blocking by default does not need it: an unattributed session already resolves to the `janus` identity for this guard, keeps the user's own model and default agent, and the blocking message tells it to dispatch.

#### Scenario: Default policy blocks plain sessions

- **WHEN** `.dreamland.json` has no `main_thread_guard` key and `guard-router` receives a `Write` payload with no `agent_type`, no `agent_id`, and no flag
- **THEN** it exits 2

#### Scenario: Opt-out leaves plain sessions unrestricted

- **WHEN** `.dreamland.json` has `"main_thread_guard": "allow"` and `guard-router` receives a `Write` payload with no `agent_type`, no `agent_id`, and no flag
- **THEN** it exits 0

#### Scenario: The opt-out does not weaken an explicit janus launch

- **WHEN** `.dreamland.json` has `"main_thread_guard": "allow"` and `guard-router` receives a `Write` payload with `agent_type` `janus`
- **THEN** it exits 2

#### Scenario: An unrecognized policy value fails closed

- **WHEN** `.dreamland.json` has `"main_thread_guard": "off"` and `guard-router` receives a `Write` payload from an unattributed session
- **THEN** it exits 2 and stderr notes the unrecognized value

#### Scenario: init does not force a default agent

- **WHEN** `dreamland init` completes with "Claude Code" selected
- **THEN** `.claude/settings.json` contains no top-level `"agent"` key

### Requirement: The guard is wired at agent scope and workspace scope on Claude Code

`.claude/agents/janus.md` SHALL declare an agent-scoped `hooks.PreToolUse` entry with matcher `Write|Edit|MultiEdit|NotebookEdit|Bash|PowerShell|Agent|Task` running `dreamland guard-router --agent-name janus`. `.claude/settings.json` (via `internal/scaffold/templates/hooks/bindings/claude-code/settings-patch.json`) SHALL register the same matcher running `dreamland guard-router` (no flag) under `PreToolUse`, so identity comes from the payload's `agent_type`, which covers a session launched as `janus` even if agent-scoped hooks do not fire, and is the only binding that reaches a plain main session and its unattributed-session policy. Both firing for one call is harmless: the command is read-only and idempotent. `.claude/settings.json` `permissions.allow` SHALL include `Bash(dreamland route *)` so routing runs without a permission prompt.

`janus.md` SHALL carry no `hooks.Stop` block. Every entry of the previous block is redundant or dead for a pure router: `coauthor --hook --agent-name janus` (identity already defaults to `janus` at `SessionStart`), `telemetry write --tool claude-code` and `version-bump --patch` (already run by the workspace `Stop` and `SubagentStop` chains, so they would run twice per turn under `claude --agent janus`), `version-bump --minor --if-agent janus` (Janus is no longer spawned as a subagent, and per-branch minor bumps already happen at `SessionStart`), and `commit --reason handoff --agent-name janus` (under `claude --agent janus` it double-commits alongside the workspace `Stop` chain's `test-and-commit`, and it is the mechanism that stamped specialist work with Janus's name). Janus's turns remain covered by the workspace-level `Stop` chain (`telemetry write`, `test-and-commit --reason turn-complete --hook`) and its dispatches by the workspace-level `PreToolUse` `coauthor` and `SubagentStop` chain.

The other nine agents' `hooks.Stop` blocks are unchanged by this requirement.

#### Scenario: janus.md carries the guard and no Stop hooks

- **WHEN** `dreamland init` completes with "Claude Code" selected
- **THEN** `.claude/agents/janus.md` has `hooks.PreToolUse` with matcher `Write|Edit|MultiEdit|NotebookEdit|Bash|PowerShell|Agent|Task` running `dreamland guard-router --agent-name janus`
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

The guard, the default-block policy for unattributed sessions, the `Agent(...)` grant, and the removal of the routing-subagent hop SHALL apply to Claude Code only. GitHub Copilot, Codex CLI, Cursor, Kiro, and Antigravity keep their existing Janus templates in this respect (tool restrictions per the `janus-router-agent` capability, prose-level "refuse to act" only); no `guard-router` binding is installed on them, because each has a different hook payload shape and event vocabulary that this change does not verify. Copilot is the natural next platform (its hook payload also carries a top-level `agent_type`).

#### Scenario: No guard binding on non-Claude platforms

- **WHEN** `dreamland init` completes with any of GitHub Copilot, Codex CLI, Cursor, Kiro, or Antigravity selected
- **THEN** none of the installed hook binding files references `guard-router`
