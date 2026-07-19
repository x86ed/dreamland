package cmd

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

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
	if !strings.Contains(commitMsg, "chore: handoff checkpoint (janus)") {
		t.Errorf("unexpected commit message: %q", commitMsg)
	}
}

func TestRunCommit_AgentNameFlagSkipsStdinRead(t *testing.T) {
	agentNames := []string{"janus", "phantasos", "nyx", "morpheus", "phobetor", "baku", "iktomi", "zhougong", "hypnos", "mengpo"}

	for _, name := range agentNames {
		t.Run(name, func(t *testing.T) {
			makeVersionBumpRepo(t, config.Config{CodingTool: "Claude Code"})

			// A pipe with nothing written and never closed: if the code attempted the
			// stdin read despite --agent-name being set, this call would block for the
			// full hookPayloadReadTimeout. It must not.
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { w.Close(); r.Close() })
			origStdin := os.Stdin
			os.Stdin = r
			t.Cleanup(func() { os.Stdin = origStdin })

			var commitMsg string
			stubRunCmd(t, func(_ string, args ...string) (string, error) {
				if len(args) > 0 && args[0] == "status" {
					return " M some/file.go\n", nil
				}
				if len(args) > 0 && args[0] == "commit" {
					commitMsg = strings.Join(args, " ")
				}
				return "", nil
			})

			origReason, origAgentName := commitReason, commitAgentName
			commitReason = "handoff"
			commitAgentName = name
			t.Cleanup(func() { commitReason = origReason; commitAgentName = origAgentName })

			start := time.Now()
			if err := runCommit(nil, nil); err != nil {
				t.Fatalf("runCommit: %v", err)
			}
			if elapsed := time.Since(start); elapsed >= hookPayloadReadTimeout {
				t.Errorf("runCommit took %s — stdin read was not skipped despite --agent-name=%s", elapsed, name)
			}

			want := "chore: handoff checkpoint (" + name + ")"
			if !strings.Contains(commitMsg, want) {
				t.Errorf("commit message = %q, want to contain %q", commitMsg, want)
			}
		})
	}
}

func TestRunCommit_AgentNameFlagAbsent_UsesHookPayloadAgentType(t *testing.T) {
	agentNames := []string{"janus", "phantasos", "nyx", "morpheus", "phobetor", "baku", "iktomi", "zhougong", "hypnos", "mengpo"}

	for _, name := range agentNames {
		t.Run(name, func(t *testing.T) {
			makeVersionBumpRepo(t, config.Config{CodingTool: "Claude Code"})
			withPipedStdin(t, `{"hook_event_name":"SubagentStop","agent_type":"`+name+`"}`)

			var commitMsg string
			stubRunCmd(t, func(_ string, args ...string) (string, error) {
				if len(args) > 0 && args[0] == "status" {
					return " M some/file.go\n", nil
				}
				if len(args) > 0 && args[0] == "commit" {
					commitMsg = strings.Join(args, " ")
				}
				return "", nil
			})

			origReason, origAgentName := commitReason, commitAgentName
			commitReason = "handoff"
			commitAgentName = ""
			t.Cleanup(func() { commitReason = origReason; commitAgentName = origAgentName })

			if err := runCommit(nil, nil); err != nil {
				t.Fatalf("runCommit: %v", err)
			}

			want := "chore: handoff checkpoint (" + name + ")"
			if !strings.Contains(commitMsg, want) {
				t.Errorf("commit message = %q, want to contain %q, not the coding-tool-name fallback", commitMsg, want)
			}
		})
	}
}

func TestRunCommit_BuiltinAgentTypeFromHookPayload(t *testing.T) {
	makeVersionBumpRepo(t, config.Config{CodingTool: "Claude Code"})
	withPipedStdin(t, `{"hook_event_name":"SubagentStop","agent_type":"general-purpose"}`)

	var commitMsg string
	stubRunCmd(t, func(_ string, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "status" {
			return " M some/file.go\n", nil
		}
		if len(args) > 0 && args[0] == "commit" {
			commitMsg = strings.Join(args, " ")
		}
		return "", nil
	})

	origReason, origAgentName := commitReason, commitAgentName
	commitReason = "handoff"
	commitAgentName = ""
	t.Cleanup(func() { commitReason = origReason; commitAgentName = origAgentName })

	if err := runCommit(nil, nil); err != nil {
		t.Fatalf("runCommit: %v", err)
	}

	want := "chore: handoff checkpoint (general-purpose)"
	if !strings.Contains(commitMsg, want) {
		t.Errorf("commit message = %q, want to contain %q — resolution doesn't distinguish dreamland agents from built-ins", commitMsg, want)
	}
}

func TestRunCommit_NoPayloadNoFlag_FallsBackToJanus(t *testing.T) {
	makeVersionBumpRepo(t, config.Config{CodingTool: "Claude Code"})

	// Pipe with nothing written and never closed: simulates no hook payload arriving
	// within the timeout. Combined with no --agent-name, must fall back through
	// resolveAgentName to "janus" (session-start-equivalent identity), not the raw
	// coding-tool name.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { w.Close(); r.Close() })
	origStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = origStdin })

	var commitMsg string
	stubRunCmd(t, func(_ string, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "status" {
			return " M some/file.go\n", nil
		}
		if len(args) > 0 && args[0] == "commit" {
			commitMsg = strings.Join(args, " ")
		}
		return "", nil
	})

	origReason, origAgentName := commitReason, commitAgentName
	commitReason = "handoff"
	commitAgentName = ""
	t.Cleanup(func() { commitReason = origReason; commitAgentName = origAgentName })

	if err := runCommit(nil, nil); err != nil {
		t.Fatalf("runCommit: %v", err)
	}

	want := "chore: handoff checkpoint (janus)"
	if !strings.Contains(commitMsg, want) {
		t.Errorf("commit message = %q, want to contain %q", commitMsg, want)
	}
}
