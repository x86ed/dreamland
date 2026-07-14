package cmd

import (
	"errors"
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

func TestRunCommit_ConfigLoadError(t *testing.T) {
	orig := commitReason
	commitReason = "turn-complete"
	t.Cleanup(func() { commitReason = orig })

	root := t.TempDir() // not a git repo, so config.Load fails via FindRepoRoot
	origGetwd := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = origGetwd })

	if err := runCommit(nil, nil); err == nil {
		t.Fatal("expected error when config.Load fails")
	}
}

func TestRunCommit_GitStatusError(t *testing.T) {
	makeVersionBumpRepo(t, config.Config{CodingTool: "Claude Code"})

	stubRunCmd(t, func(_ string, args ...string) (string, error) {
		return "", errors.New("git status failed")
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
	if !strings.Contains(err.Error(), "commit output") {
		t.Errorf("expected error to include commit output, got: %v", err)
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
	makeVersionBumpRepo(t, config.Config{CodingTool: "Claude Code"})

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
	if !strings.Contains(commitMsg, "chore: handoff checkpoint (Claude Code)") {
		t.Errorf("unexpected commit message: %q", commitMsg)
	}
}
