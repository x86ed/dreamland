package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"dreamland/internal/config"
	"dreamland/internal/telemetry"
	"dreamland/internal/telemetry/cursor"
	"dreamland/internal/telemetry/otelreceiver"
)

// CopilotCollector reads a GitHub Copilot Stop/SubagentStop hook payload. GitHub Copilot
// ships two payload shapes for the same event depending on invocation context: camelCase
// (sessionId/transcriptPath — the GitHub Copilot CLI's native "agentStop" naming) and
// snake_case (session_id/transcript_path, with hook_event_name — VS Code's "compatible"
// naming, which is what the VS Code extension's .github/hooks/*.json hooks actually send).
// Both are checked.
//
// Token usage is sourced, in order, from:
//  1. VS Code's own chat-session log (workspaceStorage/<hash>/chatSessions/<session_id>.jsonl)
//     — the real, verified source: confirmed by parsing a live file and finding real
//     non-zero promptTokens/completionTokens. Neither the hook payload, the transcript
//     file, nor Copilot's OTel export (confirmed empirically: nothing ever arrives at a
//     real running OTLP receiver in this environment) carry usage data.
//  2. The shared OTLP receiver's per-session mailbox in the per-user state directory (see
//     internal/telemetry/otelreceiver), kept as a fallback in case the OTel export does
//     start working in some environment. The mailbox holds running totals, so this source
//     is incremental: only the usage added since the last report (a per-session cursor in
//     <repo>/.dreamland/otel-cursors/) is returned, because telemetry.Write accumulates.
//     The cursor advances before Write runs; if Write then fails, that delta is lost.
//  3. ParseTranscript, kept as a last-resort forward-compatible fallback.
type CopilotCollector struct{}

func (c *CopilotCollector) Collect(stdin io.Reader, cfg *config.Config) (*telemetry.SnapshotResult, error) {
	data, err := io.ReadAll(stdin)
	if err != nil {
		return nil, fmt.Errorf("read stdin: %w", err)
	}

	var raw map[string]any
	_ = json.Unmarshal(data, &raw) // best-effort

	transcriptPath, _ := firstStringField(raw, "transcript_path", "transcriptPath")
	sessionID, _ := firstStringField(raw, "session_id", "sessionId")

	var tu telemetry.TranscriptUsage
	var sourceFound bool

	if sessionID != "" {
		if sessionFile, findErr := findChatSessionFile(sessionID); findErr == nil {
			if promptTokens, completionTokens, parseErr := parseChatSessionTokens(sessionFile); parseErr == nil && (promptTokens > 0 || completionTokens > 0) {
				tu.InputTokens = promptTokens
				tu.OutputTokens = completionTokens
				sourceFound = true
			}
		}
	}

	if !sourceFound && cfg != nil && cfg.RepoRoot != "" && otelreceiver.ValidConversationID(sessionID) {
		if usage, oerr := otelreceiver.ReadSessionUsage(otelreceiver.StateDir(), sessionID); oerr == nil && usage != nil {
			delta := incrementalUsage(cfg.RepoRoot, sessionID, usage)
			tu.InputTokens = delta.Input
			tu.OutputTokens = delta.Output
			tu.CachedTokens = delta.Cached
			if usage.Model != "" {
				tu.Model = usage.Model
			}
			sourceFound = true
		}
	}

	var parseErr error
	if !sourceFound {
		tu, parseErr = telemetry.ParseTranscript(transcriptPath)
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "dreamland telemetry: transcript parse warning (copilot format undocumented): %v\n", parseErr)
		}
		if tu.InputTokens != 0 || tu.OutputTokens != 0 || tu.CachedTokens != 0 {
			sourceFound = true
		}
	}

	// None of the three sources had anything: capture the raw hook payload and a
	// transcript sample so any new/changed schema can be confirmed from live data instead
	// of guessed from docs — see .dreamland/copilot-hook-debug.jsonl.
	if !sourceFound {
		captureDebugPayload(cfg, data, transcriptPath, parseErr)
	}

	model := tu.Model
	if model == "" && cfg != nil {
		model = cfg.ModelID
	}

	return &telemetry.SnapshotResult{
		Tool:         "github-copilot",
		Model:        model,
		InputTokens:  tu.InputTokens,
		OutputTokens: tu.OutputTokens,
		CachedTokens: tu.CachedTokens,
	}, nil
}

// incrementalUsage returns the usage in the mailbox totals that has not been reported yet
// for sessionID (per field max(total - cursor, 0)) and advances the cursor to the totals.
// Cursor problems never fail the command: they print a warning and report zero for this
// call. An unreadable cursor is overwritten so later calls recover; when the cursor cannot
// be advanced, zero is reported too, because returning the delta again on every Stop would
// double count.
func incrementalUsage(repoRoot, sessionID string, totals *otelreceiver.SessionUsage) cursor.Counts {
	cursor.Prune(repoRoot, 7*24*time.Hour)

	total := cursor.Counts{Input: totals.InputTokens, Output: totals.OutputTokens, Cached: totals.CachedTokens}
	last, _, loadErr := cursor.Load(repoRoot, sessionID)
	if loadErr != nil {
		fmt.Fprintf(telemetry.Stderr, "dreamland telemetry: unreadable otel cursor for session %s, reporting no new usage this time: %v\n", sessionID, loadErr)
		last = total
	}
	if err := cursor.Store(repoRoot, sessionID, total); err != nil {
		fmt.Fprintf(telemetry.Stderr, "dreamland telemetry: cannot write otel cursor for session %s, reporting no new usage: %v\n", sessionID, err)
		return cursor.Counts{}
	}
	return cursor.Counts{
		Input:  positive(total.Input - last.Input),
		Output: positive(total.Output - last.Output),
		Cached: positive(total.Cached - last.Cached),
	}
}

func positive(n int64) int64 {
	if n < 0 {
		return 0
	}
	return n
}

// captureDebugPayload appends the raw hook payload and (if resolvable) a sample of the
// transcript file to .dreamland/copilot-hook-debug.jsonl. Best-effort: failures are silent,
// this must never break telemetry write itself.
func captureDebugPayload(cfg *config.Config, hookPayload []byte, transcriptPath string, parseErr error) {
	if cfg == nil || cfg.RepoRoot == "" {
		return
	}

	entry := map[string]any{
		"captured_at":     time.Now().UTC().Format(time.RFC3339),
		"hook_payload":    json.RawMessage(hookPayload),
		"transcript_path": transcriptPath,
	}
	if parseErr != nil {
		entry["parse_error"] = parseErr.Error()
	}
	if transcriptPath != "" {
		const maxSample = 4000
		if b, err := os.ReadFile(transcriptPath); err == nil {
			if len(b) > maxSample {
				b = b[:maxSample]
			}
			entry["transcript_sample"] = string(b)
		} else {
			entry["transcript_read_error"] = err.Error()
		}
	}

	line, err := json.Marshal(entry)
	if err != nil {
		return
	}

	path := filepath.Join(cfg.RepoRoot, ".dreamland", "copilot-hook-debug.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	f.Write(append(line, '\n'))
}

// firstStringField returns the first non-empty string value found in m for any of keys, in order.
func firstStringField(m map[string]any, keys ...string) (string, bool) {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && v != "" {
			return v, true
		}
	}
	return "", false
}
