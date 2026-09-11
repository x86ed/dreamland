package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestCheckArtifactOwnership_OwnerAllowed(t *testing.T) {
	payload := hookToolInputPayload{AgentType: "phantasos"}
	payload.ToolInput.FilePath = "/repo/openspec/changes/x/proposal.md"

	if got := checkArtifactOwnership(payload, "/repo"); got != "" {
		t.Errorf("expected allowed (empty reason), got %q", got)
	}
}

func TestCheckArtifactOwnership_NonOwnerBlocked(t *testing.T) {
	payload := hookToolInputPayload{AgentType: "morpheus"}
	payload.ToolInput.FilePath = "/repo/openspec/changes/x/proposal.md"

	got := checkArtifactOwnership(payload, "/repo")
	if got == "" {
		t.Fatal("expected a block reason, got none")
	}
	if !strings.Contains(got, "phantasos") || !strings.Contains(got, "morpheus") {
		t.Errorf("block reason should name both the owner and the actual agent, got: %q", got)
	}
}

func TestCheckArtifactOwnership_MorpheusMayFlipTasksCheckbox(t *testing.T) {
	payload := hookToolInputPayload{AgentType: "morpheus"}
	payload.ToolInput.FilePath = "/repo/openspec/changes/x/tasks.md"
	payload.ToolInput.OldString = "- [ ] implement the task"
	payload.ToolInput.NewString = "- [x] implement the task"

	if got := checkArtifactOwnership(payload, "/repo"); got != "" {
		t.Errorf("expected checkbox-only task edit to be allowed, got %q", got)
	}
}

func TestCheckArtifactOwnership_MorpheusTaskProseEditBlocked(t *testing.T) {
	payload := hookToolInputPayload{AgentType: "morpheus"}
	payload.ToolInput.FilePath = "/repo/openspec/changes/x/tasks.md"
	payload.ToolInput.OldString = "- [ ] implement the task"
	payload.ToolInput.NewString = "- [ ] rewrite the task"

	if got := checkArtifactOwnership(payload, "/repo"); got == "" {
		t.Error("expected task prose edit to be blocked")
	}
}

func TestCheckArtifactOwnership_MorpheusWriteTasksBlocked(t *testing.T) {
	payload := hookToolInputPayload{AgentType: "morpheus"}
	payload.ToolInput.FilePath = "/repo/openspec/changes/x/tasks.md"

	if got := checkArtifactOwnership(payload, "/repo"); got == "" {
		t.Error("expected full tasks.md write to be blocked")
	}
}

func TestIsCheckboxOnlyEdit(t *testing.T) {
	tests := []struct {
		name string
		old  string
		new  string
		want bool
	}{
		{name: "check", old: "  - [ ] task  ", new: "- [x] task", want: true},
		{name: "uncheck", old: "- [x] task", new: "- [ ] task", want: true},
		{name: "prose", old: "- [ ] task", new: "- [ ] changed", want: false},
		{name: "multiline", old: "- [ ] task\n- [ ] next", new: "- [x] task\n- [ ] next", want: false},
		{name: "write", old: "", new: "- [x] task", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isCheckboxOnlyEdit(tt.old, tt.new); got != tt.want {
				t.Errorf("isCheckboxOnlyEdit(%q, %q) = %v, want %v", tt.old, tt.new, got, tt.want)
			}
		})
	}
}

func TestCheckArtifactOwnership_HypnosOwnsAgentTemplates(t *testing.T) {
	payload := hookToolInputPayload{AgentType: "hypnos"}
	payload.ToolInput.FilePath = "/repo/internal/scaffold/templates/agents/claude-code/foo.md"

	if got := checkArtifactOwnership(payload, "/repo"); got != "" {
		t.Errorf("expected allowed (empty reason), got %q", got)
	}
}

func TestCheckArtifactOwnership_NonHypnosBlockedFromAgentTemplates(t *testing.T) {
	payload := hookToolInputPayload{AgentType: "iktomi"}
	payload.ToolInput.FilePath = "/repo/internal/scaffold/templates/agents/claude-code/foo.md"

	if got := checkArtifactOwnership(payload, "/repo"); got == "" {
		t.Error("expected a block reason, got none")
	}
}

func TestCheckArtifactOwnership_ZhougongOwnsReports(t *testing.T) {
	payload := hookToolInputPayload{AgentType: "zhougong"}
	payload.ToolInput.FilePath = "/repo/.dreamland/reports/2026-01-01-agent-report.md"

	if got := checkArtifactOwnership(payload, "/repo"); got != "" {
		t.Errorf("expected allowed (empty reason), got %q", got)
	}
}

func TestCheckArtifactOwnership_UnprotectedPathAlwaysAllowed(t *testing.T) {
	for _, agent := range []string{"nyx", "morpheus", "iktomi", "phantasos", "hypnos", "zhougong", "general-purpose"} {
		payload := hookToolInputPayload{AgentType: agent}
		payload.ToolInput.FilePath = "/repo/internal/telemetry/snapshot.go"

		if got := checkArtifactOwnership(payload, "/repo"); got != "" {
			t.Errorf("agent %s: expected unprotected path always allowed, got %q", agent, got)
		}
	}
}

func TestCheckArtifactOwnership_NoAgentTypeFailsOpen(t *testing.T) {
	payload := hookToolInputPayload{} // no AgentType at all
	payload.ToolInput.FilePath = "/repo/openspec/changes/x/proposal.md"

	if got := checkArtifactOwnership(payload, "/repo"); got != "" {
		t.Errorf("expected fail-open (empty reason) when agent_type is absent, got %q", got)
	}
}

func TestCheckArtifactOwnership_ArchivedSpecOwnedByPhantasos(t *testing.T) {
	payload := hookToolInputPayload{AgentType: "morpheus"}
	payload.ToolInput.FilePath = "/repo/openspec/specs/agent-scaffolding/spec.md"

	if got := checkArtifactOwnership(payload, "/repo"); got == "" {
		t.Error("expected a block reason for non-phantasos write to an archived spec, got none")
	}
}

func TestRunGuardArtifact_BlocksAndExits2(t *testing.T) {
	c := &cobra.Command{}
	var stderr bytes.Buffer
	c.SetErr(&stderr)
	c.SetIn(strings.NewReader(`{"agent_type":"morpheus","tool_input":{"file_path":"openspec/changes/x/proposal.md"}}`))

	origExit := guardArtifactExit
	var exitCode int
	exitCalled := false
	guardArtifactExit = func(code int) { exitCalled = true; exitCode = code }
	t.Cleanup(func() { guardArtifactExit = origExit })

	origGetwd := osGetwd
	osGetwd = func() (string, error) { return "", nil } // FindRepoRoot will fail on "", repoRoot stays ""
	t.Cleanup(func() { osGetwd = origGetwd })

	if err := runGuardArtifact(c, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exitCalled || exitCode != 2 {
		t.Errorf("expected guardArtifactExit(2) to be called, exitCalled=%v exitCode=%d", exitCalled, exitCode)
	}
	if stderr.Len() == 0 {
		t.Error("expected a non-empty stderr reason")
	}
}

func TestRunGuardArtifact_AllowsAndDoesNotExit(t *testing.T) {
	c := &cobra.Command{}
	var stderr bytes.Buffer
	c.SetErr(&stderr)
	c.SetIn(strings.NewReader(`{"agent_type":"phantasos","tool_input":{"file_path":"openspec/changes/x/proposal.md"}}`))

	origExit := guardArtifactExit
	exitCalled := false
	guardArtifactExit = func(int) { exitCalled = true }
	t.Cleanup(func() { guardArtifactExit = origExit })

	if err := runGuardArtifact(c, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exitCalled {
		t.Error("expected no exit call for an allowed write")
	}
	if stderr.Len() != 0 {
		t.Errorf("expected empty stderr for an allowed write, got: %q", stderr.String())
	}
}

func TestRunGuardArtifact_NoPayloadAllowsSilently(t *testing.T) {
	c := &cobra.Command{}
	c.SetIn(strings.NewReader(""))

	origExit := guardArtifactExit
	exitCalled := false
	guardArtifactExit = func(int) { exitCalled = true }
	t.Cleanup(func() { guardArtifactExit = origExit })

	if err := runGuardArtifact(c, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exitCalled {
		t.Error("expected no exit call when no payload arrives at all")
	}
}
