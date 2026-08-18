package oneiroi

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitExec runs `git` with the given arguments and returns its combined output. It is a
// package-level var so callers (cmd/oneiroi.go) can wire it to their own testable git
// seam (e.g. the cmd package's runCmd, already stubbed by existing tests) instead of
// shelling out for real.
var GitExec = func(args ...string) (string, error) {
	c := exec.Command("git", args...)
	out, err := c.CombinedOutput()
	return string(out), err
}

// commitAuthor is the self-authored git identity every `dreamland oneiroi
// seed`/`revise`/`fork` commit uses — see the "Scaffold-generated commits are
// self-authored, zero-token, and coauthor-free" requirement.
const commitAuthor = "dreamland-oneiroi-seed <oneiroi-seed@github.com>"

// CommitScaffold stages exactly paths (never `git add -A`) and creates one commit,
// authored as the script itself via `git commit --author`, without touching the
// repository-local `git config user.name`/`user.email` the invoking session's coauthor
// may have already set. The commit message is subject followed by a blank line, a
// zero-tokens report line, and the Generated-By trailer that `dreamland coauthor
// --trailer` recognizes and skips further processing of.
func CommitScaffold(repoRoot string, paths []string, subject string) error {
	if len(paths) == 0 {
		return fmt.Errorf("oneiroi: CommitScaffold called with no paths to stage")
	}

	addArgs := append([]string{"-C", repoRoot, "add", "--"}, paths...)
	if out, err := GitExec(addArgs...); err != nil {
		return fmt.Errorf("git add: %w\n%s", err, out)
	}

	message := fmt.Sprintf("%s\n\nTokens: input=0 output=0 cached=0 total=0\nGenerated-By: dreamland-oneiroi-seed\n", subject)

	tmp, err := os.CreateTemp("", "dreamland-oneiroi-commit-*.txt")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.WriteString(message); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	out, err := GitExec("-C", repoRoot, "commit", "--author", commitAuthor, "-F", tmpName)
	if err != nil {
		if isNothingToCommit(out) {
			return nil
		}
		return fmt.Errorf("git commit: %w\n%s", err, out)
	}
	return nil
}

// isNothingToCommit mirrors cmd/commit.go's benign-no-op treatment of a concurrent
// writer having already landed the same staged changes.
func isNothingToCommit(gitCommitOutput string) bool {
	return strings.Contains(gitCommitOutput, "nothing to commit")
}
