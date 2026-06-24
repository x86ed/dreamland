package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"dreamland/internal/config"
	"dreamland/internal/telemetry"
)

// KiroCollector handles Kiro hook payloads.
// In the open-telemetry-commit-hooks change, the stop phase is a stub (zero tokens).
// The kiro-bedrock-telemetry change extends this with AWS CloudWatch Logs querying.
type KiroCollector struct {
	// Phase is "start" or "stop". Set via the --phase flag in cmd/telemetry.go.
	Phase string
}

func (c *KiroCollector) Collect(stdin io.Reader, cfg *config.Config) (*telemetry.SnapshotResult, error) {
	data, _ := io.ReadAll(stdin) // best-effort; Kiro payload is minimal

	type payload struct {
		SessionID string `json:"session_id"`
		CWD       string `json:"cwd"`
	}
	var p payload
	_ = json.Unmarshal(data, &p)

	model := ""
	if cfg != nil {
		model = cfg.ModelID
	}

	switch c.Phase {
	case "start":
		return &telemetry.SnapshotResult{
			Tool:       "kiro",
			Model:      model,
			CapturedAt: time.Now().UTC().Format(time.RFC3339),
		}, nil

	default: // "stop" or empty
		logGroup := ""
		if cfg != nil && cfg.BedrockLogGroup != "" {
			logGroup = cfg.BedrockLogGroup
		}

		// Attempt Bedrock CloudWatch query if log group configured.
		if logGroup != "" {
			if result := queryBedrockLogs(logGroup, model); result != nil {
				return result, nil
			}
		}

		return &telemetry.SnapshotResult{
			Tool:  "kiro",
			Model: model,
		}, nil
	}
}

// queryBedrockLogs queries AWS CloudWatch for Bedrock model invocation logs.
// Returns nil if the query fails or produces no usable results.
func queryBedrockLogs(logGroup, fallbackModel string) *telemetry.SnapshotResult {
	startTime := fmt.Sprintf("%d", time.Now().Add(-4*time.Hour).UnixMilli())
	args := []string{
		"logs", "filter-log-events",
		"--log-group-name", logGroup,
		"--start-time", startTime,
		"--filter-pattern", `{ $.schemaType = "ModelInvocationLog" }`,
		"--query", "events[*].message",
		"--output", "json",
	}
	out, err := exec.Command("aws", args...).Output()
	if err != nil {
		return nil
	}

	var messages []string
	if err := json.Unmarshal(out, &messages); err != nil {
		return nil
	}

	type invocInput struct {
		InputTokenCount int64 `json:"inputTokenCount"`
	}
	type invocOutput struct {
		OutputTokenCount int64 `json:"outputTokenCount"`
	}
	type invocLog struct {
		ModelID string      `json:"modelId"`
		Input   invocInput  `json:"input"`
		Output  invocOutput `json:"output"`
	}

	var totalIn, totalOut int64
	var lastModelID string
	for _, msg := range messages {
		var l invocLog
		if err := json.Unmarshal([]byte(msg), &l); err != nil {
			continue
		}
		totalIn += l.Input.InputTokenCount
		totalOut += l.Output.OutputTokenCount
		if l.ModelID != "" {
			lastModelID = l.ModelID
		}
	}

	model := normalizeBedrockModelID(lastModelID)
	if model == "" {
		model = fallbackModel
	}

	return &telemetry.SnapshotResult{
		Tool:         "kiro",
		Model:        model,
		InputTokens:  totalIn,
		OutputTokens: totalOut,
	}
}

// normalizeBedrockModelID strips provider prefix and version suffix from a Bedrock modelId.
// e.g. "anthropic.claude-sonnet-4-6-20251001" → "claude-sonnet-4-6"
func normalizeBedrockModelID(id string) string {
	if id == "" {
		return ""
	}
	// Strip provider prefix (e.g. "anthropic.", "amazon.", "meta.")
	if idx := strings.Index(id, "."); idx >= 0 {
		id = id[idx+1:]
	}
	// Strip trailing version suffix of the form "-YYYYMMDD" or "-v1:0"
	if idx := strings.LastIndex(id, "-"); idx >= 0 {
		suffix := id[idx+1:]
		if len(suffix) == 8 && isDigits(suffix) {
			id = id[:idx]
		} else if strings.HasPrefix(suffix, "v") && strings.Contains(suffix, ":") {
			id = id[:idx]
		}
	}
	return id
}

func isDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
