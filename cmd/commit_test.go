package cmd

import (
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
