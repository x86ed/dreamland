package tools

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dreamland/internal/config"
	"dreamland/internal/telemetry"
)

// stdinErrReader is an io.Reader that always returns an error, used to test stdin failure paths.
type stdinErrReader struct{}

func (stdinErrReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

var testCfg = &config.Config{ModelID: "default-model"}

func writeTempTranscript(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "transcript-*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return f.Name()
}

const sampleClaudeTranscript = `{"type":"assistant","message":{"model":"claude-sonnet-4-6","usage":{"input_tokens":1000,"output_tokens":200,"cache_read_input_tokens":50}}}
`

// --- Claude Code ---

func TestClaudeCollector_Normal(t *testing.T) {
	path := writeTempTranscript(t, sampleClaudeTranscript)
	stdin := strings.NewReader(`{"transcript_path":"` + path + `","effort":{"level":"high"}}`)
	c := &ClaudeCollector{}
	res, err := c.Collect(stdin, testCfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Tool != "claude-code" {
		t.Errorf("Tool = %q, want claude-code", res.Tool)
	}
	if res.ThinkingEffort != "high" {
		t.Errorf("ThinkingEffort = %q, want high", res.ThinkingEffort)
	}
	if res.InputTokens != 1000 {
		t.Errorf("InputTokens = %d, want 1000", res.InputTokens)
	}
	if res.Model != "claude-sonnet-4-6" {
		t.Errorf("Model = %q, want claude-sonnet-4-6", res.Model)
	}
}

func TestClaudeCollector_EmptyStdin(t *testing.T) {
	res, err := (&ClaudeCollector{}).Collect(strings.NewReader(""), testCfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Tool != "claude-code" {
		t.Errorf("Tool = %q, want claude-code", res.Tool)
	}
	if res.Model != testCfg.ModelID {
		t.Errorf("Model = %q, want %q (fallback)", res.Model, testCfg.ModelID)
	}
}

func TestClaudeCollector_MalformedJSON(t *testing.T) {
	res, err := (&ClaudeCollector{}).Collect(strings.NewReader("{not json}"), testCfg)
	if err != nil {
		t.Fatal(err)
	}
	if res == nil {
		t.Fatal("expected non-nil result even for malformed JSON")
	}
}

// --- Codex ---

func TestCodexCollector_Normal(t *testing.T) {
	path := writeTempTranscript(t, sampleClaudeTranscript)
	stdin := strings.NewReader(`{"model":"o4-mini","transcript_path":"` + path + `"}`)
	res, err := (&CodexCollector{}).Collect(stdin, testCfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Model != "o4-mini" {
		t.Errorf("Model = %q, want o4-mini", res.Model)
	}
	if res.InputTokens != 1000 {
		t.Errorf("InputTokens = %d, want 1000", res.InputTokens)
	}
}

func TestCodexCollector_TranscriptParseError(t *testing.T) {
	// Unreadable transcript — Codex falls through with zero tokens.
	protected := filepath.Join(t.TempDir(), "no-read.jsonl")
	os.WriteFile(protected, []byte(`line`), 0o000)
	stdin := strings.NewReader(`{"model":"o4-mini","transcript_path":"` + filepath.ToSlash(protected) + `"}`)
	res, err := (&CodexCollector{}).Collect(stdin, testCfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.InputTokens != 0 {
		t.Errorf("expected zero tokens on transcript error, got %d", res.InputTokens)
	}
	if res.Model != "o4-mini" {
		t.Errorf("Model = %q, want o4-mini (from stdin, despite parse error)", res.Model)
	}
}

func TestCodexCollector_EmptyStdin(t *testing.T) {
	res, err := (&CodexCollector{}).Collect(strings.NewReader(""), testCfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Model != testCfg.ModelID {
		t.Errorf("Model fallback = %q, want %q", res.Model, testCfg.ModelID)
	}
}

// --- Cursor ---

func TestCursorCollector_Normal(t *testing.T) {
	path := writeTempTranscript(t, sampleClaudeTranscript)
	pathJSON := `"` + filepath.ToSlash(path) + `"`
	stdin := strings.NewReader(`{"model":"claude-sonnet-4-5","transcript_path":` + pathJSON + `}`)
	res, err := (&CursorCollector{}).Collect(stdin, testCfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Model != "claude-sonnet-4-5" {
		t.Errorf("Model = %q, want claude-sonnet-4-5", res.Model)
	}
}

func TestCursorCollector_NullTranscriptFallbackToEnv(t *testing.T) {
	path := writeTempTranscript(t, sampleClaudeTranscript)
	t.Setenv("CURSOR_TRANSCRIPT_PATH", path)
	stdin := strings.NewReader(`{"model":"gpt-4o","transcript_path":null}`)
	res, err := (&CursorCollector{}).Collect(stdin, testCfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.InputTokens != 1000 {
		t.Errorf("InputTokens = %d, want 1000 (from env path)", res.InputTokens)
	}
}

// --- Kiro ---

func TestKiroCollector_StopStub(t *testing.T) {
	res, err := (&KiroCollector{}).Collect(strings.NewReader(`{"session_id":"abc"}`), testCfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Tool != "kiro" {
		t.Errorf("Tool = %q, want kiro", res.Tool)
	}
	if res.Model != testCfg.ModelID {
		t.Errorf("Model = %q, want %q", res.Model, testCfg.ModelID)
	}
}

func TestKiroCollector_StartPhase(t *testing.T) {
	c := &KiroCollector{Phase: "start"}
	res, err := c.Collect(strings.NewReader(`{}`), testCfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Tool != "kiro" {
		t.Errorf("Tool = %q, want kiro", res.Tool)
	}
}

// --- Antigravity ---

const antigravityPayload = `{"transcriptPath":"%s","conversationId":"conv-1","stepIdx":3}`

func TestAntigravityCollector_Normal(t *testing.T) {
	path := writeTempTranscript(t, `{"model":"gemini-2.5-pro","usageMetadata":{"promptTokenCount":500,"candidatesTokenCount":100,"cachedContentTokenCount":20}}
`)
	stdin := strings.NewReader(`{"transcriptPath":"` + filepath.ToSlash(path) + `","conversationId":"conv-1"}`)
	res, err := (&AntigravityCollector{}).Collect(stdin, testCfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Model != "gemini-2.5-pro" {
		t.Errorf("Model = %q, want gemini-2.5-pro", res.Model)
	}
	if res.InputTokens != 500 {
		t.Errorf("InputTokens = %d, want 500", res.InputTokens)
	}
}

func TestAntigravityCollector_EmptyStdin(t *testing.T) {
	res, err := (&AntigravityCollector{}).Collect(strings.NewReader(""), testCfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Model != testCfg.ModelID {
		t.Errorf("Model fallback = %q, want %q", res.Model, testCfg.ModelID)
	}
}

// --- Copilot ---

func TestCopilotCollector_Normal(t *testing.T) {
	path := writeTempTranscript(t, sampleClaudeTranscript)
	stdin := strings.NewReader(`{"sessionId":"s1","transcriptPath":"` + filepath.ToSlash(path) + `","stopReason":"end_turn"}`)
	res, err := (&CopilotCollector{}).Collect(stdin, testCfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Tool != "github-copilot" {
		t.Errorf("Tool = %q, want github-copilot", res.Tool)
	}
}

func TestCopilotCollector_MissingTranscript(t *testing.T) {
	stdin := strings.NewReader(`{"sessionId":"s1","transcriptPath":"/nonexistent/path.jsonl","stopReason":"end_turn"}`)
	res, err := (&CopilotCollector{}).Collect(stdin, testCfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.InputTokens != 0 {
		t.Errorf("expected zero tokens for missing transcript, got %d", res.InputTokens)
	}
	if res.Model != testCfg.ModelID {
		t.Errorf("Model fallback = %q, want %q", res.Model, testCfg.ModelID)
	}
}

func TestClaudeCollector_StdinError(t *testing.T) {
	_, err := (&ClaudeCollector{}).Collect(stdinErrReader{}, nil)
	if err == nil {
		t.Error("expected error when stdin fails")
	}
}

func TestCodexCollector_StdinError(t *testing.T) {
	_, err := (&CodexCollector{}).Collect(stdinErrReader{}, nil)
	if err == nil {
		t.Error("expected error when stdin fails")
	}
}

func TestAntigravityCollector_StdinError(t *testing.T) {
	_, err := (&AntigravityCollector{}).Collect(stdinErrReader{}, nil)
	if err == nil {
		t.Error("expected error when stdin fails")
	}
}

func TestCopilotCollector_StdinError(t *testing.T) {
	_, err := (&CopilotCollector{}).Collect(stdinErrReader{}, nil)
	if err == nil {
		t.Error("expected error when stdin fails")
	}
}

func TestAntigravityCollector_TranscriptError(t *testing.T) {
	// Unreadable transcript — Collect should log a warning but still return a result.
	protected := filepath.Join(t.TempDir(), "unread.jsonl")
	if err := os.WriteFile(protected, []byte("line\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(protected, 0o644) })
	stdin := strings.NewReader(`{"transcriptPath":"` + filepath.ToSlash(protected) + `","conversationId":"c"}`)
	res, err := (&AntigravityCollector{}).Collect(stdin, testCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil result even on transcript error")
	}
}

func TestCopilotCollector_TranscriptError(t *testing.T) {
	// Unreadable transcript — Collect should log a warning but still return a result.
	protected := filepath.Join(t.TempDir(), "unread.jsonl")
	if err := os.WriteFile(protected, []byte("line\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(protected, 0o644) })
	stdin := strings.NewReader(`{"sessionId":"s1","transcriptPath":"` + filepath.ToSlash(protected) + `","stopReason":"end_turn"}`)
	res, err := (&CopilotCollector{}).Collect(stdin, testCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil result even on transcript error")
	}
}

// --- Cursor (additional branches) ---

func TestCursorCollector_ModelIDField(t *testing.T) {
	// model is empty, model_id is set — should use model_id.
	stdin := strings.NewReader(`{"model":"","model_id":"gpt-4o"}`)
	res, err := (&CursorCollector{}).Collect(stdin, testCfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Model != "gpt-4o" {
		t.Errorf("Model = %q, want gpt-4o from model_id", res.Model)
	}
}

func TestCursorCollector_ModelFallbackToCfg(t *testing.T) {
	// Both model and model_id empty — should fall back to cfg.ModelID.
	stdin := strings.NewReader(`{"model":"","model_id":""}`)
	res, err := (&CursorCollector{}).Collect(stdin, testCfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Model != testCfg.ModelID {
		t.Errorf("Model = %q, want %q (cfg fallback)", res.Model, testCfg.ModelID)
	}
}

func TestCursorCollector_TranscriptParseError(t *testing.T) {
	// Pass a read-protected file — ParseTranscript should error; Collect falls through.
	protected := filepath.Join(t.TempDir(), "no-read.jsonl")
	os.WriteFile(protected, []byte(`line`), 0o000)
	stdin := strings.NewReader(`{"model":"gpt-4o","transcript_path":"` + filepath.ToSlash(protected) + `"}`)
	res, err := (&CursorCollector{}).Collect(stdin, testCfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.InputTokens != 0 {
		t.Errorf("expected zero tokens on transcript error, got %d", res.InputTokens)
	}
}

// --- Kiro (stop phase with log group configured) ---

func TestKiroCollector_StopWithLogGroupFallback(t *testing.T) {
	// BedrockLogGroup is set but aws is absent — should fall back gracefully.
	t.Setenv("PATH", t.TempDir())
	cfg := &config.Config{ModelID: "claude-sonnet", BedrockLogGroup: "my-log-group"}
	res, err := (&KiroCollector{}).Collect(strings.NewReader(`{}`), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Tool != "kiro" {
		t.Errorf("Tool = %q, want kiro", res.Tool)
	}
	if res.Model != "claude-sonnet" {
		t.Errorf("Model fallback = %q, want claude-sonnet", res.Model)
	}
}

// --- normalizeBedrockModelID ---

func TestNormalizeBedrockModelID(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"anthropic.claude-sonnet-4-6-20251001", "claude-sonnet-4-6"},
		{"amazon.nova-pro-v1:0", "nova-pro"},
		{"meta.llama3-70b-instruct-v1:0", "llama3-70b-instruct"},
		{"", ""},
		{"claude-sonnet-4-6", "claude-sonnet-4-6"},
		// 8-char suffix with non-digits — should NOT strip
		{"provider.model-abcdefgh", "model-abcdefgh"},
	}
	for _, tt := range tests {
		got := normalizeBedrockModelID(tt.in)
		if got != tt.want {
			t.Errorf("normalizeBedrockModelID(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// --- queryBedrockLogs (via fake aws binary in PATH) ---

func fakeAwsBin(t *testing.T, output string) string {
	t.Helper()
	binDir := t.TempDir()
	script := "#!/bin/sh\nprintf '%s'" + " '" + output + "'\n"
	path := filepath.Join(binDir, "aws")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return binDir
}

func TestQueryBedrockLogs_Success(t *testing.T) {
	payload := `["{\"modelId\":\"anthropic.claude-sonnet-4-6-20251001\",\"input\":{\"inputTokenCount\":1000},\"output\":{\"outputTokenCount\":200}}"]`
	binDir := fakeAwsBin(t, payload)
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))

	result := queryBedrockLogs("test-log-group", "fallback-model")
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.InputTokens != 1000 {
		t.Errorf("InputTokens = %d, want 1000", result.InputTokens)
	}
	if result.OutputTokens != 200 {
		t.Errorf("OutputTokens = %d, want 200", result.OutputTokens)
	}
	if result.Model != "claude-sonnet-4-6" {
		t.Errorf("Model = %q, want claude-sonnet-4-6", result.Model)
	}
}

func TestQueryBedrockLogs_MultipleEvents(t *testing.T) {
	payload := `["{\"modelId\":\"anthropic.claude-sonnet-4-6-20251001\",\"input\":{\"inputTokenCount\":500},\"output\":{\"outputTokenCount\":100}}","{\"modelId\":\"anthropic.claude-sonnet-4-6-20251001\",\"input\":{\"inputTokenCount\":300},\"output\":{\"outputTokenCount\":80}}"]`
	binDir := fakeAwsBin(t, payload)
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))

	result := queryBedrockLogs("test-log-group", "fallback")
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.InputTokens != 800 {
		t.Errorf("InputTokens = %d, want 800", result.InputTokens)
	}
}

func TestQueryBedrockLogs_EmptyEvents(t *testing.T) {
	binDir := fakeAwsBin(t, "[]")
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))

	result := queryBedrockLogs("test-log-group", "fallback-model")
	if result == nil {
		t.Fatal("expected non-nil result for empty events (fallback model)")
	}
	if result.Model != "fallback-model" {
		t.Errorf("Model = %q, want fallback-model", result.Model)
	}
	if result.InputTokens != 0 {
		t.Errorf("expected zero tokens for empty events")
	}
}

func TestQueryBedrockLogs_AwsNotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // empty dir — no aws binary
	result := queryBedrockLogs("test-log-group", "fallback-model")
	if result != nil {
		t.Errorf("expected nil result when aws not found, got %+v", result)
	}
}

func TestQueryBedrockLogs_InvalidJSON(t *testing.T) {
	binDir := fakeAwsBin(t, "not-json")
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))
	result := queryBedrockLogs("test-log-group", "fallback-model")
	if result != nil {
		t.Errorf("expected nil result for invalid JSON output, got %+v", result)
	}
}

// Ensure telemetry package types are usable.
var _ telemetry.Collector = &ClaudeCollector{}
var _ telemetry.Collector = &CodexCollector{}
var _ telemetry.Collector = &CursorCollector{}
var _ telemetry.Collector = &KiroCollector{}
var _ telemetry.Collector = &AntigravityCollector{}
var _ telemetry.Collector = &CopilotCollector{}
