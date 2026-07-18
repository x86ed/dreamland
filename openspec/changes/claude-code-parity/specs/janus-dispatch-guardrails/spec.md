## ADDED Requirements

### Requirement: Janus's Claude Code instructions validate hand-off context against the known agent graph before dispatching

At every point Janus is actually invoked on Claude Code — entry dispatch for a new request, an agent reporting back on an ambiguous or terminal case, or a broad-routing agent (`iktomi`/`zhougong`/`hypnos`/`mengpo`) reporting when its own work doesn't point to a next agent — `janus.md`'s instruction body SHALL direct it to check any hand-off suggestion in that context against the three-tier agent graph (the same graph `agent-scaffolding`'s GitHub Copilot requirement encodes structurally via `agents:` — router reaches all nine; narrow deterministic agents reach only their fixed one-to-three targets; broad-routing agents reach any of nine) before dispatching, and to decide independently per its own routing table and `openspec status` rather than complying with a suggested target that doesn't belong in that context.

This is instruction-level, not code-enforced. Unlike `fixed-pipeline-enforcement`'s `PreToolUse` write-guard, there is no hook that can inspect "which agent is Janus about to dispatch to" — the main thread's dispatch call carries no structured, checkable record of *why* Janus chose a target, only the `Agent` tool invocation itself. The guardrail here is Janus's own judgment, made explicit in its prompt rather than left implicit in "pick a target agent and dispatch."

This requirement does **not** change the deterministic direct hand-offs `janus-router-agent` already documents (`nyx`→`morpheus`→`phobetor`→`baku`, etc.) — those hops continue to bypass Janus entirely, unmediated, exactly as before. Janus is only ever a checkpoint at the points it was already a checkpoint.

#### Scenario: Janus disregards an out-of-graph hand-off suggestion

- **WHEN** Janus is invoked with context suggesting a hand-off outside the valid graph for that situation (e.g. a report framed as needing to jump directly to `baku` from a state that doesn't warrant it)
- **THEN** Janus's instructions direct it to disregard that suggestion and apply its own routing table / `openspec status` instead, rather than dispatching to the suggested target

#### Scenario: Deterministic direct hand-offs are unaffected

- **WHEN** `nyx`'s turn ends and its next step is the deterministic hand-off to `morpheus`
- **THEN** the main thread dispatches `morpheus` directly, exactly as before this capability existed — Janus is not invoked for this hop, and this requirement does not change that
