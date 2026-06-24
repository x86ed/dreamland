package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"dreamland/internal/config"
	"dreamland/internal/telemetry"
)

// AntigravityCollector reads an Antigravity PostTurnHook payload and parses the transcript.
type AntigravityCollector struct{}

func (c *AntigravityCollector) Collect(stdin io.Reader, cfg *config.Config) (*telemetry.SnapshotResult, error) {
	type payload struct {
		TranscriptPath        string   `json:"transcriptPath"`
		ConversationID        string   `json:"conversationId"`
		StepIdx               int      `json:"stepIdx"`
		WorkspacePaths        []string `json:"workspacePaths"`
		ArtifactDirectoryPath string   `json:"artifactDirectoryPath"`
	}

	data, err := io.ReadAll(stdin)
	if err != nil {
		return nil, fmt.Errorf("read stdin: %w", err)
	}

	var p payload
	_ = json.Unmarshal(data, &p) // best-effort

	tu, transcriptModel, parseErr := telemetry.ParseAntigravityTranscript(p.TranscriptPath)
	if parseErr != nil {
		fmt.Fprintf(os.Stderr, "dreamland telemetry: transcript parse warning (antigravity): %v\n", parseErr)
	}

	model := transcriptModel
	if model == "" && cfg != nil {
		model = cfg.ModelID
	}

	return &telemetry.SnapshotResult{
		Tool:         "antigravity",
		Model:        model,
		InputTokens:  tu.InputTokens,
		OutputTokens: tu.OutputTokens,
		CachedTokens: tu.CachedTokens,
	}, nil
}
