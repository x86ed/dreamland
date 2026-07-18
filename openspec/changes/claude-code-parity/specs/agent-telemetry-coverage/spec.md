## ADDED Requirements

### Requirement: Claude Code hook matchers are unscoped so every subagent type, built-in or dreamland-defined, triggers the identity/telemetry/version-bump/commit chain

`dreamland init`'s Claude Code hook bindings SHALL use unscoped matchers — `""` (or omitted) for `SubagentStop`, and the tool-name matcher `Task|Agent` (which filters by *tool*, not by *target agent type*) for `PreToolUse` — so that `dreamland coauthor`, `dreamland telemetry write --tool claude-code`, `dreamland version-bump --patch`, and `dreamland commit --reason handoff` run on every subagent completion, regardless of whether the completed subagent is one of the ten dreamland agents or one of Claude Code's built-in types (`general-purpose`, `Explore`, `Plan`, `statusline-setup`).

This is a deliberate, tested guarantee, not an accident of the current template: Claude Code's hook-matcher semantics explicitly support filtering `SubagentStop` by agent type (`general-purpose`, `Explore`, `Plan`, or a custom agent's frontmatter `name`) — a narrower matcher naming only the ten dreamland agents would silently exclude built-in subagents from the hook chain the moment someone "tightened" the matcher believing it was scoping out noise. This requirement exists specifically to prevent that regression: the matcher SHALL remain unscoped.

Because this coverage is unconditional, dreamland agents (in particular `iktomi`, dreamland's free-form specialist) MAY rely on Claude Code's built-in subagents for research, exploration, or planning sub-steps without any loss of telemetry, identity, or commit-trail fidelity — the same hook chain fires whether the completed work was done by `iktomi` itself or by a built-in agent Claude Code dispatched in service of the same request. No Claude-Code-specific permission denylist is required to preserve telemetry coverage.

**Platform contrast**: this requirement exists only for Claude Code. GitHub Copilot has no unscoped, agent-type-independent hook mechanism — its hooks are either workspace-wide-but-fixed-event-list (`.github/hooks/*.json`, not filterable by which agent ran) or agent-scoped frontmatter requiring each agent to declare its own `hooks:` block individually. Copilot exposes no built-in free-form agent a workspace-level hook could transparently cover, which is why `iktomi` exists as its own fully-defined Copilot agent (with its own agent-scoped `hooks:` block) in the first place — Copilot has no equivalent to Claude Code's "any subagent, named or built-in, triggers this hook" mechanism for this requirement to apply to.

#### Scenario: Built-in subagent completion triggers the full hook chain

- **WHEN** Claude Code's built-in `general-purpose` (or `Explore`, `Plan`, `statusline-setup`) subagent completes inside a dreamland-scaffolded repository
- **THEN** `.claude/settings.json`'s `SubagentStop` hook fires `dreamland coauthor`, `dreamland telemetry write --tool claude-code`, `dreamland version-bump --patch`, and `dreamland commit --reason handoff`, exactly as it would for any of the ten dreamland agents

#### Scenario: SubagentStop matcher stays unscoped across re-runs of init

- **WHEN** `dreamland init` runs (or re-runs) with "Claude Code" selected
- **THEN** `.claude/settings.json`'s `hooks.SubagentStop` entry has matcher `""` (or is otherwise unscoped), never a matcher naming only the ten dreamland agent names

#### Scenario: PreToolUse matcher targets the dispatch tool, not the destination agent

- **WHEN** `.claude/settings.json` is inspected after `dreamland init` with "Claude Code" selected
- **THEN** the `hooks.PreToolUse` entry's matcher is `Task|Agent` — the tool name used to dispatch *any* subagent — not a list of specific agent names
