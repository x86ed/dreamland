## MODIFIED Requirements

### Requirement: Janus's tool bindings exclude file-editing capabilities

On every platform, Janus's tool/capability binding SHALL exclude any tool capable of writing or editing files (e.g. `Edit`, `Write`, `apply_patch`), retaining only read and dispatch tools (e.g. `Read`, `Bash`, sub-agent invocation).

On Claude Code, Janus's `tools` frontmatter SHALL be `Read, Bash, Agent(<dispatch targets>)`, where `<dispatch targets>` is exactly the set of registered dispatch targets: the nine non-router built-in agents plus every agent registered via `dreamland oneiroi seed`/`fork` (see the `agent-lifecycle-management` capability), and never `janus` itself. The `Agent(...)` grant exists so that Janus, when it is the session's main-thread agent (`claude --agent janus`, or `"agent": "janus"` in settings), can dispatch the specialist its routing decision names rather than having no dispatch capability and doing the work itself. It restricts Janus to the roster: Janus SHALL NOT be able to spawn built-in Claude Code subagents such as `general-purpose`. `Bash` is retained only for read-only routing inputs; the `router-dispatch-guard` capability restricts what a `Bash` call from Janus may do, because `Bash` can otherwise write files and so would silently defeat the exclusion above.

#### Scenario: Janus cannot edit files on Claude Code

- **WHEN** `.claude/agents/janus.md` is installed
- **THEN** its `tools` frontmatter field does not include `Edit`, `Write`, `MultiEdit`, or `NotebookEdit`

#### Scenario: Janus can dispatch exactly the roster on Claude Code

- **WHEN** `.claude/agents/janus.md` is installed on a repository whose registry holds no seeded agents
- **THEN** its `tools` frontmatter field is `Read, Bash, Agent(phantasos, nyx, morpheus, phobetor, baku, iktomi, zhougong, hypnos, mengpo)`
- **AND** the list does not contain `janus`, `general-purpose`, or any other name outside the registered roster

#### Scenario: Codex Janus cannot apply patches

- **WHEN** `.codex/agents/janus.toml` is installed
- **THEN** it does not grant `apply_patch` capability

### Requirement: Janus refuses to act outside the dispatch role

Beyond the existing tool-binding restriction (no `Edit`/`Write`), Janus's instructions SHALL state explicitly that its only valid action is deciding a target agent and dispatching to it. If a request or a mid-routing situation asks Janus to implement a change, answer the substance of a question, or investigate beyond what `openspec status` (or platform equivalent) provides, Janus SHALL delegate that request to the appropriate agent (typically `iktomi` if no specialized agent fits) rather than performing or attempting the work itself.

On Claude Code this refusal is additionally enforced by the `router-dispatch-guard` capability rather than relying on the prompt alone: a file-modifying tool call, or a `Bash`/`PowerShell` call outside the read-only allowlist, made under Janus's identity is blocked with a message naming the dispatch step.

#### Scenario: Janus declines to answer a substantive question directly

- **WHEN** a request asks Janus to explain, implement, or otherwise directly resolve something rather than route it
- **THEN** Janus's instructions direct it to delegate the request to the matching agent (or `iktomi` if none fits) instead of responding to the substance itself

#### Scenario: Janus's diagnostic reads stay bounded to routing decisions

- **WHEN** Janus needs information to decide where to route a request
- **THEN** its instructions limit that investigation to what's needed for the routing decision (e.g. `openspec status`, `dreamland route`), not open-ended exploration of the codebase on Janus's own behalf

#### Scenario: A Bash write attempted as Janus is blocked on Claude Code

- **WHEN** a session whose acting identity is `janus` attempts a `Bash` call that writes a file (e.g. `sed -i` or a `>` redirect)
- **THEN** the `router-dispatch-guard` blocks the call and the file is not modified

## ADDED Requirements

### Requirement: On Claude Code, Janus is the in-session orchestrator and is never spawned as a routing subagent

On Claude Code, no routing entry point (`/dreamland`, `/drmlnd:route`, `/janus`, `/opsx:apply`, `openspec-apply-change`, and the per-agent `/drmlnd:<agent>` commands) SHALL make the `Agent` tool call `janus` to obtain a routing decision. "Janus" on Claude Code denotes the orchestrator role played by whichever session is at the top of the conversation: either the main-thread agent when the session was launched as `janus`, or the plain main session when a routing command was invoked. That session obtains the decision by running `dreamland route` (see the `deterministic-routing` capability), dispatches the returned `target` with the `Agent` tool forwarding the request verbatim (attachments included), and, only when the result is `ambiguous`, chooses among the returned candidates itself or asks the user.

The name `janus` SHALL remain a registered agent identity: it is the fallback identity for every turn that has no dispatched subagent (see the `session-agent-identity` capability), the value of `agent_type` under `claude --agent janus`, and the identity Copilot/Codex/Cursor/Kiro/Antigravity route through. `.claude/agents/janus.md` SHALL remain installed so `claude --agent janus` works and so that the roster registration mechanism (see the `agent-lifecycle-management` capability) keeps one routing table.

`janus.md`'s instruction body SHALL define the orchestrator protocol in at most a short paragraph that defers the rules to `dreamland route`'s output rather than restating them: run `dreamland route` with the request; if `decision` is `target`, dispatch it; if `ambiguous`, choose among `candidates` or ask the user when `ask_user` is true; never perform the work itself. The routing table in `janus.md` SHALL be retained as the human-readable statement of the same rules and is checked against the binary's rule table by test (see the `deterministic-routing` capability's consistency requirement).

#### Scenario: Plain session runs /dreamland without spawning janus

- **WHEN** a plain `claude` session (no `--agent`) invokes `/dreamland add a retry to the uploader` in a repository with no active OpenSpec change
- **THEN** the session runs `dreamland route`, receives `decision: target` with `target: iktomi`, and dispatches `iktomi` with the `Agent` tool
- **AND** no `Agent` call with `subagent_type` `janus` is made

#### Scenario: claude --agent janus dispatches the routed target

- **WHEN** a session launched with `claude --agent janus` receives a free-form request
- **THEN** Janus runs `dreamland route`, and dispatches the returned target with its `Agent(...)` grant rather than performing the work itself

#### Scenario: Ambiguous result is decided in-session, not by a second agent

- **WHEN** `dreamland route` returns `decision: ambiguous` with `ask_user: false` and candidates `nyx` and `morpheus`
- **THEN** the orchestrating session chooses one candidate using its own conversation context and dispatches it, without spawning any additional routing agent

#### Scenario: Ambiguity that needs the user asks the user

- **WHEN** `dreamland route` returns `decision: ambiguous` with `ask_user: true` (e.g. several active changes and the request names none)
- **THEN** the orchestrating session asks the user which candidate to proceed with, presenting the returned candidates, and does not guess

#### Scenario: Janus remains a valid identity and launch target

- **WHEN** `dreamland init` completes with "Claude Code" selected
- **THEN** `.claude/agents/janus.md` exists and `janus` is still one of the registered agent names for identity resolution
