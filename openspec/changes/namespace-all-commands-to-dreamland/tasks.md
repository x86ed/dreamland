## 1. Rename this repo's hand-authored Claude Code commands

- [ ] 1.1 Move `.claude/commands/route.md` and the nine per-agent files (`phantasos.md`, `nyx.md`, `morpheus.md`, `phobetor.md`, `baku.md`, `iktomi.md`, `zhougong.md`, `hypnos.md`, `mengpo.md`) into `.claude/commands/drmlnd/`
- [ ] 1.2 Leave `.claude/commands/opsx/**` untouched
- [ ] 1.3 Update any self-references inside the moved files (e.g. a file referring to its own command name) to the `drmlnd:`-prefixed spelling
- [ ] 1.4 Update Janus's routing-table instructions (wherever they live for this repo's own Claude Code setup) to reference `/drmlnd:route` and the nine `/drmlnd:<agent>` names

## 2. Rename Cursor's templated/installed commands

- [ ] 2.1 In each `internal/scaffold/templates/commands/cursor/{route,phantasos,nyx,morpheus,phobetor,baku,iktomi,zhougong,hypnos,mengpo}.md`, add/set frontmatter `name: drmlnd-route` / `name: drmlnd-<agent>` (Cursor's naming model is flat kebab-case only — no colon namespacing — per cursor.com/docs/reference/plugins). Files stay flat under `commands/cursor/`, no directory restructuring.
- [ ] 2.2 Update each file's own body text (if it self-references its command name) to the hyphenated `drmlnd-<agent>` spelling
- [ ] 2.3 Confirm `installCommands`/`installFlatAgents` in `internal/scaffold/scaffold.go` needs no structural change (flat copy already matches the flat layout) — only add godoc if any new exported helper is introduced for cleanup (task 3)

## 3. Stale-file cleanup on re-init

- [ ] 3.1 Add a cleanup step to Cursor's install path that, for each of the ten known command files, rewrites/replaces it if its `name:` frontmatter (or filename-derived identifier) is still the old unprefixed form, so re-running `dreamland init` self-heals existing installs
- [ ] 3.2 Confirm the cleanup runs unconditionally (not gated behind `--force`), per design.md's leaning — adjust if testing surfaces a reason to gate it

## 4. Update documentation

- [ ] 4.1 Update `README.md` references to `/route` and the per-agent commands: Claude Code examples use `/drmlnd:phantasos` etc., Cursor examples (if any) use `/drmlnd-phantasos` etc. Leave `/opsx:*` references unchanged.

## 5. Tests and coverage

- [ ] 5.1 Update `internal/scaffold/scaffold_test.go` assertions (`TestInstall_Cursor_Commands` and related) to assert the new `name: drmlnd-<agent>` frontmatter in installed files
- [ ] 5.2 Add a test covering stale-file cleanup: pre-seed `.cursor/commands/` with an old-style file (unprefixed `name:` or none), run `Install`, assert it now carries the `drmlnd-`-prefixed identifier
- [ ] 5.3 Manually verify against a real Cursor install (or the Cursor CLI if available) that `/drmlnd-phantasos` actually resolves and runs — the frontmatter `name:` field's exact resolution behavior isn't fully documented upstream
- [ ] 5.4 Run `go test ./... -cover` and confirm changed packages reach ≥95% line coverage; add tests for any uncovered branches introduced by this change

## 6. Final verification

- [ ] 6.1 Grep the repo for any remaining unprefixed mentions of `/route`, `/phantasos`, `/nyx`, `/morpheus`, `/phobetor`, `/baku`, `/iktomi`, `/zhougong`, `/hypnos`, `/mengpo` outside of `/opsx:*` context and fix any stragglers
- [ ] 6.2 Run `openspec status --change namespace-all-commands-to-dreamland` and confirm all tasks are checked off before archiving
