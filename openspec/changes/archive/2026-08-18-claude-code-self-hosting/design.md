## Context

The Claude Code binding was built by generalizing from the GitHub Copilot binding, which does expose an `agent_type` field on `SubagentStart`/`SubagentStop` hook payloads and was verified live. The Claude Code path was never verified against real Claude Code hook payloads and carries two wrong assumptions baked into both `openspec/specs/dev-workflow-hooks/spec.md` and `cmd/coauthor.go`:

1. A `CLAUDE_AGENT_ID` environment variable exists. It does not — Claude Code does not set a per-agent env var for Task-tool sub-agents.
2. Hook stdin payloads carry `agent_type`. On Claude Code, the field that identifies which sub-agent is running is `tool_input.subagent_type`, and it only appears on the `PreToolUse`/`PostToolUse` payload for the `Task` tool call itself — not on `SessionStart`, `Stop`, or `SubagentStop` payloads, which carry `session_id`/`transcript_path`/`hook_event_name` instead.

Separately, this repository never applied its own scaffolder to itself: `.claude/` has `commands/` and `skills/` but no `agents/` directory, and `.claude/settings.json` has only a single `Stop` → `pre-merge-check.sh` hook, none of the SessionStart/PreToolUse/SubagentStop/Stop entries already authored in `internal/scaffold/templates/hooks/bindings/claude-code/settings-patch.json`. The `.claude/commands/drmlnd/*.md` files that exist are prose instructions telling the top-level assistant to roleplay as an agent ("delegate to the `janus` agent") — there is no real, tool-restricted Task-tool sub-agent for it to delegate *to*.

## Goals / Non-Goals

**Goals:**
- Make agent-identity resolution correct for Claude Code's actual hook payload shape, without breaking the already-verified GitHub Copilot path.
- Give every Claude Code session under dreamland an explicit, valid agent identity by default, so telemetry/commits never fall back to a generic or unrelated identity.
- Make version bumping deterministic (hook-driven) rather than dependent on an agent remembering a prose instruction.
- Add bare per-agent slash commands without removing the existing `/drmlnd:*` namespaced ones.
- Bring this repository itself up to the state `dreamland init` is supposed to produce (self-hosting / dogfooding).

**Non-Goals:**
- Redesigning the ten-agent roster or the Janus routing table (`janus-router-agent` capability is unchanged).
- Changing the CLI flag surface of `version-bump`, `coauthor`, `transition-log`, or `test`.
- Fixing identity resolution for platforms other than Claude Code (Copilot's `agent_type` path is already correct; Cursor/Codex/Kiro/Antigravity are out of scope for this change).
- Building a general "plugin" mechanism for arbitrary third-party agents — the ten dreamland agents remain the fixed, closed roster.

## Decisions

**1. Identity resolution: layered lookup, Claude Code path added, nothing removed.**
`agentNameFromHookPayload()` gains a second key check: after failing to find top-level `agent_type`, it also checks `payload["tool_input"].(map)["subagent_type"]`. `resolveAgentName()`'s env-var loop is unchanged (harmless dead lookups on Claude Code, still correct for whichever platform does set them). Alternative considered: replace the whole identity mechanism with a single `.dreamland-session.json` file written by a `SessionStart`/`PreToolUse` hook and read by every downstream hook — rejected for this change because it's a larger structural change than the bug requires; captured as an open question below in case the layered lookup proves fragile in practice.

**2. Session identity default: `SessionStart` hook writes an explicit identity, defaulting to `janus`.**
Rather than leaving identity undefined until a sub-agent Task call happens (which is how the current bug produces a generic fallback for the *first* turn of every session), the `SessionStart` hook entry writes the resolved identity — or `janus` if nothing resolves — to the same session-scoped state `coauthor` already reads. This directly satisfies "no default agents, no unknown agents": there is always a named dreamland agent on record, and if a hook payload ever supplies a name outside the ten registered agents, `coauthor` treats it as unresolved and falls back to `janus` rather than trusting an arbitrary string. Alternative considered: hard-fail the session if identity can't be resolved — rejected, since a hard failure on every ordinary Claude Code turn (most of which are plain `janus`-routed conversation, not a named sub-agent invocation) would be far more disruptive than defaulting to the router agent.

**3. Bare slash commands are added as aliases alongside `/drmlnd:*`, not a replacement.**
`router-slash-commands` already carries a deliberate requirement ("No unprefixed dreamland command artifacts remain after install", from the prior `18-namespace-all-commands-to-dreamland` change) that the namespaced form is the collision-safe default. Explicitly confirmed with the user rather than assumed: this change adds a second, bare-named command per agent (`/janus`, `/nyx`, `/phobetor`, ...) and a bare `/dreamland` generic entry point, generated from the same per-agent template as its `/drmlnd:<agent>` counterpart and differing only in file location (`.claude/commands/<agent>.md` vs `.claude/commands/drmlnd/<agent>.md`) — the namespaced set is not removed. This reintroduces the exact collision risk the prior change eliminated (a user's own `/janus.md` would be overwritten by `dreamland init`); see Risks. Alternatives considered: renaming `drmlnd` → `dreamland` (keeps collision safety, fixes the abbreviation-vs-PR-title mismatch, but doesn't give bare names) and dropping the namespace entirely (bare names only, reverts the prior change outright) — both rejected by the user in favor of keeping both forms.

**4. Change-scoped minor bump moves from prose to a real hook.**
`phantasos.md` currently just *tells* the agent to run `dreamland version-bump --change <slug>` after `openspec new change` succeeds. Since Claude Code hooks can't currently match on arbitrary Bash command content pre-execution in a way that reliably captures the change slug, the trigger point moves to a `PostToolUse` hook matching `Bash` with a command-output grep for `openspec new change`, extracting `<slug>` from the tool input rather than the agent's memory. This is scoped narrowly to the `openspec new` / `openspec change create` command shapes already used by the `opsx:propose` skill. Alternative considered: keep it in prose but add a `Stop`-hook lint that fails the turn if a new change directory exists without a corresponding `change-bumps` entry — rejected as a worse user experience (fails after the fact instead of just doing the bump).

**5. Self-hosting is applied by running the fixed scaffolder against this repo, not by hand-authoring `.claude/agents/*.md`.**
Once decisions 1-4 land in `internal/scaffold/templates/...`, `dreamland init` (or a targeted re-run of the agent/hook scaffolding step) is executed against the dreamland repo itself so the installed files are exactly what any consumer of dreamland would get — keeping this repo a faithful dogfood instance instead of a hand-maintained fork of the templates. The existing `pre-merge-check.sh` Stop hook entry must survive the merge (settings-patch application appends/merges hook arrays; it must not clobber existing entries under the same event key).

## Risks / Trade-offs

- [Claude Code changes its hook payload shape again in a future release] → Identity resolution already degrades gracefully today (falls back to `janus` instead of crashing); no new failure mode introduced, and the fallback path is exercised by tests either way.
- [Merging `settings-patch.json` into a `.claude/settings.json` that already has a hand-written `Stop` hook could silently drop or duplicate entries] → Task requires an explicit test asserting `pre-merge-check.sh` and the new dreamland `Stop` hooks both survive the merge, in that order or documented order.
- [`PostToolUse` slug-extraction for the change-scoped bump is coupled to today's `openspec new change <name>` CLI shape] → If the OpenSpec CLI's argument shape changes, the hook silently stops firing (same failure mode as today, not a regression) rather than erroring loudly; flagged as an open question below.
- [Bare command names collide with a user's own custom slash commands in a downstream consumer's repo — the exact risk the prior `18-namespace-all-commands-to-dreamland` change eliminated] → Accepted trade-off, explicitly chosen by the user over the namespace-rename and full-revert alternatives; `dreamland init` SHALL still refuse to overwrite a pre-existing bare command file it did not itself install (same non-destructive-write behavior already used elsewhere in scaffolding), so the collision surfaces as a skipped file, not a silent clobber.

## Open Questions

- Should identity resolution eventually move to a single `.dreamland-session.json` written once per session rather than layered per-hook lookups, to reduce the number of places that need updating if Claude Code's payload shape changes again?
- Is grepping the `Bash` tool input for `openspec new change` robust enough, or should the change-scoped bump instead be driven by a filesystem watch on `openspec/changes/*/` appearing (platform-agnostic, but requires a hook type dreamland doesn't currently use)?
