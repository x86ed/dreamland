package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func writeChatSessionFixture(t *testing.T, lines []string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "session-*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range lines {
		if _, err := f.WriteString(line + "\n"); err != nil {
			t.Fatal(err)
		}
	}
	f.Close()
	return f.Name()
}

func TestParseChatSessionTokens_DirectPatches(t *testing.T) {
	path := writeChatSessionFixture(t, []string{
		`{"kind":1,"k":["requests",0,"promptTokens"],"v":100}`,
		`{"kind":1,"k":["requests",0,"completionTokens"],"v":50}`,
	})
	prompt, completion, err := parseChatSessionTokens(path)
	if err != nil {
		t.Fatal(err)
	}
	if prompt != 100 || completion != 50 {
		t.Errorf("got prompt=%d completion=%d, want 100/50", prompt, completion)
	}
}

func TestParseChatSessionTokens_ResultMetadata(t *testing.T) {
	path := writeChatSessionFixture(t, []string{
		`{"kind":1,"k":["requests",3,"result"],"v":{"metadata":{"promptTokens":6027,"outputTokens":610}}}`,
	})
	prompt, completion, err := parseChatSessionTokens(path)
	if err != nil {
		t.Fatal(err)
	}
	if prompt != 6027 || completion != 610 {
		t.Errorf("got prompt=%d completion=%d, want 6027/610", prompt, completion)
	}
}

func TestParseChatSessionTokens_TakesLatestRequest(t *testing.T) {
	path := writeChatSessionFixture(t, []string{
		`{"kind":1,"k":["requests",0,"promptTokens"],"v":100}`,
		`{"kind":1,"k":["requests",0,"completionTokens"],"v":50}`,
		`{"kind":1,"k":["requests",5,"promptTokens"],"v":9999}`,
		`{"kind":1,"k":["requests",5,"completionTokens"],"v":8888}`,
		`{"kind":1,"k":["requests",2,"promptTokens"],"v":1}`,
		`{"kind":1,"k":["requests",2,"completionTokens"],"v":1}`,
	})
	prompt, completion, err := parseChatSessionTokens(path)
	if err != nil {
		t.Fatal(err)
	}
	if prompt != 9999 || completion != 8888 {
		t.Errorf("got prompt=%d completion=%d, want the highest-index request (9999/8888)", prompt, completion)
	}
}

func TestParseChatSessionTokens_IgnoresUnrelatedLines(t *testing.T) {
	path := writeChatSessionFixture(t, []string{
		`{"kind":2,"k":["requests"],"v":[]}`,
		`not even json`,
		`{"kind":1,"k":["someOtherField"],"v":"x"}`,
		`{"kind":1,"k":["requests",0,"someUnrelatedField"],"v":"y"}`,
	})
	prompt, completion, err := parseChatSessionTokens(path)
	if err != nil {
		t.Fatal(err)
	}
	if prompt != 0 || completion != 0 {
		t.Errorf("got prompt=%d completion=%d, want 0/0 for a file with no usage data", prompt, completion)
	}
}

func TestParseChatSessionTokens_FileNotFound(t *testing.T) {
	_, _, err := parseChatSessionTokens("/nonexistent/session.jsonl")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestDecodeJSONInt_InvalidValue(t *testing.T) {
	if _, ok := decodeJSONInt([]byte(`"not a number"`)); ok {
		t.Error("expected ok=false for a non-numeric JSON value")
	}
}

func TestDecodeJSONInt_ValidValue(t *testing.T) {
	n, ok := decodeJSONInt([]byte(`42`))
	if !ok || n != 42 {
		t.Errorf("got (%d, %v), want (42, true)", n, ok)
	}
}

func TestFindChatSessionFile_EmptySessionID(t *testing.T) {
	_, err := findChatSessionFile("")
	if err == nil {
		t.Fatal("expected error for empty session ID")
	}
}

func TestFindChatSessionFile_Found(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	roots := vscodeWorkspaceStorageRoots()
	if len(roots) == 0 {
		t.Skip("no workspace storage roots resolved for this OS")
	}
	sessionDir := filepath.Join(roots[0], "some-workspace-hash", "chatSessions")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sessionFile := filepath.Join(sessionDir, "abc-123.jsonl")
	if err := os.WriteFile(sessionFile, []byte(`{"kind":1,"k":["requests",0,"promptTokens"],"v":42}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := findChatSessionFile("abc-123")
	if err != nil {
		t.Fatal(err)
	}
	if got != sessionFile {
		t.Errorf("got %q, want %q", got, sessionFile)
	}
}

func TestFindChatSessionFile_BadGlobPatternSkipped(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	roots := vscodeWorkspaceStorageRoots()
	if len(roots) == 0 {
		t.Skip("no workspace storage roots resolved for this OS")
	}
	sessionDir := filepath.Join(roots[0], "some-workspace-hash", "chatSessions")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// filepath.Glob only surfaces ErrBadPattern once it actually matches entries
	// against the malformed pattern, so a directory entry must exist here.
	if err := os.WriteFile(filepath.Join(sessionDir, "dummy.jsonl"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	// An unterminated "[" makes the glob pattern invalid (filepath.ErrBadPattern),
	// exercising the continue-on-error branch across every storage root.
	if _, err := findChatSessionFile("["); err == nil {
		t.Fatal("expected error for a session ID producing an invalid glob pattern")
	}
}

func TestFindChatSessionFile_NotFound(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	_, err := findChatSessionFile("nonexistent-session-id")
	if err == nil {
		t.Fatal("expected error when no matching session file exists")
	}
}
