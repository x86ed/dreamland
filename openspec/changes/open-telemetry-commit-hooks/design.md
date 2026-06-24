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

`dreamland telemetry write --tool <name>` is registered in each platform's end-of-turn hook (the same binding files defined in `dev-workflow-hooks`). What data is available at hook execution time differs significantly by tool:

| Tool | Hook file | Event | Data available at hook time |
| ---- | --------- | ----- | --------------------------- |
| **Claude Code** | `.claude/settings.json` | `Stop` | stdin JSON: `transcript_path`, `effort.level`, `session_id`, `stop_hook_active`; env: `CLAUDE_EFFORT`. Token counts summed from transcript JSONL (`message.usage.*`). Model from `.dreamland.json`. |
| **Codex CLI** | `.codex/hooks.json` | `Stop` | stdin JSON: `model` (directly), `transcript_path`, `session_id`, `turn_id`, `permission_mode`. Token counts from transcript JSONL (unstable format — parse with error recovery). |
| **Cursor** | `.cursor/hooks.json` | `stop` | stdin JSON: `model`, `model_id`, `model_params`, `conversation_id`, `generation_id`, `transcript_path`, `status`, `loop_count`, `cursor_version`, `workspace_roots`, `user_email`. Model directly available; token counts from transcript JSONL (same parser as Claude Code/Codex). `CURSOR_TRANSCRIPT_PATH` env var also set automatically. |
| **Kiro** | `.kiro/agent.json` | `stop` | Stub only in this change. Kiro's `stop` hook payload contains only `{hook_event_name, cwd, session_id, assistant_response}` — no tokens, no model, no transcript path. `dreamland telemetry write --tool kiro` writes `tool: "kiro"` and `model` from `.dreamland.json`; all token counts zero. Real token data via AWS Bedrock model invocation logging is implemented in the `kiro-bedrock-telemetry` change (depends on this change). |
| **Antigravity** | `~/.gemini/antigravity-cli/plugins/dreamland/hooks.json` | `PostTurnHook` | Payload schema not publicly documented. Best-effort stdin parsing; unknown fields zeroed. Model from `.dreamland.json`. |
| **GitHub Copilot** | `.github/copilot-hooks/hooks-manifest.json` | stub | No public hook API. Write command is a no-op that documents two alternatives: (1) OTel traces via Copilot CLI SDK (`OTEL_EXPORTER_OTLP_ENDPOINT`); (2) `token-usage.jsonl` artifact (input/output/cache tokens, model, provider per API call). |

**`context_size` removed from SnapshotResult**: No supported tool exposes context window size through its hook payload or a stable programmatic API at hook execution time.

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
7. Existing `.dreamland.json` — `otel_endpoint` field added (optional, additive); old configs without it work unchanged

Rollback: the `commit-msg` hook is removed by `dreamland telemetry uninstall` or by deleting the marker block manually. The OTEL TracerProvider is a no-op if `OTEL_SDK_DISABLED=true`.

---

### D8: Platform-specific OTEL environment injection strategy

**Decision:** Each platform requires a different mechanism to inject `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_PROTOCOL`, and `OTEL_SERVICE_NAME` into the agent session. The Go scaffold code generates a small, platform-specific shell script or config merge for each tool at `dreamland init` time, with the endpoint baked in from `.dreamland.json`.

The three core OTEL env vars are identical across all platforms. What varies is *how* they reach the session:

**Claude Code — `CLAUDE_ENV_FILE` (SessionStart hook):**
Claude Code does not have native OTEL support. `SessionStart` hooks receive the `CLAUDE_ENV_FILE` environment variable pointing to a file whose contents are sourced into subsequent Bash tool invocations. The scaffold writes `.claude/scripts/dreamland-otel-env.sh` and registers it in the `SessionStart` hook array in `.claude/settings.json`. The script appends `KEY=VALUE` lines (no `export` prefix — Claude Code sources them as-is) to `$CLAUDE_ENV_FILE`.

**Codex CLI — user-level `config.toml` only (example file written):**
Codex has native OTEL support via `[otel]` in `config.toml`, but Codex explicitly blocks project-level `config.toml` from setting OTEL keys — only `~/.codex/config.toml` is respected. The scaffold writes `.codex/otel-config.example.toml` with the correct TOML and a merge instruction, and prints a one-time warning at `init` time. The `Stop` hook in `.codex/hooks.json` runs `dreamland telemetry write` as normal.

**Cursor — `sessionStart` hook JSON output:**
Cursor `sessionStart` hooks can return `{"env": {"KEY": "VALUE"}}` JSON on stdout. These vars are session-scoped and propagated to all subsequent hooks (`preToolUse`, `postToolUse`, `stop`, etc.). The scaffold writes `.cursor/hooks/dreamland-otel-env.sh` that prints this JSON, and registers it in the `sessionStart` array in `.cursor/hooks.json`.

**Kiro — `agentSpawn` hook (best-effort):**
Kiro's `agentSpawn` shell command hook runs at agent start. Whether its exported env vars propagate to subsequent hooks is not publicly documented. The scaffold writes `.kiro/hooks/dreamland-otel-env.sh` and registers it, but also prints a notice at init time that env propagation from Kiro's `agentSpawn` shell hooks should be validated against the current Kiro version.

**Antigravity — `.agents/hooks.json` `SessionStart` + `IDE_OTEL_IDE_NAME`:**
Antigravity uses `.agents/hooks.json` for project-scoped hooks. The `SessionStart` event runs the OTEL env script. An additional env var `IDE_OTEL_IDE_NAME=antigravity` is required by the `opentelemetry-hooks` ecosystem for Antigravity-specific span attribution. Both the project-scoped `.agents/hooks.json` and the plugin bundle `~/.gemini/antigravity-cli/plugins/dreamland/hooks.json` are written — the former for OTEL env setup, the latter for `PostTurnHook` telemetry write (per D1).

**GitHub Copilot — `.vscode/settings.json` (native OTel support):**
GitHub Copilot has native OpenTelemetry support exposed through VS Code settings. The scaffold merges three keys into `.vscode/settings.json`: `github.copilot.chat.otel.enabled`, `github.copilot.chat.otel.exporterType`, and `github.copilot.chat.otel.otlpEndpoint`. Because Copilot's native OTel uses OTLP HTTP (port 4318), the endpoint is derived from the configured value (replacing port 4317 with 4318 if the user used the gRPC default).

**Alternatives considered:**
- *Single `.env` file that all tools source*: tools don't share a universal `.env` loading mechanism. Rejected.
- *Requiring a running OTEL collector sidecar*: out of scope for `dreamland init`. Rejected.
- *Using a `dreamland serve` MCP sidecar as the OTEL relay*: valid for future work but adds a required running process. Deferred.

---

## Open Questions

1. **Kiro `agentSpawn` env propagation**: Does env set in `agentSpawn` shell hook stdout propagate to `stop` hook? Needs validation against current Kiro release.
2. **Codex user-level OTEL**: Should `dreamland init` offer to write `~/.codex/config.toml` directly (with user consent) rather than just writing the example file? Recommend asking: makes the setup complete for Codex users.
3. **Stale snapshot warning threshold**: Default 4h configurable via `--max-age`. Resolved — keep configurable.
4. **Copilot port translation**: When `otel_endpoint` uses gRPC port 4317, auto-translate to 4318 for `github.copilot.chat.otel.otlpEndpoint`. Risk: breaks if user intentionally runs HTTP on 4317. Recommend storing `otel_endpoint_http` separately in `.dreamland.json`.
