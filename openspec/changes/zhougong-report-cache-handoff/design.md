## Context

The dashboard and the agent must see identical numbers. Recomputing per question is slow and may drift from what the human sees.

## Decisions

1. **Single cache, two readers.** Dashboard API and `zhougong_snapshot` both read `.dreamland/cache/zhougong/`, so agent and human numbers match by construction. Alternative rejected: agent recomputes from git, which can disagree with the dashboard.
2. **Sha-based staleness, no TTL.** History for a given sha is immutable, so sha is exact. Trade-off: rewritten history (rebase) changes the sha and forces a recollect, which is the desired behavior.
3. **Snapshot is a tool call, not a hook.** "As soon as the user starts asking" is enforced by instruction plus the tool. A `UserPromptSubmit` hook cannot tell a report question from other prompts for this agent without brittle parsing. Trade-off: relies on the model following instructions; open to converting to a hook if the user prefers.
4. **Atomic writes** (write temp file, rename) so a concurrent dashboard read never sees partial JSON.
5. **Branch slug** replaces `/` with `-`; collisions are resolved by storing the real `branch` inside the file and suffixing a short hash of the branch name.
