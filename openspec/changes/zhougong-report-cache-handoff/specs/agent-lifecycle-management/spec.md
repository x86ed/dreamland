## MODIFIED Requirements

### Requirement: Zhou Gong generates agent-performance reports from git history and telemetry

Report and diff analysis SHALL be interpreted from the numbers returned by `zhougong_snapshot` (see the `zhougong-report-cache` capability), not from memory or unstated recomputation. The remaining requirements of this capability, including the report file location and the per-agent breakdown, are unchanged.

#### Scenario: Interpretation cites the snapshot

- **WHEN** the user asks zhougong to explain a diff between two branches
- **THEN** zhougong calls `zhougong_snapshot` for both branches first and cites their `collectedAt`
