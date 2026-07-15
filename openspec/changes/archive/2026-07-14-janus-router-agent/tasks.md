## 1. Rename orchestrator to janus across agent templates

- [x] 1.1 Rename `internal/scaffold/templates/agents/claude-code/orchestrator.md` to `janus.md`; update frontmatter `name: janus`, add `role: router`, keep `tools: Read, Bash`
- [x] 1.2 Rename `internal/scaffold/templates/agents/codex/orchestrator.toml` to `janus.toml`; update `name = "janus"`, add `role = "router"`, confirm no `apply_patch` capability is granted
- [x] 1.3 Rename `internal/scaffold/templates/agents/cursor/orchestrator.mdc` to `janus.mdc`; update frontmatter, add "pure router, never uses edit tools" statement to the instruction body
- [x] 1.4 Rename `internal/scaffold/templates/agents/kiro/orchestrator.md` to `janus.md`; add "pure router" statement to the instruction body
- [x] 1.5 Rename `internal/scaffold/templates/agents/antigravity/orchestrator/` directory to `janus/`; update `SKILL.md` frontmatter (`name: janus`) and add "pure router" statement
- [x] 1.6 Rename `internal/scaffold/templates/agents/github-copilot/orchestrator.agent.md` to `janus.agent.md`; update frontmatter `name: janus`, add `role: router`, confirm `tools` excludes `Edit`/`Write`

## 2. Rename pr-closer to baku across agent templates

- [x] 2.1 Rename `internal/scaffold/templates/agents/claude-code/pr-closer.md` to `baku.md`; update frontmatter `name: baku`
- [x] 2.2 Rename `internal/scaffold/templates/agents/codex/pr-closer.toml` to `baku.toml`; update `name = "baku"`
- [x] 2.3 Rename `internal/scaffold/templates/agents/cursor/pr-closer.mdc` to `baku.mdc`; update frontmatter
- [x] 2.4 Rename `internal/scaffold/templates/agents/kiro/pr-closer.md` to `baku.md`
- [x] 2.5 Rename `internal/scaffold/templates/agents/antigravity/pr-closer/` directory to `baku/`; update `SKILL.md` frontmatter (`name: baku`)
- [x] 2.6 Rename `internal/scaffold/templates/agents/github-copilot/pr-closer.agent.md` to `baku.agent.md`; update frontmatter `name: baku`
- [x] 2.7 In each renamed file, update any self-referential mentions of the old `pr-closer` name in the instruction body to `baku`

## 3. Rename spec-writer to phantasos across agent templates

- [x] 3.1 Rename `internal/scaffold/templates/agents/claude-code/spec-writer.md` to `phantasos.md`; update frontmatter `name: phantasos`
- [x] 3.2 Rename `internal/scaffold/templates/agents/codex/spec-writer.toml` to `phantasos.toml`; update `name = "phantasos"`
- [x] 3.3 Rename `internal/scaffold/templates/agents/cursor/spec-writer.mdc` to `phantasos.mdc`; update frontmatter
- [x] 3.4 Rename `internal/scaffold/templates/agents/kiro/spec-writer.md` to `phantasos.md`
- [x] 3.5 Rename `internal/scaffold/templates/agents/antigravity/spec-writer/` directory to `phantasos/`; update `SKILL.md` frontmatter (`name: phantasos`)
- [x] 3.6 Rename `internal/scaffold/templates/agents/github-copilot/spec-writer.agent.md` to `phantasos.agent.md`; update frontmatter `name: phantasos`
- [x] 3.7 In each renamed file, update any self-referential mentions of the old `spec-writer` name in the instruction body to `phantasos`

## 4. Rename tester to phobetor across agent templates

- [x] 4.1 Rename `internal/scaffold/templates/agents/claude-code/tester.md` to `phobetor.md`; update frontmatter `name: phobetor`
- [x] 4.2 Rename `internal/scaffold/templates/agents/codex/tester.toml` to `phobetor.toml`; update `name = "phobetor"`
- [x] 4.3 Rename `internal/scaffold/templates/agents/cursor/tester.mdc` to `phobetor.mdc`; update frontmatter
- [x] 4.4 Rename `internal/scaffold/templates/agents/kiro/tester.md` to `phobetor.md`
- [x] 4.5 Rename `internal/scaffold/templates/agents/antigravity/tester/` directory to `phobetor/`; update `SKILL.md` frontmatter (`name: phobetor`)
- [x] 4.6 Rename `internal/scaffold/templates/agents/github-copilot/tester.agent.md` to `phobetor.agent.md`; update frontmatter `name: phobetor`
- [x] 4.7 In each renamed file, update any self-referential mentions of the old `tester` name in the instruction body to `phobetor`; role and instructions (run the suite, check spec scenarios, report failures) stay otherwise unchanged

## 5. Rename implementer to morpheus across agent templates

- [x] 5.1 Rename `internal/scaffold/templates/agents/claude-code/implementer.md` to `morpheus.md`; update frontmatter `name: morpheus`
- [x] 5.2 Rename `internal/scaffold/templates/agents/codex/implementer.toml` to `morpheus.toml`; update `name = "morpheus"`
- [x] 5.3 Rename `internal/scaffold/templates/agents/cursor/implementer.mdc` to `morpheus.mdc`; update frontmatter
- [x] 5.4 Rename `internal/scaffold/templates/agents/kiro/implementer.md` to `morpheus.md`
- [x] 5.5 Rename `internal/scaffold/templates/agents/antigravity/implementer/` directory to `morpheus/`; update `SKILL.md` frontmatter (`name: morpheus`)
- [x] 5.6 Rename `internal/scaffold/templates/agents/github-copilot/implementer.agent.md` to `morpheus.agent.md`; update frontmatter `name: morpheus`
- [x] 5.7 In each renamed file, update any self-referential mentions of the old `implementer` name in the instruction body to `morpheus`

## 6. Add Nyx as a new agent (acceptance-test writer) across all platforms

- [x] 6.1 Create `internal/scaffold/templates/agents/claude-code/nyx.md` with frontmatter `name: nyx`, `description` describing it as the acceptance-test writer, `tools: Read, Edit, Write, Bash`; instruction body: when the current task implements new behavior described by a spec scenario (WHEN/THEN) with no covering test, write a failing acceptance test for that scenario before any implementation exists, then hand off directly to `morpheus` (a deterministic next step — see the `janus-router-agent` capability)
- [x] 6.2 Create `internal/scaffold/templates/agents/codex/nyx.toml` with the same content in Codex's TOML format
- [x] 6.3 Create `internal/scaffold/templates/agents/cursor/nyx.mdc` with the same instruction body
- [x] 6.4 Create `internal/scaffold/templates/agents/kiro/nyx.md` as a plain markdown steering document
- [x] 6.5 Create `internal/scaffold/templates/agents/antigravity/nyx/SKILL.md` with the same instruction body
- [x] 6.6 Create `internal/scaffold/templates/agents/github-copilot/nyx.agent.md` with `tools: Read, Edit, Write, Bash`
- [x] 6.7 In each file, confirm Nyx hands off directly to `morpheus` once the acceptance test is written, not to Janus (this is a deterministic hop, not a judgment call)

## 7. Add Iktomi as a new agent (free-form coding fallback) across all platforms

- [x] 7.1 Create `internal/scaffold/templates/agents/claude-code/iktomi.md` with frontmatter `name: iktomi`, `description` describing it as a general-purpose, free-form coding agent selected when no specialized agent fits, `tools: Read, Edit, Write, Bash`; instruction body: handle the request directly, using good judgment; if the work turns out to fit a specialized agent's role, hand off directly to that agent; otherwise report completion or blockers to Janus when done
- [x] 7.2 Create `internal/scaffold/templates/agents/codex/iktomi.toml` with the same content in Codex's TOML format
- [x] 7.3 Create `internal/scaffold/templates/agents/cursor/iktomi.mdc` with the same instruction body
- [x] 7.4 Create `internal/scaffold/templates/agents/kiro/iktomi.md` as a plain markdown steering document
- [x] 7.5 Create `internal/scaffold/templates/agents/antigravity/iktomi/SKILL.md` with the same instruction body
- [x] 7.6 Create `internal/scaffold/templates/agents/github-copilot/iktomi.agent.md` with `tools: Read, Edit, Write, Bash`
- [x] 7.7 In each file, direct Iktomi to hand off directly to a specialist agent when its work clearly points there, and to report to Janus only when it doesn't (broad-routing capability, not a fixed edge — see the `janus-router-agent` capability)

## 8. Add Zhou Gong as a new agent (analytics/reporting) across all platforms

- [x] 8.1 Create `internal/scaffold/templates/agents/claude-code/zhougong.md` with frontmatter `name: zhougong`, `description` describing it as the agent-performance analytics agent, `tools: Read, Write, Bash` (no `Edit`); instruction body: gather per-agent commit counts and time-between-commits from `git log` (grouped by `git config user.name`), per-agent token totals from `Tokens:` commit trailers, and turn timing from `.dreamland/transition.log`; write a report to `.dreamland/reports/<date>-agent-report.md` with a per-agent breakdown and narrative tuning suggestions; when a recurring pattern suggests a new agent is needed, include a "recommended new agent" section; hand off to Janus when the report is written
- [x] 8.2 Create `internal/scaffold/templates/agents/codex/zhougong.toml` with the same content, no `apply_patch` capability
- [x] 8.3 Create `internal/scaffold/templates/agents/cursor/zhougong.mdc` with the same instruction body
- [x] 8.4 Create `internal/scaffold/templates/agents/kiro/zhougong.md` as a plain markdown steering document
- [x] 8.5 Create `internal/scaffold/templates/agents/antigravity/zhougong/SKILL.md` with the same instruction body
- [x] 8.6 Create `internal/scaffold/templates/agents/github-copilot/zhougong.agent.md` with `tools: Read, Write, Bash` (no `Edit`)
- [x] 8.7 In each file, direct Zhou Gong to hand off directly to `hypnos` (or another agent) when the report clearly points to one specific action, and to report to Janus otherwise (broad-routing capability — see the `janus-router-agent` capability)

## 9. Add Hypnos as a new agent (agent-authoring) across all platforms

- [x] 9.1 Create `internal/scaffold/templates/agents/claude-code/hypnos.md` with frontmatter `name: hypnos`, `description` describing it as the agent-authoring agent, `tools: Read, Edit, Write, Bash`; instruction body: given a role description (direct request or a `zhougong` report recommendation), author the new agent's template file for all six platforms following existing frontmatter/instruction-body conventions, decide its tool tier per the `agent-scaffolding` matrix, register it in all six `janus.*` routing tables, add its per-agent slash command, then hand off directly to `phobetor` to validate the new agent's definition/tests (a deterministic next step — see the `janus-router-agent` capability)
- [x] 9.2 Create `internal/scaffold/templates/agents/codex/hypnos.toml` with the same content in Codex's TOML format
- [x] 9.3 Create `internal/scaffold/templates/agents/cursor/hypnos.mdc` with the same instruction body
- [x] 9.4 Create `internal/scaffold/templates/agents/kiro/hypnos.md` as a plain markdown steering document
- [x] 9.5 Create `internal/scaffold/templates/agents/antigravity/hypnos/SKILL.md` with the same instruction body
- [x] 9.6 Create `internal/scaffold/templates/agents/github-copilot/hypnos.agent.md` with `tools: Read, Edit, Write, Bash`
- [x] 9.7 In each file, confirm Hypnos hands off directly to `phobetor` (its default deterministic next step) once the new agent is authored, but also grant it broad-routing capability to hand off directly to any other agent (e.g. `mengpo`) when its own work points there; note explicitly in the instruction body that this is a different role from any earlier "Hypnos as router" meaning — there is none active in the shipped templates

## 10. Add Meng Po as a new agent (archival/deletion) across all platforms

- [x] 10.1 Create `internal/scaffold/templates/agents/claude-code/mengpo.md` with frontmatter `name: mengpo`, `description` describing it as the agent archival/deletion agent, `tools: Read, Write, Bash` (no `Edit`); instruction body: given an agent name to retire, default to archiving — move its template files (all six platforms) to `internal/scaffold/templates/agents/_archive/<platform>/`, remove it from all six `janus.*` routing tables, remove its per-agent slash command files, and append an entry to `.dreamland/archived-agents.md`; on an explicit "delete permanently" instruction, remove the files without archiving instead, still logging the action; report completion to Janus
- [x] 10.2 Create `internal/scaffold/templates/agents/codex/mengpo.toml` with the same content, no `apply_patch` capability
- [x] 10.3 Create `internal/scaffold/templates/agents/cursor/mengpo.mdc` with the same instruction body
- [x] 10.4 Create `internal/scaffold/templates/agents/kiro/mengpo.md` as a plain markdown steering document
- [x] 10.5 Create `internal/scaffold/templates/agents/antigravity/mengpo/SKILL.md` with the same instruction body
- [x] 10.6 Create `internal/scaffold/templates/agents/github-copilot/mengpo.agent.md` with `tools: Read, Write, Bash` (no `Edit`)
- [x] 10.7 In each file, direct Meng Po to hand off directly to another agent when archival reveals a specific follow-up, and to report completion to Janus otherwise (broad-routing capability — see the `janus-router-agent` capability)

## 11. Rewrite hand-off language per the deterministic-vs-judgment split

- [x] 11.1 In all six `morpheus.*` files (renamed from `implementer.*` in group 5), replace "ask the spec-writer to clarify" with language directing escalation to Janus (e.g. "escalate to Janus for routing to phantasos") — this is a judgment call, stays through Janus
- [x] 11.2 In all six `phobetor.*` files (renamed from `tester.*` in group 4), replace "report the specific failing scenario to the implementer" with "hand off directly to morpheus" (an implementation-bug failure — deterministic, no Janus round-trip)
- [x] 11.3 In all six `phobetor.*` files, replace "signal to the orchestrator that the change is ready for the pr-closer" with "hand off directly to baku" (success — deterministic)
- [x] 11.4 In all six `baku.*` files (renamed from `pr-closer.*` in group 2), replace "confirm with the orchestrator" with "confirm with Janus" (terminal — no fixed next agent, stays through Janus)
- [x] 11.5 Confirm all six `iktomi.*`, `zhougong.*`, `hypnos.*`, and `mengpo.*` files (created in groups 7–10) describe broad-routing capability (direct hand-off to any agent when their own work points there, report to Janus otherwise — not a fixed edge or a Janus-only report); confirm `nyx.*` hands off directly to `morpheus` (its one narrow deterministic edge, per group 6) and `hypnos.*` additionally hands off directly to `phobetor` as its default post-authoring step (per group 9)
- [x] 11.6 Grep every agent file across all six platforms for the strings `spec-writer`, `implementer`, `tester`, `pr-closer`, `orchestrator`; confirm zero remain. Separately, confirm every agent's hand-off target matches the deterministic-edge list in the `janus-router-agent` spec exactly (no extra direct edges invented, no deterministic edge incorrectly routed through Janus)

## 12. Verify the three-tier per-agent tool-binding matrix

- [x] 12.1 Confirm `phantasos.*`/`nyx.*`/`morpheus.*`/`iktomi.*`/`hypnos.*` retain `Edit`/`Write`/`apply_patch` capability on every platform that has one
- [x] 12.2 Confirm `janus.*`/`phobetor.*`/`baku.*` exclude `Edit`/`Write`/`apply_patch` entirely on every platform that has one
- [x] 12.3 Confirm `zhougong.*`/`mengpo.*` are granted `Write` (and `apply_patch` where Codex would otherwise imply it — verify Codex's format supports write-without-patch, or document the closest equivalent) but not `Edit`, on every platform
- [x] 12.4 Where a template currently has no explicit tool/capability field (e.g. Cursor `.mdc`, Kiro plain markdown), add an instruction-level statement of the applicable tier's constraint

## 13. Add subagent-routing graph and hooks to GitHub Copilot (VS Code) agent frontmatter

- [x] 13.1 Add `agents:` and `hooks:` keys to `internal/scaffold/templates/agents/github-copilot/janus.agent.md`'s frontmatter: `agents:` lists all nine other agents (`phantasos`, `nyx`, `morpheus`, `phobetor`, `baku`, `iktomi`, `zhougong`, `hypnos`, `mengpo`); `hooks:` lists `coauthor`, `telemetry-write`, `commit`, `version-bump`
- [x] 13.2 Add `agents: [janus, phobetor]` and `hooks: [coauthor, telemetry-write, commit]` to `morpheus.agent.md` (its one narrow deterministic edge)
- [x] 13.3 Add `agents: [janus, baku, morpheus, phantasos]` and `hooks: [coauthor, telemetry-write, commit]` to `phobetor.agent.md` (its three narrow deterministic outcomes)
- [x] 13.4 Add `agents: [janus]` and `hooks: [coauthor, telemetry-write, commit]` to `phantasos.agent.md`, `nyx.agent.md`, and `baku.agent.md` (narrow agents with no deterministic hand-off of their own)
- [x] 13.5 Add the full nine-agent `agents:` list (same set as `janus.agent.md`) and `hooks: [coauthor, telemetry-write, commit]` to `iktomi.agent.md`, `zhougong.agent.md`, `hypnos.agent.md`, and `mengpo.agent.md` (the broad-routing tier)
- [x] 13.6 Confirm each affected agent's instruction body says the same thing the frontmatter grants: `morpheus.agent.md` says "hand off directly to Phobetor"; `phobetor.agent.md` says "hand off directly to Baku/Morpheus/Phantasos" depending on outcome; `iktomi.agent.md`/`zhougong.agent.md`/`hypnos.agent.md`/`mengpo.agent.md` say they may hand off directly to any agent when their own work points there, and report to Janus otherwise — frontmatter and prose SHALL agree on GitHub Copilot, same as every other platform
- [x] 13.7 Update `internal/scaffold/` installer logic if GitHub Copilot agent frontmatter is generated from a shared template structure that doesn't currently have `agents:`/`hooks:` fields

## 14. Write Janus's routing table, two-flow decision, and agent-roster-maintenance branch into each renamed template

- [x] 14.1 Add the routing table to all six `janus.*` files: `/opsx:propose`/`/opsx:explore` → phantasos; `/opsx:archive` → baku; `/opsx:apply` → Janus decides per task between the direct-implementation flow (straight to morpheus) and the acceptance-test flow (nyx first, then morpheus); no OpenSpec context → iktomi; agent-performance/analytics requests → zhougong; new-agent-authoring requests → hypnos; agent-retirement requests → mengpo
- [x] 14.2 Add the flow-selection heuristic to all six `janus.*` files: route to `nyx` first when the task implements a new spec scenario with no covering test; route directly to `morpheus` for mechanical/internal tasks or when a covering test already exists
- [x] 14.3 In all six `morpheus.*` files, add the "hand off directly to Phobetor when implementation is complete" instruction (distinct from the ambiguity-escalation-via-Janus hand-off added in task 11.1 — implementation-complete is deterministic, ambiguity is not)
- [x] 14.4 In all six `phobetor.*` files, confirm the failure hand-off distinguishes an implementation bug (direct to `morpheus`) from a spec defect (direct to `phantasos`), in addition to the existing success signal (direct to `baku`) — all three are direct hand-offs, none routes through Janus
- [x] 14.5 For Cursor, Kiro, Antigravity, Codex, and GitHub Copilot templates, add the instruction directing Janus to run `dreamland coauthor` and `dreamland telemetry write` immediately before and after each delegation (manual-invocation convention per design.md)
- [x] 14.6 Diff the routing-table prose across all six files to confirm they name all nine target agents, the same two-flow decision, the same Iktomi fallback condition, and the same zhougong/hypnos/mengpo branch

## 15. Add the `/route` command and one explicit command per agent

- [x] 15.1 Create `.claude/commands/route.md` that delegates to the `janus` agent, following the existing `.claude/commands/opsx/*.md` frontmatter style
- [x] 15.2 Create `.claude/commands/phantasos.md`, `.claude/commands/nyx.md`, `.claude/commands/morpheus.md`, `.claude/commands/phobetor.md`, `.claude/commands/baku.md`, `.claude/commands/iktomi.md`, `.claude/commands/zhougong.md`, `.claude/commands/hypnos.md`, and `.claude/commands/mengpo.md`, each delegating to `janus` with an explicit "route directly to `<agent>`" instruction that overrides Janus's own judgment
- [x] 15.3 Add the equivalent `/route` and nine per-agent command entries for each other platform that supports user-invocable commands (Codex CLI, Cursor, Kiro; skip GitHub Copilot and Antigravity if no public slash-command mechanism exists, noting this in the file's `_note` if a stub is required). Researched actual conventions: Cursor supports project-scoped `.cursor/commands/*.md` (filename → command name) — implemented and installer-wired. Codex CLI's custom prompts are user-home-only (`~/.codex/prompts/`), not project-scoped, and deprecated upstream in favor of "skills" — no repo-installable target exists, so skipped (same category as Copilot/Antigravity). Kiro's slash commands are `inclusion: manual` steering files in the same `.kiro/steering/` directory already used by the always-on agent files — a routing command reusing an agent's name (e.g. `iktomi.md`) would collide with the agent's own steering file, so skipped rather than forcing a naming workaround.
- [x] 15.4 Update `.claude/commands/opsx/apply.md` (and platform equivalents) to state that it routes through `janus`, which then picks `nyx` or `morpheus` per task
- [x] 15.5 Update `.claude/commands/opsx/propose.md`/`explore.md`/`archive.md` (and platform equivalents) to state their direct routing targets (`phantasos`, `baku`)

## 16. Extend Claude Code hook bindings for agent handoff

- [x] 16.1 Update `internal/scaffold/templates/hooks/bindings/claude-code/settings-patch.json` to add a `PreToolUse` entry (matcher: `Task`) running `dreamland coauthor`
- [x] 16.2 Add a `SubagentStop` entry to the same file running `dreamland coauthor` and `dreamland telemetry write --tool claude-code`
- [x] 16.3 Confirm the existing `SessionStart`/`Stop` entries are unchanged

## 17. Add token-usage report to commit messages and auto-commit on turn-complete/handoff

- [x] 17.1 Extend `cmd/coauthor.go`'s `--trailer` mode to append a `Tokens: input=<n> output=<n> cached=<n> total=<n>` line after the `Co-authored-by:` trailer, sourced from the same turn-telemetry data `dreamland telemetry write` collects; omit the line silently when that data isn't available
- [x] 17.2 Add `cmd/commit.go` implementing `dreamland commit --reason <turn-complete|handoff>`: no-op on a clean `git status --porcelain`; otherwise `git add -A` and `git commit -m "chore: <reason> checkpoint (<agent-name>)"`, reusing the agent-name resolution logic already in `cmd/coauthor.go`
- [x] 17.3 Update `internal/scaffold/templates/hooks/bindings/claude-code/settings-patch.json`: add `dreamland commit --reason turn-complete` to the `Stop` entry, and `dreamland commit --reason handoff` to the `SubagentStop` entry added in task 16.2
- [x] 17.4 Add tests for the `Tokens:` trailer line (present when telemetry available, absent when not) and for `dreamland commit` (commits when dirty, no-op when clean, correct subject line per `--reason`)

## 18. Wire per-turn patch bumps and the new-change/breaking-change bump triggers

- [x] 18.1 Update `internal/scaffold/templates/hooks/bindings/claude-code/settings-patch.json`: add `dreamland version-bump --patch` to the `SubagentStop` entry added in task 16.2, alongside `dreamland coauthor` and `dreamland telemetry write`
- [x] 18.2 In all six `phantasos.*` files, add the instruction: immediately after successfully running `openspec new change <slug>`, run `dreamland version-bump`
- [x] 18.3 In all six `baku.*` files, add the instruction: before confirming closure with Janus, check `proposal.md` for a **BREAKING** marker; if present, run `dreamland version-bump --breaking` first
- [x] 18.4 Implement the `.dreamland/change-bumps` marker (append-and-check-membership by change slug, mirroring the existing `.dreamland/branch-bumps` logic) so `dreamland version-bump` triggered by `phantasos` only fires once per change
- [x] 18.5 Add tests: `version-bump --patch` fires on `SubagentStop`; a second `openspec new change` invocation for an already-bumped slug does not double-bump; `baku` runs `--breaking` only when `proposal.md` contains **BREAKING**

## 19. Update installer logic and tests

- [x] 19.1 Search `internal/scaffold/` for any code that enumerates agent file names (not just template content) and update `orchestrator`/`spec-writer`/`implementer`/`tester`/`pr-closer` references to `janus`/`phantasos`/`morpheus`/`phobetor`/`baku`; add `nyx`, `iktomi`, `zhougong`, `hypnos`, and `mengpo` as new entries (no old name to map from)
- [x] 19.2 Update `cmd/init_test.go`, `internal/scaffold/scaffold_test.go`, and `cmd/coauthor_test.go` fixtures/assertions to the new ten-agent set
- [x] 19.3 Add a test asserting `.claude/settings.json` contains the new `PreToolUse`/`SubagentStop` entries (including `dreamland commit` and `dreamland version-bump --patch`) after `dreamland init` with Claude Code selected
- [x] 19.4 Add a test asserting pre-existing `orchestrator.md`/`spec-writer.md`/`implementer.md`/`tester.md`/`pr-closer.md` files are left untouched when the renamed files (plus the five new agents) are installed alongside them
- [x] 19.5 Add a test asserting GitHub Copilot's `janus.agent.md`/`morpheus.agent.md`/`hypnos.agent.md`/`phobetor.agent.md` frontmatter contains the correct `agents:` list per task group 13, and that the remaining six agent files list only `janus`

## 20. Update specs and verify

- [x] 20.1 Run `go build ./...` and `go test ./...`
- [x] 20.2 Run `dreamland init --force` in a scratch directory for at least one platform (Claude Code) and manually inspect all ten `.claude/agents/*.md` files, `.claude/commands/route.md`, the nine per-agent command files, and `.claude/settings.json` for correctness
- [x] 20.3 Run `dreamland init --force` in a scratch directory with "GitHub Copilot" selected and manually inspect all ten `.github/agents/*.agent.md` files for correct `agents:`/`hooks:` frontmatter per task group 13; confirm every single one lists the identical `hooks: [coauthor, telemetry-write, commit, version-bump]` regardless of its `agents:` tier
- [ ] 20.4 In the Claude Code scratch repo, make a change, let a turn end, and confirm a `chore: turn-complete checkpoint (...)` commit was created with a `Co-authored-by:` trailer and (if telemetry is available) a `Tokens:` line, and that the patch version was bumped
- [ ] 20.5 Manually walk through a mechanical task with `/opsx:apply`: confirm Janus routes the entry point directly to `morpheus` (skipping `nyx`), then `morpheus` hands off directly to `phobetor` (no Janus round-trip), then `phobetor` hands off directly to `baku` on success (no Janus round-trip)
- [ ] 20.6 Manually walk through a new-behavior task with `/opsx:apply`: confirm Janus routes the entry point to `nyx`, which hands off directly to `morpheus`, which hands off directly to `phobetor`, which hands off directly to `baku` — Janus is only involved at entry, not at any of the three downstream hops
- [ ] 20.7 Manually trigger a Phobetor validation failure and confirm it hands off directly to `morpheus` for an implementation bug, or directly to `phantasos` for a spec defect, in each case without a Janus round-trip
- [ ] 20.8 Manually invoke `/morpheus` and `/hypnos` directly and confirm Janus routes to the correct agent in each case (and that `/hypnos` does not invoke the router)
- [ ] 20.9 Manually invoke `/route` with a free-form, non-OpenSpec request and confirm Janus delegates to `iktomi`; then manually invoke `/route` with a free-form request that clearly turns into a spec-drafting task partway through, and confirm Iktomi hands off directly to `phantasos` rather than reporting back to Janus first
- [ ] 20.10 Manually invoke `/zhougong` (or ask Janus a usage-analytics question) and confirm a report file is created under `.dreamland/reports/` with a per-agent breakdown
- [ ] 20.11 Manually ask Janus to author a small test agent via `/hypnos` and confirm a new agent template appears on all six platforms, is registered in all six `janus.*` routing tables, gets a working per-agent slash command, and that `hypnos` hands off directly to `phobetor` to validate it (not via Janus)
- [ ] 20.12 Manually ask Janus to archive that test agent via `/mengpo` and confirm its template files move to `_archive/`, it's removed from `janus.*` routing tables and command files, and `.dreamland/archived-agents.md` gains an entry
- [x] 20.13 Run `/opsx:propose` for a second change on the same branch and confirm `phantasos` triggers a minor bump for the new change even though the branch already had its session-start minor bump. Verified the underlying mechanic directly in a scratch repo (not via a live agent persona, since `phantasos`'s action here is exactly this one CLI call): pre-set `.dreamland/branch-bumps` for the branch, ran `dreamland version-bump --change add-second-feature` — minor bump fired (v1.0.0→v1.1.0) despite the branch marker already existing, and `.dreamland/change-bumps` gained the slug entry; re-running for the same slug created no new tag (idempotent).
- [x] 20.14 Run `/opsx:archive` (or the `baku` finalization step) on a change whose `proposal.md` contains a **BREAKING** marker and confirm `baku` triggers a major bump before confirming closure. Verified directly: a `proposal.md` containing `**BREAKING**` correctly triggers `dreamland version-bump --breaking` (major bump fired, v1.1.0→v2.0.0); a `proposal.md` with no marker correctly does not trigger it.
- [x] 20.15 Run `openspec validate janus-router-agent --strict` and fix any reported issues
