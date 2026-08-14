package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dreamland/internal/config"
)

// TestRunTestAndCommit_FailingTestBlocksCommitInOneProcess is the regression
// test for the race dogfooding this repo's own hooks surfaced: `test` and
// `commit --reason turn-complete` as separate hook-bound commands can be
// dispatched by Claude Code's Stop hook runner before the first has finished
// writing .dreamland/last-test-result.json, so commit sees no record instead
// of the fail it should see. Running both steps as one process removes the
// race by construction — there's nothing else to race against.
func TestRunTestAndCommit_FailingTestBlocksCommitInOneProcess(t *testing.T) {
	root := makeTestRepo(t, config.Config{Language: "Go", TestCommand: "false"})

	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	origRunCmd := runCmd
	runCmd = func(_ string, args ...string) (string, error) {
		switch {
		case len(args) > 0 && args[0] == "status":
			return "M  main.go\n", nil
		case len(args) > 0 && args[0] == "rev-parse":
			return "deadbeef\n", nil
		default:
			return "", nil
		}
	}
	t.Cleanup(func() { runCmd = origRunCmd })

	orig := tacReason
	tacReason = "turn-complete"
	t.Cleanup(func() { tacReason = orig })

	err := runTestAndCommit(nil, nil)
	if err == nil {
		t.Fatal("expected commit to be refused when the test step fails")
	}
	if !IsBlocking(err) {
		t.Errorf("expected a blocking error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "commit refused") {
		t.Errorf("expected the commit-refused error, got: %v", err)
	}

	result, readErr := readLastTestResult(root)
	if readErr != nil {
		t.Fatalf("readLastTestResult: %v", readErr)
	}
	if result == nil || result.Status != "fail" {
		t.Fatalf("expected a recorded fail result, got: %+v", result)
	}
}

// TestRunTestAndCommit_PassingTestAllowsCommit confirms the happy path still
// commits when the test step succeeds, using the same single-process call.
func TestRunTestAndCommit_PassingTestAllowsCommit(t *testing.T) {
	root := makeTestRepo(t, config.Config{Language: "Go", TestCommand: "true"})

	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var commitCalled bool
	origRunCmd := runCmd
	runCmd = func(_ string, args ...string) (string, error) {
		switch {
		case len(args) > 0 && args[0] == "status":
			return "M  main.go\n", nil
		case len(args) > 0 && args[0] == "rev-parse":
			return "deadbeef\n", nil
		case len(args) > 0 && args[0] == "commit":
			commitCalled = true
			return "", nil
		default:
			return "", nil
		}
	}
	t.Cleanup(func() { runCmd = origRunCmd })

	orig := tacReason
	tacReason = "turn-complete"
	t.Cleanup(func() { tacReason = orig })

	if err := runTestAndCommit(nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !commitCalled {
		t.Error("expected git commit to be invoked when tests pass")
	}
}
