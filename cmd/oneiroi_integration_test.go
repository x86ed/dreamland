package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// realGitRepo initializes an actual git repository (not the .git-directory-only fake
// used by oneiroiGitRepo/fakeGitRepo elsewhere in this package) via the real `git`
// binary, since task 6.5 requires asserting real commit-author behavior a stubbed
// runCmd can't exercise. Skips if git isn't on PATH.
func realGitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found on PATH")
	}

	root := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = root
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	runGit("init", "-q")
	runGit("config", "--local", "user.email", "hypnos@github.com")
	// Simulates `dreamland coauthor` having already set the invoking agent's identity
	// earlier in the same turn — oneiroi seed must not disturb this.
	runGit("config", "--local", "user.name", "hypnos")

	// An initial commit so HEAD exists before oneiroi seed's own commit.
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# scratch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "README.md")
	runGit("commit", "-q", "-m", "initial commit")

	orig, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	return root
}

func gitOutput(t *testing.T, root string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = root
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// TestOneiroiSeed_CommitAuthorIsScriptNotInvokingAgent covers the "Commit author is the
// script, not the invoking agent" scenario (dev-workflow-hooks capability delta) end to
// end against a real git repository and a real `git commit`: after `dreamland oneiroi
// seed` runs, the repo-local git identity set beforehand (simulating `hypnos` having
// already run `coauthor` this turn) is untouched, and the new commit's author is
// `dreamland-oneiroi-seed`, not `hypnos`.
func TestOneiroiSeed_CommitAuthorIsScriptNotInvokingAgent(t *testing.T) {
	root := realGitRepo(t)

	beforeUserName := gitOutput(t, root, "config", "--local", "user.name")
	if beforeUserName != "hypnos" {
		t.Fatalf("test setup broken: user.name = %q before oneiroi seed, want hypnos", beforeUserName)
	}

	if _, _, err := execCLI(t, "oneiroi", "seed", "--role", "example role"); err != nil {
		t.Fatalf("oneiroi seed: %v", err)
	}

	afterUserName := gitOutput(t, root, "config", "--local", "user.name")
	if afterUserName != "hypnos" {
		t.Errorf("git config --local user.name changed to %q; oneiroi seed must not touch the invoking agent's git identity", afterUserName)
	}

	authorName := gitOutput(t, root, "log", "-1", "--format=%an")
	if authorName != "dreamland-oneiroi-seed" {
		t.Errorf("git log -1 --format=%%an = %q, want dreamland-oneiroi-seed", authorName)
	}
}

// TestOneiroiSeed_CommitMessageBody covers "Commit message declares zero tokens and no
// coauthor": the real commit body carries `Tokens: input=0 output=0 cached=0 total=0`
// and `Generated-By: dreamland-oneiroi-seed`, and no `Co-authored-by:` line.
func TestOneiroiSeed_CommitMessageBody(t *testing.T) {
	root := realGitRepo(t)

	if _, _, err := execCLI(t, "oneiroi", "seed", "--role", "example role"); err != nil {
		t.Fatalf("oneiroi seed: %v", err)
	}

	body := gitOutput(t, root, "log", "-1", "--format=%B")
	if !strings.Contains(body, "Tokens: input=0 output=0 cached=0 total=0") {
		t.Errorf("commit body missing zero-tokens line, got:\n%s", body)
	}
	if !strings.Contains(body, "Generated-By: dreamland-oneiroi-seed") {
		t.Errorf("commit body missing Generated-By trailer, got:\n%s", body)
	}
	if strings.Contains(body, "Co-authored-by:") {
		t.Errorf("commit body must not contain a Co-authored-by line, got:\n%s", body)
	}
}

// TestOneiroiSeed_OnlyInvocationFilesStaged covers "Only the invocation's own files are
// staged": unrelated pending changes in the working tree at the time oneiroi seed runs
// must remain uncommitted (and unstaged) afterward — CommitScaffold (task 6.1) stages an
// explicit path list, never `git add -A`.
func TestOneiroiSeed_OnlyInvocationFilesStaged(t *testing.T) {
	root := realGitRepo(t)

	unrelated := filepath.Join(root, "unrelated-work-in-progress.txt")
	if err := os.WriteFile(unrelated, []byte("someone's pending edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, _, err := execCLI(t, "oneiroi", "seed", "--role", "example role"); err != nil {
		t.Fatalf("oneiroi seed: %v", err)
	}

	status := gitOutput(t, root, "status", "--porcelain", "--", "unrelated-work-in-progress.txt")
	if status == "" {
		t.Error("expected unrelated-work-in-progress.txt to remain uncommitted/untracked after oneiroi seed")
	}

	body := gitOutput(t, root, "log", "-1", "--format=%B")
	if strings.Contains(body, "unrelated-work-in-progress") {
		t.Error("unrelated file was swept into the oneiroi seed commit")
	}
}
