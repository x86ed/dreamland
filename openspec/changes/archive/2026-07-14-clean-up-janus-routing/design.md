## Context

Janus (`internal/scaffold/templates/agents/*/janus.*`) routes purely by an LLM reading prose — there is no structured routing config anywhere in the repo. The originating `janus-router-agent` change (`openspec/changes/janus-router-agent/`) is implemented (120/129 tasks checked) but not yet archived into `openspec/specs/`, so its spec files are still the live, editable source of truth for Janus's required behavior.

Two concrete defects motivate this change:

1. `.claude/skills/openspec-{propose,explore,apply-change,archive-change}/` (legacy names) sit side-by-side with `.claude/commands/opsx/{propose,explore,apply,archive}.md` (current names). Claude Code's skill matcher auto-discovers the legacy skills on a natural-language description match, so a request phrased close to the old skill's description never reaches Janus at all — no routing decision, no `dreamland coauthor`/`telemetry write` hand-off.
2. Nothing in `janus.md`/`iktomi.md` distinguishes "this request names/uses the OpenSpec tooling" from Janus's own definition of `iktomi`'s trigger, "no OpenSpec context at all." A request that simply contains the word "openspec" is at risk of being read as generic/no-context and sent to `iktomi` instead of `phantasos`/`nyx`/`morpheus`/`baku`.

## Goals / Non-Goals

**Goals:**
- Exactly one non-bypassing entry path per OpenSpec lifecycle stage — no skill/command pair that reaches the filesystem/agent layer without going through either a documented direct-route command or Janus.
- Historical `openspec-*` command spellings keep working (same target agent), so existing muscle memory isn't broken by this cleanup.
- Janus's `iktomi` fallback criterion no longer triggers on the literal keyword "openspec."
- Janus's instructions carry an explicit, enumerated table of every command/skill spelling (current and historical) mapped to its target agent, on all six platform templates, consistent with each command file's own documented target (extends the existing "stays consistent with the slash command definitions" requirement).
- Janus's instructions state an explicit out-of-scope refusal: it dispatches, and only dispatches — it does not itself implement, answer, or partially complete the underlying request.

**Non-Goals:**
- Building a machine-readable routing config file (JSON/YAML) that code parses at runtime. The project's existing pattern is prose-embedded instructions per platform; introducing a structured config plus a loader is a materially larger change (new parsing code, new sync-checking, six new file formats to keep in lockstep) than what's needed to fix the mis-routing bug. The enumerated table stays prose, embedded per-platform, same as the rest of Janus's instructions.
- Changing any non-router agent's internal responsibilities, deterministic hand-off edges, or tool bindings — only Janus/Iktomi's routing text and the legacy-skill reconciliation are in scope.
- Archiving the `janus-router-agent` change. This change edits that change's spec files in place since it hasn't been archived yet; archival is a separate, later action.

## Decisions

**Legacy skills become redirect stubs, not deletions.** `.claude/skills/openspec-{propose,explore,apply-change,archive-change}/SKILL.md` are rewritten to a one-line body that delegates to the same target the corresponding `/opsx:*` command already documents (`propose`/`explore` → `phantasos`, `archive` → `baku`, `apply-change` → `janus`, matching the two-flow decision). This preserves old invocation spellings working exactly like their `/opsx:*` counterparts, rather than deleting a working entry point users may have muscle-memoried.
- *Alternative considered*: delete the legacy skills outright. Rejected — the proposal explicitly calls out supporting "all variants of openspec historically," and outright deletion breaks that without a compatibility path.
- *Alternative considered*: leave legacy skills untouched and just document the overlap. Rejected — this is the actual bypass bug; documenting it without fixing it doesn't close the gap.

**Iktomi disambiguation is a rule, not a keyword blocklist.** Rather than pattern-matching the literal string "openspec" (fragile, and the word legitimately appears in on-topic requests), Janus's and Iktomi's instructions gain one explicit sentence: the `iktomi` fallback is based on the *absence of a matching proposal, task list, or spec-scenario context* — never on whether the request happens to mention OpenSpec, the tool names, or command spellings by name.
- *Alternative considered*: a hard keyword denylist ("never route to iktomi if request contains 'openspec'"). Rejected — too brittle (a genuinely free-form request can mention "openspec" in passing, e.g. "does this repo use openspec?", which is legitimately Iktomi's territory) and contradicts the goal of judgment-based routing.

**Routing table enumerates historical names inline, grouped by target agent.** Rather than a separate "aliases" section, each bullet in Janus's existing per-agent routing list gains the historical command/skill spellings that resolve to it, so there's one place per agent to check, matching the existing prose style (`janus.md:14` already shows the pattern `phantasos — ... (/opsx:propose, /opsx:explore)`).
- *Alternative considered*: a standalone lookup table (markdown table) separate from the prose list. Rejected — doubles the surface that must stay in sync (per the existing "Janus's own routing table stays consistent with the slash command definitions" requirement) with no added clarity over extending the existing bullets.

**Scope guardrail is an explicit refusal instruction, not a further tool restriction.** Janus's tools (`Read, Bash`, no `Edit`/`Write`) already make direct file changes impossible; the gap is Janus *answering* a request conversationally or running exploratory `Bash` beyond `openspec status` instead of dispatching. The fix is an instruction-level rule: "If a request asks you to do anything other than decide and dispatch — implement, explain, answer, or investigate beyond `openspec status` — delegate to the appropriate agent instead of complying yourself."
- *Alternative considered*: restrict `Bash` further (e.g. allow-list only `openspec status`). Rejected — over-constrains legitimate diagnostic reads Janus may need before deciding, and the actual failure mode is instruction-level scope creep, not tool misuse.

## Risks / Trade-offs

- **[Risk]** Editing spec files inside a not-yet-archived change (`openspec/changes/janus-router-agent/specs/`) instead of `openspec/specs/` is non-standard OpenSpec flow. → **Mitigation**: this is the correct target precisely because those capabilities haven't been archived yet; the delta files in *this* change (`clean-up-janus-routing/specs/`) apply against that change's current spec content, and both changes' tasks should land before either is archived, so the archive step folds in the corrected version once.
- **[Risk]** Redirect-stub legacy skills could drift from `/opsx:*` commands again in the future if someone edits one without the other. → **Mitigation**: tasks.md should include a step that makes the redirect stub textually reference the `/opsx:*` file (or reuse its content) rather than duplicating routing logic, minimizing duplicate-edit surface.
- **[Risk]** Six platform templates (Claude Code, Codex, Cursor, Kiro, Antigravity, GitHub Copilot) must all get the same fix — easy to update one and miss another, recreating the exact "enumeration gap" this change is meant to close. → **Mitigation**: tasks.md includes an explicit cross-platform parity check as its own step, not folded silently into "update janus.md."

## Migration Plan

1. Update `openspec/changes/janus-router-agent/specs/janus-router-agent/spec.md` and `.../router-slash-commands/spec.md` with the corrected requirements (this change's own `specs/` delta targets those files, since they're the live source of truth pre-archive).
2. Update `internal/scaffold/templates/agents/*/janus.*` and `iktomi.*` (six platforms) with the enumerated table, iktomi disambiguation, and scope guardrail.
3. Rewrite the four legacy skill files under `.claude/skills/openspec-*/SKILL.md` as redirect stubs.
4. Re-run/update the already-installed copies in this repo's own `.claude/` (since `dreamland` is dogfooding its own scaffolding) so the fix is live here immediately, not just in the templates future installs draw from.
5. No rollback complexity beyond a normal git revert — no data migration, no runtime state.

## Open Questions

- Should `openspec-apply-change` redirect straight to `janus` (matching `/opsx:apply`'s through-Janus behavior), or is there a reason to special-case it? Current lean: mirror `/opsx:apply` exactly, no special-case.
