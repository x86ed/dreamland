## Context

Dreamland is a Go CLI + MCP server used to orchestrate AI coding tool sessions. The `init` wizard already captures which tool is active and writes `.dreamland.json`. Today, no telemetry leaves the AI tool's own session — token counts, model names, and thinking effort are ephemeral. This design adds a thin collection layer that captures session telemetry from each supported tool and appends it as structured git trailers on every commit.

Six tools are in scope: **Claude Code**, **GitHub Copilot / VSCode**, **Cursor**, **OpenAI Codex CLI**, **AWS Kiro**, and **Antigravity**. Each has a different telemetry surface; the design normalizes them to a common `SnapshotResult` struct.

## Goals / Non-Goals

**Goals:**
- Define a `Collector` interface in `internal/telemetry/` with one implementation per tool
- Add `dreamland telemetry snapshot` CLI command that outputs a normalized JSON snapshot
- Install a `commit-msg` git hook during `dreamland init` that appends telemetry as git trailers
- Scaffold per-tool OTEL configuration files during `dreamland init`
- Wire OTEL spans into the existing MCP server for tool-call observability

**Non-Goals:**
- Real-time streaming telemetry dashboard
- Organization-level aggregation (this is per-session, per-repo)
- Modifying third-party tool APIs or extensions
- Supporting tools not in the six-tool list above

## Decisions

### D1: File-based session cache as the telemetry bus

**Decision:** Each tool writes its session telemetry to `.dreamland-session.json` at the repo root (or `~/.dreamland/telemetry/<repo-hash>.json` for tools that can't write to the repo). The `commit-msg` hook reads this file at commit time.

**Alternatives considered:**
- *Live API call at commit time*: Adds latency and fails if the session has ended or the network is unavailable. Rejected.
- *MCP server in-memory store*: Requires the server to be running at commit time. Rejected as a primary path; retained as an optional real-time path for tools that call the MCP server.
- *stdin pipe from tool*: Not available for tools that run as background LSP servers. Rejected.

**Rationale:** The file is written by each tool's hook at `PostToolUse` or equivalent, giving us the most recent snapshot before the developer runs `git commit`. The hook appends the data; the commit-msg hook reads the latest entry.

---

### D2: Per-tool data collection mechanisms

Each tool has a distinct telemetry surface:

| Tool | Collection mechanism |
|------|---------------------|
| **Claude Code** | `Stop` hook receives JSON on stdin (`usage.inputTokens`, `usage.outputTokens`, `usage.cacheReadInputTokens`, `usage.cacheCreationInputTokens`, `model`) — write these to `.dreamland-session.json` |
| **GitHub Copilot / VSCode** | VSCode task (`tasks.json`) calls `gh api /orgs/{org}/copilot/usage` and writes output to file; for per-user fallback, reads Copilot Language Server log at `~/.config/github-copilot/logs/` |
| **Cursor** | MCP tool call — Cursor calls the dreamland MCP server with session context; MCP handler writes to file. Cursor also supports a `postContext` rule that invokes a terminal command |
| **OpenAI Codex CLI** | Codex CLI writes session records to `~/.codex/sessions/<id>.json` containing `usage` block; `dreamland telemetry snapshot` reads the most-recently-modified file |
| **AWS Kiro** | `.kiro/hooks/` YAML defines a `postToolUse` hook; hook script calls `dreamland telemetry write` which writes to the session file |
| **Antigravity** | Antigravity supports a `hooks.afterResponse` in `.antigravity/config.json`; hook calls `dreamland telemetry write` |

---

### D3: Normalized `SnapshotResult` struct

```go
type SnapshotResult struct {
    Tool          string `json:"tool"`
    Model         string `json:"model"`
    ThinkingEffort string `json:"thinking_effort,omitempty"`
    ContextSize   int64  `json:"context_size,omitempty"`
    InputTokens   int64  `json:"input_tokens"`
    OutputTokens  int64  `json:"output_tokens"`
    CachedTokens  int64  `json:"cached_tokens,omitempty"`
    TotalTokens   int64  `json:"total_tokens"`
    CapturedAt    string `json:"captured_at"`
}
```

Fields missing from a tool's data source are omitted (zero-value + `omitempty`). `TotalTokens` is always computed as `InputTokens + OutputTokens` if not provided natively.

---

### D4: Git trailer format for commit messages

**Decision:** Append telemetry as [git trailer](https://git-scm.com/docs/git-interpret-trailers) lines — `Key: value` pairs separated from the body by a blank line. This is machine-parseable via `git interpret-trailers --parse` and human-readable.

```
feat: add user login

Implements the login flow with JWT.

AI-Tool: Claude Code
AI-Model: claude-sonnet-4-6
AI-ThinkingEffort: high
AI-InputTokens: 15234
AI-OutputTokens: 2341
AI-CachedTokens: 8921
AI-TotalTokens: 26496
AI-CapturedAt: 2026-06-23T14:32:01Z
```

**Alternatives considered:**
- *JSON block in commit body*: Not parseable by standard git tools. Rejected.
- *Separate commit annotation (git notes)*: Requires extra push configuration (`push.pushOption`). Rejected for simplicity.

---

### D5: Hook installation strategy

`dreamland init` installs `.git/hooks/commit-msg` directly (no husky dependency). If the file already exists, it appends the dreamland block guarded by `# BEGIN dreamland-telemetry` / `# END dreamland-telemetry` markers to avoid clobbering existing hooks.

---

### D6: OTEL SDK integration in MCP server

The existing `dreamland serve` (MCP server) gains an OTEL `TracerProvider` initialized at startup. Each MCP tool-call handler wraps its execution in a span with attributes mapping to `SnapshotResult` fields. The exporter is configured via `OTEL_EXPORTER_OTLP_ENDPOINT` (default: stdout JSON for local dev). This follows the [OTLP environment variable spec](https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/).

---

### D7: Codex and Cursor tool additions to init wizard

`dreamland init` currently offers Claude Code, GitHub Copilot, Antigravity, Kiro. **Codex** and **Cursor** are added to the Step 1 tool select list to complete the six-tool coverage.

## Risks / Trade-offs

**[Stale telemetry]** → If the developer commits hours after their last AI interaction, the `.dreamland-session.json` snapshot will be stale. Mitigation: include `CapturedAt` in the trailer so reviewers can judge freshness. The hook can optionally warn if the snapshot is older than a configurable threshold (default: 4 hours).

**[Tool API changes]** → Each tool's telemetry surface is undocumented or subject to change (especially Cursor and Antigravity). Mitigation: each collector is isolated behind the `Collector` interface; updating one doesn't affect others. Collectors that fail return a zero-value result + a `--verbose` warning, never blocking the commit.

**[Commit-msg hook conflicts]** → Many projects already have a `commit-msg` hook (commitlint, etc.). Mitigation: the marker-guarded append strategy (D5) preserves existing hook content.

**[MCP server dependency for Cursor telemetry]** → Cursor telemetry only flows if the MCP server is running. Mitigation: fall back to Cursor's `~/.cursor/logs/` (if accessible) before returning empty.

**[Token count accuracy]** → Some tools report tokens per-request, not per-session. `dreamland telemetry write` sums counts across the session by accumulating into `.dreamland-session.json` rather than overwriting. The commit hook reads the accumulated totals.

## Migration Plan

1. `go get go.opentelemetry.io/otel` — add SDK to `go.mod` (no breaking changes)
2. New `internal/telemetry/` package — additive
3. New `cmd/telemetry.go` — additive command
4. `cmd/init.go` — extend to add Codex/Cursor options and call scaffolding step post-save
5. `internal/scaffold/` — new package; templates embedded via `go:embed`
6. `scripts/` — no changes needed
7. Existing `.dreamland.json` — new optional `telemetry` object added; old configs without it work unchanged

Rollback: the `commit-msg` hook is removed by `dreamland telemetry uninstall` or by deleting the marker block manually. The OTEL TracerProvider is a no-op if `OTEL_SDK_DISABLED=true`.

## Open Questions

1. **Antigravity hooks API**: Is `hooks.afterResponse` the correct key in `.antigravity/config.json`, or has the schema changed? Needs verification against latest Antigravity docs.
2. **Cursor MCP context**: Does Cursor pass session usage data in the MCP `CallTool` request, or only in its own telemetry stream? Needs testing with Cursor 0.44+.
3. **Threshold for stale snapshot warning**: Should the default be 1 hour, 4 hours, or configurable only? Recommend making it configurable with a 4-hour default.
4. **`.dreamland-session.json` in `.gitignore`**: Should `init` add this file to `.gitignore` automatically? It contains token counts but no secrets; leaning toward yes for cleanliness.
