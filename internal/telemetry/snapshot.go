package telemetry

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const sessionFile = ".dreamland-session.json"

// SnapshotResult is the normalized session telemetry snapshot written after each AI turn.
type SnapshotResult struct {
	Tool           string `json:"tool"`
	Model          string `json:"model"`
	ThinkingEffort string `json:"thinking_effort,omitempty"`
	InputTokens    int64  `json:"input_tokens"`
	OutputTokens   int64  `json:"output_tokens"`
	CachedTokens   int64  `json:"cached_tokens,omitempty"`
	TotalTokens    int64  `json:"total_tokens"`
	CapturedAt     string `json:"captured_at"`
}

// TranscriptUsage holds token counts parsed from a session transcript JSONL.
// Used by ParseTranscript (Claude Code/Codex/Cursor) and ParseAntigravityTranscript.
type TranscriptUsage struct {
	InputTokens  int64
	OutputTokens int64
	CachedTokens int64
	Model        string
}

// ComputeTotals sets TotalTokens = InputTokens + OutputTokens when TotalTokens is zero.
func (s *SnapshotResult) ComputeTotals() {
	if s.TotalTokens == 0 {
		s.TotalTokens = s.InputTokens + s.OutputTokens
	}
}

// Read reads .dreamland-session.json from repoRoot. Returns nil, nil when absent.
func Read(repoRoot string) (*SnapshotResult, error) {
	data, err := os.ReadFile(filepath.Join(repoRoot, sessionFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read session file: %w", err)
	}
	var s SnapshotResult
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse session file: %w", err)
	}
	return &s, nil
}

// Write accumulates token counts from s into the existing session file at repoRoot,
// then writes the merged result atomically via temp-file rename.
func Write(repoRoot string, s *SnapshotResult) error {
	existing, err := Read(repoRoot)
	if err != nil {
		return err
	}
	if existing != nil {
		s.InputTokens += existing.InputTokens
		s.OutputTokens += existing.OutputTokens
		s.CachedTokens += existing.CachedTokens
	}
	s.CapturedAt = time.Now().UTC().Format(time.RFC3339)
	s.ComputeTotals()

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	target := filepath.Join(repoRoot, sessionFile)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), ".dreamland-session-tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, target)
}
