package tools

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"dreamland/internal/config"
	"dreamland/internal/telemetry"
	"dreamland/internal/telemetry/cursor"
)

// seedMailbox writes a receiver-owned mailbox at <stateDir>/sessions/<id>.json.
func seedMailbox(t *testing.T, stateDir, id, body string) {
	t.Helper()
	dir := filepath.Join(stateDir, "sessions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mailboxJSON(input, output, cached int64) string {
	return `{"version":2,"model":"gpt-4o","input_tokens":` + strconv.FormatInt(input, 10) + `,"output_tokens":` + strconv.FormatInt(output, 10) +
		`,"cached_tokens":` + strconv.FormatInt(cached, 10) + `,"span_count":1,"captured_at":"2026-07-14T00:00:00Z"}`
}

// otelEnv isolates the collector from the developer's real VS Code chat sessions and
// receiver state, returning the temp state dir.
func otelEnv(t *testing.T) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("LOCALAPPDATA", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())
	stateDir := t.TempDir()
	t.Setenv("DREAMLAND_STATE_DIR", stateDir)
	return stateDir
}

func collectCopilot(t *testing.T, repo, sessionID string) *telemetry.SnapshotResult {
	t.Helper()
	cfg := &config.Config{ModelID: "default-model", RepoRoot: repo}
	stdin := strings.NewReader(`{"hook_event_name":"Stop","session_id":"` + sessionID + `","transcript_path":"/nonexistent.jsonl"}`)
	res, err := (&CopilotCollector{}).Collect(stdin, cfg)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	return res
}

// collectAndWrite mimics `dreamland telemetry write --tool github-copilot`.
func collectAndWrite(t *testing.T, repo, sessionID string) *telemetry.SnapshotResult {
	t.Helper()
	res := collectCopilot(t, repo, sessionID)
	if err := telemetry.Write(repo, res); err != nil {
		t.Fatalf("telemetry.Write: %v", err)
	}
	snap, err := telemetry.Read(repo)
	if err != nil || snap == nil {
		t.Fatalf("telemetry.Read = (%v, %v)", snap, err)
	}
	return snap
}

func cursorFile(repo, id string) string {
	return filepath.Join(repo, ".dreamland", "otel-cursors", id+".json")
}

func TestCopilotOtel_TwoStopsWithNoNewSpanDoNotDoubleCount(t *testing.T) {
	stateDir := otelEnv(t)
	repo := t.TempDir()
	seedMailbox(t, stateDir, "sess-1", mailboxJSON(1000, 200, 0))

	first := collectAndWrite(t, repo, "sess-1")
	second := collectAndWrite(t, repo, "sess-1")

	if first.InputTokens != 1000 || first.OutputTokens != 200 {
		t.Errorf("after first Stop: %+v, want 1000/200", first)
	}
	if second.InputTokens != 1000 || second.OutputTokens != 200 {
		t.Errorf("after second Stop with no new span: %+v, want still 1000/200", second)
	}
}

func TestCopilotOtel_NewSpanBetweenStopsAddsOnlyTheDelta(t *testing.T) {
	stateDir := otelEnv(t)
	repo := t.TempDir()
	seedMailbox(t, stateDir, "sess-1", mailboxJSON(1000, 200, 0))
	collectAndWrite(t, repo, "sess-1")

	// The receiver adds a 300/50 span: the mailbox is a running total, now 1300/250.
	seedMailbox(t, stateDir, "sess-1", mailboxJSON(1300, 250, 0))
	snap := collectAndWrite(t, repo, "sess-1")

	if snap.InputTokens != 1300 || snap.OutputTokens != 250 {
		t.Errorf("snapshot = %+v, want 1300/250", snap)
	}
}

func TestCopilotOtel_CollectReturnsDeltaAndAdvancesCursor(t *testing.T) {
	stateDir := otelEnv(t)
	repo := t.TempDir()
	seedMailbox(t, stateDir, "sess-1", mailboxJSON(1000, 200, 40))

	res := collectCopilot(t, repo, "sess-1")
	if res.InputTokens != 1000 || res.OutputTokens != 200 || res.CachedTokens != 40 || res.Model != "gpt-4o" {
		t.Errorf("first Collect = %+v, want 1000/200/40 gpt-4o", res)
	}
	c, found, err := cursor.Load(repo, "sess-1")
	if err != nil || !found || c != (cursor.Counts{Input: 1000, Output: 200, Cached: 40}) {
		t.Errorf("cursor after first Collect = (%+v, %v, %v), want the mailbox totals", c, found, err)
	}

	seedMailbox(t, stateDir, "sess-1", mailboxJSON(1300, 250, 45))
	res = collectCopilot(t, repo, "sess-1")
	if res.InputTokens != 300 || res.OutputTokens != 50 || res.CachedTokens != 5 {
		t.Errorf("second Collect = %+v, want delta 300/50/5", res)
	}

	res = collectCopilot(t, repo, "sess-1")
	if res.InputTokens != 0 || res.OutputTokens != 0 || res.CachedTokens != 0 {
		t.Errorf("third Collect with no new span = %+v, want zero delta", res)
	}
	if res.Model != "gpt-4o" {
		t.Errorf("Model = %q, want the mailbox model even on a zero delta", res.Model)
	}
}

func TestCopilotOtel_DeltaIsClampedAtZeroPerField(t *testing.T) {
	stateDir := otelEnv(t)
	repo := t.TempDir()
	if err := cursor.Store(repo, "sess-1", cursor.Counts{Input: 1000, Output: 200, Cached: 0}); err != nil {
		t.Fatal(err)
	}
	// Input grew, output shrank (e.g. the mailbox was recreated after GC or a restart).
	seedMailbox(t, stateDir, "sess-1", mailboxJSON(1200, 50, 0))

	res := collectCopilot(t, repo, "sess-1")
	if res.InputTokens != 200 || res.OutputTokens != 0 {
		t.Errorf("got %+v, want per-field max(total-cursor, 0) = 200/0", res)
	}
	c, _, _ := cursor.Load(repo, "sess-1")
	if c.Input != 1200 || c.Output != 50 {
		t.Errorf("cursor = %+v, want it set to the mailbox totals 1200/50", c)
	}
}

func TestCopilotOtel_CursorIsPerSession(t *testing.T) {
	stateDir := otelEnv(t)
	repo := t.TempDir()
	seedMailbox(t, stateDir, "A", mailboxJSON(100, 10, 0))
	seedMailbox(t, stateDir, "B", mailboxJSON(7000, 700, 0))

	collectCopilot(t, repo, "A")
	collectCopilot(t, repo, "B")
	for _, id := range []string{"A", "B"} {
		if _, err := os.Stat(cursorFile(repo, id)); err != nil {
			t.Errorf("cursor for %s not at .dreamland/otel-cursors/%s.json: %v", id, id, err)
		}
	}

	seedMailbox(t, stateDir, "A", mailboxJSON(150, 15, 0))
	res := collectCopilot(t, repo, "A")
	if res.InputTokens != 50 || res.OutputTokens != 5 {
		t.Errorf("A's delta = %+v, want 50/5 (B's usage must not be consumed or mixed in)", res)
	}
	if b, _, _ := cursor.Load(repo, "B"); b.Input != 7000 {
		t.Errorf("B's cursor changed by A's write: %+v", b)
	}
}

func TestCopilotOtel_TwoReposShareOneReceiverStateButKeepOwnTokens(t *testing.T) {
	stateDir := otelEnv(t)
	repoX, repoY := t.TempDir(), t.TempDir()
	seedMailbox(t, stateDir, "A", mailboxJSON(1000, 100, 0))
	seedMailbox(t, stateDir, "B", mailboxJSON(20, 2, 0))

	x := collectAndWrite(t, repoX, "A")
	y := collectAndWrite(t, repoY, "B")

	if x.InputTokens != 1000 || x.OutputTokens != 100 {
		t.Errorf("repo X = %+v, want only session A's 1000/100", x)
	}
	if y.InputTokens != 20 || y.OutputTokens != 2 {
		t.Errorf("repo Y = %+v, want only session B's 20/2", y)
	}
}

func TestCopilotOtel_DoesNotReadRepoLocalLegacyMailbox(t *testing.T) {
	otelEnv(t)
	repo := t.TempDir()
	legacy := filepath.Join(repo, ".dreamland", "otel-sessions")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "sess-1.json"), []byte(mailboxJSON(999, 99, 0)), 0o644); err != nil {
		t.Fatal(err)
	}
	res := collectCopilot(t, repo, "sess-1")
	if res.InputTokens != 0 || res.OutputTokens != 0 {
		t.Errorf("collector read the legacy repo-local mailbox: %+v", res)
	}
}

func TestCopilotOtel_UnsafeSessionIDDisablesReceiverSource(t *testing.T) {
	ids := []string{"../x", "a:b", strings.Repeat("a", 129)}
	for _, id := range ids {
		name := id
		if len(name) > 12 {
			name = "129-chars"
		}
		t.Run(name, func(t *testing.T) {
			stateDir := otelEnv(t)
			repo := t.TempDir()
			// Bait: where a traversal ("../x") would resolve, and a real-looking mailbox.
			if err := os.WriteFile(filepath.Join(stateDir, "x.json"), []byte(mailboxJSON(9999, 999, 0)), 0o644); err != nil {
				t.Fatal(err)
			}
			if len(id) <= 128 {
				seedMailbox(t, stateDir, strings.ReplaceAll(strings.ReplaceAll(id, "/", "_"), ":", "_"), mailboxJSON(9999, 999, 0))
			}
			transcript := writeTempTranscript(t, sampleClaudeTranscript)

			cfg := &config.Config{ModelID: "default-model", RepoRoot: repo}
			stdin := strings.NewReader(`{"session_id":"` + id + `","transcript_path":"` + filepath.ToSlash(transcript) + `"}`)
			res, err := (&CopilotCollector{}).Collect(stdin, cfg)
			if err != nil {
				t.Fatalf("Collect must succeed (exit 0), got %v", err)
			}
			// Fell through to the transcript fallback, not the mailbox.
			if res.InputTokens != 1000 || res.OutputTokens != 200 {
				t.Errorf("got %+v, want transcript fallback 1000/200, not the mailbox 9999/999", res)
			}
			filepath.WalkDir(filepath.Join(repo, ".dreamland", "otel-cursors"), func(p string, d os.DirEntry, err error) error {
				if err == nil && !d.IsDir() {
					t.Errorf("cursor file created for unsafe session id: %s", p)
				}
				return nil
			})
			if _, err := os.Stat(filepath.Join(repo, ".dreamland", "x.json")); err == nil {
				t.Error("traversal wrote outside the cursor directory")
			}
		})
	}
}

func TestCopilotOtel_StaleCursorsArePruned(t *testing.T) {
	stateDir := otelEnv(t)
	repo := t.TempDir()
	if err := cursor.Store(repo, "old", cursor.Counts{Input: 1}); err != nil {
		t.Fatal(err)
	}
	if err := cursor.Store(repo, "recent", cursor.Counts{Input: 1}); err != nil {
		t.Fatal(err)
	}
	eightDays := time.Now().Add(-8 * 24 * time.Hour)
	if err := os.Chtimes(cursorFile(repo, "old"), eightDays, eightDays); err != nil {
		t.Fatal(err)
	}
	seedMailbox(t, stateDir, "sess-1", mailboxJSON(10, 1, 0))

	collectCopilot(t, repo, "sess-1")

	if _, err := os.Stat(cursorFile(repo, "old")); !os.IsNotExist(err) {
		t.Errorf("cursor with mtime 8 days ago must be deleted (err=%v)", err)
	}
	if _, err := os.Stat(cursorFile(repo, "recent")); err != nil {
		t.Errorf("recent cursor must be kept: %v", err)
	}
}

// captureStderr redirects both os.Stderr and telemetry.Stderr for the duration of fn.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	origOS, origTel := os.Stderr, telemetry.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	var tel bytes.Buffer
	os.Stderr, telemetry.Stderr = w, &tel
	done := make(chan string)
	go func() { b, _ := io.ReadAll(r); done <- string(b) }()
	func() {
		defer func() { os.Stderr, telemetry.Stderr = origOS, origTel }()
		fn()
	}()
	w.Close()
	return <-done + tel.String()
}

func TestCopilotOtel_CorruptCursorWarnsAndReportsZero(t *testing.T) {
	stateDir := otelEnv(t)
	repo := t.TempDir()
	seedMailbox(t, stateDir, "sess-1", mailboxJSON(1000, 200, 0))
	if err := os.MkdirAll(filepath.Dir(cursorFile(repo, "sess-1")), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cursorFile(repo, "sess-1"), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	var res *telemetry.SnapshotResult
	warn := captureStderr(t, func() { res = collectCopilot(t, repo, "sess-1") })

	if res.InputTokens != 0 || res.OutputTokens != 0 || res.CachedTokens != 0 {
		t.Errorf("corrupt cursor must report zero for that call, got %+v", res)
	}
	if !strings.Contains(strings.ToLower(warn), "cursor") {
		t.Errorf("expected a stderr warning mentioning the cursor, got %q", warn)
	}
}

func TestCopilotOtel_UnwritableCursorNeverFailsTheCommand(t *testing.T) {
	stateDir := otelEnv(t)
	repo := t.TempDir()
	seedMailbox(t, stateDir, "sess-1", mailboxJSON(1000, 200, 0))
	// A regular file where the cursor directory must go: Store cannot succeed.
	if err := os.MkdirAll(filepath.Join(repo, ".dreamland"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".dreamland", "otel-cursors"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{ModelID: "default-model", RepoRoot: repo}
	stdin := strings.NewReader(`{"session_id":"sess-1"}`)
	var err error
	captureStderr(t, func() { _, err = (&CopilotCollector{}).Collect(stdin, cfg) })
	if err != nil {
		t.Fatalf("a cursor failure must not fail Collect: %v", err)
	}
}

func TestCopilotOtel_ChatSessionSourceUnchangedAndCreatesNoCursor(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stateDir := t.TempDir()
	t.Setenv("DREAMLAND_STATE_DIR", stateDir)

	roots := vscodeWorkspaceStorageRoots()
	if len(roots) == 0 {
		t.Skip("no workspace storage roots resolved for this OS")
	}
	sessionDir := filepath.Join(roots[0], "workspace-hash", "chatSessions")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fixture := `{"kind":1,"k":["requests",0,"promptTokens"],"v":6027}` + "\n" +
		`{"kind":1,"k":["requests",0,"completionTokens"],"v":610}` + "\n"
	if err := os.WriteFile(filepath.Join(sessionDir, "real-session.jsonl"), []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	seedMailbox(t, stateDir, "real-session", mailboxJSON(1, 1, 0))
	repo := t.TempDir()

	// Whole-file sum semantics are untouched: two calls report the same numbers.
	for i := 0; i < 2; i++ {
		res := collectCopilot(t, repo, "real-session")
		if res.InputTokens != 6027 || res.OutputTokens != 610 {
			t.Errorf("call %d = %+v, want the chat-session numbers 6027/610", i+1, res)
		}
	}
	if _, err := os.Stat(cursorFile(repo, "real-session")); !os.IsNotExist(err) {
		t.Errorf("source 1 must not create a cursor (err=%v)", err)
	}
}
