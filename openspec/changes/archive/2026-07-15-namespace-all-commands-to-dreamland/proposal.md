## Why

Every dreamland-installed agent-routing slash command (`/route`, `/phantasos`, `/nyx`, etc.) is installed with no namespace prefix, and — before this change — real command installation only existed for Cursor; Claude Code's copy was a hand-authored, disconnected-from-the-template-system file set that only existed in this repo's own working tree. Neither state is acceptable: unprefixed commands collide with other tools, and un-templated commands can't be shipped to users at all. This change gives every dreamland agent-routing command a `drmlnd` prefix and makes it a real, scaffold-installed artifact on every platform `dreamland init` supports.

## What Changes

- **BREAKING**: Rename dreamland's own agent-routing slash commands to carry a `drmlnd` prefix, on all six supported platforms. The separator/mechanism is platform-native, not a literal requirement:
  - **Claude Code**: colon namespace via nested directory — `/phantasos` → `/drmlnd:phantasos` (`.claude/commands/drmlnd/phantasos.md`), same for `route` and the other eight agents.
  - **Cursor**: flat kebab-case via frontmatter `name:` — `/phantasos` → `/drmlnd-phantasos` (`.cursor/commands/phantasos.md`, `name: drmlnd-phantasos`).
  - **GitHub Copilot (VS Code)**: project-scoped prompt files — `.github/prompts/phantasos.prompt.md` with `name: drmlnd-phantasos`, `agent: janus`, invoked as `/drmlnd-phantasos`.
  - **Kiro**: manual-inclusion steering files act as real slash commands — `.kiro/steering/drmlnd-phantasos.md` (`inclusion: manual`), invoked as `/drmlnd-phantasos`. The `drmlnd-` prefix is required in the **filename** here, not just frontmatter, because Kiro installs agent personas and commands into the same `.kiro/steering/` directory — an unprefixed command file would silently overwrite the agent's own steering file.
  - **Antigravity**: a flat `.md` file under `.agents/skills/` becomes a slash command — `.agents/skills/drmlnd-phantasos.md`, invoked as `/drmlnd-phantasos`, alongside (not inside) the existing per-agent skill directories in that same folder.
  - **Codex CLI**: project-level Skills, directory + `SKILL.md` — `.codex/skills/drmlnd-phantasos/SKILL.md`, invoked explicitly via `$drmlnd-phantasos` or the `/skills` menu.
- **Out of scope, explicitly**: `/opsx:propose`, `/opsx:explore`, `/opsx:apply`, `/opsx:archive` are OpenSpec-workflow commands, not dreamland-specific — they keep their current names, unaffected on every platform.
- **New capability, not just a rename**: Claude Code, GitHub Copilot, Kiro, Antigravity, and Codex CLI previously had no scaffold-installed command mechanism at all (Claude Code's was hand-authored only; the other four were undocumented/assumed unsupported). This change adds real `internal/scaffold/templates/commands/<platform>/` template sets and wires `installCommands` to install them for all six platforms.
- Remove stale unprefixed Cursor command artifacts left behind by prior scaffolds so `dreamland init` doesn't leave both old and new names installed side by side (the other five platforms are new installs with nothing pre-existing to clean up).
- Add godoc comments to any new exported Go functions/types introduced to implement this.
- Add/extend tests so changed packages reach as close to ≥95% line coverage as practical given pre-existing gaps elsewhere in the package.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `router-slash-commands`: the requirements naming `/route` and the nine per-agent commands change to reference the `drmlnd`-prefixed name (platform-native separator) instead, and now cover real installation on all six supported platforms rather than just Cursor. The `/opsx:*` requirements are unchanged.

## Impact

- **Code**: `internal/scaffold/templates/commands/{claude-code,cursor,github-copilot,kiro,antigravity,codex}/**` (new/updated template sets) and `internal/scaffold/scaffold.go` (`platformCommandSpec`, `installCommands`, `installFlatCommands`, `commandName`).
- **This repo's own installed commands**: `.claude/commands/drmlnd/*.md` — seeded from (and should be kept in sync with) `internal/scaffold/templates/commands/claude-code/`, since this repo is not self-hosted via `dreamland init`. `.claude/commands/opsx/*.md` is unaffected.
- **Docs**: `README.md` references to the renamed commands (checked — none existed as literal slash strings beyond `/opsx:*`, which is unchanged).
- **No API/dependency changes** — this is a naming/path change plus new template artifacts, not a change to agent behavior or hand-off machinery.
