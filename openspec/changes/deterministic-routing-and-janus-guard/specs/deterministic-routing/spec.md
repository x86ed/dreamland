## ADDED Requirements

### Requirement: `dreamland route` resolves a routing target without an LLM call

The `dreamland` binary SHALL provide `dreamland route [--to <agent>] [--command <spelling>] [--change <name>] [--stdin] [--] [request...]`. It SHALL resolve which agent a request should be dispatched to using only the invocation arguments, the request text, the agent registry, and the OpenSpec CLI's read-only output (`openspec list --json`, `openspec status --change <name> --json`) plus the selected change's `tasks.md`. It SHALL NOT call any model, network service, or agent. The request is taken from the trailing arguments, or from stdin when `--stdin` is given (so callers can pass arbitrary text without shell quoting).

`dreamland route` SHALL write nothing: no files, no `git config`, no `.dreamland-session.json`, no telemetry. It is safe to run concurrently from any number of sessions in the same repository.

Output SHALL be a single JSON object on stdout with these fields:

| Field | Presence | Meaning |
| --- | --- | --- |
| `decision` | always | `"target"` or `"ambiguous"` |
| `target` | iff `decision` is `target` | a registered agent name |
| `rule` | always | stable rule identifier from the tables below |
| `reason` | always | one sentence, human-readable |
| `change`, `task` | when derived from OpenSpec state | change name; number and text of the first unchecked task |
| `candidates` | iff `ambiguous` | array of `{agent, change?, why}`, at least two entries |
| `ask_user` | iff `ambiguous` | `true` when the choice depends on user intent that no candidate can infer (e.g. which of several active changes), else `false` |
| `roster` | iff `ambiguous` | array of `{agent, role}` for every registered dispatch target, so a newly seeded agent is visible without editing any prompt |
| `next` | always | one imperative sentence telling the orchestrator what to do, e.g. "Dispatch `morpheus` with the Agent tool; forward the request verbatim, attachments included." |

An internal error (unreadable registry, `openspec` unavailable when a state rule needs it) SHALL NOT produce a guessed target: the command exits 1 (non-blocking) with a message on stderr, and callers treat that as `ambiguous` with `ask_user: false`. It SHALL NEVER exit 2, so it can never block a session's lifecycle event.

#### Scenario: Output is valid JSON with required fields and no side effects

- **WHEN** `dreamland route -- "fix the flaky test in cmd/serve_test.go"` runs in a repository with no active OpenSpec change
- **THEN** stdout is one JSON object with `decision`, `rule`, `reason`, and `next`
- **AND** `git status --porcelain` and `.dreamland-session.json` are byte-identical before and after

#### Scenario: Request is read from stdin without shell interpretation

- **WHEN** `dreamland route --stdin` receives the literal text `use "quotes", $HOME and backticks` on stdin
- **THEN** the request is treated as that literal text and no expansion occurs inside the command

#### Scenario: Internal failure never yields a guessed target

- **WHEN** `dreamland route --command opsx:apply` runs and the `openspec` executable cannot be found
- **THEN** it exits 1 with a stderr message and prints no `target`
- **AND** it does not exit 2

### Requirement: Routing rules are evaluated in a fixed order, first match wins

`dreamland route` SHALL evaluate the following rules in order and return on the first that matches. Rule identifiers are part of the output contract.

| # | `rule` | Matches when | Result |
| --- | --- | --- | --- |
| 1 | `explicit-target` | `--to <agent>` is given | `target` = that agent if it is registered; otherwise exit 1 (never fall through to another rule) |
| 2 | `command-spelling` | `--command` is `opsx:propose`, `opsx:explore`, `openspec-propose`, or `openspec-explore` | `target: phantasos` |
| 2 | `command-spelling` | `--command` is `opsx:archive` or `openspec-archive-change` | `target: baku` |
| 3 | `state-*` | `--command` is `opsx:apply` or `openspec-apply-change`; or the request is empty; or the request (trimmed, case-insensitive) is one of `continue`, `next`, `resume`, `go`, `what's next`, `whats next` | the state resolver below |
| 4 | `intent-keyword` | the request, trimmed, starts with a phrase from the embedded intent table | `target` per the table |
| 5 | `no-openspec-context` | no active change exists | `target: iktomi` |
| 6 | `free-text-with-active-change` | otherwise (non-empty free text and at least one active change) | `ambiguous`; candidates are `iktomi` plus the agent the state resolver would return for the selected change; `ask_user: false` |

Initial intent table for rule 4 (anchored at the start of the request, case-insensitive):

| Phrase prefix | Target |
| --- | --- |
| `propose`, `draft a proposal`, `draft a spec`, `write a spec`, `new agent`, `create an agent`, `retire an agent`, `retire agent`, `delete an agent` | `phantasos` (agent-roster changes are drafted first, per the `janus-router-agent` capability) |
| `archive`, `close out` | `baku` |
| `agent performance`, `token usage`, `tokens used`, `tune the agents` | `zhougong` |

The table SHALL live in one Go data structure and SHALL NOT match on keywords that appear later in the request, so a request that merely mentions "openspec" or "archive" mid-sentence is not misrouted.

Free text that reaches rule 5 goes to `iktomi`, not to a specialist: `iktomi` is the catch-all and MAY redirect directly to any specialist when its work turns out to fit one (see the `janus-router-agent` capability), so a wrong default costs one redirect rather than a wrong pipeline.

**State resolver.** An *active change* is one returned by `openspec list --json` that is not archived. Change selection, in order: `--change <name>`; a change slug that appears as a whole token in the request; the single active change when exactly one exists; otherwise, with several active changes, `ambiguous` with `rule: state-multiple-changes`, `ask_user: true`, one candidate per active change. When rule 3 is reached and no change is active at all (`/opsx:apply` or "continue" with nothing to continue), the result is `ambiguous` with `rule: state-no-active-change`, `ask_user: true`, candidates `phantasos` (draft a change first) and `iktomi` (free-form work). For the selected change, in order:

1. Any artifact reported by `openspec status --change <name> --json` that is not `done` → `target: phantasos`, `rule: state-artifacts-incomplete`.
2. `tasks.md` absent → `target: phantasos`, `rule: state-artifacts-incomplete`.
3. All tasks checked → `target: baku`, `rule: state-tasks-complete`.
4. Otherwise take the first unchecked task (`- [ ]`). If its line carries a flow tag `[flow: nyx]`, `[flow: morpheus]`, `[flow: hypnos]`, or `[flow: mengpo]` → that agent, `rule: state-task-tagged`. A tag naming an unregistered agent exits 1.
5. Untagged → `ambiguous`, `rule: state-task-untagged`, `ask_user: false`, candidates `nyx` and `morpheus`, plus `hypnos` and `mengpo` when the task text contains `agent`, `routing edge`, `hook`, or `skill` (word-bounded, case-insensitive). A task is never silently defaulted to one flow, because choosing `morpheus` for new behavior would skip the acceptance-test step.

#### Scenario: Explicit target from a per-agent command

- **WHEN** `dreamland route --to morpheus -- "do the thing"` runs
- **THEN** it returns `decision: target`, `target: morpheus`, `rule: explicit-target`

#### Scenario: Explicit target that is not registered is an error, not a fallback

- **WHEN** `dreamland route --to not-an-agent` runs
- **THEN** it exits 1 and does not return `iktomi` or any other target

#### Scenario: Archive command spelling resolves to Baku

- **WHEN** `dreamland route --command opsx:archive` runs
- **THEN** it returns `target: baku`, `rule: command-spelling`

#### Scenario: Apply with all artifacts done and a tagged first task

- **WHEN** `dreamland route --command opsx:apply` runs with exactly one active change, all artifacts `done`, and first unchecked task `- [ ] 2.1 add retry to uploader [flow: nyx]`
- **THEN** it returns `target: nyx`, `rule: state-task-tagged`, `change` set to that change, and `task` starting with `2.1`

#### Scenario: Apply with an untagged task is ambiguous, not defaulted

- **WHEN** `dreamland route --command opsx:apply` runs and the first unchecked task carries no `[flow: ...]` tag
- **THEN** it returns `decision: ambiguous`, `rule: state-task-untagged`, `ask_user: false`, and candidates including `nyx` and `morpheus`

#### Scenario: Apply with every task checked routes to Baku

- **WHEN** `dreamland route --command opsx:apply` runs and every task in the selected change is `- [x]`
- **THEN** it returns `target: baku`, `rule: state-tasks-complete`

#### Scenario: Incomplete artifacts route to Phantasos

- **WHEN** `dreamland route --command opsx:apply` runs and `openspec status` reports `design` not `done`
- **THEN** it returns `target: phantasos`, `rule: state-artifacts-incomplete`

#### Scenario: Several active changes and an unnamed request require the user

- **WHEN** `dreamland route` runs with empty request text and two active changes
- **THEN** it returns `decision: ambiguous`, `rule: state-multiple-changes`, `ask_user: true`, with one candidate per active change

#### Scenario: Free text with no OpenSpec context goes to Iktomi

- **WHEN** `dreamland route -- "why does the build fail on windows?"` runs with no active change
- **THEN** it returns `target: iktomi`, `rule: no-openspec-context`

#### Scenario: Free text with an active change is ambiguous but needs no user

- **WHEN** `dreamland route -- "rename the helper in cmd/serve.go"` runs with one active change whose next task is tagged `[flow: morpheus]`
- **THEN** it returns `decision: ambiguous`, `rule: free-text-with-active-change`, `ask_user: false`, with candidates `iktomi` and `morpheus`

#### Scenario: A keyword mid-sentence does not trigger the intent table

- **WHEN** `dreamland route -- "please explain how openspec archive works"` runs with no active change
- **THEN** it returns `target: iktomi`, `rule: no-openspec-context`, not `baku`

#### Scenario: Agent-performance request routes to Zhou Gong

- **WHEN** `dreamland route -- "token usage by agent this week"` runs
- **THEN** it returns `target: zhougong`, `rule: intent-keyword`

### Requirement: Valid targets are the registered roster, resolved at run time

Every `target`, candidate `agent`, and `--to` value SHALL be checked against the agent registry (`agentidentity.Registered(repoRoot)`: the built-in ten plus `.dreamland/oneiroi/registry.json`) at run time. `janus` SHALL NOT be a valid `target`. `dreamland route` SHALL require no per-agent registration step of its own: a freshly seeded agent is immediately a valid `--to` value and appears in `roster` on the next `ambiguous` result, with no rebuild and no edit to any rule table. Seeded agents are never returned by rules 3-6 (their triggers are not deterministic); they are reached by `--to` (their per-agent slash command) or by the orchestrator choosing from `roster`.

#### Scenario: Seeded agent is a valid explicit target immediately

- **WHEN** `dreamland oneiroi seed --role "example"` has just registered `amber-falcon` and `dreamland route --to amber-falcon` runs
- **THEN** it returns `target: amber-falcon`, `rule: explicit-target`

#### Scenario: Roster in an ambiguous result includes seeded agents with their roles

- **WHEN** `dreamland route` returns `decision: ambiguous` in a repository with `amber-falcon` registered with role `example role`
- **THEN** `roster` contains an entry with `agent` `amber-falcon` and `role` `example role`

#### Scenario: Janus is never a routing target

- **WHEN** `dreamland route --to janus` runs
- **THEN** it exits 1

### Requirement: The binary's rule table and Janus's routing table cannot drift

A Go test SHALL fail when any of the following diverge: (a) every agent named as a target by rules 1-6 is registered and appears in the routing table of every platform's installed `janus.*` template; (b) every `--command` spelling recognized by rule 2 and rule 3 appears in the `janus.*` routing-table entry for the same agent (the `janus-router-agent` capability already requires the table to enumerate every command spelling); (c) the flow-tag names in the state resolver equal the set `{nyx, morpheus, hypnos, mengpo}` documented in `janus.*`'s `/opsx:apply` entry. The human-readable table in `janus.*` remains authoritative for platforms where Janus's own judgment routes; the binary implements the deterministic subset of it.

#### Scenario: Adding a rule target that Janus's table omits fails the build

- **WHEN** a rule is added that returns an agent absent from `.claude/agents/janus.md`'s routing table template
- **THEN** the consistency test fails

### Requirement: Task flow tags are part of the tasks.md convention Phantasos writes

Phantasos's instructions on every platform SHALL direct it to end each checkbox task line in a `tasks.md` with exactly one flow tag: `[flow: nyx]` (new externally observable behavior with no covering test), `[flow: morpheus]` (mechanical or internal work, or a covering test already exists), `[flow: hypnos]` (creates an agent or changes workflow-graph structure), or `[flow: mengpo]` (retires an agent). A `tasks.md` written before this convention (no tags) SHALL keep working: `dreamland route` returns `ambiguous` for such tasks per the state resolver and the orchestrator decides, exactly as Janus did before.

#### Scenario: Phantasos instructions carry the tag convention on every platform

- **WHEN** any platform's `phantasos.*` agent file is installed
- **THEN** its instruction body names the four `[flow: ...]` tags and states that every task line ends with exactly one

#### Scenario: Legacy tasks.md without tags still routes

- **WHEN** the first unchecked task of a change has no flow tag
- **THEN** `dreamland route --command opsx:apply` returns `ambiguous` rather than an error

### Requirement: `dreamland route` is platform-independent and Windows-safe

`dreamland route` SHALL contain no dependence on a specific coding tool, on POSIX path separators, or on a POSIX shell: repository paths are handled with `filepath` and normalized with `filepath.ToSlash` before comparison; the `openspec` executable is located with `exec.LookPath` (so the `openspec.cmd` shim resolves on Windows) and invoked without a shell. Callers on Claude Code pass free text through `--stdin`; that choice assumes Claude Code's `Bash` tool (Git Bash on Windows) and is documented as such in the command files.

#### Scenario: Windows path in a change directory is handled

- **WHEN** the change's `tasks.md` is located under a path containing backslashes on Windows
- **THEN** `dreamland route` reads it and returns the same result it would on macOS or Linux
