package cmd

import (
	"strings"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// testResult records the outcome of a test run.
type testResult struct {
	Status    string `json:"status"`    // "pass", "fail", or "skipped" (no tracked source changes since last commit, so no run was needed)
	HeadSHA   string `json:"head_sha"`  // git rev-parse HEAD at time of test
	WrittenAt string `json:"written_at"` // RFC3339 timestamp
}

// writeLastTestResult writes a test result file to .dreamland/last-test-result.json
// in the repository root, recording the given status and current HEAD SHA.
func writeLastTestResult(repoRoot, status string) error {
	// Get current HEAD SHA
	headSHA, err := runCmd("git", "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	headSHA = strings.TrimSpace(headSHA)

	result := testResult{
		Status:    status,
		HeadSHA:   headSHA,
		WrittenAt: time.Now().UTC().Format(time.RFC3339),
	}

	dreamlandDir := filepath.Join(repoRoot, ".dreamland")
	if err := os.MkdirAll(dreamlandDir, 0o755); err != nil {
		return err
	}

	resultFile := filepath.Join(dreamlandDir, "last-test-result.json")
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(resultFile, append(data, '\n'), 0o644)
}

// readLastTestResult reads the test result file from .dreamland/last-test-result.json
// Returns (nil, nil) if the file doesn't exist; otherwise returns the parsed result or an error.
func readLastTestResult(repoRoot string) (*testResult, error) {
	resultFile := filepath.Join(repoRoot, ".dreamland", "last-test-result.json")
	data, err := os.ReadFile(resultFile)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var result testResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
