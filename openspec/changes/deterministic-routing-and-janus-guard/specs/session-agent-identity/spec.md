## ADDED Requirements

### Requirement: On Claude Code, the session's current agent identity is recorded per session, keyed by the hook payload's `session_id`

The identity that end-of-turn attribution uses (the `(<agent-name>)` in a `chore: turn-complete checkpoint` subject and that commit's author) SHALL NOT depend on a value shared between concurrent sessions. `git config --local user.name`/`user.email` are repository-wide and last-writer-wins: two Claude Code windows in one repository overwrite each other's identity between a `PreToolUse` dispatch and the next `Stop`, so a turn that dispatched `nyx` in one window could be committed as `morpheus` (or `janus`) because another window's hook ran last. The current identity is therefore also recorded in a per-session file.

**Location.** `<root>/identity/<session_id>`, where `<root>` is `$DREAMLAND_STATE_DIR` if set, else `<os.UserCacheDir()>/dreamland`, else `<os.TempDir()>/dreamland`. This is the per-user state directory introduced by the `parallel-session-otel-receiver` change (which places its own files under `$DREAMLAND_STATE_DIR`, or `<UserCacheDir>/dreamland/otel` by default; the `identity/` subdirectory never collides with `sessions/`, `receiver.log`, or `receiver-<port>.*`). The path is built with `filepath`, so it is valid on Windows (`%LocalAppData%\dreamland\identity\<session_id>`). The directory is created with mode 0700 and files with mode 0600 (best effort; a no-op on Windows).

**Key.** `session_id` is read from the hook payload's top-level `session_id` string, which every Claude Code hook payload carries, including bare `Stop`. It is valid only when it matches `^[A-Za-z0-9._-]{1,128}$` and is not `.` or `..` (the same rule as `ValidConversationID` in the receiver's state package, so it can never traverse out of the directory). An absent or invalid `session_id` means "no session key": nothing is read or written, and callers use the legacy git-config behavior described in the `dev-workflow-hooks` capability's `dreamland commit` requirement.

**Content and writes.** The file holds one JSON object `{"agent":"<name>","updated_at":"<RFC3339>"}`. Each write is atomic: the object is written to a temporary file in the same directory and renamed over the target (`os.Rename`, which replaces an existing file on both Unix and Windows), so a concurrent reader sees the old or the new value, never a partial one. Each write also makes a best-effort pass deleting entries in that directory whose modification time is older than 7 days; errors from that pass are ignored.

**Writers.** `dreamland coauthor --hook`, when its payload has a valid `session_id`, writes the resolved identity to the session's file in the same step in which it sets `git config --local user.name`/`user.email`: at `SessionStart` (identity `janus`, the documented default) and at `PreToolUse` for `Agent`/`Task` (identity from `tool_input.subagent_type`, subject to the existing registered-name filter). The git config write is retained, unchanged, for humans, for other tools, and for platforms and invocations without a `session_id`; it is no longer authoritative for attribution when a `session_id` is available. `coauthor --agent-name <x>` (the per-agent frontmatter `Stop` blocks) has no payload and does not touch the session file. A failure to write the session file is advisory (a message on stderr, exit 0): the retained git config write is the fallback, and the failure of the file must not block a lifecycle event that the git config write already covers.

**Readers.** `dreamland commit --hook` and `dreamland test-and-commit --hook`, when invoked with `--reason turn-complete`, read the session's file by the payload's `session_id` (see the `dreamland commit` requirement). The `agent` value read is subject to the registered-name filter from the "An unrecognized identity value never propagates" requirement: an unregistered, empty, or unreadable value is treated as `janus`. `dreamland guard-router` never reads it: an access decision must not depend on file state (see the `router-dispatch-guard` capability). `dreamland telemetry write` is unchanged: on a bare `Stop` it continues to attribute the turn to `janus`, which is accurate for the main-thread turn, and it never read the shared git config.

`test-and-commit` SHALL accept `--hook` and forward it, together with the payload it read from stdin, to its commit step, reading stdin at most once. Without `--hook` it behaves exactly as before.

**Scope.** Only Claude Code's bindings pass `--hook` to the stop-event command in this change. Other platforms' bindings are unchanged and keep the git-config behavior; their payload shapes for a session key are not verified here.

**Degradation.** A session that started before the binary was upgraded has no file; its `Stop` commits are attributed to `janus` until its next dispatch writes one. This is stated, not hidden: the alternative (falling back to the shared git config for a valid `session_id`) reintroduces the cross-session mislabel.

#### Scenario: coauthor records the dispatched agent under the session's id

- **WHEN** `dreamland coauthor --hook` runs at `PreToolUse` with payload `{"session_id":"S1","tool_name":"Agent","tool_input":{"subagent_type":"nyx"}}`
- **THEN** `<root>/identity/S1` contains `"agent":"nyx"`
- **AND** `git config --local user.name` is also set to `nyx`, as before

#### Scenario: SessionStart records janus for the new session

- **WHEN** `dreamland coauthor --hook` runs at `SessionStart` with payload `{"session_id":"S1"}`
- **THEN** `<root>/identity/S1` contains `"agent":"janus"`

#### Scenario: Parallel sessions do not overwrite each other

- **WHEN** session `S1` dispatches `nyx` and session `S2` then dispatches `morpheus`, both in the same repository, and each then ends a turn with pending changes
- **THEN** `S1`'s Stop commit is attributed to `nyx` and `S2`'s to `morpheus`, regardless of which `git config --local user.name` write happened last

#### Scenario: A hostile session_id cannot leave the directory

- **WHEN** a payload's `session_id` is `../../etc/passwd` or is longer than 128 characters
- **THEN** no file is read or written, and no error is raised beyond an advisory note

#### Scenario: An unregistered recorded value reads back as janus

- **WHEN** the file for `S1` contains `"agent":"general-purpose"` (not a registered dreamland agent) and `dreamland commit --reason turn-complete --hook` runs for `S1`
- **THEN** the commit is attributed to `janus`

#### Scenario: The state root honors DREAMLAND_STATE_DIR

- **WHEN** `DREAMLAND_STATE_DIR` is set to a temp directory and `coauthor --hook` records an identity
- **THEN** the file is `<that directory>/identity/<session_id>` and nothing is written under the user cache directory

#### Scenario: A failed session-file write does not block the hook

- **WHEN** the identity directory cannot be created (e.g. a read-only cache directory) during `coauthor --hook`
- **THEN** the git config write still happens, an advisory note is written to stderr, and the command exits 0

#### Scenario: Old entries are pruned

- **WHEN** `coauthor --hook` writes an entry and the identity directory holds a file whose modification time is older than 7 days
- **THEN** the old file is removed and the new entry is present

#### Scenario: test-and-commit forwards the payload once

- **WHEN** `dreamland test-and-commit --reason turn-complete --hook` runs with a `Stop` payload on stdin
- **THEN** stdin is read once, the test step runs, and the commit step resolves identity from that same payload's `session_id`
