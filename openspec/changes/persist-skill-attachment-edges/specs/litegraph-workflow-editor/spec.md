## ADDED Requirements

### Requirement: Graph-only edges survive a graph rebuild

An edge kind with no representation in any of the six live platform files (currently: `EdgeAttachment`, recorded by skill attach/detach; any future edge kind introduced with the same property) SHALL be preserved across every graph rebuild — the same on-disk-cache-and-merge mechanism `AgentNode` position already uses, since `Import` cannot re-derive either from the six live platform directories. A rebuild triggered by a mutation, by the filesystem-watcher poll, or by a fresh server start after a process restart MUST NOT silently drop a graph-only edge whose endpoints both still exist after that rebuild. A graph-only edge whose skill or agent endpoint no longer exists after a rebuild (e.g. the skill or agent was deleted through some other operation) MAY be dropped rather than resurrected as a dangling edge.

#### Scenario: An attached skill survives a subsequent unrelated mutation

- **WHEN** a user attaches the `openspec-propose` skill node to an agent's `skills` input in `/hypnos-interactive` and saves, then performs a second, unrelated save (e.g. moving a different node) in the same session
- **THEN** the second save's rebuild-from-disk step still reflects the skill attachment made in the first save — it is not dropped

#### Scenario: An attached skill survives the filesystem-watcher's periodic rebuild

- **WHEN** a skill has been attached to an agent and saved, and the ~2-second filesystem-watcher poll subsequently triggers its own rebuild with no further user action
- **THEN** the attachment is still present in the graph served after that poll-triggered rebuild

#### Scenario: An attached skill survives a server restart

- **WHEN** a skill has been attached to an agent and saved, and the local server process is then stopped and a new `/hypnos-interactive` (or `/hypnos-view`) process is started against the same repository
- **THEN** the newly started process's initial graph reflects the previously saved attachment, not a graph with zero attachment edges

#### Scenario: Deleting an attached skill or agent does not resurrect a dangling attachment edge on the next rebuild

- **WHEN** an agent (or skill) that had an active attachment edge is deleted, and a subsequent rebuild occurs (mutation-triggered, watcher-poll-triggered, or a fresh process start)
- **THEN** the rebuilt graph contains no attachment edge referencing the deleted agent or skill id
