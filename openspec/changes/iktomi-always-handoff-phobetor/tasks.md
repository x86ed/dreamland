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
- [ ] 7.4 `internal/scaffold/scaffold_test.go` lines 420-421 and 440-441 assert `strings.Contains(content, "hand off directly to \`phobetor\` for validation once complete")` for the installed `iktomi.md` (claude-code) and `iktomi.agent.md` (github-copilot) — that substring no longer appears once 1.1/2.1 land (the new sentence reads "hand off directly to `phobetor` for validation — a fixed, unconditional next step..."). Update both assertions to check for the new substring instead (e.g. `` "hand off directly to `phobetor` for validation" `` alone, or the fuller unconditional phrase — pick whichever is specific enough not to also match a stale/reverted string).
- [ ] 7.5 Add equivalent string-presence assertions for the four platforms that had no phobetor-related test coverage before this change (`cursor` iktomi.mdc, `codex` iktomi.toml, `kiro` iktomi.md, `antigravity` iktomi/SKILL.md), matching the existing pattern at 420-421/440-441 — same canonical substring check, one per platform's installed content.
- [ ] 7.6 Run `go build ./...` and `go test ./...` to confirm everything passes, including the updated/new assertions from 7.4/7.5.
- [ ] 7.7 Hand off directly to `phobetor` to validate.
