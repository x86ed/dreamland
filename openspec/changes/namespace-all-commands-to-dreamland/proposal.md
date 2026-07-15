## Why

Every dreamland-installed agent-routing slash command (`/route`, `/phantasos`, `/nyx`, etc.) is installed with no namespace prefix. In a repo where other tools or plugins also install commands, this is a collision risk and makes it hard to tell at a glance which commands come from dreamland. Prefixing dreamland's own commands with `drmlnd:` fixes both, without touching commands that aren't dreamland-specific.

## What Changes

- **BREAKING**: Rename dreamland's own agent-routing slash commands to carry a `drmlnd` prefix, on every platform `dreamland init` supports a real command mechanism for (Claude Code, Cursor). The separator is platform-native, not a literal requirement — Claude Code supports colon namespacing, Cursor's naming model is flat kebab-case only:
  - Claude Code: `/phantasos` → `/drmlnd:phantasos` (and so on for `nyx`, `morpheus`, `phobetor`, `baku`, `iktomi`, `zhougong`, `hypnos`, `mengpo`); `/route` → `/drmlnd:route`
  - Cursor: `/phantasos` → `/drmlnd-phantasos` (hyphen, same agent list); `/route` → `/drmlnd-route`, set via each command file's frontmatter `name:` field
  - Codex CLI, GitHub Copilot, Kiro, and Antigravity have no real slash-command mechanism today and don't reference these command names as invocation strings — nothing to rename there
- **Out of scope, explicitly**: `/opsx:propose`, `/opsx:explore`, `/opsx:apply`, `/opsx:archive` are OpenSpec-workflow commands, not dreamland-specific — they keep their current names. VSCode-level settings/commands (e.g. `chat.useCustomAgentHooks`) are not slash commands dreamland owns and are not touched either.
- Update Janus's routing table and every renamed command's own definition file so their documented names match the new prefixed names.
- Remove stale unprefixed per-agent/`route` command artifacts left behind by prior scaffolds so `dreamland init` doesn't leave both old and new names installed side by side.
- Update README.md's references to the renamed commands (agent table, etc.) — `/opsx:*` references stay unchanged.
- Add godoc comments to any new exported Go functions/types introduced to implement the rename.
- Add/extend tests so changed packages reach ≥95% line coverage.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `router-slash-commands`: the requirements naming `/route` and the nine per-agent commands change to reference the `drmlnd:`-prefixed name instead, on every platform that installs them. The `/opsx:*` requirements are unchanged.

## Impact

- **Code**: `internal/scaffold/templates/commands/cursor/*.md` (route + per-agent files only) and any Go code in `internal/scaffold` that generates or names those installed command files (`installCommands`/`installFlatAgents` in `scaffold.go`).
- **Docs**: `README.md` references to the renamed commands.
- **This repo's own installed commands**: `.claude/commands/*.md` (route + per-agent files) — this repo is not self-hosted via `dreamland init` (these were hand-authored) and will need the same rename applied directly. `.claude/commands/opsx/*.md` is unaffected.
- **No API/dependency changes** — this is a naming/path change to generated and hand-authored command artifacts, not a runtime behavior change.
