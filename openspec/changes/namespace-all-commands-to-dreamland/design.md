## Context

`platformCommandSpec` in [scaffold.go](internal/scaffold/scaffold.go) previously listed only Cursor, on the assumption (documented in a code comment) that Claude Code commands weren't scaffold-installed and that Codex CLI, GitHub Copilot, Kiro, and Antigravity had no usable slash-command mechanism at all. Research done as part of this change found that assumption was wrong for four of the five:

- **GitHub Copilot (VS Code)**: real project-scoped prompt files at `.github/prompts/*.prompt.md`, frontmatter `name:`/`agent:`, invoked as `/name`.
- **Kiro**: manual-inclusion steering files (`inclusion: manual`) genuinely appear as slash commands in the `/` menu — the old blocker was a **naming collision**, not a missing mechanism: a routing command named the same as its target agent's always-on steering file (e.g. `iktomi.md`) would overwrite it in the same `.kiro/steering/` directory. The `drmlnd-` prefix this change introduces resolves that collision as a side effect.
- **Antigravity**: any flat `.md` file dropped into `.agents/skills/` becomes a slash command, distinct from the directory-per-skill layout (`<name>/SKILL.md`) already used there for agent personas.
- **Codex CLI**: project-scoped custom prompts are still user-home-only and deprecated upstream, confirming that part of the old comment — but project-level **Skills** (directory + `SKILL.md`, explicit `$name` or `/skills`-menu invocation) are the current supported mechanism and are project-scoped.

Claude Code was a different problem: `dreamland init` never installed Claude Code commands at all — `.claude/commands/*.md` in this repo were hand-authored dogfood files, disconnected from `internal/scaffold/templates/`. The user flagged this directly: those files are configuration for developing *this* repository, not the dreamland product's shipped templates, and shouldn't be treated as the implementation.

## Goals / Non-Goals

**Goals:**

- All ten dreamland agent-routing commands (`route` + nine agents) are real, scaffold-installed artifacts on all six supported platforms, each carrying a `drmlnd` prefix in whatever form that platform's naming model supports.
- `internal/scaffold/templates/commands/<platform>/` is the source of truth for every platform, including Claude Code. This repo's own `.claude/commands/drmlnd/*.md` is seeded from those templates (kept in sync manually, since this repo isn't self-hosted via `dreamland init` — running `dreamland init` against this repo's own working tree is a separate, larger decision than this rename and is out of scope here).
- Re-running `dreamland init` on a repo with old unprefixed Cursor command files removes them rather than leaving both versions installed. The other five platforms are new installs with nothing pre-existing to migrate.
- No dreamland command file overwrites an unrelated agent persona file — verified per platform, not assumed (this is exactly the bug caught mid-implementation for Kiro; see Risks).

**Non-Goals:**

- Renaming or touching `/opsx:propose`, `/opsx:explore`, `/opsx:apply`, `/opsx:archive` — explicitly out of scope.
- Running `dreamland init` against this repo's own working tree to fully self-host (would also touch `.claude/agents/`, `.claude/settings.json`, etc.) — a bigger, separate decision.
- Changing agent behavior, routing logic, or hand-off machinery — this is a naming/path/new-artifact change only.

## Decisions

**Per-platform command identity and location:**

| Platform | Directory | Naming mechanism | Invocation |
|---|---|---|---|
| Claude Code | `.claude/commands/drmlnd/<agent>.md` | colon namespace via nested directory | `/drmlnd:<agent>` |
| Cursor | `.cursor/commands/<agent>.md` | flat kebab-case, frontmatter `name:` | `/drmlnd-<agent>` |
| GitHub Copilot | `.github/prompts/<agent>.prompt.md` | frontmatter `name:` + `agent: janus` | `/drmlnd-<agent>` |
| Kiro | `.kiro/steering/drmlnd-<agent>.md` | `inclusion: manual`, filename-derived | `/drmlnd-<agent>` |
| Antigravity | `.agents/skills/drmlnd-<agent>.md` | flat file, filename-derived | `/drmlnd-<agent>` |
| Codex CLI | `.codex/skills/drmlnd-<agent>/SKILL.md` | directory + `SKILL.md` | `$drmlnd-<agent>` or `/skills` menu |

Claude Code and Cursor need the prefix only in frontmatter/directory structure, because their command directories are disjoint from their agent-persona directories (`.claude/agents/`, `.cursor/rules/`). **Kiro and Antigravity share their command directory with agent personas** (`.kiro/steering/`, `.agents/skills/`), so the prefix must additionally be baked into the **filename** to avoid a command silently overwriting an agent file — Kiro's agents are flat files at the same path depth as commands, so this is a real collision risk; Antigravity's agents are one level deeper (`<name>/SKILL.md`), so it's a defensive-consistency choice there rather than a strict requirement, but is done anyway for uniformity.

**All six platforms are installed through one dispatch point.** `installCommands` now looks up all six tools in `platformCommandSpec` and dispatches on whether `platformSpec.skillFile` is set: Codex CLI (directory + `SKILL.md`) reuses the existing `installSkills` function already built for Antigravity's agent-persona install; the other five reuse `installFlatCommands`.

**Stale-file cleanup only matters for Cursor.** `installFlatCommands` compares an existing destination file's frontmatter `name:` against the template's (via the `commandName` helper) and overwrites unconditionally on a mismatch — this only has real pre-rename files to act on for Cursor, since every other platform's command installation is brand new in this change (nothing to migrate).

**Claude Code templates, not hand-authored files, are the source of truth.** `internal/scaffold/templates/commands/claude-code/*.md` was seeded from this repo's own `.claude/commands/*.md` (content was already correct; only the location/role changes — the templates are canonical, this repo's own copy is a downstream dogfood artifact, kept in sync manually rather than by running `dreamland init` here, which is a bigger action left out of scope).

## Risks / Trade-offs

- **[Caught during implementation]** The first pass of Kiro command templates used bare filenames (`phantasos.md`), which collided with — and would have overwritten — the existing always-on agent steering file at the same path. The existing `TestInstall_Kiro` test caught this immediately (asserted `inclusion: always` frontmatter, which the overwritten file no longer had). Fixed by renaming every Kiro (and, defensively, Antigravity) command template to carry the `drmlnd-` prefix in the filename itself, not just inside frontmatter. This is exactly why the full test suite — not just the new tests — was re-run before considering any platform done.
- **[Risk]** Codex CLI's exact skill-directory convention (`.codex/skills/` vs. the newer, possibly-shared `.agents/skills/` convention some sources point to) is not fully settled upstream — chose `.codex/skills/` to avoid any directory collision with Antigravity's `.agents/skills/` output if both platforms are ever scaffolded into the same repo → **[Mitigation]** flagged as an open question; low blast radius since Codex CLI's own agent-persona install already uses `.codex/agents/`, a precedent for a `.codex/`-rooted convention.
- **[Risk]** None of GitHub Copilot's `/name`, Kiro's `/name`, Antigravity's `/name`, or Codex's `$name`/`/skills` invocation has been verified against a real running instance of each tool from this environment (same limitation already flagged for Cursor in the original design) → **[Mitigation]** existing open task to manually verify Cursor extends naturally to covering all five non-Claude-Code platforms; flagged as a single follow-up verification task rather than five separate ones.
- **[Risk]** This is a **BREAKING** rename — anyone already using an unprefixed command loses it. `/opsx:*` is deliberately left alone to minimize blast radius; the other five platforms have no prior installs to break (this is their first time getting commands at all).

## Migration Plan

1. Seed `internal/scaffold/templates/commands/claude-code/` from this repo's own `.claude/commands/*.md`, then move those files to `.claude/commands/drmlnd/**` (already done via `git mv`).
2. Generate template sets for GitHub Copilot, Kiro, Antigravity, and Codex CLI under `internal/scaffold/templates/commands/<platform>/`, each following that platform's real naming/invocation mechanism from the table above.
3. Extend `platformCommandSpec` to list all six platforms; extend `installCommands` to dispatch to `installSkills` (Codex CLI) or `installFlatCommands` (the other five).
4. Fix the Kiro/Antigravity filename collision (caught by the existing test suite — see Risks).
5. Add per-platform install tests; re-run the **full** test suite (not just new tests) after each platform to catch cross-platform regressions early, as happened with Kiro.
6. Update `router-slash-commands` capability spec and this change's own tasks.md to reflect six-platform scope.

No runtime rollback concerns — this ships as a normal code change; reverting the commit reverts the naming and the new template files.

## Open Questions

- Should `dreamland init`'s stale-file cleanup for Cursor be unconditional, or gated behind `--force`? Implemented as unconditional (a `name:` mismatch bypasses the `!cfg.Force` skip check) — no counter-argument surfaced during implementation.
- Is `.codex/skills/` the right target directory for Codex CLI's project-level skills, or should it be `.agents/skills/` (shared with Antigravity)? Needs confirmation against a real Codex CLI instance.
- Real-world invocation verification (Cursor `/drmlnd-phantasos`, GitHub Copilot `/drmlnd-phantasos`, Kiro `/drmlnd-phantasos`, Antigravity `/drmlnd-phantasos`, Codex CLI `$drmlnd-phantasos`) has not been done from this environment — needs a human with access to each tool.
