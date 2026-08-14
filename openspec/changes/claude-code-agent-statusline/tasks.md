## 1. `dreamland agent-status --start`/`--stop`

- [x] 1.1 Add `cmd/agent_status.go`: an `agent-status` cobra command with `--start`/`--stop` bool flags, reading a hook JSON payload from stdin and extracting `session_id` and (for `--start`) `tool_input.subagent_type` — reuse the existing payload-parsing approach `agentNameFromHookPayloadFrom` already uses (`cmd/coauthor.go`), don't duplicate it; extract a shared helper if needed rather than copy-pasting the JSON-unmarshal logic.
- [x] 1.2 Add a read-merge-write helper for `.dreamland/agent-status.json`: read the file (missing file = empty map, not an error), unmarshal into `map[string]entry`, apply a caller-supplied mutation, drop any remaining entry whose `started_at` exceeds the staleness threshold, write back. Plain read-then-write, no file locking — an accepted trade-off for a cosmetic feature, see design.md Risks. Not `atomicJSONMerge` (`internal/scaffold/scaffold.go`) — that helper deep-merges nested JSON for settings patches and doesn't support deleting a key, which `--stop` needs.
- [x] 1.3 `--start`: mutation sets this session's key to `{"agent": "<name>", "tool_use_id": "<id>", "started_at": "<RFC3339>"}`. No `subagent_type` in the payload, or an unparseable payload → exit 0, file untouched.
- [x] 1.4 `--stop`: mutation deletes this session's key if present. Missing key, missing file, or unparseable payload → exit 0, no error.
- [x] 1.5 Unit tests: valid `--start` payload sets the expected key/contents; missing `subagent_type` leaves the file untouched; a stale entry from another session is pruned on write; `--stop` removes an existing key; `--stop` on a session with no key is a no-op; malformed JSON on either flag exits 0.
- [x] 1.6 Add `.dreamland/agent-status.json` to `.gitignore` — per-dispatch churn, no history value, would otherwise get swept into every `dreamland commit` checkpoint (`git add -A`). See design.md Decisions.

## 2. `dreamland statusline`

- [x] 2.1 Add `cmd/statusline.go`: a `statusline` cobra command reading the `statusLine` JSON payload from stdin (`session_id` field), looking up that key in `.dreamland/agent-status.json`. Read-only — never writes the file, never prunes (that's `agent-status`'s job).
- [x] 2.2 No matching key (or file doesn't exist) → print an idle indicator (no active dispatch) to stdout, exit 0.
- [x] 2.3 Matching key, `started_at` within the staleness threshold (10 minutes, named constant shared with `agent-status`'s pruning logic) → print an indicator containing the recorded agent name, exit 0.
- [x] 2.4 Matching key, `started_at` exceeds the staleness threshold → treat as stale, print the idle indicator (same as 2.2), exit 0.
- [x] 2.5 Unit tests covering all three cases (no key, active-recent, active-stale), plus malformed/missing `session_id` in the payload (exit 0, idle indicator — statusline must never error out and blank the user's status bar).

## 3. Claude Code hook binding

- [x] 3.1 In `internal/scaffold/templates/hooks/bindings/claude-code/settings-patch.json`, add `{"type": "command", "command": "dreamland agent-status --start"}` to the existing `PreToolUse` entry matched on `Task|Agent`, after the existing `coauthor --hook` entry (order-preserving, don't reorder the existing one).
- [x] 3.2 Add a `PostToolUse` entry matched on `Task|Agent` with `{"type": "command", "command": "dreamland agent-status --stop"}` — a new matcher entry in the `PostToolUse` array (which today only has the `Bash`-matched `version-bump --change-from-command` entry).
- [x] 3.3 Add a top-level `"statusLine": {"type": "command", "command": "dreamland statusline"}` key.
- [x] 3.4 Scaffold tests: after `dreamland init` with Claude Code selected, `.claude/settings.json` contains `dreamland agent-status --start` in `PreToolUse` (`Task|Agent`), `dreamland agent-status --stop` in a `PostToolUse` (`Task|Agent`) entry, and the `statusLine` key invoking `dreamland statusline`.
- [x] 3.5 Scaffold test: installing GitHub Copilot (or any other platform) produces no `agent-status` or `statusLine` entry anywhere in its output — confirms this change is Claude-Code-only, matching the proposal's stated scope.
- [x] 3.6 Run `go test ./internal/scaffold/...` and confirm no existing scaffold test breaks.

## 4. Verification

- [x] 4.1 Run `go test ./...` and confirm all new and existing tests pass.
- [x] 4.2 Run `bash scripts/pre-merge-check.sh` (or `MERGE_CHECK=1` locally) and confirm the full gate passes, including the 90% coverage floor.
- [x] 4.3 Dogfood: run `dreamland init` against this repo, confirm `.claude/settings.json` picks up the new hook/statusLine entries, and manually exercise a real Task/Agent dispatch to confirm `dreamland statusline` reflects it (and clears afterward).
