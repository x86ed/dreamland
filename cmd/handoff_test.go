package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dreamland/internal/handoff"
)

func newHandoffEnv(t *testing.T, mode string) (*handoffEnv, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	repo := t.TempDir()
	var out, errOut bytes.Buffer
	return &handoffEnv{
		store:    handoff.NewStoreAt(t.TempDir(), repo),
		mode:     mode,
		repoRoot: repo,
		out:      &out,
		errOut:   &errOut,
		active:   func() ([]string, error) { return nil, errors.New("no openspec") },
	}, &out, &errOut
}

func sub(agent, report string) hookPayload {
	return hookPayload{SessionID: "s1", AgentID: "a-" + agent, AgentType: agent, LastAssistantMessage: report}
}

func disp() hookPayload { return hookPayload{SessionID: "s1"} }

func agentCall(target string) hookPayload {
	p := disp()
	p.ToolName = "Agent"
	p.ToolInput.SubagentType = target
	return p
}

func mustRecord(t *testing.T, e *handoffEnv, agent, report string) {
	t.Helper()
	if err := e.record(sub(agent, report)); err != nil {
		t.Fatal(err)
	}
}

func entries(t *testing.T, e *handoffEnv) []handoff.Entry {
	t.Helper()
	es, err := e.store.ReadPending("s1")
	if err != nil {
		t.Fatal(err)
	}
	return es
}

func TestRecordWritesPendingEntry(t *testing.T) {
	e, _, _ := newHandoffEnv(t, "block")
	mustRecord(t, e, "morpheus", "done\n[handoff: complete]")
	es := entries(t, e)
	if len(es) != 1 || es[0].Directive.Target != "phobetor" || !es[0].Blocking() {
		t.Fatalf("entries = %+v", es)
	}
}

func TestRecordMissingTagIsFlagged(t *testing.T) {
	e, _, _ := newHandoffEnv(t, "block")
	mustRecord(t, e, "morpheus", "no tag")
	if es := entries(t, e); len(es) != 1 || !es[0].TagMissing {
		t.Fatalf("entries = %+v", es)
	}
}

func TestRecordIdempotent(t *testing.T) {
	e, _, _ := newHandoffEnv(t, "block")
	report := "[verdict: fail]\n[change: c1]"
	mustRecord(t, e, "phobetor", report)
	mustRecord(t, e, "phobetor", report)
	if es := entries(t, e); len(es) != 1 {
		t.Fatalf("entries = %d, want 1", len(es))
	}
	if c, _ := e.store.ReadCounter("c1"); c.PhobetorFailures != 1 {
		t.Errorf("counter = %d, want 1 (record applied twice)", c.PhobetorFailures)
	}
}

func TestRecordIgnoresDispatcherAndUnknownAgents(t *testing.T) {
	e, _, _ := newHandoffEnv(t, "block")
	if err := e.record(disp()); err != nil {
		t.Fatal(err)
	}
	mustRecord(t, e, "hypnos", "[handoff: complete]")
	if es := entries(t, e); len(es) != 0 {
		t.Fatalf("entries = %+v", es)
	}
}

func TestRecordPhobetorCounterEscalation(t *testing.T) {
	e, _, _ := newHandoffEnv(t, "block")
	mustRecord(t, e, "phobetor", "one\n[verdict: fail]\n[change: c1]")
	mustRecord(t, e, "phobetor", "two\n[verdict: fail]\n[change: c1]")
	es := entries(t, e)
	if len(es) != 2 || es[0].Directive.Target != "morpheus" || es[1].Directive.Target != "phantasos" {
		t.Fatalf("entries = %+v", es)
	}
	mustRecord(t, e, "phobetor", "ok\n[verdict: pass]\n[change: c1]")
	if _, err := os.Stat(filepath.Join(e.store.Dir, "c1.json")); !os.IsNotExist(err) {
		t.Error("counter not deleted on pass")
	}
}

func TestRecordUnverifiedIsReportOnly(t *testing.T) {
	e, _, _ := newHandoffEnv(t, "block")
	mustRecord(t, e, "phobetor", "[verdict: unverified]\n[change: c1]")
	es := entries(t, e)
	if len(es) != 1 || es[0].Blocking() || es[0].Directive.Kind != handoff.KindReport {
		t.Fatalf("entries = %+v", es)
	}
	if err := e.stopCheck(disp()); err != nil {
		t.Errorf("report entry blocked stop: %v", err)
	}
}

func TestRecordUsesTranscriptFallback(t *testing.T) {
	e, _, _ := newHandoffEnv(t, "block")
	tp := filepath.Join(t.TempDir(), "t.jsonl")
	line, _ := json.Marshal(map[string]any{"type": "assistant", "message": map[string]any{
		"role": "assistant", "content": []map[string]string{{"type": "text", "text": "done\n[handoff: complete]"}}}})
	if err := os.WriteFile(tp, append(line, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	p := sub("morpheus", "")
	p.AgentTranscriptPath = tp
	if err := e.record(p); err != nil {
		t.Fatal(err)
	}
	if es := entries(t, e); len(es) != 1 || es[0].TagMissing {
		t.Fatalf("entries = %+v", es)
	}
}

func TestInjectText(t *testing.T) {
	e, out, _ := newHandoffEnv(t, "block")
	mustRecord(t, e, "morpheus", "[handoff: complete]")
	if err := e.inject(disp()); err != nil {
		t.Fatal(err)
	}
	var got struct {
		H struct {
			Ctx string `json:"additionalContext"`
			Ev  string `json:"hookEventName"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("%v: %s", err, out.String())
	}
	want := "REQUIRED NEXT STEP (dreamland handoff): call Agent with subagent_type=phobetor now."
	if !strings.HasPrefix(got.H.Ctx, want) || got.H.Ev != "PostToolUse" {
		t.Errorf("ctx = %q ev = %q", got.H.Ctx, got.H.Ev)
	}
}

func TestInjectNothingWhenNoEntryOrSubagent(t *testing.T) {
	e, out, _ := newHandoffEnv(t, "block")
	if err := e.inject(disp()); err != nil || out.Len() != 0 {
		t.Fatalf("err=%v out=%q", err, out.String())
	}
	mustRecord(t, e, "morpheus", "[handoff: complete]")
	sp := disp()
	sp.AgentID = "x"
	if err := e.inject(sp); err != nil || out.Len() != 0 {
		t.Fatalf("subagent payload injected: %q", out.String())
	}
}

func TestEnforceBlocksWrongTargetAndClearsRight(t *testing.T) {
	e, _, errOut := newHandoffEnv(t, "block")
	mustRecord(t, e, "morpheus", "[handoff: complete]")
	err := e.enforce(agentCall("nyx"))
	if !errors.Is(err, errBlock) || !strings.Contains(errOut.String(), "phobetor") {
		t.Fatalf("err=%v stderr=%q", err, errOut.String())
	}
	if err := e.enforce(agentCall("phobetor")); err != nil {
		t.Fatal(err)
	}
	if es := entries(t, e); len(es) != 0 {
		t.Fatalf("entry not cleared: %+v", es)
	}
}

func TestEnforceIgnoresSubagentCalls(t *testing.T) {
	e, _, _ := newHandoffEnv(t, "block")
	mustRecord(t, e, "morpheus", "[handoff: complete]")
	p := agentCall("nyx")
	p.AgentID, p.AgentType = "a-morpheus", "morpheus"
	if err := e.enforce(p); err != nil {
		t.Fatalf("subagent call blocked: %v", err)
	}
	if es := entries(t, e); len(es) != 1 || es[0].Blocks != 0 {
		t.Fatalf("entries = %+v", es)
	}
}

func TestEnforceOldestFirstWithTwoEntries(t *testing.T) {
	e, _, _ := newHandoffEnv(t, "block")
	mustRecord(t, e, "morpheus", "first\n[handoff: complete]")
	mustRecord(t, e, "iktomi", "second\n[handoff: complete]")
	if err := e.enforce(agentCall("phobetor")); err != nil {
		t.Fatal(err)
	}
	es := entries(t, e)
	if len(es) != 1 || es[0].Agent != "iktomi" {
		t.Fatalf("entries = %+v", es)
	}
	if err := e.stopCheck(disp()); !errors.Is(err, errBlock) {
		t.Fatalf("stop should still block, got %v", err)
	}
}

func TestEnforceDropsReportEntryOnNextDispatch(t *testing.T) {
	e, _, _ := newHandoffEnv(t, "block")
	mustRecord(t, e, "morpheus", "[handoff: blocked]")
	if err := e.enforce(agentCall("phantasos")); err != nil {
		t.Fatal(err)
	}
	if es := entries(t, e); len(es) != 0 {
		t.Fatalf("report entry lingered: %+v", es)
	}
}

func TestStopCheckBlocksAndIgnoresStopHookActive(t *testing.T) {
	e, _, errOut := newHandoffEnv(t, "block")
	mustRecord(t, e, "morpheus", "[handoff: complete]")
	for i := 0; i < 2; i++ {
		if err := e.stopCheck(disp()); !errors.Is(err, errBlock) {
			t.Fatalf("stop %d: err=%v", i, err)
		}
	}
	if !strings.Contains(errOut.String(), "subagent_type=phobetor") {
		t.Errorf("stderr = %q", errOut.String())
	}
}

func TestThirdBlockAbandons(t *testing.T) {
	e, _, errOut := newHandoffEnv(t, "block")
	mustRecord(t, e, "morpheus", "[handoff: complete]")
	_ = e.stopCheck(disp())
	_ = e.enforce(agentCall("nyx"))
	if err := e.stopCheck(disp()); err != nil {
		t.Fatalf("third block should abandon and exit 0, got %v", err)
	}
	es := entries(t, e)
	if len(es) != 1 || es[0].State != handoff.StateAbandoned {
		t.Fatalf("entries = %+v", es)
	}
	if !strings.Contains(errOut.String(), "abandoned") {
		t.Errorf("no warning: %q", errOut.String())
	}
	log, err := os.ReadFile(filepath.Join(e.repoRoot, ".dreamland", "transition.log"))
	if err != nil || !strings.Contains(string(log), "abandoned") {
		t.Errorf("transition log = %q err=%v", log, err)
	}
	if err := e.stopCheck(disp()); err != nil {
		t.Errorf("abandoned entry still blocks: %v", err)
	}
}

func TestReleaseMarksEntries(t *testing.T) {
	e, _, _ := newHandoffEnv(t, "block")
	mustRecord(t, e, "morpheus", "[handoff: complete]")
	if err := e.release(disp()); err != nil {
		t.Fatal(err)
	}
	if es := entries(t, e); es[0].State != handoff.StateReleased {
		t.Fatalf("entries = %+v", es)
	}
	if err := e.stopCheck(disp()); err != nil {
		t.Errorf("released entry blocks: %v", err)
	}
	if err := e.enforce(agentCall("nyx")); err != nil {
		t.Errorf("released entry blocks enforce: %v", err)
	}
}

func TestCorruptStateFailsOpen(t *testing.T) {
	e, _, _ := newHandoffEnv(t, "block")
	mustRecord(t, e, "morpheus", "[handoff: complete]")
	if err := os.WriteFile(filepath.Join(e.store.Dir, "pending", "s1.json"), []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, f := range map[string]func(hookPayload) error{
		"stop": e.stopCheck, "enforce": func(p hookPayload) error { return e.enforce(agentCall("nyx")) },
	} {
		if err := f(disp()); err == nil || errors.Is(err, errBlock) {
			t.Errorf("%s: want a non-blocking error, got %v", name, err)
		}
	}
}

func TestWarnMode(t *testing.T) {
	e, out, errOut := newHandoffEnv(t, "warn")
	mustRecord(t, e, "morpheus", "[handoff: complete]")
	if err := e.enforce(agentCall("nyx")); err != nil {
		t.Errorf("enforce blocked in warn mode: %v", err)
	}
	if err := e.stopCheck(disp()); err != nil {
		t.Errorf("stop-check blocked in warn mode: %v", err)
	}
	if !strings.Contains(errOut.String(), "warning") {
		t.Errorf("no warning logged: %q", errOut.String())
	}
	if err := e.inject(disp()); err != nil || out.Len() == 0 {
		t.Errorf("inject silent in warn mode")
	}
	if es := entries(t, e); es[0].Blocks != 0 {
		t.Errorf("warn mode counted blocks: %+v", es)
	}
	log, err := os.ReadFile(filepath.Join(e.repoRoot, ".dreamland", "transition.log"))
	if err != nil || strings.Count(string(log), "warn:") != 2 {
		t.Errorf("warn violations not in transition log: %q err=%v", log, err)
	}
}

func TestUntaggedCompletionResetsSessionCounter(t *testing.T) {
	e, _, _ := newHandoffEnv(t, "block")
	e.active = func() ([]string, error) { return []string{"a", "b"}, nil }
	mustRecord(t, e, "phobetor", "[verdict: fail]\n[change: c1]")
	mustRecord(t, e, "phantasos", "spec fixed")
	if _, err := os.Stat(filepath.Join(e.store.Dir, "c1.json")); !os.IsNotExist(err) {
		t.Error("untagged phantasos did not reset the session's counter")
	}
}

func TestHookCommandModes(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DREAMLAND_STATE_DIR", t.TempDir())
	oldWd := osGetwd
	osGetwd = func() (string, error) { return repo, nil }
	defer func() { osGetwd = oldWd }()

	run := func(mode, payload string) (string, error) {
		var out, errOut bytes.Buffer
		rootCmd.SetOut(&out)
		rootCmd.SetErr(&errOut)
		rootCmd.SetIn(strings.NewReader(payload))
		rootCmd.SetArgs([]string{"handoff", mode, "--hook"})
		err := rootCmd.Execute()
		rootCmd.SetArgs(nil)
		return out.String() + errOut.String(), err
	}
	if out, err := run("record", `{"session_id":"h1","agent_id":"a","agent_type":"morpheus","last_assistant_message":"x\n[handoff: complete]"}`); err != nil {
		t.Fatalf("record: %v %s", err, out)
	}
	_, err := run("stop-check", `{"session_id":"h1"}`)
	if err == nil || !IsBlocking(err) {
		t.Fatalf("stop-check err = %v, want blocking", err)
	}
	if _, err := run("stop-check", `not json`); err != nil {
		t.Errorf("malformed payload must fail open: %v", err)
	}
	if _, err := run("stop-check", `{"session_id":"../x"}`); err != nil {
		t.Errorf("bad session id must fail open: %v", err)
	}
}

func TestHandoffOffMode(t *testing.T) {
	e, _, _ := newHandoffEnv(t, "block")
	mustRecord(t, e, "morpheus", "[handoff: complete]")

	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".dreamland.json"), []byte(`{"handoff_enforcement":"off"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DREAMLAND_STATE_DIR", t.TempDir())
	oldWd := osGetwd
	osGetwd = func() (string, error) { return repo, nil }
	defer func() { osGetwd = oldWd }()
	rootCmd.SetIn(strings.NewReader(`{"session_id":"s1","agent_id":"a","agent_type":"morpheus","last_assistant_message":"[handoff: complete]"}`))
	rootCmd.SetArgs([]string{"handoff", "record", "--hook"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatal(err)
	}
	rootCmd.SetArgs(nil)
	if es, _ := handoff.NewStore(repo).ReadPending("s1"); len(es) != 0 {
		t.Errorf("off mode recorded: %+v", es)
	}
}

func TestHandoffNextAppliesCounter(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DREAMLAND_STATE_DIR", t.TempDir())
	oldWd := osGetwd
	osGetwd = func() (string, error) { return repo, nil }
	defer func() { osGetwd = oldWd }()

	next := func() string {
		var out bytes.Buffer
		rootCmd.SetOut(&out)
		rootCmd.SetArgs([]string{"handoff", "next", "--from", "phobetor", "--verdict", "fail", "--change", "c1"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatal(err)
		}
		rootCmd.SetArgs(nil)
		var d handoff.Directive
		if err := json.Unmarshal(out.Bytes(), &d); err != nil {
			t.Fatalf("%v: %s", err, out.String())
		}
		return d.Target
	}
	if got := next(); got != "morpheus" {
		t.Errorf("first = %q", got)
	}
	if got := next(); got != "phantasos" {
		t.Errorf("second = %q", got)
	}
}

func TestUnknownBoundCommands(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	settings := `{"hooks":{"SubagentStop":[{"matcher":"","hooks":[
		{"type":"command","command":"dreamland handoff record --hook"},
		{"type":"command","command":"dreamland telemetry write --tool claude-code"},
		{"type":"command","command":"other-tool thing"}]}]}}`
	if err := os.WriteFile(filepath.Join(repo, ".claude", "settings.json"), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}
	old := func(args []string) bool { return args[0] != "handoff" }
	got := unknownBoundCommands(repo, old)
	if len(got) != 1 || got[0] != "dreamland handoff record --hook" {
		t.Fatalf("got %v", got)
	}
	var w bytes.Buffer
	warnUnknownBoundSubcommands(repo, &w)
	if w.Len() != 0 {
		t.Errorf("current binary should know every binding, got %q", w.String())
	}
}

func TestHandoffStatusListsEntries(t *testing.T) {
	e, _, _ := newHandoffEnv(t, "block")
	mustRecord(t, e, "morpheus", "[handoff: complete]")
	_ = e.release(disp())
	var w bytes.Buffer
	printHandoffStatus(e.store, &w)
	if !strings.Contains(w.String(), "released-by-user") || !strings.Contains(w.String(), "target=phobetor") {
		t.Errorf("status = %q", w.String())
	}
}

func promptPayload(text string) hookPayload {
	p := disp()
	p.Prompt = text
	return p
}

func TestPromptClassification(t *testing.T) {
	cases := []struct {
		prompt string
		human  bool
	}{
		{"fix the tests please", true},
		{"/drmlnd:morpheus complete the coding", true},
		{"<agent-message from=\"a1\">\n[Subagent hand-back] ...", false},
		{"<task-notification>\n<task-id>a1</task-id>", false},
		{"[SYSTEM NOTIFICATION] something", false},
		{"  <system-reminder>x</system-reminder>", false},
		{"", false},
		{"   ", false},
	}
	for _, c := range cases {
		e, out, _ := newHandoffEnv(t, "block")
		mustRecord(t, e, "morpheus", "[handoff: complete]")
		if err := e.prompt(promptPayload(c.prompt)); err != nil {
			t.Fatal(err)
		}
		released := entries(t, e)[0].State == handoff.StateReleased
		if released != c.human {
			t.Errorf("%q: released=%v, want %v", c.prompt, released, c.human)
		}
		if !c.human && !strings.Contains(out.String(), `"hookEventName":"UserPromptSubmit"`) {
			t.Errorf("%q: system prompt did not inject: %q", c.prompt, out.String())
		}
		if c.human && out.Len() != 0 {
			t.Errorf("%q: human prompt injected: %q", c.prompt, out.String())
		}
	}
}

func TestPromptIgnoresSubagentPayload(t *testing.T) {
	e, out, _ := newHandoffEnv(t, "block")
	mustRecord(t, e, "morpheus", "[handoff: complete]")
	p := promptPayload("hello")
	p.AgentID = "x"
	if err := e.prompt(p); err != nil || out.Len() != 0 || entries(t, e)[0].State != handoff.StatePending {
		t.Fatalf("subagent payload acted: err=%v out=%q", err, out.String())
	}
}

func TestPromptHumanPathDoesNoOpenspecShellOut(t *testing.T) {
	e, _, _ := newHandoffEnv(t, "block")
	e.active = func() ([]string, error) { t.Fatal("openspec shell-out on the prompt path"); return nil, nil }
	if err := e.prompt(promptPayload("typed")); err != nil {
		t.Fatal(err)
	}
	mustRecord(t, e, "morpheus", "[handoff: complete]")
	if err := e.prompt(promptPayload("typed again")); err != nil {
		t.Fatal(err)
	}
}

func TestBackgroundSequence(t *testing.T) {
	e, out, errOut := newHandoffEnv(t, "block")
	// PostToolUse at launch: nothing recorded yet.
	if err := e.inject(agentCall("morpheus")); err != nil || out.Len() != 0 {
		t.Fatalf("launch inject: err=%v out=%q", err, out.String())
	}
	// hand-back prompt precedes SubagentStop: still nothing.
	if err := e.prompt(promptPayload("<agent-message from=\"a1\">\nreport")); err != nil || out.Len() != 0 {
		t.Fatalf("hand-back prompt: err=%v out=%q", err, out.String())
	}
	mustRecord(t, e, "morpheus", "[handoff: complete]")
	// completion notification arrives after SubagentStop: injects, does not release.
	if err := e.prompt(promptPayload("<task-notification>\n<task-id>a1</task-id>")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "REQUIRED NEXT STEP") || entries(t, e)[0].State != handoff.StatePending {
		t.Fatalf("notification: out=%q entries=%+v", out.String(), entries(t, e))
	}
	// enforce reads the entry written after launch.
	if err := e.enforce(agentCall("Explore")); !errors.Is(err, errBlock) || !strings.Contains(errOut.String(), "phobetor") {
		t.Fatalf("enforce err=%v stderr=%q", err, errOut.String())
	}
	if err := e.enforce(agentCall("phobetor")); err != nil || len(entries(t, e)) != 0 {
		t.Fatalf("phobetor did not clear: err=%v", err)
	}
}

func TestRecordIgnoresEmptyAgentType(t *testing.T) {
	e, _, _ := newHandoffEnv(t, "block")
	p := sub("", "[handoff: complete]")
	p.AgentID = "a1"
	if err := e.record(p); err != nil || len(entries(t, e)) != 0 {
		t.Fatalf("err=%v entries=%+v", err, entries(t, e))
	}
}

func TestPartialPassReportsAndFullPassDispatchesBaku(t *testing.T) {
	e, _, _ := newHandoffEnv(t, "block")
	dir := filepath.Join(e.repoRoot, "openspec", "changes", "c1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tasks.md"), []byte("- [x] 1\n- [ ] 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRecord(t, e, "phobetor", "ok\n[verdict: pass]\n[change: c1]")
	es := entries(t, e)
	if len(es) != 1 || es[0].Blocking() || !strings.Contains(es[0].Directive.Reason, "partial pass: 1 tasks of c1 unticked") {
		t.Fatalf("partial pass entries = %+v", es)
	}
	if err := os.WriteFile(filepath.Join(dir, "tasks.md"), []byte("- [x] 1\n- [x] 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRecord(t, e, "phobetor", "ok again\n[verdict: pass]\n[change: c1]")
	es = entries(t, e)
	if len(es) != 2 || es[1].Directive.Target != "baku" {
		t.Fatalf("full pass entries = %+v", es)
	}
}

func TestUntaggedCompletionLeavesOtherSessionCounters(t *testing.T) {
	e, _, _ := newHandoffEnv(t, "block")
	e.active = func() ([]string, error) { return []string{"c9"}, nil }
	other := sub("phobetor", "[verdict: fail]\n[change: c9]")
	other.SessionID = "s2"
	if err := e.record(other); err != nil {
		t.Fatal(err)
	}
	mustRecord(t, e, "baku", "closed")
	if c, _ := e.store.ReadCounter("c9"); c.PhobetorFailures != 1 {
		t.Errorf("another session's counter was touched: %+v", c)
	}
}

func TestStopCheckBlocksWithStopHookActivePayload(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DREAMLAND_STATE_DIR", t.TempDir())
	oldWd := osGetwd
	osGetwd = func() (string, error) { return repo, nil }
	defer func() { osGetwd = oldWd }()
	run := func(mode, payload string) error {
		rootCmd.SetOut(&bytes.Buffer{})
		rootCmd.SetErr(&bytes.Buffer{})
		rootCmd.SetIn(strings.NewReader(payload))
		rootCmd.SetArgs([]string{"handoff", mode, "--hook"})
		defer rootCmd.SetArgs(nil)
		return rootCmd.Execute()
	}
	if err := run("record", `{"session_id":"h2","agent_id":"a","agent_type":"morpheus","last_assistant_message":"[handoff: complete]"}`); err != nil {
		t.Fatal(err)
	}
	if err := run("stop-check", `{"session_id":"h2","stop_hook_active":true}`); err == nil || !IsBlocking(err) {
		t.Fatalf("stop_hook_active must not disable blocking: %v", err)
	}
	if err := run("prompt", `{"session_id":"h2","prompt":"go on"}`); err != nil {
		t.Fatalf("prompt: %v", err)
	}
	if err := run("stop-check", `{"session_id":"h2"}`); err != nil {
		t.Fatalf("released entry blocks: %v", err)
	}
}
