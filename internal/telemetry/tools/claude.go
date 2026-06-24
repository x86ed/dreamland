package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"dreamland/internal/config"
	"dreamland/internal/telemetry"
)

// ClaudeCollector reads a Claude Code Stop hook payload and parses the transcript.
type ClaudeCollector struct{}

func (c *ClaudeCollector) Collect(stdin io.Reader, cfg *config.Config) (*telemetry.SnapshotResult, error) {
	type effort struct {
		Level string `json:"level"`
	}
	type payload struct {
		TranscriptPath string `json:"transcript_path"`
		Effort         effort `json:"effort"`
		SessionID      string `json:"session_id"`
	}

	data, err := io.ReadAll(stdin)
	if err != nil {
		return nil, fmt.Errorf("read stdin: %w", err)
	}

	var p payload
	_ = json.Unmarshal(data, &p) // best-effort; proceed even on parse failure

	thinkingEffort := p.Effort.Level
	if thinkingEffort == "" {
		thinkingEffort = os.Getenv("CLAUDE_EFFORT")
	}

	tu, parseErr := telemetry.ParseTranscript(p.TranscriptPath)
	if parseErr != nil {
		fmt.Fprintf(os.Stderr, "dreamland telemetry: transcript parse warning: %v\n", parseErr)
	}

	model := tu.Model
	if model == "" {
		if cfg != nil {
			model = cfg.ModelID
		}
	}

	return &telemetry.SnapshotResult{
		Tool:           "claude-code",
		Model:          model,
		ThinkingEffort: thinkingEffort,
		InputTokens:    tu.InputTokens,
		OutputTokens:   tu.OutputTokens,
		CachedTokens:   tu.CachedTokens,
	}, nil
}
