## ADDED Requirements

### Requirement: `dreamland agent-status --start` records a dispatch

`dreamland agent-status --start` SHALL read a hook payload from stdin, extract the dispatched agent's identity the same way `agentNameFromHookPayloadFrom` already does (`tool_input.subagent_type` on Claude Code) and `session_id` from the same payload, then read-merge-write `.dreamland/agent-status.json` — a single JSON object keyed by `session_id` — setting `{agent, tool_use_id, started_at}` under this session's key. Any other key in the file whose `started_at` exceeds the staleness threshold (10 minutes) SHALL be dropped as part of this write, so orphaned sessions from missed `--stop` signals self-prune on the next dispatch anywhere in the repo. If no `subagent_type` is present, or the payload can't be parsed, the command SHALL exit 0 and leave the file untouched — dispatch tracking is best-effort and must never block the `PreToolUse` hook it's bound to.

#### Scenario: Valid dispatch payload

- **WHEN** `dreamland agent-status --start` receives a `PreToolUse` payload with `session_id: "abc"` and `tool_input.subagent_type: "morpheus"`
- **THEN** `.dreamland/agent-status.json` contains a key `"abc"` with `agent: "morpheus"` and a `started_at` timestamp

#### Scenario: Payload with no subagent_type

- **WHEN** `dreamland agent-status --start` receives a payload with no `tool_input.subagent_type` field
- **THEN** the command exits 0 and `.dreamland/agent-status.json` is not modified

#### Scenario: Write prunes stale entries from other sessions

- **WHEN** `dreamland agent-status --start` runs for session `"abc"` and `.dreamland/agent-status.json` already has an entry for session `"old"` with `started_at` 20 minutes ago
- **THEN** the resulting file has the new `"abc"` entry and no `"old"` entry

### Requirement: `dreamland agent-status --stop` clears a dispatch

`dreamland agent-status --stop` SHALL read a hook payload from stdin, extract `session_id`, and read-merge-write `.dreamland/agent-status.json` to remove this session's key if present, pruning stale entries from other sessions the same way `--start` does. Absence of this session's key, or an unparseable payload, SHALL NOT cause a non-zero exit — clearing is best-effort, matching `--start`.

#### Scenario: Stop clears an existing entry

- **WHEN** `dreamland agent-status --stop` receives a payload with `session_id: "abc"` and `.dreamland/agent-status.json` has a key `"abc"`
- **THEN** the key `"abc"` is removed from the file

#### Scenario: Stop with no matching entry

- **WHEN** `dreamland agent-status --stop` runs for a session with no matching key in the file (or the file doesn't exist)
- **THEN** the command exits 0 without error

### Requirement: `dreamland statusline` renders the current dispatch state

`dreamland statusline` SHALL read a `statusLine` JSON payload from stdin, extract `session_id`, look up that key in `.dreamland/agent-status.json` (read-only — this command never writes the file), and print a status segment to stdout:

- If no entry exists for the session (or the file doesn't exist): print an idle indicator (no active dispatch).
- If an entry exists and its `started_at` is within the staleness threshold (10 minutes): print the recorded `agent` name.
- If an entry exists but `started_at` exceeds the staleness threshold: treat it as stale — print the idle indicator, identical to the no-entry case. A missed `--stop` signal SHALL NOT cause a permanently incorrect display.

The command SHALL exit 0 in all three cases; a missing or malformed state file is not an error.

#### Scenario: No active dispatch

- **WHEN** `dreamland statusline` runs for a session with no matching key in `.dreamland/agent-status.json`
- **THEN** it prints the idle indicator and exits 0

#### Scenario: Active, recent dispatch

- **WHEN** `dreamland statusline` runs for a session whose entry has `agent: "phobetor"` and `started_at` 30 seconds ago
- **THEN** it prints an indicator containing `phobetor` and exits 0

#### Scenario: Stale dispatch entry

- **WHEN** `dreamland statusline` runs for a session whose entry has `started_at` 15 minutes ago (past the 10-minute threshold)
- **THEN** it prints the idle indicator, not the stale agent name, and exits 0

### Requirement: Claude Code hook binding wires start/stop/statusline

The Claude Code `settings-patch.json` binding SHALL add `dreamland agent-status --start` to the existing `PreToolUse` entry matched on `Task|Agent` (alongside `coauthor --hook`, order-preserving) and `dreamland agent-status --stop` to a `PostToolUse` entry matched on `Task|Agent`. It SHALL also add a top-level `statusLine` key: `{"type": "command", "command": "dreamland statusline"}`. No other platform's binding (GitHub Copilot, Cursor, Codex, Kiro, Antigravity) is modified by this change.

#### Scenario: Claude Code install wires all three

- **WHEN** `dreamland init` scaffolds Claude Code
- **THEN** the resulting `.claude/settings.json` contains `dreamland agent-status --start` in its `PreToolUse` (`Task|Agent`) array, `dreamland agent-status --stop` in its `PostToolUse` (`Task|Agent`) array, and a `statusLine` entry invoking `dreamland statusline`

#### Scenario: Other platforms unaffected

- **WHEN** `dreamland init` scaffolds GitHub Copilot
- **THEN** no `agent-status` or `statusLine` entry is present anywhere in its output
