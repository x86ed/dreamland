## 1. Verify Cursor namespacing convention

- [ ] 1.1 Confirm whether Cursor supports colon-style command namespacing from a nested directory, or only flat filenames — resolves the open question in design.md and determines the file layout for task group 3.

## 2. Rename this repo's hand-authored Claude Code commands

- [ ] 2.1 Move `.claude/commands/route.md` and the nine per-agent files (`phantasos.md`, `nyx.md`, `morpheus.md`, `phobetor.md`, `baku.md`, `iktomi.md`, `zhougong.md`, `hypnos.md`, `mengpo.md`) into `.claude/commands/drmlnd/`
- [ ] 2.2 Leave `.claude/commands/opsx/**` untouched
- [ ] 2.3 Update any self-references inside the moved files (e.g. a file referring to its own command name) to the `drmlnd:`-prefixed spelling
- [ ] 2.4 Update Janus's routing-table instructions (wherever they live for this repo's own Claude Code setup) to reference `/drmlnd:route` and the nine `/drmlnd:<agent>` names

## 3. Rename Cursor's templated/installed commands

- [ ] 3.1 Based on task 1.1's finding, lay out `internal/scaffold/templates/commands/cursor/{route,phantasos,nyx,morpheus,phobetor,baku,iktomi,zhougong,hypnos,mengpo}.md` so the installed command names come out `drmlnd:`-prefixed
- [ ] 3.2 Update `installCommands`/`installFlatAgents` in `internal/scaffold/scaffold.go` if the new layout needs directory-walking instead of a flat copy
- [ ] 3.3 Add godoc comments to any newly exported functions/types introduced by this change

## 4. Stale-file cleanup on re-init

- [ ] 4.1 Add a cleanup step to Cursor's install path that removes the ten known unprefixed filenames (`route.md` + the nine agent names) from `.cursor/commands/` when present
- [ ] 4.2 Confirm the cleanup runs unconditionally (not gated behind `--force`), per design.md's leaning — adjust if task 1.1 or testing surfaces a reason to gate it

## 5. Update documentation

- [ ] 5.1 Update `README.md` references to `/route` and the per-agent commands to their `drmlnd:`-prefixed names (leave `/opsx:*` references unchanged)

## 6. Tests and coverage

- [ ] 6.1 Update `internal/scaffold/scaffold_test.go` assertions (`TestInstall_Cursor_Commands` and related) to expect the new prefixed file layout
- [ ] 6.2 Add a test covering stale-file cleanup: pre-seed `.cursor/commands/` with old unprefixed files, run `Install`, assert they're removed and only prefixed files remain
- [ ] 6.3 Run `go test ./... -cover` and confirm changed packages reach ≥95% line coverage; add tests for any uncovered branches introduced by this change

## 7. Final verification

- [ ] 7.1 Grep the repo for any remaining unprefixed mentions of `/route`, `/phantasos`, `/nyx`, `/morpheus`, `/phobetor`, `/baku`, `/iktomi`, `/zhougong`, `/hypnos`, `/mengpo` outside of `/opsx:*` context and fix any stragglers
- [ ] 7.2 Run `openspec status --change namespace-all-commands-to-dreamland` and confirm all tasks are checked off before archiving
