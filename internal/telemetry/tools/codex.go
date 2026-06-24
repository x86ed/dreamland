package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"dreamland/internal/config"
	"dreamland/internal/telemetry"
)

// CodexCollector reads a Codex CLI Stop hook payload and parses the transcript.
// Note: the Codex transcript format is explicitly unstable per official docs.
type CodexCollector struct{}

func (c *CodexCollector) Collect(stdin io.Reader, cfg *config.Config) (*telemetry.SnapshotResult, error) {
	type payload struct {
		Model          string `json:"model"`
		TranscriptPath string `json:"transcript_path"`
		SessionID      string `json:"session_id"`
	}

	data, err := io.ReadAll(stdin)
	if err != nil {
		return nil, fmt.Errorf("read stdin: %w", err)
	}

	var p payload
	_ = json.Unmarshal(data, &p) // best-effort

	model := p.Model
	if model == "" && cfg != nil {
		model = cfg.ModelID
	}

	tu, parseErr := telemetry.ParseTranscript(p.TranscriptPath)
	if parseErr != nil {
		fmt.Fprintf(os.Stderr, "dreamland telemetry: transcript parse warning (codex format unstable): %v\n", parseErr)
	}

	return &telemetry.SnapshotResult{
		Tool:         "codex",
		Model:        model,
		InputTokens:  tu.InputTokens,
		OutputTokens: tu.OutputTokens,
		CachedTokens: tu.CachedTokens,
	}, nil
}
