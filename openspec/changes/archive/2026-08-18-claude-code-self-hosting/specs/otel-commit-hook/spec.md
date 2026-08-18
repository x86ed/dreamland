## MODIFIED Requirements

### Requirement: Trailer key format
AI telemetry trailers SHALL use the prefix `AI-` followed by a PascalCase field name. The defined trailer keys are:

| Trailer Key | Maps to SnapshotResult field |
|-------------|------------------------------|
| `AI-Tool` | `tool` |
| `AI-Agent` | `agent` |
| `AI-Model` | `model` |
| `AI-ThinkingEffort` | `thinking_effort` |
| `AI-InputTokens` | `input_tokens` |
| `AI-OutputTokens` | `output_tokens` |
| `AI-CachedTokens` | `cached_tokens` |
| `AI-TotalTokens` | `total_tokens` |
| `AI-CapturedAt` | `captured_at` |

`AI-Tool` identifies the coding tool (e.g. `"claude-code"`) and stays constant for a given platform; `AI-Agent` identifies which of the ten registered dreamland agents (`janus`, `phantasos`, `nyx`, `morpheus`, `phobetor`, `baku`, `iktomi`, `zhougong`, `hypnos`, `mengpo`) produced this commit, sourced from the same identity resolution `dreamland coauthor` uses (see the `dev-workflow-hooks` and `session-agent-identity` capabilities) — never a raw, unresolved value.

`AI-ContextSize` is intentionally absent — no supported tool exposes context window size through its hook payload.

Fields with zero or empty values SHALL be omitted from the trailer output.

#### Scenario: Only populated fields appear in trailers
- **WHEN** `thinking_effort` and `context_size` are not available for the active tool
- **THEN** the commit message contains no `AI-ThinkingEffort` or `AI-ContextSize` trailer lines

#### Scenario: AI-Agent trailer identifies the dreamland agent, not just the coding tool
- **WHEN** a commit is produced during a turn resolved to the `phobetor` agent identity
- **THEN** the commit message contains `AI-Tool: claude-code` and `AI-Agent: phobetor` as separate trailer lines

#### Scenario: AI-Agent omitted when no dreamland agent context applies
- **WHEN** `dreamland telemetry snapshot` runs outside any dreamland-scaffolded agent session (e.g. a plain manual commit with no resolvable agent identity)
- **THEN** the commit message contains no `AI-Agent` trailer line, consistent with the zero/empty-value omission rule
