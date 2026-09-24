package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/pflag"

	"dreamland/internal/agentissue"
)

func runAgentIssueArgs(t *testing.T, stdin string, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	agentIssueCmd.SetOut(&buf)
	agentIssueCmd.SetIn(strings.NewReader(stdin))
	t.Cleanup(func() {
		agentIssueCmd.SetOut(nil)
		agentIssueCmd.SetIn(nil)
		agentIssueCmd.Flags().VisitAll(func(f *pflag.Flag) { _ = f.Value.Set(f.DefValue); f.Changed = false })
	})
	agentIssueCmd.SetArgs(args)
	agentIssueCmd.ParseFlags(args)
	err := runAgentIssue(agentIssueCmd, nil)
	return buf.String(), err
}

var validIssueArgs = []string{"--create", "--name", "sandman", "--role", "r", "--rationale", "why", "--tier", "full-edit", "--routing", "a->b", "--criteria", "ok"}

func ghFake(t *testing.T) *[][]string {
	var calls [][]string
	t.Cleanup(agentissue.SetRunGh(func(args ...string) (string, error) {
		calls = append(calls, args)
		if args[1] == "list" {
			return "", nil
		}
		return "https://example.test/issues/1\n", nil
	}))
	return &calls
}

func TestAgentIssueMissingFlagNoGh(t *testing.T) {
	calls := ghFake(t)
	_, err := runAgentIssueArgs(t, "y\n", "--create", "--role", "r")
	if err == nil || !strings.Contains(err.Error(), "name") {
		t.Fatalf("err = %v", err)
	}
	if len(*calls) != 0 {
		t.Error("gh called")
	}
}

func TestAgentIssueDeclined(t *testing.T) {
	calls := ghFake(t)
	out, err := runAgentIssueArgs(t, "n\n", validIssueArgs...)
	if err == nil {
		t.Fatal("declined should error (exit 1)")
	}
	if !strings.Contains(out, "Create this issue? [y/N]") || !strings.Contains(out, "New agent: sandman") {
		t.Errorf("preview/prompt missing: %s", out)
	}
	if len(*calls) != 0 {
		t.Error("gh called after decline")
	}
}

func TestAgentIssueConfirmedAndYes(t *testing.T) {
	calls := ghFake(t)
	out, err := runAgentIssueArgs(t, "y\n", validIssueArgs...)
	if err != nil || !strings.Contains(out, "https://example.test/issues/1") {
		t.Fatalf("%v %s", err, out)
	}
	if len(*calls) != 2 {
		t.Errorf("calls = %d", len(*calls))
	}
	*calls = nil
	if _, err := runAgentIssueArgs(t, "", append([]string{"--yes"}, validIssueArgs...)...); err != nil || len(*calls) != 2 {
		t.Errorf("--yes: %v calls=%d", err, len(*calls))
	}
}

func TestAgentIssueGhMissing(t *testing.T) {
	t.Cleanup(agentissue.SetRunGh(func(...string) (string, error) { return "", agentissue.ErrGhMissing }))
	_, err := runAgentIssueArgs(t, "", append([]string{"--yes"}, validIssueArgs...)...)
	if err == nil || !strings.Contains(err.Error(), "gh is required") {
		t.Fatalf("err = %v", err)
	}
}

func TestAgentIssueTemplateIdempotent(t *testing.T) {
	root := makeGitRepo(t)
	path := filepath.Join(root, ".github", "ISSUE_TEMPLATE", "new-agent.yml")
	if _, err := runAgentIssueArgs(t, "", "--template"); err != nil {
		t.Fatal(err)
	}
	a, _ := os.ReadFile(path)
	if _, err := runAgentIssueArgs(t, "", "--template"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	if len(a) == 0 || !bytes.Equal(a, b) {
		t.Error("not byte-identical")
	}
}

func TestAgentIssueRequiresMode(t *testing.T) {
	if _, err := runAgentIssueArgs(t, ""); err == nil {
		t.Error("want error")
	}
}
