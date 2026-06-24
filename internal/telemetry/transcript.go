package telemetry

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// ParseTranscript reads a Claude Code / Codex / Cursor session JSONL transcript
// and returns summed token usage and the model from the most recent assistant turn.
// Returns zero counts (not an error) when the file does not exist.
// The transcript format is not a documented stable interface — all parsing is best-effort.
func ParseTranscript(path string) (TranscriptUsage, error) {
	if path == "" {
		return TranscriptUsage{}, nil
	}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return TranscriptUsage{}, nil
	}
	if err != nil {
		return TranscriptUsage{}, fmt.Errorf("open transcript %q: %w", path, err)
	}
	defer f.Close()

	type usage struct {
		InputTokens              int64 `json:"input_tokens"`
		OutputTokens             int64 `json:"output_tokens"`
		CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
		CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
	}
	type message struct {
		Usage usage  `json:"usage"`
		Model string `json:"model"`
	}
	type line struct {
		Type    string  `json:"type"`
		Message message `json:"message"`
	}

	var tu TranscriptUsage
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		var l line
		if err := json.Unmarshal(scanner.Bytes(), &l); err != nil {
			continue // skip unparseable lines
		}
		if l.Type != "assistant" {
			continue
		}
		tu.InputTokens += l.Message.Usage.InputTokens
		tu.OutputTokens += l.Message.Usage.OutputTokens
		tu.CachedTokens += l.Message.Usage.CacheCreationInputTokens + l.Message.Usage.CacheReadInputTokens
		if l.Message.Model != "" {
			tu.Model = l.Message.Model // keep updating — last non-empty wins
		}
	}
	return tu, nil
}

// ParseAntigravityTranscript reads an Antigravity session JSONL transcript and returns
// summed token usage, the model from the most recent non-empty model field, and any error.
// Returns zero counts (not an error) when the file does not exist.
func ParseAntigravityTranscript(path string) (TranscriptUsage, string, error) {
	if path == "" {
		return TranscriptUsage{}, "", nil
	}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return TranscriptUsage{}, "", nil
	}
	if err != nil {
		return TranscriptUsage{}, "", fmt.Errorf("open transcript %q: %w", path, err)
	}
	defer f.Close()

	type usageMetadata struct {
		PromptTokenCount          int64 `json:"promptTokenCount"`
		CandidatesTokenCount      int64 `json:"candidatesTokenCount"`
		CachedContentTokenCount   int64 `json:"cachedContentTokenCount"`
		TotalTokenCount           int64 `json:"totalTokenCount"`
	}
	type aline struct {
		Model         string        `json:"model"`
		UsageMetadata usageMetadata `json:"usageMetadata"`
	}

	var tu TranscriptUsage
	var lastModel string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		var l aline
		if err := json.Unmarshal(scanner.Bytes(), &l); err != nil {
			continue
		}
		tu.InputTokens += l.UsageMetadata.PromptTokenCount
		tu.OutputTokens += l.UsageMetadata.CandidatesTokenCount
		tu.CachedTokens += l.UsageMetadata.CachedContentTokenCount
		if l.Model != "" {
			lastModel = l.Model
		}
	}
	return tu, lastModel, nil
}
