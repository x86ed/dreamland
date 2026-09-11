package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dreamland/internal/config"
)

func TestRunCommit_InvalidReason(t *testing.T) {
	orig := commitReason
	commitReason = "bogus"
	t.Cleanup(func() { commitReason = orig })

	if err := runCommit(nil, nil); err == nil {
		t.Fatal("expected error for invalid --reason")
	}
}

func TestRunCommit_GetwdError(t *testing.T) {
	orig := commitReason
	commitReason = "turn-complete"
	t.Cleanup(func() { commitReason = orig })

	origGetwd := osGetwd
	osGetwd = func() (string, error) { return "", errors.New("getwd failed") }
	t.Cleanup(func() { osGetwd = origGetwd })

	if err := runCommit(nil, nil); err == nil {
		t.Fatal("expected error when osGetwd fails")
	}
}

func TestRunCommit_NoOpOutsideGitRepo(t *testing.T) {
	orig := commitReason
	commitReason = "turn-complete"
	t.Cleanup(func() { commitReason = orig })

	root := t.TempDir() // not a git repo, so FindRepoRoot fails
	origGetwd := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = origGetwd })

	// Should skip (return nil) outside git repo, not error
	if err := runCommit(nil, nil); err != nil {
		t.Fatalf("expected nil (skip) when outside git repo, got error: %v", err)
	}
}

func TestRunCommit_GitStatusError(t *testing.T) {
	makeVersionBumpRepo(t, config.Config{CodingTool: "Claude Code"})

	stubRunCmd(t, func(_ string, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "status" {
			return "", errors.New("git status failed")
		}
		return "", nil
	})

	orig := commitReason
	commitReason = "turn-complete"
	t.Cleanup(func() { commitReason = orig })

	if err := runCommit(nil, nil); err == nil {
		t.Fatal("expected error when git status fails")
	}
}

func TestRunCommit_GitAddError(t *testing.T) {
	makeVersionBumpRepo(t, config.Config{CodingTool: "Claude Code"})

	stubRunCmd(t, func(_ string, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "status" {
			return " M some/file.go\n", nil
		}
		if len(args) > 0 && args[0] == "add" {
			return "", errors.New("git add failed")
		}
		return "", nil
	})

	orig := commitReason
	commitReason = "turn-complete"
	t.Cleanup(func() { commitReason = orig })

	if err := runCommit(nil, nil); err == nil {
		t.Fatal("expected error when git add fails")
	}
}

func TestRunCommit_HandoffGitAddErrorIsNonBlocking(t *testing.T) {
	makeVersionBumpRepo(t, config.Config{CodingTool: "Claude Code"})

	stubRunCmd(t, func(_ string, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "status" {
			return " M some/file.go\n", nil
		}
		if len(args) > 0 && args[0] == "add" {
			return "", errors.New("git add failed")
		}
		return "", nil
	})

	orig := commitReason
	commitReason = "handoff"
	t.Cleanup(func() { commitReason = orig })

	err := runCommit(nil, nil)
	if err == nil {
		t.Fatal("expected error when git add fails")
	}
	if IsBlocking(err) {
		t.Fatalf("handoff git add failure must be non-blocking, got: %v", err)
	}
}

func TestRunCommit_GitCommitError(t *testing.T) {
	makeVersionBumpRepo(t, config.Config{CodingTool: "Claude Code"})

	stubRunCmd(t, func(_ string, args ...string) (string, error) {
		switch {
		case len(args) > 0 && args[0] == "status":
			return " M some/file.go\n", nil
		case len(args) > 0 && args[0] == "commit":
			return "commit output", errors.New("git commit failed")
		default:
			return "", nil
		}
	})

	orig := commitReason
	commitReason = "handoff"
	t.Cleanup(func() { commitReason = orig })

	err := runCommit(nil, nil)
	if err == nil {
		t.Fatal("expected error when git commit fails")
	}
	if IsBlocking(err) {
		t.Fatalf("handoff git commit failure must be non-blocking, got: %v", err)
	}
	if !strings.Contains(err.Error(), "commit output") {
		t.Errorf("expected error to include commit output, got: %v", err)
	}
}

func TestRunCommit_TurnCompleteGitCommitErrorIsBlocking(t *testing.T) {
	makeVersionBumpRepo(t, config.Config{CodingTool: "Claude Code"})

	stubRunCmd(t, func(_ string, args ...string) (string, error) {
		switch {
		case len(args) > 0 && args[0] == "status":
			return " M some/file.go\n", nil
		case len(args) > 0 && args[0] == "commit":
			return "commit output", errors.New("git commit failed")
		default:
			return "", nil
		}
	})

	orig := commitReason
	commitReason = "turn-complete"
	t.Cleanup(func() { commitReason = orig })

	err := runCommit(nil, nil)
	if err == nil {
		t.Fatal("expected error when git commit fails")
	}
	if !IsBlocking(err) {
		t.Fatalf("turn-complete git commit failure must be blocking, got: %v", err)
	}
}

// TestRunCommit_GitCommitNothingToCommit_IsBenignNoOp is the regression test for the
// live "Error: git commit: exit status 1 ... nothing to commit, working tree clean"
// Stop hook feedback: a concurrent agent session can land its own commit covering the
// exact same staged changes between our git status check and our own git commit call.
// That must be treated as a benign no-op (the work is committed, just not by us),
// not a blocking failure.
func TestRunCommit_GitCommitNothingToCommit_IsBenignNoOp(t *testing.T) {
	makeVersionBumpRepo(t, config.Config{CodingTool: "Claude Code"})

	stubRunCmd(t, func(_ string, args ...string) (string, error) {
		switch {
		case len(args) > 0 && args[0] == "status":
			return " M some/file.go\n", nil
		case len(args) > 0 && args[0] == "commit":
			return "On branch main\nnothing to commit, working tree clean", errors.New("exit status 1")
		default:
			return "", nil
		}
	})

	orig := commitReason
	commitReason = "handoff"
	t.Cleanup(func() { commitReason = orig })

	if err := runCommit(nil, nil); err != nil {
		t.Fatalf("expected nil (benign no-op) when a concurrent writer already committed, got: %v", err)
	}
}

func TestRunCommit_NoOpOnCleanTree(t *testing.T) {
	makeVersionBumpRepo(t, config.Config{CodingTool: "Claude Code"})

	var calls []string
	stubRunCmd(t, func(_ string, args ...string) (string, error) {
		calls = append(calls, strings.Join(args, " "))
		if len(args) > 0 && args[0] == "status" {
			return "", nil // clean tree
		}
		return "", nil
	})

	orig := commitReason
	commitReason = "turn-complete"
	t.Cleanup(func() { commitReason = orig })

	if err := runCommit(nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, c := range calls {
		if strings.HasPrefix(c, "commit") {
			t.Errorf("expected no commit call on clean tree, got calls: %v", calls)
		}
	}
}

func TestRunCommit_CommitsOnDirtyTree(t *testing.T) {
	makeVersionBumpRepo(t, config.Config{CodingTool: "GitHub Copilot"})

	var calls []string
	stubRunCmd(t, func(_ string, args ...string) (string, error) {
		calls = append(calls, strings.Join(args, " "))
		if len(args) > 0 && args[0] == "status" {
			return " M some/file.go\n", nil
		}
		return "", nil
	})

	orig := commitReason
	commitReason = "handoff"
	t.Cleanup(func() { commitReason = orig })

	if err := runCommit(nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var addCalled, commitCalled bool
	var commitMsg string
	for _, c := range calls {
		if strings.HasPrefix(c, "add -A") {
			addCalled = true
		}
		if strings.HasPrefix(c, "commit -m") {
			commitCalled = true
			commitMsg = c
		}
	}
	if !addCalled {
		t.Error("expected 'git add -A' call")
	}
	if !commitCalled {
		t.Error("expected 'git commit' call")
	}
	if !strings.Contains(commitMsg, "chore: handoff checkpoint (GitHub Copilot)") {
		t.Errorf("unexpected commit message: %q", commitMsg)
	}
}

// TestRunCommit_AgentNameFlagOverridesGitIdentity verifies --agent-name takes
// precedence over currentGitIdentityName's git-config-read fallback. Unlike coauthor,
// runCommit never reads stdin/hook payloads directly — it trusts the git identity
// coauthor already set — so there's no stdin-blocking scenario to guard against here.
func TestRunCommit_AgentNameFlagOverridesGitIdentity(t *testing.T) {
	makeVersionBumpRepo(t, config.Config{CodingTool: "GitHub Copilot"})

	var commitMsg string
	stubRunCmd(t, func(_ string, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "status" {
			return " M some/file.go\n", nil
		}
		if len(args) >= 4 && args[0] == "config" && args[3] == "user.name" {
			return "morpheus\n", nil // configured git identity — must be overridden below
		}
		if len(args) > 0 && args[0] == "commit" {
			commitMsg = strings.Join(args, " ")
		}
		return "", nil
	})

	origReason, origAgentName := commitReason, commitAgentName
	commitReason = "handoff"
	commitAgentName = "phantasos"
	t.Cleanup(func() { commitReason = origReason; commitAgentName = origAgentName })

	if err := runCommit(nil, nil); err != nil {
		t.Fatalf("runCommit: %v", err)
	}

	want := "chore: handoff checkpoint (phantasos)"
	if !strings.Contains(commitMsg, want) {
		t.Errorf("commit message = %q, want to contain %q", commitMsg, want)
	}
}

func TestRunCommit_NilConfig(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	// No .dreamland.json — cfg will be nil, so no test-result gating applies.
	orig := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = orig })

	stubRunCmd(t, func(_ string, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "status" {
			return "", nil // clean tree
		}
		return "", nil
	})

	orig2 := commitReason
	commitReason = "turn-complete"
	t.Cleanup(func() { commitReason = orig2 })

	if err := runCommit(nil, nil); err != nil {
		t.Fatalf("unexpected error with nil config: %v", err)
	}
}

func TestRunCommit_ConfigLoadError(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Malformed JSON so config.Load fails with a non-ErrNoGitRepo error.
	if err := os.WriteFile(filepath.Join(root, ".dreamland.json"), []byte("not-json"), 0o644); err != nil {
		t.Fatal(err)
	}
	orig := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = orig })

	origReason := commitReason
	commitReason = "turn-complete"
	t.Cleanup(func() { commitReason = origReason })

	if err := runCommit(nil, nil); err == nil {
		t.Fatal("expected error from config.Load with invalid JSON")
	}
}

func TestRunCommit_ReadLastTestResultError(t *testing.T) {
	root := makeVersionBumpRepo(t, config.Config{CodingTool: "Claude Code", TestCommand: "go test ./..."})

	// Corrupt the last-test-result.json so readLastTestResult fails to unmarshal.
	dreamlandDir := filepath.Join(root, ".dreamland")
	if err := os.MkdirAll(dreamlandDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dreamlandDir, "last-test-result.json"), []byte("not-json"), 0o644); err != nil {
		t.Fatal(err)
	}

	stubRunCmd(t, func(_ string, _ ...string) (string, error) { return "", nil })

	orig := commitReason
	commitReason = "turn-complete"
	t.Cleanup(func() { commitReason = orig })

	if err := runCommit(nil, nil); err == nil {
		t.Fatal("expected error when readLastTestResult fails")
	}
}

func TestRunCommit_TestResultMissing_Blocking(t *testing.T) {
	// TestCommand configured but no last-test-result.json recorded at all.
	makeVersionBumpRepo(t, config.Config{CodingTool: "Claude Code", TestCommand: "go test ./..."})

	stubRunCmd(t, func(_ string, _ ...string) (string, error) { return "", nil })

	orig := commitReason
	commitReason = "turn-complete"
	t.Cleanup(func() { commitReason = orig })

	err := runCommit(nil, nil)
	if err == nil {
		t.Fatal("expected blocking error when no test result is recorded")
	}
	if !strings.Contains(err.Error(), "no result recorded") {
		t.Errorf("expected 'no result recorded' in error, got: %v", err)
	}
}

func TestRunCommit_TestFailed_HeadMismatch_Blocking(t *testing.T) {
	root := makeVersionBumpRepo(t, config.Config{CodingTool: "Claude Code", TestCommand: "go test ./..."})

	if err := writeLastTestResult(root, "fail"); err != nil {
		t.Fatal(err)
	}

	stubRunCmd(t, func(_ string, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "rev-parse" {
			return "", errors.New("git rev-parse HEAD failed")
		}
		return "", nil
	})

	orig := commitReason
	commitReason = "turn-complete"
	t.Cleanup(func() { commitReason = orig })

	err := runCommit(nil, nil)
	if err == nil {
		t.Fatal("expected error when git rev-parse HEAD fails")
	}
}

func TestCurrentGitIdentityName_UsesConfiguredGitIdentity(t *testing.T) {
	stubRunCmd(t, func(_ string, args ...string) (string, error) {
		if len(args) >= 4 && args[0] == "config" && args[3] == "user.name" {
			return "morpheus\n", nil
		}
		return "", nil
	})

	cfg := &config.Config{CodingTool: "GitHub Copilot"}
	got := currentGitIdentityName(cfg, "")
	if got != "morpheus" {
		t.Errorf("got %q, want morpheus (from git config)", got)
	}
}
