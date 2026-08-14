## 1. `dreamland agent-status --start`/`--stop`

- [ ] 1.1 Add `cmd/agent_status.go`: a `agent-status` cobra command with `--start`/`--stop` bool flags, reading a hook JSON payload from stdin and extracting `session_id` and (for `--start`) `tool_input.subagent_type` — reuse the existing payload-parsing approach `agentNameFromHookPayloadFrom` already uses (`cmd/coauthor.go`), don't duplicate it; extract a shared helper if needed rather than copy-pasting the JSON-unmarshal logic.
- [ ] 1.2 `--start`: write `{"agent": "<name>", "tool_use_id": "<id>", "started_at": "<RFC3339>"}` to `.dreamland/agent-status/<session_id>.json`, creating the `agent-status` directory if needed. No `subagent_type` in the payload, or an unparseable payload → exit 0, write nothing.
- [ ] 1.3 `--stop`: delete `.dreamland/agent-status/<session_id>.json` if it exists. Missing file, or unparseable payload → exit 0, no error.
- [ ] 1.4 Unit tests: valid `--start` payload writes the expected file/contents; missing `subagent_type` writes nothing; `--stop` deletes an existing file; `--stop` on a session with no file is a no-op; malformed JSON on either flag exits 0.
- [ ] 1.5 Add `.dreamland/agent-status/` to `.gitignore` — per-session churn, no history value, would otherwise get swept into every `dreamland commit` checkpoint (`git add -A`). See design.md Decisions.

## 2. `dreamland statusline`

- [ ] 2.1 Add `cmd/statusline.go`: a `statusline` cobra command reading the `statusLine` JSON payload from stdin (`session_id` field), looking up `.dreamland/agent-status/<session_id>.json`.
- [ ] 2.2 No state file for the session → print an idle indicator (no active dispatch) to stdout, exit 0.
- [ ] 2.3 State file exists, `started_at` within the staleness threshold (10 minutes, named constant) → print an indicator containing the recorded agent name, exit 0.
- [ ] 2.4 State file exists but `started_at` exceeds the staleness threshold → treat as stale, print the idle indicator (same as 2.2), exit 0. Do not delete the stale file here — `--stop` owns deletion; statusline only reads.
- [ ] 2.5 Unit tests covering all three cases (no file, active-recent, active-stale), plus malformed/missing `session_id` in the payload (exit 0, idle indicator — statusline must never error out and blank the user's status bar).

## 3. Claude Code hook binding

- [ ] 3.1 In `internal/scaffold/templates/hooks/bindings/claude-code/settings-patch.json`, add `{"type": "command", "command": "dreamland agent-status --start"}` to the existing `PreToolUse` entry matched on `Task|Agent`, after the existing `coauthor --hook` entry (order-preserving, don't reorder the existing one).
- [ ] 3.2 Add a `PostToolUse` entry matched on `Task|Agent` with `{"type": "command", "command": "dreamland agent-status --stop"}` — a new matcher entry in the `PostToolUse` array (which today only has the `Bash`-matched `version-bump --change-from-command` entry).
- [ ] 3.3 Add a top-level `"statusLine": {"type": "command", "command": "dreamland statusline"}` key.
- [ ] 3.4 Scaffold tests: after `dreamland init` with Claude Code selected, `.claude/settings.json` contains `dreamland agent-status --start` in `PreToolUse` (`Task|Agent`), `dreamland agent-status --stop` in a `PostToolUse` (`Task|Agent`) entry, and the `statusLine` key invoking `dreamland statusline`.
- [ ] 3.5 Scaffold test: installing GitHub Copilot (or any other platform) produces no `agent-status` or `statusLine` entry anywhere in its output — confirms this change is Claude-Code-only, matching the proposal's stated scope.
- [ ] 3.6 Run `go test ./internal/scaffold/...` and confirm no existing scaffold test breaks.

## 4. Verification

- [ ] 4.1 Run `go test ./...` and confirm all new and existing tests pass.
- [ ] 4.2 Run `bash scripts/pre-merge-check.sh` (or `MERGE_CHECK=1` locally) and confirm the full gate passes, including the 90% coverage floor.
- [ ] 4.3 Dogfood: run `dreamland init` against this repo, confirm `.claude/settings.json` picks up the new hook/statusLine entries, and manually exercise a real Task/Agent dispatch to confirm `dreamland statusline` reflects it (and clears afterward).
