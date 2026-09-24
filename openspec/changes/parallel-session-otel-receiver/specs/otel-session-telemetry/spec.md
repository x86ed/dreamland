## ADDED Requirements

### Requirement: GitHub Copilot collector reports incremental receiver usage

When `dreamland telemetry write --tool github-copilot` sources token counts from the OTLP receiver mailbox (`<state-dir>/sessions/<session_id>.json`, a running total), it SHALL report only the usage added since it last reported for that session, because `telemetry.Write` accumulates whatever the collector returns.

The collector SHALL keep a per-session cursor at `<repo-root>/.dreamland/otel-cursors/<session_id>.json` holding the last-reported `input_tokens`, `output_tokens`, and `cached_tokens` (missing cursor means zero). It SHALL return, per field, `max(total - cursor, 0)`, then set the cursor to the mailbox totals. A `session_id` not matching `^[A-Za-z0-9._-]{1,128}$` SHALL disable this source (no mailbox read, no cursor file). Cursor files older than 7 days in that repository SHALL be deleted on each invocation. `.dreamland/otel-cursors/` SHALL be gitignored alongside the existing `.dreamland/otel-sessions/` and `.dreamland/otel-receiver.log` entries. A cursor read or write failure SHALL NOT fail the command: on read failure the cursor is treated as zero only if no cursor file exists, otherwise the source reports zero for that call and a stderr warning is printed.

This requirement does not change the VS Code chat-session-log source or the transcript fallback, nor any other tool's collector.

#### Scenario: Two Stops with no new span do not double count

- **WHEN** the mailbox totals are 1000 input / 200 output, `telemetry write --tool github-copilot` runs (Stop), and then runs again (SubagentStop or Stop) with no new spans in between
- **THEN** `.dreamland-session.json` contains `input_tokens: 1000`, `output_tokens: 200` after both calls

#### Scenario: New span between Stops adds only the new usage

- **WHEN** the first write occurs at mailbox totals 1000/200, a span adds 300/50, and a second write occurs
- **THEN** `.dreamland-session.json` contains `input_tokens: 1300`, `output_tokens: 250`

#### Scenario: Two repos with the same shared receiver each get their own tokens

- **WHEN** repo X session `A` and repo Y session `B` each produce spans through one shared receiver and each repo's `telemetry write --tool github-copilot` runs with its own `session_id`
- **THEN** repo X's `.dreamland-session.json` contains only session `A`'s tokens and repo Y's contains only session `B`'s

#### Scenario: Cursor is per session

- **WHEN** two sessions in the same repository (`A` and `B`) have mailboxes and each has written once
- **THEN** `.dreamland/otel-cursors/A.json` and `.dreamland/otel-cursors/B.json` exist independently and a later write for `A` does not consume `B`'s usage

#### Scenario: Unsafe session id disables the receiver source

- **WHEN** the hook payload's `session_id` is `../x`
- **THEN** no mailbox is read, no cursor file is created, and collection falls through to the transcript fallback with exit code 0

#### Scenario: Stale cursors are pruned

- **WHEN** the command runs and `.dreamland/otel-cursors/old.json` has a modification time 8 days ago
- **THEN** it is deleted
