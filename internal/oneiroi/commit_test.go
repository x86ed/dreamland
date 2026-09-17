package oneiroi

import (
	"errors"
	"strings"
	"testing"
)

func TestIsNothingToCommit(t *testing.T) {
	if !isNothingToCommit("On branch main\nnothing to commit, working tree clean") {
		t.Error("expected true for git's nothing-to-commit output")
	}
	if isNothingToCommit("1 file changed") {
		t.Error("expected false for normal commit output")
	}
}

func TestCommitScaffold_NoPaths(t *testing.T) {
	if err := CommitScaffold(t.TempDir(), nil, "subject"); err == nil {
		t.Error("expected error when no paths are given")
	}
}

func TestCommitScaffold_AddFails(t *testing.T) {
	orig := GitExec
	defer func() { GitExec = orig }()
	GitExec = func(args ...string) (string, error) {
		if len(args) > 2 && args[2] == "add" {
			return "boom", errors.New("add failed")
		}
		return "", nil
	}

	err := CommitScaffold(t.TempDir(), []string{"foo.txt"}, "subject")
	if err == nil || !strings.Contains(err.Error(), "git add") {
		t.Fatalf("expected git add error, got %v", err)
	}
}

func TestCommitScaffold_NothingToCommitIsNotAnError(t *testing.T) {
	orig := GitExec
	defer func() { GitExec = orig }()
	GitExec = func(args ...string) (string, error) {
		if len(args) > 2 && args[2] == "add" {
			return "", nil
		}
		return "nothing to commit, working tree clean", errors.New("exit status 1")
	}

	if err := CommitScaffold(t.TempDir(), []string{"foo.txt"}, "subject"); err != nil {
		t.Fatalf("expected nothing-to-commit to be treated as success, got %v", err)
	}
}

func TestCommitScaffold_CommitFails(t *testing.T) {
	orig := GitExec
	defer func() { GitExec = orig }()
	GitExec = func(args ...string) (string, error) {
		if len(args) > 2 && args[2] == "add" {
			return "", nil
		}
		return "some other failure", errors.New("exit status 1")
	}

	err := CommitScaffold(t.TempDir(), []string{"foo.txt"}, "subject")
	if err == nil || !strings.Contains(err.Error(), "git commit") {
		t.Fatalf("expected git commit error, got %v", err)
	}
}

func TestCommitScaffold_Success(t *testing.T) {
	orig := GitExec
	defer func() { GitExec = orig }()
	var gotArgs [][]string
	GitExec = func(args ...string) (string, error) {
		gotArgs = append(gotArgs, args)
		return "", nil
	}

	if err := CommitScaffold(t.TempDir(), []string{"foo.txt", "bar.txt"}, "subject"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gotArgs) != 2 {
		t.Fatalf("expected add + commit calls, got %d calls", len(gotArgs))
	}
}
