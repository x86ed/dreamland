package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"dreamland/internal/config"
	"dreamland/internal/telemetry"
)

// CopilotCollector reads a GitHub Copilot agentStop hook payload and parses the transcript.
// The Copilot transcript format is not publicly documented — all parsing is best-effort.
type CopilotCollector struct{}

func (c *CopilotCollector) Collect(stdin io.Reader, cfg *config.Config) (*telemetry.SnapshotResult, error) {
	type payload struct {
		SessionID      string `json:"sessionId"`
		Timestamp      int64  `json:"timestamp"`
		CWD            string `json:"cwd"`
		TranscriptPath string `json:"transcriptPath"`
		StopReason     string `json:"stopReason"`
	}

	data, err := io.ReadAll(stdin)
	if err != nil {
		return nil, fmt.Errorf("read stdin: %w", err)
	}

	var p payload
	_ = json.Unmarshal(data, &p) // best-effort

	tu, parseErr := telemetry.ParseTranscript(p.TranscriptPath)
	if parseErr != nil {
		fmt.Fprintf(os.Stderr, "dreamland telemetry: transcript parse warning (copilot format undocumented): %v\n", parseErr)
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
