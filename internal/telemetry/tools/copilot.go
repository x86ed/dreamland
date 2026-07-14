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
)

// CopilotCollector reads a GitHub Copilot Stop/SubagentStop hook payload and parses the transcript.
// GitHub Copilot ships two payload shapes for the same event depending on invocation context:
// camelCase (sessionId/transcriptPath — the GitHub Copilot CLI's native "agentStop" naming) and
// snake_case (session_id/transcript_path, with hook_event_name — VS Code's "compatible" naming,
// which is what the VS Code extension's .github/hooks/*.json hooks actually receive). Both are
// checked; the transcript format itself remains best-effort/undocumented beyond its file path.
type CopilotCollector struct{}

func (c *CopilotCollector) Collect(stdin io.Reader, cfg *config.Config) (*telemetry.SnapshotResult, error) {
	data, err := io.ReadAll(stdin)
	if err != nil {
		return nil, fmt.Errorf("read stdin: %w", err)
	}

	var raw map[string]any
	_ = json.Unmarshal(data, &raw) // best-effort

	transcriptPath, _ := firstStringField(raw, "transcript_path", "transcriptPath")

	tu, parseErr := telemetry.ParseTranscript(transcriptPath)
	if parseErr != nil {
		fmt.Fprintf(os.Stderr, "dreamland telemetry: transcript parse warning (copilot format undocumented): %v\n", parseErr)
	}

	// The real Copilot transcript schema is unverified (ParseTranscript assumes the
	// Claude Code JSONL shape). When extraction comes back empty, capture the raw hook
	// payload and a transcript sample so the real schema can be confirmed from live data
	// instead of guessed from docs — see .dreamland/copilot-hook-debug.jsonl.
	if parseErr != nil || (tu.InputTokens == 0 && tu.OutputTokens == 0 && tu.CachedTokens == 0) {
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
