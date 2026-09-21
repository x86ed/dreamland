## Context

Verified in the repository (not assumed):

- `.claude/agents/janus.md`: `tools: Read, Bash`, `model: haiku`, `hooks.Stop` with five commands (`coauthor --hook --agent-name janus`, `telemetry write`, `version-bump --patch`, `version-bump --minor --if-agent janus`, `commit --reason handoff --agent-name janus`). No `Agent` tool.
- `.claude/settings.json` (this repo, live) has no top-level `agent` key; `PreToolUse` matches only `Task|Agent` (`coauthor --hook`). The shipped `settings-patch.json` additionally binds `guard-artifact` on `Write|Edit`, but this repo's live file does not have it yet (drift, fixed by re-running `dreamland init`).
- `cmd/guard_artifact.go` is the precedent for a hook-based guard: reads the payload, resolves `agent_type`, exits 2 through `guardArtifactExit`; fails open on empty/malformed payload and on absent `agent_type`. `cmd/hookexit.go` (`Blocking`, `IsBlocking`) makes `Execute()` exit 2 for blocking errors (from `harden-commit-hook-enforcement`, tasks 1.x done).
- `claude-code-parity/design.md` records, from Anthropic's hook reference: `agent_type` in a hook payload is the subagent's type inside a subagent, or the session's `--agent` value on the main thread; a plain session has none. Frontmatter `hooks` fire for a subagent or for the main session under `--agent`. Running as the main-thread agent inherits that agent's tools and model, and `Agent` must be granted explicitly in `tools:`; `Agent(a, b)` restricts which subagents a main-thread agent can spawn. These are the platform facts this design depends on; Task 0 re-verifies them live because the previous change did the same and that is where its blocked tasks came from.
- Per-agent commands (`.claude/commands/drmlnd/morpheus.md`, bare `morpheus.md`) delegate to Janus; only `/opsx:propose|explore|archive` are direct. `internal/scaffold/stub.go` generates the per-agent commands with that text, and `dreamland oneiroi seed` writes them the same way.
- `dreamland oneiroi seed` edits the *installed* `janus.*` files (`internal/oneiroi/routing.go`, `AddStubEdge`), not the templates. `dreamland init` (`internal/scaffold/scaffold.go`) skips an existing agent file unless `Config.Force` is set (agent files have no staleness check; `installFlatCommands` self-heals a command file only when its frontmatter `name:` differs from the template's; the bare `dreamland.md`/`janus.md`/per-agent command copies carrying the `dreamland-managed` marker are rewritten on every init). Two consequences: a forced re-init re-renders `janus.md` from the template, dropping anything a seed patched in that the template does not know about, so the `Agent(...)` list must be generated from the registry at render time (Decision 6); and an existing install will not pick up the new `janus.md` or command text on a plain `dreamland init`, so this change adds a staleness rule (task 8.1) rather than asking users to remember `--force`.
- Artifact ownership (`guard-artifact`): everything under `internal/scaffold/templates/agents/**` and `.../commands/**` is writable only by `hypnos`, so template tasks below are dispatched to `hypnos`, and Go tasks to `nyx`/`morpheus`.

- `cmd/commit.go` already pins a commit's author and committer to the resolved identity (`git -c user.name=... -c user.email=... commit`, from the archived `fix-claude-code-handoff-integrity`), so a `dreamland commit` no longer *authors* from shared git config; what still reads shared git config is the identity *resolution*: `currentGitIdentityName` returns `git config --local user.name` for `--reason turn-complete` at a bare `Stop`, because a `Stop` payload carries no sub-agent identity. `coauthor` writes that value at `SessionStart` and at `PreToolUse` `Task|Agent`. Two windows in one repository overwrite each other there (Decision 7). `dreamland telemetry write` does not read git config (a bare `Stop` is attributed `janus`); `internal/oneiroi/commit.go` commits with its own fixed author. Neither is in scope.
- The `session_id` field is present on every Claude Code hook payload, including bare `Stop` (recorded in `harden-commit-hook-enforcement`'s coauthor requirement and re-verified in task 0.6).

Not measured: token cost of the current routing hop. The saving is structural (one subagent turn and its hooks per routing decision, plus keeping attachments out of a paraphrase hop), not a number. Task 0.5 records a before/after with `zhougong` so the claim is checkable rather than asserted.

## Goals / Non-Goals

**Goals**

- A launch-mode-independent fix: the workflow must not get stuck on Janus whether the session was started with `claude --agent janus`, through a launcher, or as plain `claude` plus `/dreamland`.
- Routing costs zero model turns for every case that has a computable answer, and never spawns a second agent to decide.
- Janus cannot do specialist work, enforced by a hook, with an identity resolution that survives parallel windows.
- Roster registration stays one coherent mechanism.
- The invariant "no work happens without a dispatched, attributed specialist" holds for a plain main session too, by default, with an explicit and narrow opt-out.
- End-of-turn attribution on Claude Code is per session, so parallel windows in one repository cannot mislabel each other's commits.

**Non-Goals**

- No per-session identity outside Claude Code's main-session `Stop` commit path: `telemetry write`, other platforms, and `.dreamland-session.json` (itself a repo-shared file) are unchanged (Decision 7).
- No natural-language or shell-semantics analysis in the guard: the Bash policy is a token allowlist, nothing cleverer.
- No guard, hop removal, or `Agent(...)` grant on Copilot, Codex, Cursor, Kiro, or Antigravity (Decision 8).
- No change to the nine other agents' definitions beyond one tag-convention line in `phantasos.*`; no change to `guard-artifact` or `telemetry`. `commit`, `test-and-commit`, and `coauthor` change only in how the Stop-time identity is resolved and recorded (Decision 7).
- No natural-language intent classifier. The intent table is a short, anchored, embedded list.
- Not installing a default `"agent": "janus"` in project settings.

## Decisions

### Decision 1: Deterministic routing in the binary, with a tiny ambiguous tail (recommended; question 1)

`dreamland route` evaluates six ordered rules (see the `deterministic-routing` spec): explicit target, command spelling, OpenSpec state, anchored intent phrase, and two free-text outcomes. Two design choices carry the token saving:

- **Free text with no OpenSpec context goes to `iktomi`, not to "ambiguous".** Iktomi already is the catch-all and can redirect to any specialist, so the worst case of a wrong default is one redirect. Treating that case as ambiguous would send the most common request type through an LLM for no gain.
- **`/opsx:apply` becomes deterministic through a task tag, not through heuristics.** The nyx-vs-morpheus choice is "new behavior with no covering test" versus "mechanical, or test exists", which is semantic; the person who knows is Phantasos, at drafting time. A `[flow: ...]` tag on each task line records that decision once. Untagged tasks stay `ambiguous` (never silently defaulted, since defaulting to `morpheus` would skip the acceptance-test step), so existing changes keep working.

**Where an LLM still adds value:** only for `ambiguous`, which occurs in five cases: several active changes and none named (`ask_user: true`); no active change with an apply/continue request (`ask_user: true`); free text while a change is active (iktomi vs the change's next agent); an untagged task (nyx vs morpheus, plus hypnos/mengpo on keywords); and any internal error. In every case the result carries at most a handful of candidates and their reasons.

**Fallback is the orchestrating session itself, not a janus subagent.** The session that ran `dreamland route` already has the conversation, attachments, and history; a Haiku Janus subagent would see strictly less. So the LLM's judgment among candidates is exercised in the same turn, at the marginal cost of a few output tokens, and asks the user when `ask_user` is true. This is the answer to "can it fall back to a cheap path (Janus only when ambiguous)": yes, but "Janus" in that path is the current session, not a new agent.

Rejected: (a) a stateless keyword classifier for free text (false positives route work to the wrong pipeline; iktomi default is safer); (b) calling a model from inside `dreamland route` (adds a network dependency, an API key, and a hidden cost inside a tool that runs in every session); (c) keeping the Janus subagent hop as the "ambiguous" fallback (pays a hop with less context than the caller has).

### Decision 2: Janus remains a definition, but is the in-session orchestrator, never a spawned routing subagent (recommended; question 2)

Options considered:

| | A. Keep spawning Janus (status quo) | B. Delete `janus.md`, make it a skill/command only | **C. Keep `janus.md`; orchestrator protocol runs in the invoking session (chosen)** |
| --- | --- | --- | --- |
| Routing hop cost | one subagent turn + hooks per decision | none | none |
| `claude --agent janus` works | yes, but cannot dispatch | no such agent | yes, with `Agent(...)` grant |
| `janus` remains a registered identity (fallback for unattributed turns, `agent_type` value) | yes | must invent a replacement identity, or break `session-agent-identity` | yes |
| One routing table, one place Hypnos registers agents | yes | table moves to a skill on Claude Code only; the other five platforms keep `janus.*`, so two tables | yes |
| Attribution of commits/telemetry | janus-labelled hop commits | n/a | routing produces no hook events of its own; orchestrator turns fall under the workspace `Stop` chain as `janus` |

Why C: B forks the roster mechanism across platforms and breaks the identity requirement; A is the cost being removed. Under C, the protocol text is deliberately tiny and defers rules to the binary (`dreamland route` returns a `next` sentence), so command files and `janus.md` do not each restate rules that can drift. The routing table in `janus.md` stays as the human-readable statement of the same rules, checked by the drift test.

Per-agent slash commands dispatch their fixed target directly and do not call `dreamland route` at all: the destination is fixed, and the command file is generated from the roster so the name is valid by construction. This removes the hop that `/drmlnd:morpheus` currently pays.

Model: `janus.md` keeps `model: haiku`. A main-thread Janus now only runs `dreamland route`, dispatches, and reads reports, which Haiku handles at the lowest price; the guard, not the model's obedience, is what stops it doing specialist work.

Optional optimization, gated by Task 0.4: slash-command files can pre-execute `dreamland route` inline (the `!` command-substitution feature with `allowed-tools`) so the result is already in the prompt and no tool-call turn is spent. Adopted only for commands that interpolate no user text (`/opsx:apply`, empty `/dreamland`); free-text routing passes the request through a quoted heredoc on `--stdin` from a `Bash` tool call, because splicing `$ARGUMENTS` into a shell command line is unsafe for arbitrary text.

### Decision 3: The guard and how the acting identity is resolved (recommended; question 3)

`guard-router` is a new command, not an extension of `guard-artifact`, because the two answer different questions (path ownership versus "may this identity act at all") and `guard-artifact` is deliberately fail-open on absent identity, which the new guard must be able to override by policy.

**Identity resolution** (payload and flags only): `--agent-name` flag, else payload `agent_type`, else unattributed. Never `git config user.name`, never `.dreamland-session.json`: those are the shared mutable state that parallel windows overwrite, so using them for an access decision would let window A's Janus be treated as window B's `morpheus` and vice versa. This is the concrete reason the guard does not depend on Decision 7's deferred work.

**Wired twice.** Agent-scoped in `janus.md` frontmatter with `--agent-name janus` (identity known statically, active whenever Janus is active, whether spawned or `--agent`), and workspace-scoped in `settings.json` reading `agent_type` (covers `--agent janus` if frontmatter hooks did not fire, and carries the unattributed policy). Double firing is harmless: read-only and idempotent.

**Bash is an allowlist.** `claude-code-parity` rejected Bash mutation detection as "too fragile to trust", and that reasoning is right for a denylist over arbitrary agents. It does not apply to the orchestrator identity, whose legitimate shell needs are a small, enumerable set (below). An allowlist fails closed on anything unrecognized, rejects shell metacharacters outright (this amendment adds `'` and `"`, because a quoted `"--output=x"` token otherwise slips past the flag checks), and has exactly one exception (a quoted heredoc feeding `dreamland route --stdin`) so that free text containing `(`, `$`, or quotes can be passed verbatim. `Edit`/`Write` are already outside Janus's `tools`; blocking them in the guard is belt and braces for the case where identity is `janus` but the tool set was not restricted, and it is the only thing that stops them for a plain main session. `PowerShell` calls under this identity are always blocked.

**Unattributed main session: treated as `janus`, block by default (user decision, 2026-09-20).** The user's instruction was "block and route to a subagent should assume janus by default". So an unattributed session (no `--agent-name`, no `agent_type`, no `agent_id`) is not a separate identity with a separate policy; it *is* `janus` for the guard's purposes, with one code path and one allowlist. This replaces the earlier allow-by-default design, whose rationale (an unattributed session is indistinguishable from an ordinary Claude Code session) is now the point: an ordinary session in a dreamland repository should route work through specialists, because that is what makes commits and telemetry attributable. Consequences designed for explicitly:

1. **The main session still needs to run its own verification.** `go vet` and `go test` are allowlisted, in a strict flag-and-package grammar (spec: value flags only as `-name=value`, packages only as relative `./...` patterns, no `-o`/`-c`/`-exec`/`-*profile`/`-fuzz`/`-vettool`/`-args`). They are permitted because they are how a session checks that the repository compiles and passes, they leave nothing in the working tree, and `dreamland test` already runs the project's tests at every `Stop` under the same trust. They do run repository code, which is stated in the spec rather than argued away. Rejected alternative: allow any `go` subcommand (would include `go build -o`, `go run`, `go generate`, `go install`, all of which write files or run arbitrary programs).
2. **Build and install are delegated, not allowlisted.** `go build` may write a binary (a single `main` package matched by `./...` writes one into the working directory), and `mv`/`cp` to `~/.local/bin` are arbitrary-destination mutations, so no token grammar for them is both safe and cross-platform (the destination is `~/.local/bin` on Unix and a different, user-specific path on Windows). The blocking message names `morpheus` for this work. The cost is one subagent dispatch per rebuild-and-install; that dispatch is what produces an attributed commit for the change, and it matches the standing rule that specialists, not the orchestrator, do the work. If the cost proves too high, a follow-up should add a dedicated, narrowly scoped `dreamland` subcommand for build-and-install (a fixed destination resolved in Go, no shell), not widen the shell allowlist; that is an open question, not part of this change.
3. **Git.** Read commands only (`status`, `log`, `diff`, `show`, `rev-parse`). Everything else (`add`, `commit`, `checkout`, `restore`, `stash`, `branch`, ...) is delegated; commits are made by hooks, not by the orchestrator.
4. **`Agent` calls need their own check.** `Agent(<roster>)` in `janus.md` restricts only a session launched as Janus; a plain main session has the unrestricted `Agent` tool and could dispatch `general-purpose`, a subagent with `Edit`/`Write` and no dreamland identity, walking around a default-block guard entirely. So `guard-router` also checks `Agent`/`Task` calls under the janus identity: allowed only for a registered dispatch target (`agentidentity.IsRegistered`) or the read-only built-ins `Explore` and `Plan`. This is the same rule as the grant, applied to the launch mode the grant cannot reach. Rejected alternative: `permissions.deny: ["Agent(general-purpose)"]` in `settings.json` (a second, parallel mechanism, that cannot express "registered roster", and denies `general-purpose` to research uses that are harmless).
5. **The identity signal must be reliable, because a wrong "unattributed" is now a hard block.** Under allow-by-default a missing `agent_type` inside a subagent was harmless (fail-open); under block-by-default it would block every specialist's edits. The resolver therefore treats a payload with a non-empty `agent_id` but no `agent_type` as an in-subagent call (`subagent`, not blocked), and task 0.6 is a gate: if a dispatched subagent's `PreToolUse` payload for an `Edit` or `Bash` call does not reliably carry `agent_type` (for each of the nine agents, a seeded agent, a built-in, and a parallel/background dispatch), the default flips back to allow and this design returns to `phantasos` rather than shipping a guard that bricks specialists.
6. **Sessions with no dispatch step are blocked too.** `claude -p`, SDK, and CI sessions in a dreamland repository have no `agent_type` and no way to dispatch a specialist unless they use the `Agent` tool. They are the reason the opt-out exists.

**The opt-out is kept, narrowly.** `.dreamland.json` `main_thread_guard: "allow"` disables the default for unattributed sessions only; it never relaxes an explicit `janus` identity (`claude --agent janus` is always guarded). Judged necessary because (a) scripted/CI/SDK sessions and users who want an ordinary Claude Code session in a repository that happens to have dreamland installed need a documented switch, and (b) it is the recovery path when the allowlist over-blocks, which is otherwise "edit `.claude/settings.json` to remove the hook". It lives in `.dreamland.json` rather than an environment variable so a session that is itself blocked from writing files cannot turn its own guard off. Unknown values fail closed (treated as `block`) with a stderr note. The previous name `main_thread_guard` and the value `"block"` are retained, so a repository that already set `"block"` keeps working.

**Upgrade impact.** This is a breaking change for plain-session users of a repository once its `.claude/settings.json` gains the guard binding (on the next `dreamland init`, which merges the settings patch; the self-heal in group 8 does not silently install it). `dreamland init` prints a one-line note when it adds the binding, and the README states the opt-out. It is a deliberate change of policy, decided by the user, not an accident of implementation.

Rejected: installing `"agent": "janus"` in project `settings.json` so every session is Janus (user decision: no). Block-by-default does not need it: the unattributed session is already treated as `janus` by the guard, keeps the user's own model, and changes nothing about which agent the platform reports.

### Decision 4: Grant `Agent(<roster>)`, restricted, generated from the registry with a drift test (decided: user, 2026-09-20)

Janus gets `tools: Read, Bash, Agent(phantasos, nyx, ..., <seeded agents>)`. Alternatives: no grant (current; the observed failure), unrestricted `Agent` (Janus could spawn `general-purpose`, a subagent with `Edit`/`Write` and no dreamland identity, which would launder specialist work past the guard and mislabel it), or the restricted list (chosen). `Agent(...)` restriction takes effect only when the agent is the main thread, which is exactly Janus's `--agent` mode; when Janus is a subagent it cannot spawn anyone regardless.

This reverses (supersedes) `claude-code-parity`'s decision (design.md there: "Granting `Agent`/`Task` to any dreamland agent, Janus included... rejected"). The grounds for reversing: that decision was made assuming a session never starts as Janus; the reported failure is precisely a session that does, and without `Agent` it can only do the work itself. The rest of that change's reasoning (agents dispatching each other directly, nest-spawning) does not apply: the grant is on Janus only and only the main thread honors it.

### Decision 5: Which of Janus's Stop hooks survive (recommended; question 4)

None. Per command:

| Command | Verdict | Reason |
| --- | --- | --- |
| `coauthor --hook --agent-name janus` | remove | Identity already defaults to `janus` (`SessionStart` + fallback); its only job was resetting identity after the routing hop, and the hop no longer exists. |
| `telemetry write --tool claude-code` | remove | Workspace `Stop` and `SubagentStop` already run it. Under `claude --agent janus` it would run twice per turn. |
| `version-bump --patch` | remove | Same duplication. |
| `version-bump --minor --if-agent janus` | remove from `janus.md` only | Fires only for a `janus` SubagentStop, which no longer occurs; per-branch minor bump already happens at `SessionStart`. Left on the other nine agents and in the workspace `SubagentStop`: harmless, self-filtering, and outside this change. |
| `commit --reason handoff --agent-name janus` | remove | Under `--agent janus` it double-commits beside the workspace `Stop` chain's `test-and-commit`, and it is the mechanism that stamped work with Janus's name. |

What remains for Janus is the workspace chain (`Stop`: `test-and-commit --reason turn-complete`, telemetry; `PreToolUse` `coauthor` on dispatch; `SubagentStop` per specialist), which is all it needs.

### Decision 6: Roster coherence (question 5)

Four touchpoints, one owner each (table in the `agent-lifecycle-management` spec). Two design points:

- `dreamland route` reads the registry; it needs no registration step. A seeded agent is immediately a valid `--to` target and shows up in `roster` on the next ambiguous result. Seeded agents are never returned by automatic rules (their triggers are not deterministic); they are reached by their slash command or by the orchestrator choosing from `roster`. That makes the binary and the LLM-facing prose agree without Hypnos hand-maintaining a rule table.
- The `Agent(...)` list is generated by `dreamland init` from the registry, with `seed`/`fork`/`mengpo` patching the installed file for immediacy. Generating at render time is what makes it survive a forced re-init: a value that only `seed` had patched in would be lost. The existing routing-table stub edge has the same exposure today (it is patched into the installed file only); that is a pre-existing gap this change does not widen, and task 4.4 records whether it needs its own fix. A drift test ties registry, `Agent(...)`, and routing tables together so a missed touchpoint fails the build.

`hypnos`'s authoring steps are unchanged; the `Agent(...)` entry is mechanical.

### Decision 7: Per-session identity for the main-session Stop hook: bundled into this change (decided: user, 2026-09-20; earlier recommendation was to defer)

The user chose to bundle it. It is designed here and specified in the `session-agent-identity` capability (ADDED requirement) and the `dev-workflow-hooks` capability (MODIFIED `dreamland commit` requirement).

**The bug.** `coauthor` writes `git config --local user.name` at `SessionStart` and at `PreToolUse` `Task|Agent`; `commit --reason turn-complete` at `Stop` reads it back (`currentGitIdentityName`) because a `Stop` payload has no sub-agent identity. Two windows in one repository overwrite each other in between, so window A's turn can be committed as window B's agent. Since `fix-claude-code-handoff-integrity`, `dreamland commit` pins author and committer from the resolved identity, so the subject and author agree; but the resolved identity is still the shared, wrong value.

**The design.**

- A per-session record: `<root>/identity/<session_id>`, JSON `{"agent","updated_at"}`, atomic temp-and-rename write, 0600/0700 permissions, entries older than 7 days pruned on write. `<root>` is `$DREAMLAND_STATE_DIR` if set, else `os.UserCacheDir()/dreamland`, else `os.TempDir()/dreamland`. That is the same per-user state directory `parallel-session-otel-receiver` introduced (`otelreceiver.StateDir()` returns `$DREAMLAND_STATE_DIR`, or `<UserCacheDir>/dreamland/otel` by default). A new package `internal/sessionidentity` owns the path helper and does not import `otelreceiver`, so that a build without the receiver does not depend on it; a test asserts that with `DREAMLAND_STATE_DIR` set the two resolve to the same directory, and that `identity/` never collides with the receiver's `sessions/`, `receiver.log`, or `receiver-<port>.*` names. No change to the receiver's code.
- `session_id` comes from the hook payload, validated with the receiver's `ValidConversationID` rule (charset `[A-Za-z0-9._-]`, 1-128, not `.`/`..`). An invalid or absent id means "no key": legacy behavior, nothing written.
- Writers: `coauthor --hook` at `SessionStart` (`janus`) and `PreToolUse` `Task|Agent` (subagent type, already filtered to registered names). It keeps writing git config as before (humans, other tools, and non-Claude platforms rely on it); it is no longer authoritative when a `session_id` is present. `coauthor --agent-name` (the per-agent frontmatter `Stop` blocks) has no payload and does not touch the record.
- Reader: `commit --hook` and `test-and-commit --hook` for `--reason turn-complete`. Resolution order in `runCommit`: `--agent-name`; for handoff only, the payload's `agent_type`; then the session record (registered names only, else `janus`); then shared git config only when no valid `session_id` exists; then `coauthor`'s resolution. For turn-complete the payload's own `agent_type` is deliberately ignored: at `Stop` it is the main-thread agent (e.g. `janus` under `--agent janus`), and honoring it would relabel a turn that dispatched `nyx`. A valid `session_id` with no record yields `janus`, not the shared value (stated degradation for sessions that predate the upgrade).
- Binding: `settings-patch.json` `Stop` becomes `dreamland test-and-commit --reason turn-complete --hook`. `test-and-commit` reads stdin once and forwards `--hook` and the payload to its commit step. This template is not under `agents/**` or `commands/**`, so it is `nyx`/`morpheus` work, not `hypnos`.
- Not changed: `telemetry write` (bare `Stop` is `janus`, and it never read git config), `guard-router` (never reads the record, by design), the `oneiroi` scaffold commit (fixed author), the `SubagentStop` handoff path (identity comes from the payload's `agent_type` or the static `--agent-name`, neither shared), other platforms (their payloads' session key is unverified).
- Not solved and named: `.dreamland-session.json`, the telemetry snapshot file that `prepare-commit-msg` reads for the `Tokens:` line and `dreamland serve` reads for `CurrentAgent`, is also repo-shared and last-writer-wins. Token totals in a commit trailer can therefore still be another window's. This change fixes identity, not that file; it is an open question.

**Overlap with `harden-commit-hook-enforcement` (57/59), handled explicitly.** That change rewrote the same commands and its implementation tasks (groups 1-9 and 11) are all checked, and their code is in the tree (`cmd/hookexit.go`, `cmd/testandcommit.go`, `cmd/testresult.go`, `--hook` on `coauthor`). Its two open tasks, 10.1 and 10.2, are evidence-and-bookkeeping tasks for `claude-code-self-hosting` (capture a real subagent commit; tick that change's 6.4/7.3); they touch no code and no spec text, so **they do not block this change's implementation**. Where they do matter is archiving, below.

Requirement headers this change touches, by capability:

| Capability | Header | Kind | Also touched by `harden-commit-hook-enforcement`? |
| --- | --- | --- | --- |
| `dev-workflow-hooks` | `dreamland commit auto-commits pending changes on turn completion and agent handoff` | MODIFIED (full text: identity chain in item 4, `--hook` on the Stop binding) | **Yes, MODIFIED there too** |
| `session-agent-identity` | `On Claude Code, the session's current agent identity is recorded per session, keyed by the hook payload's session_id` | ADDED | No |
| (referenced, not edited) `dev-workflow-hooks` | `coauthor sets agent identity and installs prepare-commit-msg hook` | not touched here; `coauthor`'s new session-file write is specified in the ADDED requirement above so that harden's MODIFIED text stays the single owner of that header | Yes (theirs) |
| (referenced, not edited) `dev-workflow-hooks` | `dreamland test-and-commit runs test then commit in one process...` | not touched; `test-and-commit --hook` is specified in the ADDED requirement above because that requirement exists only in harden's unarchived delta and cannot be MODIFIED from here | Yes (ADDED there) |
| (referenced, not edited) `otel-commit-hook` | `Hook appends AI telemetry as git trailers` | not touched | Yes (theirs) |

**A drift found while doing this, which the user should know about.** Harden's MODIFIED text for `dreamland commit auto-commits...` is stale against the main spec: the archived `fix-claude-code-handoff-integrity` already changed that requirement (handoff git failures are non-blocking; author and committer are pinned; the `--agent-name`/`--hook` identity chain), and harden's delta predates it. Archiving harden as written would overwrite those with older text (for instance, making a handoff git failure blocking again). This change's MODIFIED text for that header is therefore written as the *union* (main's current text, harden's test-gating and `test-and-commit` additions, and this change's identity chain). To avoid a silent regression either way: **archive `harden-commit-hook-enforcement` first, after rebasing its delta for that header onto main, and archive this change second**, so this change's union text is the last writer and wins. If harden is archived unrebased and this change second, the result is still correct (ours overwrites). The one wrong order is this change first and harden second.

**Task sequencing** (see tasks.md group 11): implementation starts after task group 0 passes; it edits the files harden edited (`cmd/coauthor.go`, `cmd/commit.go`, `cmd/testandcommit.go`, `settings-patch.json`) on top of the tree as it stands, so there is no merge to resolve, only an ordering of archives.

### Decision 8: Platform scope (question 6)

| Platform | This change | Not this change |
| --- | --- | --- |
| Claude Code | `dreamland route`, hop removal, `Agent(...)` grant, `guard-router` (both scopes, default-block for plain sessions, `Agent`-call check), Stop-hook removal, per-agent command dispatch, phantasos tag line, per-session Stop identity (Decision 7) | `telemetry`/`.dreamland-session.json` per-session state |
| GitHub Copilot | `dreamland route` available; optional one-line instruction in `janus.agent.md`; Copilot's Janus already has the `agent` tool, so no grant change | `guard-router` binding (Copilot's hook payload also has `agent_type`, so it is the natural next platform, but its event vocabulary and `useCustomAgentHooks` gating are unverified here) |
| Codex CLI, Cursor, Kiro, Antigravity | `dreamland route` available; optional one-line instruction in their `janus.*` | guard, hop change, grant: no hook event or payload identity verified for them |
| Windows (any platform) | `dreamland route` and `guard-router` are Go with no shell dependence; `openspec` located via `exec.LookPath` (npm `.cmd` shim), paths via `filepath`, backslash-safe path matching; matcher includes `PowerShell` | A PowerShell-native heredoc equivalent for free text: Claude Code's `Bash` tool (Git Bash) is assumed for the `--stdin` form |

The optional instruction line on the five non-Claude platforms is separable (tasks group 9); if it is cut, the Claude Code fix is unaffected.

### Decision 9: Launch-mode matrix (all modes must hold)

| Launch mode | Payload `agent_type` | Routing | Guard | Notes |
| --- | --- | --- | --- | --- |
| `claude --agent janus` (or a launcher doing this) | `janus` | Janus runs `dreamland route`, dispatches with its `Agent(...)` grant | Blocks Edit/Write and non-allowlisted Bash (both scopes) | The reported failure; Janus can no longer do the work and no longer lacks dispatch. |
| Plain `claude` + `/dreamland <text>` | none (unattributed, treated as `janus`) | The command file runs `dreamland route` in the main session and dispatches; no Janus subagent | Same rules as `janus` (default `block`); specialists guarded by `guard-artifact` | The previously working mode, minus the hop; the main session can no longer do the work itself. |
| Plain `claude` + `/drmlnd:morpheus` | none (as `janus`) | Direct dispatch, no route call | as above | Also loses the hop. |
| Plain `claude`, no command | none (as `janus`) | None: nothing routes automatically; the session is told to dispatch when it tries to act | as above | Edits and build/install are blocked with a dispatch message; `go vet`/`go test`/read-only git are allowed. Opt out with `.dreamland.json` `main_thread_guard: "allow"`. |
| `claude -p` / SDK / CI in the repo | none (as `janus`) | none | as above | Blocked from editing unless it dispatches through `Agent`; set `main_thread_guard: "allow"` for such runs. |
| Unknown launcher | whatever it passes | Same as one of the above depending on `--agent` | Workspace hooks are agent_type-driven, so nothing depends on how the session started | Works because no rule keys on launch mechanism, only on payload identity. |

## Overlap with existing specs and active changes

- **`claude-code-parity` (open, 36/47).** Direct conflicts: (1) its `fixed-pipeline-enforcement` scenario "Janus's own permissions are unchanged" asserts `tools: Read, Bash` and no `agent` key; this change makes `tools: Read, Bash, Agent(<roster>)`. (2) Its `agent-scaffolding` requirement says every one of the ten agents, `janus` included, carries the same five-command `hooks.Stop` block; this change removes Janus's. (3) Its design rejects both `Agent` for Janus and Janus as main thread. It also adds `janus-dispatch-guardrails` (a prompt-level graph check), which this change keeps as prose in `janus.md`. Because those requirements exist only in that unarchived change, this change cannot MODIFY them in its own deltas. **Archive order: `claude-code-parity` first, then this change, and the two contradicted requirements are amended at that point (task 10.1).** Until then, `openspec validate` passes for both independently.
- **`iktomi-always-handoff-phobetor` (0/13).** Modifies `janus-router-agent`'s "Janus routes free-form requests to Iktomi" requirement; this change modifies two different requirements in the same capability and adds a third. No header overlap. Interaction: `dreamland route`'s free-text default sends work to Iktomi, so that change's "Iktomi always hands off to Phobetor" fixes the completion path for the most common routing outcome. No sequencing constraint.
- **`harden-commit-hook-enforcement` (57/59).** This change reuses its `Blocking`/exit-2 mechanism (`cmd/hookexit.go`, already in the tree). It does not touch `coauthor`, `commit`, `test-and-commit`, or telemetry code. Both edit `settings-patch.json`, but in different `hooks` entries.
- **`parallel-session-otel-receiver` (27/32).** No requirement overlap. Its per-user state directory is where Decision 7's follow-up would live. `dreamland route` deliberately writes no state, so it adds no cross-window contention.
- **`kiro-bedrock-telemetry`, `persist-skill-attachment-edges`.** Unrelated.
- **Main specs.** `session-agent-identity` stays true (`janus` remains the default and a registered identity). `router-slash-commands`' last legacy-skills scenario ("routes through Janus or a documented direct target") stays true if "Janus" is read as the routing decision; the modified requirements above define that reading for Claude Code.

## Risks / Trade-offs

- **[Risk] Frontmatter hooks may not fire for a main-thread `--agent` session on the installed Claude Code version** (the parity design says they do; not verified live). → Mitigation: the workspace-scoped guard binding covers `--agent janus` from `agent_type` alone. Task 0.1 verifies both.
- **[Risk] `agent_type` may be absent from the `PreToolUse` payload under `--agent janus`.** → If so, the workspace-scoped binding sees an unattributed session and applies the default-allow policy, leaving only the agent-scoped binding. Task 0.1 verifies; if both fail, the guard degrades to the existing prompt-only rule and this design's enforcement claim is withdrawn rather than papered over.
- **[Risk] The `Agent(...)` restriction may not behave as documented for `--agent` sessions.** → Task 0.2. Fallback: unrestricted `Agent` plus the guard, with the `general-purpose` laundering risk documented.
- **[Risk] Allowlist over-blocks a legitimate Janus command** (e.g. `openspec change ...`, `git branch`). → Blocking message says what to do and Janus can dispatch; the list is a Go table extended by a one-line change. The failure is loud and recoverable, unlike the current silent overreach.
- **[Risk] Rule table and prose table drift.** → Drift test (spec) plus `next` sentence defers to the binary.
- **[Risk] Tag convention adds a line to every task and an instruction to Phantasos.** → Optional for legacy changes; untagged tasks degrade to `ambiguous`, i.e. today's behavior.
- **[Trade-off] Haiku as a main-thread orchestrator reads specialist reports at Haiku quality.** Accepted: its job is relay, and Janus's `model` can be changed in one place.
- **[Trade-off] Unattributed-session default-allow leaves "main thread does the work in plain claude" unguarded.** Accepted and opt-in-closable; see Decision 3.

## Decisions the user must make

1. **`main_thread_guard` default.** Recommended `allow` (opt-in `block`). Alternative: default `block`, which enforces the invariant for everyone but breaks plain-`claude` use of the repo.
2. **Install `"agent": "janus"` as the project default?** Recommended no. Alternative: yes, which makes every launch mode Janus-orchestrated and fully guarded, at the cost of Haiku as every session's model and overriding personal defaults.
3. **`Agent(...)` restricted list (recommended) versus unrestricted `Agent`.** Restricted needs registry-driven generation and a drift test; unrestricted is simpler and risks `general-purpose` laundering.
4. **Defer per-session Stop identity (recommended) or bundle it.** Bundling touches the files `harden-commit-hook-enforcement` just changed.
5. **Include the optional instruction line on the other five platforms (group 9)?** Recommended yes for the shared binary's sake; cutting it does not affect the Claude Code fix.
6. **Adopt the `[flow: ...]` task tag convention.** Recommended yes; without it every `/opsx:apply` on a new change is `ambiguous` (still decided in-session, no extra hop, but not deterministic).
7. **Archive order**: confirm `claude-code-parity` archives first (task 10.1 depends on it).
