package cmd

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"dreamland/internal/config"
	"dreamland/internal/handoff"
)

var handoffCmd = &cobra.Command{
	Use:   "handoff",
	Short: "Deterministic next-step mechanism for fixed agent hand-offs",
}

var (
	handoffHook       bool
	handoffFrom       string
	handoffVerdict    string
	handoffHandoffTag string
	handoffChange     string
	handoffSession    string
)

func init() {
	rootCmd.AddCommand(handoffCmd)
	for _, m := range []struct {
		use, short string
		run        func(*handoffEnv, hookPayload) error
	}{
		{"record", "SubagentStop: compute and store the next-step directive", (*handoffEnv).record},
		{"inject", "PostToolUse (Task|Agent): inject the required next step into the dispatcher's context", (*handoffEnv).inject},
		{"enforce", "PreToolUse (Task|Agent): block dispatching anything but the required next agent", (*handoffEnv).enforce},
		{"stop-check", "Stop: block ending the turn while a hand-off is pending", (*handoffEnv).stopCheck},
		{"release", "UserPromptSubmit: release pending hand-offs", (*handoffEnv).release},
	} {
		run := m.run
		c := &cobra.Command{
			Use:   m.use,
			Short: m.short,
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				return runHandoffHook(cmd, run)
			},
		}
		c.Flags().BoolVar(&handoffHook, "hook", false, "read the hook payload from stdin (set by dreamland's hook bindings)")
		handoffCmd.AddCommand(c)
	}

	next := &cobra.Command{
		Use:   "next",
		Short: "Print the next-step directive for --from/--verdict/--handoff/--change (applies the failure counter)",
		Args:  cobra.NoArgs,
		RunE:  runHandoffNext,
	}
	next.Flags().StringVar(&handoffFrom, "from", "", "agent that just finished (required)")
	next.Flags().StringVar(&handoffVerdict, "verdict", "", "phobetor verdict: pass|fail|spec-defect|unverified")
	next.Flags().StringVar(&handoffHandoffTag, "handoff", "", "handoff tag: complete|blocked")
	next.Flags().StringVar(&handoffChange, "change", "", "change slug")
	handoffCmd.AddCommand(next)

	clear := &cobra.Command{
		Use:   "clear",
		Short: "Remove pending hand-off entries (human command)",
		Args:  cobra.NoArgs,
		RunE:  runHandoffClear,
	}
	clear.Flags().StringVar(&handoffSession, "session", "", "only clear this session's entries")
	handoffCmd.AddCommand(clear)

	rootCmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show hand-off entries and warn about hook bindings this binary does not know",
		Args:  cobra.NoArgs,
		RunE:  runStatus,
	})
}

// hookPayload is the subset of Claude Code hook payloads the handoff modes read.
type hookPayload struct {
	SessionID            string `json:"session_id"`
	AgentID              string `json:"agent_id"`
	AgentType            string `json:"agent_type"`
	ToolName             string `json:"tool_name"`
	LastAssistantMessage string `json:"last_assistant_message"`
	AgentTranscriptPath  string `json:"agent_transcript_path"`
	ToolInput            struct {
		SubagentType string `json:"subagent_type"`
	} `json:"tool_input"`
}

// isDispatcher reports whether the hook fired for the dispatching session rather
// than a subagent: subagent-originated hooks carry agent_id, the dispatcher's do
// not (in plain sessions, and under `claude --agent X` alike).
func (p hookPayload) isDispatcher() bool { return p.AgentID == "" }

// handoffEnv carries everything a mode needs so tests can drive it without a
// process, a real repo, or the openspec binary.
type handoffEnv struct {
	store    *handoff.Store
	mode     string
	repoRoot string
	errOut   io.Writer
	out      io.Writer
	active   func() ([]string, error)
}

// exitCode2 signals that a mode wants the process to exit 2.
var errBlock = errors.New("handoff block")

func runHandoffHook(cmd *cobra.Command, run func(*handoffEnv, hookPayload) error) error {
	cmd.SilenceUsage = true
	cwd, err := osGetwd()
	if err != nil {
		return nil
	}
	repoRoot, err := config.FindRepoRoot(cwd)
	if err != nil {
		return nil
	}
	cfg, _ := config.Load(cwd)
	env := &handoffEnv{
		store:    handoff.NewStore(repoRoot),
		mode:     cfg.HandoffMode(),
		repoRoot: repoRoot,
		errOut:   cmd.ErrOrStderr(),
		out:      cmd.OutOrStdout(),
		active:   func() ([]string, error) { return activeChanges(repoRoot) },
	}
	if env.mode == "off" {
		return nil
	}
	data, err := io.ReadAll(cmd.InOrStdin())
	if err != nil || len(data) == 0 {
		return nil
	}
	var p hookPayload
	if err := json.Unmarshal(data, &p); err != nil {
		fmt.Fprintf(env.errOut, "dreamland handoff: ignoring malformed hook payload: %v\n", err)
		return nil
	}
	if !handoff.ValidSessionID(p.SessionID) {
		fmt.Fprintf(env.errOut, "dreamland handoff: ignoring payload with invalid session_id %q\n", p.SessionID)
		return nil
	}
	err = run(env, p)
	if errors.Is(err, errBlock) {
		cmd.SilenceErrors = true
		return Blocking(errors.New("dreamland handoff: blocked"))
	}
	if err != nil {
		fmt.Fprintf(env.errOut, "dreamland handoff: %v (failing open)\n", err)
	}
	return nil
}

// activeChanges lists in-progress change names via `openspec list --json`.
func activeChanges(repoRoot string) ([]string, error) {
	c := exec.Command("openspec", "list", "--json")
	c.Dir = repoRoot
	out, err := c.Output()
	if err != nil {
		return nil, err
	}
	var names []string
	for _, ch := range parseChangeStatuses(out) {
		names = append(names, ch.Name)
	}
	return names, nil
}

// apply computes and persists the outcome for a finished agent: the counter
// update happens under lock, so this is the single writer of counters.
func (e *handoffEnv) apply(agent string, tags handoff.Tags, session string) (handoff.Directive, string, error) {
	if !handoff.KnownAgent(agent) {
		return handoff.Directive{}, "", nil
	}
	if !handoff.TouchesCounter(agent) {
		d, _ := handoff.Next(agent, tags, handoff.Counter{})
		return d, tags.Change, nil
	}
	key := handoff.ResolveChange(tags.Change, session, e.active)
	var d handoff.Directive
	err := e.store.UpdateCounter(key, func(c handoff.Counter) (handoff.Counter, bool) {
		var nc handoff.Counter
		d, nc = handoff.Next(agent, tags, c)
		return nc, d.ClearCounter
	})
	return d, key, err
}

func (e *handoffEnv) record(p hookPayload) error {
	if p.AgentType == "" || p.AgentID == "" {
		return nil
	}
	report := p.LastAssistantMessage
	if report == "" && p.AgentTranscriptPath != "" {
		report = lastAssistantText(p.AgentTranscriptPath)
	}
	id := handoff.DirectiveID(p.SessionID, p.AgentType, report)
	existing, err := e.store.ReadPending(p.SessionID)
	if err != nil {
		return err
	}
	for _, en := range existing {
		if en.ID == id {
			return nil
		}
	}
	tags := handoff.ParseTags(report)
	d, change, err := e.apply(p.AgentType, tags, p.SessionID)
	if err != nil {
		return err
	}
	if d.Kind == "" {
		return nil
	}
	entry := handoff.Entry{
		ID:         id,
		Agent:      p.AgentType,
		Change:     change,
		Directive:  d,
		TagMissing: d.TagMissing,
		State:      handoff.StatePending,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	return e.store.UpdatePending(p.SessionID, func(es []handoff.Entry) []handoff.Entry {
		return append(trimSettled(es), entry)
	})
}

// trimSettled keeps live entries and only the 20 most recent settled ones.
func trimSettled(es []handoff.Entry) []handoff.Entry {
	var settled, live []handoff.Entry
	for _, en := range es {
		if en.State == handoff.StatePending {
			live = append(live, en)
		} else {
			settled = append(settled, en)
		}
	}
	if len(settled) > 20 {
		settled = settled[len(settled)-20:]
	}
	return append(settled, live...)
}

func (e *handoffEnv) inject(p hookPayload) error {
	if !p.isDispatcher() {
		return nil
	}
	es, err := e.store.ReadPending(p.SessionID)
	if err != nil {
		return err
	}
	var parts []string
	for _, en := range es {
		if en.State != handoff.StatePending {
			continue
		}
		switch en.Directive.Kind {
		case handoff.KindDispatch:
			s := fmt.Sprintf("REQUIRED NEXT STEP (dreamland handoff): call Agent with subagent_type=%s now. Do not call any other agent and do not end your turn.", en.Directive.Target)
			if en.Change != "" {
				s += " Change: " + en.Change + "."
			}
			s += " Reason: " + en.Directive.Reason + "."
			parts = append(parts, s)
		case handoff.KindReport:
			parts = append(parts, "HANDOFF REPORT (dreamland handoff): "+en.Directive.Reason+". No agent dispatch is required for this.")
		}
	}
	if len(parts) == 0 {
		return nil
	}
	return json.NewEncoder(e.out).Encode(map[string]any{
		"hookSpecificOutput": map[string]string{
			"hookEventName":     "PostToolUse",
			"additionalContext": strings.Join(parts, "\n"),
		},
	})
}

// blockOrAbandon applies one block to every blocking entry: an entry already
// blocked MaxBlocks-1 times is abandoned instead. It reports whether any entry
// remains blocking, plus the abandoned entries.
func blockOrAbandon(es []handoff.Entry) (out []handoff.Entry, stillBlocking bool, abandoned []handoff.Entry) {
	out = make([]handoff.Entry, len(es))
	copy(out, es)
	for i, en := range out {
		if !en.Blocking() {
			continue
		}
		if en.Blocks >= handoff.MaxBlocks-1 {
			out[i].State = handoff.StateAbandoned
			abandoned = append(abandoned, out[i])
			continue
		}
		out[i].Blocks++
		stillBlocking = true
	}
	return out, stillBlocking, abandoned
}

func blockingTargets(es []handoff.Entry) []string {
	var t []string
	for _, en := range es {
		if en.Blocking() {
			t = append(t, en.Directive.Target)
		}
	}
	return t
}

func (e *handoffEnv) enforce(p hookPayload) error {
	if !p.isDispatcher() {
		return nil
	}
	if p.ToolName != "" && p.ToolName != "Agent" && p.ToolName != "Task" {
		return nil
	}
	es, err := e.store.ReadPending(p.SessionID)
	if err != nil {
		return err
	}
	if len(blockingTargets(es)) == 0 {
		return e.dropReports(p.SessionID, es)
	}
	want := p.ToolInput.SubagentType
	for _, en := range es {
		if en.Blocking() && en.Directive.Target == want {
			return e.store.UpdatePending(p.SessionID, func(cur []handoff.Entry) []handoff.Entry {
				var out []handoff.Entry
				removed := false
				for _, c := range cur {
					if !removed && c.Blocking() && c.Directive.Target == want {
						removed = true
						continue
					}
					if c.State == handoff.StatePending && c.Directive.Kind == handoff.KindReport {
						continue
					}
					out = append(out, c)
				}
				return out
			})
		}
	}
	targets := blockingTargets(es)
	msg := fmt.Sprintf("dreamland handoff: a hand-off is pending. Call Agent with subagent_type=%s before dispatching %q or ending your turn.", strings.Join(targets, " or "), want)
	if e.mode == "warn" {
		fmt.Fprintln(e.errOut, "warning: "+msg)
		return nil
	}
	return e.applyBlock(p.SessionID, msg)
}

func (e *handoffEnv) dropReports(session string, es []handoff.Entry) error {
	has := false
	for _, en := range es {
		if en.State == handoff.StatePending && en.Directive.Kind == handoff.KindReport {
			has = true
		}
	}
	if !has {
		return nil
	}
	return e.store.UpdatePending(session, func(cur []handoff.Entry) []handoff.Entry {
		var out []handoff.Entry
		for _, c := range cur {
			if c.State == handoff.StatePending && c.Directive.Kind == handoff.KindReport {
				continue
			}
			out = append(out, c)
		}
		return out
	})
}

// applyBlock counts one block against every blocking entry. It prints msg and
// returns errBlock while any entry is still blocking; abandoned entries are
// logged and the hook exits 0 with a warning once none remain.
func (e *handoffEnv) applyBlock(session, msg string) error {
	var stillBlocking bool
	var abandoned []handoff.Entry
	if err := e.store.UpdatePending(session, func(cur []handoff.Entry) []handoff.Entry {
		var out []handoff.Entry
		out, stillBlocking, abandoned = blockOrAbandon(cur)
		return out
	}); err != nil {
		return err
	}
	for _, a := range abandoned {
		line := fmt.Sprintf("hand-off abandoned after %d blocks: %s -> %s (change %s)", handoff.MaxBlocks-1, a.Agent, a.Directive.Target, a.Change)
		fmt.Fprintln(e.errOut, "warning: dreamland handoff: "+line)
		appendTransitionLine(e.repoRoot, session, line)
	}
	if stillBlocking {
		fmt.Fprintln(e.errOut, msg)
		return errBlock
	}
	return nil
}

func (e *handoffEnv) stopCheck(p hookPayload) error {
	if !p.isDispatcher() {
		return nil
	}
	es, err := e.store.ReadPending(p.SessionID)
	if err != nil {
		return err
	}
	targets := blockingTargets(es)
	if len(targets) == 0 {
		return nil
	}
	msg := fmt.Sprintf("dreamland handoff: you cannot end your turn yet. Call Agent with subagent_type=%s now.", strings.Join(targets, " and then "))
	if e.mode == "warn" {
		fmt.Fprintln(e.errOut, "warning: "+msg)
		return nil
	}
	return e.applyBlock(p.SessionID, msg)
}

func (e *handoffEnv) release(p hookPayload) error {
	if !p.isDispatcher() {
		return nil
	}
	es, err := e.store.ReadPending(p.SessionID)
	if err != nil || len(es) == 0 {
		return err
	}
	changed := false
	for _, en := range es {
		if en.State == handoff.StatePending {
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return e.store.UpdatePending(p.SessionID, func(cur []handoff.Entry) []handoff.Entry {
		out := make([]handoff.Entry, len(cur))
		copy(out, cur)
		for i := range out {
			if out[i].State == handoff.StatePending {
				out[i].State = handoff.StateReleased
			}
		}
		return out
	})
}

func appendTransitionLine(repoRoot, session, line string) {
	dir := filepath.Join(repoRoot, ".dreamland")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(dir, "transition.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s [%s] %s\n", time.Now().UTC().Format(time.RFC3339), session, line)
}

// lastAssistantText is the fallback report source: the last assistant message's
// text blocks in a JSONL transcript.
func lastAssistantText(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	var last string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 64*1024*1024)
	for sc.Scan() {
		var line struct {
			Type    string `json:"type"`
			Message struct {
				Role    string          `json:"role"`
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal(sc.Bytes(), &line) != nil {
			continue
		}
		if line.Type != "assistant" && line.Message.Role != "assistant" {
			continue
		}
		var s string
		if json.Unmarshal(line.Message.Content, &s) == nil {
			last = s
			continue
		}
		var blocks []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if json.Unmarshal(line.Message.Content, &blocks) == nil {
			var texts []string
			for _, b := range blocks {
				if b.Type == "text" {
					texts = append(texts, b.Text)
				}
			}
			if len(texts) > 0 {
				last = strings.Join(texts, "\n")
			}
		}
	}
	return last
}

func runHandoffNext(cmd *cobra.Command, _ []string) error {
	cmd.SilenceUsage = true
	if handoffFrom == "" {
		return errors.New("--from is required")
	}
	cwd, err := osGetwd()
	if err != nil {
		return err
	}
	repoRoot, err := config.FindRepoRoot(cwd)
	if err != nil {
		return err
	}
	env := &handoffEnv{
		store:  handoff.NewStore(repoRoot),
		active: func() ([]string, error) { return activeChanges(repoRoot) },
	}
	session := "cli"
	if s := os.Getenv("CLAUDE_SESSION_ID"); handoff.ValidSessionID(s) {
		session = s
	}
	tags := handoff.Tags{Handoff: handoffHandoffTag, Verdict: handoffVerdict, Change: handoffChange}
	d, _, err := env.apply(handoffFrom, tags, session)
	if err != nil {
		return err
	}
	return json.NewEncoder(cmd.OutOrStdout()).Encode(d)
}

func runHandoffClear(cmd *cobra.Command, _ []string) error {
	cwd, err := osGetwd()
	if err != nil {
		return err
	}
	repoRoot, err := config.FindRepoRoot(cwd)
	if err != nil {
		return err
	}
	store := handoff.NewStore(repoRoot)
	sessions := store.Sessions()
	if handoffSession != "" {
		if !handoff.ValidSessionID(handoffSession) {
			return fmt.Errorf("invalid session id %q", handoffSession)
		}
		sessions = []string{handoffSession}
	}
	for _, s := range sessions {
		if err := store.UpdatePending(s, func([]handoff.Entry) []handoff.Entry { return nil }); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "cleared: %s\n", s)
	}
	return nil
}

func runStatus(cmd *cobra.Command, _ []string) error {
	cwd, err := osGetwd()
	if err != nil {
		return err
	}
	repoRoot, err := config.FindRepoRoot(cwd)
	if err != nil {
		return err
	}
	printHandoffStatus(handoff.NewStore(repoRoot), cmd.OutOrStdout())
	warnUnknownBoundSubcommands(repoRoot, cmd.ErrOrStderr())
	return nil
}

func printHandoffStatus(store *handoff.Store, w io.Writer) {
	sessions := store.Sessions()
	sort.Strings(sessions)
	n := 0
	for _, s := range sessions {
		es, err := store.ReadPending(s)
		if err != nil {
			fmt.Fprintf(w, "handoff: session %s: unreadable state (%v)\n", s, err)
			continue
		}
		for _, en := range es {
			n++
			fmt.Fprintf(w, "handoff: session=%s state=%s from=%s kind=%s target=%s change=%s blocks=%d\n",
				s, en.State, en.Agent, en.Directive.Kind, en.Directive.Target, en.Change, en.Blocks)
		}
	}
	if n == 0 {
		fmt.Fprintln(w, "handoff: no entries")
	}
}

// warnUnknownBoundSubcommands warns for every `dreamland <sub>` command bound in
// .claude/settings.json that the running binary does not know: Claude Code treats
// the resulting exit 1 as non-blocking, so a stale binary would silently drop it.
func warnUnknownBoundSubcommands(repoRoot string, w io.Writer) {
	for _, cmdline := range unknownBoundCommands(repoRoot, func(args []string) bool {
		c, _, err := rootCmd.Find(args)
		return err == nil && c != rootCmd
	}) {
		fmt.Fprintf(w, "warning: .claude/settings.json binds %q but this dreamland binary does not know that subcommand; rebuild or reinstall dreamland\n", cmdline)
	}
}

func unknownBoundCommands(repoRoot string, known func(args []string) bool) []string {
	data, err := os.ReadFile(filepath.Join(repoRoot, ".claude", "settings.json"))
	if err != nil {
		return nil
	}
	var settings struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if json.Unmarshal(data, &settings) != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, groups := range settings.Hooks {
		for _, g := range groups {
			for _, h := range g.Hooks {
				fields := strings.Fields(h.Command)
				if len(fields) < 2 || fields[0] != "dreamland" || seen[h.Command] {
					continue
				}
				seen[h.Command] = true
				if !known(fields[1:]) {
					out = append(out, h.Command)
				}
			}
		}
	}
	sort.Strings(out)
	return out
}
