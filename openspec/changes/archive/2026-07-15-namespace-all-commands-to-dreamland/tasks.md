## 1. Claude Code: templates become source of truth

- [x] 1.1 Seeded `internal/scaffold/templates/commands/claude-code/{route,phantasos,nyx,morpheus,phobetor,baku,iktomi,zhougong,hypnos,mengpo}.md` from this repo's own hand-authored `.claude/commands/*.md` (content was already correct; only its role changes — templates are now canonical)
- [x] 1.2 Moved this repo's own `.claude/commands/{route,...}.md` into `.claude/commands/drmlnd/` (`git mv`, history preserved) — this repo's own dogfood copy, kept in sync with the templates manually since the repo isn't self-hosted via `dreamland init`
- [x] 1.3 Left `.claude/commands/opsx/**` untouched
- [x] 1.4 Updated the one self-reference (`route.md`'s own body mentioning `/route`) to `/drmlnd:route`
- [x] 1.5 Verified no separate "Janus routing table" file exists in this repo to update — routing description lives inline in each command file. Live confirmation: Claude Code's own skill list shows `drmlnd:phantasos`, `drmlnd:route`, etc.

## 2. Cursor

- [x] 2.1 Added frontmatter `name: drmlnd-<agent>` to each `internal/scaffold/templates/commands/cursor/*.md` (Cursor's naming model is flat kebab-case only, no colon namespacing — per cursor.com/docs/reference/plugins). Files stay flat, no directory restructuring.
- [x] 2.2 Updated `route.md`'s self-reference to the hyphenated `/drmlnd-route` spelling

## 3. GitHub Copilot (VS Code) — new platform support

- [x] 3.1 Researched: real mechanism is project-scoped prompt files at `.github/prompts/*.prompt.md`, frontmatter `name:` sets the slash-command identifier, `agent:` targets a custom chat agent directly
- [x] 3.2 Created `internal/scaffold/templates/commands/github-copilot/{route,phantasos,nyx,morpheus,phobetor,baku,iktomi,zhougong,hypnos,mengpo}.prompt.md`, each with `name: drmlnd-<agent>` and `agent: janus`

## 4. Kiro — new platform support (and a naming-collision fix)

- [x] 4.1 Researched: manual-inclusion steering files (`inclusion: manual`) really do appear as slash commands in Kiro's `/` menu — the old "no clean target" blocker was a naming collision (a routing command sharing a name with its target's always-on agent file), not a missing mechanism
- [x] 4.2 Created `internal/scaffold/templates/commands/kiro/drmlnd-{route,phantasos,...}.md`, each with `inclusion: manual` and `name: drmlnd-<agent>`
- [x] 4.3 **Bug caught by the existing test suite**: first pass used bare filenames (`phantasos.md`), which collided with and overwrote the always-on agent persona file at the same path in `.kiro/steering/`. `TestInstall_Kiro` failed immediately (missing `inclusion: always`). Fixed by renaming every template file to carry `drmlnd-` in the filename itself, not just frontmatter.

## 5. Antigravity — new platform support

- [x] 5.1 Researched: a flat `.md` file dropped into `.agents/skills/` becomes a slash command, distinct from the directory-per-skill layout (`<name>/SKILL.md`) already used there for agent personas
- [x] 5.2 Created `internal/scaffold/templates/commands/antigravity/drmlnd-{route,phantasos,...}.md` (flat files, `drmlnd-` prefix in the filename for consistency/defense, even though no hard collision exists here since agent personas are one level deeper)

## 6. Codex CLI — new platform support

- [x] 6.1 Researched: project-scoped custom prompts remain user-home-only and deprecated; project-level **Skills** (directory + `SKILL.md`, explicit `$name` or `/skills`-menu invocation) are the current supported project-scoped mechanism
- [x] 6.2 Created `internal/scaffold/templates/commands/codex/drmlnd-{route,phantasos,...}/SKILL.md` (directory-per-skill, same shape `installSkills` already uses for Antigravity's agent-persona install)

## 7. Go implementation

- [x] 7.1 Extended `platformCommandSpec` in `internal/scaffold/scaffold.go` to list all six platforms with their template/target directories
- [x] 7.2 `installCommands` now dispatches to `installSkills` (Codex CLI, which sets `skillFile`) or `installFlatCommands` (the other five)
- [x] 7.3 `installFlatCommands` (added in an earlier pass of this change) compares an existing destination file's frontmatter `name:` against the template's via the `commandName` helper, overwriting unconditionally on mismatch — this only has real pre-rename files to act on for Cursor
- [x] 7.4 Ran `gofmt`/`go vet`/`go build` — all clean

## 8. Documentation

- [x] 8.1 Checked `README.md` — no literal `/route` or per-agent slash-command mentions exist there (only bare agent names and `/opsx:*`, unchanged). No-op.

## 9. Tests and coverage

- [x] 9.1 Updated `TestInstall_Cursor_Commands` to assert `name: drmlnd-<agent>` frontmatter
- [x] 9.2 Added `TestInstall_Cursor_Commands_ReplacesStaleUnprefixedFile` and `TestInstall_Cursor_Commands_SkipsUpToDateFile`
- [x] 9.3 Added `TestInstall_ClaudeCode_Commands`, `TestInstall_GitHubCopilot_Commands`, `TestInstall_Kiro_Commands`, `TestInstall_Antigravity_Commands`, `TestInstall_Codex_Commands` — one per new platform, each asserting the right files/frontmatter exist
- [x] 9.4 Renamed `TestInstall_ClaudeCode_NoCommandsInstalled` → `TestInstall_ClaudeCode_NoCursorDirCreated` (its assertion — no stray `.cursor` dir — was never about command absence; Claude Code now does get commands)
- [x] 9.5 Added `TestInstallFlatCommands_WriteFileError` mirroring the existing `TestInstallFlatAgents_WriteFileError` convention
- [x] 9.6 Ran `go test ./... -cover` after **every** platform addition, not just at the end — this is what caught the Kiro collision bug immediately rather than at final review. All packages pass. `internal/scaffold` sits at 86.6% (essentially unchanged from the pre-change baseline of ~86%); the ≥95% target is not met package-wide, but the gap is pre-existing (otel installers, hook atomic-write error branches) and unrelated to this change. New code added by this change is covered to the same standard as its sibling functions.

## 10. Final verification

- [x] 10.1 Grepped the repo for remaining unprefixed mentions of the ten command names outside `/opsx:*` context — remaining hits are only the base `openspec/specs/router-slash-commands/spec.md` (correct until archive), immutable `openspec/changes/archive/**` history, the unrelated `agent-scaffolding` spec (agent files, not commands), and bare agent-name prose with no leading slash in Codex's `janus.toml`/`iktomi.toml`. No stragglers.
- [ ] 10.2 **Not completed — cannot verify from this environment.** No real Claude Code, Cursor, VS Code+Copilot, Kiro, Antigravity, or Codex CLI instance is available here to confirm every command actually resolves and invokes correctly at runtime (Claude Code's `drmlnd:*` commands were confirmed live via the skill list; the other five were not). Needs a human with access to each tool before/soon after merge.
- [x] 10.3 Confirmed no cross-platform directory collisions: Codex CLI's `.codex/skills/` and Antigravity's `.agents/skills/` are distinct roots; Kiro's and Antigravity's command filenames are prefixed specifically to avoid colliding with co-located agent persona files.
- [ ] 10.4 Run `openspec status --change namespace-all-commands-to-dreamland` and confirm all tasks are checked off before archiving (blocked only on 10.2)
