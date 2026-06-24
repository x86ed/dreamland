package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"dreamland/internal/config"
	"dreamland/internal/telemetry"
)

// CursorCollector reads a Cursor stop hook payload and parses the transcript.
type CursorCollector struct{}

func (c *CursorCollector) Collect(stdin io.Reader, cfg *config.Config) (*telemetry.SnapshotResult, error) {
	type payload struct {
		Model          string  `json:"model"`
		ModelID        string  `json:"model_id"`
		TranscriptPath *string `json:"transcript_path"`
		Status         string  `json:"status"`
		LoopCount      int     `json:"loop_count"`
	}

	data, err := io.ReadAll(stdin)
	if err != nil {
		return nil, fmt.Errorf("read stdin: %w", err)
	}

	var p payload
	_ = json.Unmarshal(data, &p) // best-effort

	model := p.Model
	if model == "" {
		model = p.ModelID
	}
	if model == "" && cfg != nil {
		model = cfg.ModelID
	}

	transcriptPath := ""
	if p.TranscriptPath != nil {
		transcriptPath = *p.TranscriptPath
	}
	if transcriptPath == "" {
		transcriptPath = os.Getenv("CURSOR_TRANSCRIPT_PATH")
	}

	tu, parseErr := telemetry.ParseTranscript(transcriptPath)
	if parseErr != nil {
		fmt.Fprintf(os.Stderr, "dreamland telemetry: transcript parse warning (cursor): %v\n", parseErr)
	}

	return &telemetry.SnapshotResult{
		Tool:         "cursor",
		Model:        model,
		InputTokens:  tu.InputTokens,
		OutputTokens: tu.OutputTokens,
		CachedTokens: tu.CachedTokens,
	}, nil
}
