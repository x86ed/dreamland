## 1. Claude Code template

- [ ] 1.1 `internal/scaffold/templates/agents/claude-code/iktomi.md`: replace numbered item 3 —

  ```
  3. If your work included editing or writing files, hand off directly to `phobetor` for validation once complete — a fixed next step, do not report to Janus first. If your work made no file changes (pure investigation, answering a question), report completion or blockers to Janus when done.
  ```

  with:

  ```
  3. Once your own work is complete, hand off directly to `phobetor` for validation — a fixed, unconditional next step, regardless of whether the work involved file changes. Never report a completed turn to Janus first. If you're blocked instead (unable to proceed, or the next step needs broader context only Janus has), report the blocker to Janus, unchanged from before.
  ```

  Leave items 1, 2, the trailing "You have the same broad routing capability..." sentence, and the `hooks:` frontmatter block untouched.

## 2. GitHub Copilot template

- [ ] 2.1 `internal/scaffold/templates/agents/github-copilot/iktomi.agent.md`: replace the line —

  ```
  If your work included editing or writing files, hand off directly to `phobetor` for validation once complete — a fixed next step, do not report to Janus first. If your work made no file changes, report completion or blockers to Janus when done.
  ```

  with the same unconditional sentence from 1.1. Leave the two preceding paragraph lines, the trailing broad-routing sentence, and all frontmatter (`agents:` already lists `phobetor` — no change needed there) untouched.

## 3. Cursor template

- [ ] 3.1 `internal/scaffold/templates/agents/cursor/iktomi.mdc`: replace the line —

  ```
  Otherwise, report completion or blockers to Janus when done.
  ```

  with the same unconditional sentence from 1.1. Leave the two preceding paragraph lines and the trailing broad-routing sentence untouched.

## 4. Codex template

- [ ] 4.1 `internal/scaffold/templates/agents/codex/iktomi.toml`: inside the `developer_instructions` triple-quoted string, replace the line —

  ```
  Otherwise, report completion or blockers to Janus when done.
  ```

  with the same unconditional sentence from 1.1 (plain text, no backtick-escaping concerns — TOML triple-quoted strings don't require escaping backticks). Leave the rest of the string untouched.

## 5. Kiro template

- [ ] 5.1 `internal/scaffold/templates/agents/kiro/iktomi.md`: replace the line —

  ```
  Otherwise, report completion or blockers to Janus when done.
  ```

  with the same unconditional sentence from 1.1. Leave the rest of the file untouched.

## 6. Antigravity template

- [ ] 6.1 `internal/scaffold/templates/agents/antigravity/iktomi/SKILL.md`: replace the line —

  ```
  Otherwise, report completion or blockers to Janus when done.
  ```

  with the same unconditional sentence from 1.1. Leave the rest of the file untouched.

## 7. Verify

- [ ] 7.1 Re-read all 6 edited files to confirm the exact same sentence (adjusted only for platform-specific quoting/escaping, not wording) was used in every one, and that the canonical substring `` hand off directly to `phobetor` `` is present verbatim in each (this is what `internal/workflowgraph/import.go`'s `handOffPattern` regex expects, for round-trip consistency with the graph tooling even though these files are hand-edited, not graph-generated).
- [ ] 7.2 Grep all 6 files for stray old phrasing to confirm nothing was missed: `"If your work included editing or writing files"`, `"If your work made no file changes"`, and a bare `"Otherwise, report completion or blockers to Janus when done."` with no `phobetor` mention anywhere else in the same file.
- [ ] 7.3 Confirm this task list did **not** touch `.claude/agents/iktomi.md` (this repo's live, self-hosted copy) or `.claude/agents/phobetor.md` — out of scope per design.md; that file's re-sync from templates is `claude-code-parity`'s existing task 9.4, not this change's.
- [ ] 7.4 Run `go build ./...` and `go test ./...` to confirm the (unaffected) scaffold installer still passes — template content isn't asserted verbatim in Go tests beyond the string-presence checks `claude-code-parity` already added for the old conditional text; those existing assertions will need updating if they still assert the now-removed conditional phrasing (check `internal/scaffold/scaffold_test.go` or wherever `claude-code-parity`'s task 8.4 landed its string-presence checks, and update them to assert the new unconditional sentence instead).
- [ ] 7.5 Hand off directly to `phobetor` to validate.
