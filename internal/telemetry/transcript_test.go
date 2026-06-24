package telemetry

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTranscript(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

const claudeTranscript = `{"type":"user","message":{"role":"user","content":"hello"}}
{"type":"assistant","message":{"model":"claude-sonnet-4-6","usage":{"input_tokens":1000,"output_tokens":200,"cache_creation_input_tokens":100,"cache_read_input_tokens":50}}}
{"type":"user","message":{"role":"user","content":"follow up"}}
{"type":"assistant","message":{"model":"claude-sonnet-4-6","usage":{"input_tokens":500,"output_tokens":100,"cache_read_input_tokens":250}}}
`

func TestParseTranscript_MultiTurn(t *testing.T) {
	dir := t.TempDir()
	path := writeTranscript(t, dir, "session.jsonl", claudeTranscript)

	tu, err := ParseTranscript(path)
	if err != nil {
		t.Fatalf("ParseTranscript: %v", err)
	}
	if tu.InputTokens != 1500 {
		t.Errorf("InputTokens = %d, want 1500", tu.InputTokens)
	}
	if tu.OutputTokens != 300 {
		t.Errorf("OutputTokens = %d, want 300", tu.OutputTokens)
	}
	// cache = creation(100) + read(50+250) = 400
	if tu.CachedTokens != 400 {
		t.Errorf("CachedTokens = %d, want 400", tu.CachedTokens)
	}
	if tu.Model != "claude-sonnet-4-6" {
		t.Errorf("Model = %q, want claude-sonnet-4-6", tu.Model)
	}
}

func TestParseTranscript_MissingUsageFields(t *testing.T) {
	dir := t.TempDir()
	path := writeTranscript(t, dir, "session.jsonl",
		`{"type":"assistant","message":{"model":"claude-sonnet-4-6"}}
`)
	tu, err := ParseTranscript(path)
	if err != nil {
		t.Fatal(err)
	}
	if tu.InputTokens != 0 || tu.OutputTokens != 0 {
		t.Errorf("expected zero tokens for missing usage fields, got %+v", tu)
	}
}

func TestParseTranscript_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := writeTranscript(t, dir, "empty.jsonl", "")
	tu, err := ParseTranscript(path)
	if err != nil {
		t.Fatal(err)
	}
	if tu.InputTokens != 0 {
		t.Errorf("expected zero tokens for empty file, got %+v", tu)
	}
}

func TestParseTranscript_NotFound(t *testing.T) {
	tu, err := ParseTranscript("/nonexistent/path/transcript.jsonl")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	if tu.InputTokens != 0 {
		t.Errorf("expected zero tokens for missing file, got %+v", tu)
	}
}

func TestParseTranscript_EmptyPath(t *testing.T) {
	tu, err := ParseTranscript("")
	if err != nil {
		t.Fatal(err)
	}
	if tu.InputTokens != 0 {
		t.Errorf("expected zero tokens for empty path")
	}
}

const antigravityTranscript = `{"model":"gemini-2.5-pro","usageMetadata":{"promptTokenCount":500,"candidatesTokenCount":100,"cachedContentTokenCount":50,"totalTokenCount":650}}
{"model":"gemini-2.5-pro","usageMetadata":{"promptTokenCount":600,"candidatesTokenCount":120,"cachedContentTokenCount":0,"thoughtsTokenCount":80,"totalTokenCount":720}}
{"model":"","usageMetadata":{"promptTokenCount":400,"candidatesTokenCount":90}}
`

func TestParseAntigravityTranscript_MultiTurn(t *testing.T) {
	dir := t.TempDir()
	path := writeTranscript(t, dir, "transcript.jsonl", antigravityTranscript)

	tu, model, err := ParseAntigravityTranscript(path)
	if err != nil {
		t.Fatalf("ParseAntigravityTranscript: %v", err)
	}
	// prompt: 500+600+400 = 1500
	if tu.InputTokens != 1500 {
		t.Errorf("InputTokens = %d, want 1500", tu.InputTokens)
	}
	// candidates: 100+120+90 = 310
	if tu.OutputTokens != 310 {
		t.Errorf("OutputTokens = %d, want 310", tu.OutputTokens)
	}
	// cached: 50+0+0 = 50
	if tu.CachedTokens != 50 {
		t.Errorf("CachedTokens = %d, want 50", tu.CachedTokens)
	}
	// last non-empty model is gemini-2.5-pro (line 2)
	if model != "gemini-2.5-pro" {
		t.Errorf("model = %q, want gemini-2.5-pro", model)
	}
}

func TestParseAntigravityTranscript_NotFound(t *testing.T) {
	tu, model, err := ParseAntigravityTranscript("/nonexistent/path/transcript.jsonl")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	if tu.InputTokens != 0 || model != "" {
		t.Errorf("expected zero result for missing file, got %+v %q", tu, model)
	}
}

func TestParseAntigravityTranscript_MissingUsageMetadata(t *testing.T) {
	dir := t.TempDir()
	path := writeTranscript(t, dir, "t.jsonl", `{"model":"gemini-2.5-pro"}
`)
	tu, model, err := ParseAntigravityTranscript(path)
	if err != nil {
		t.Fatal(err)
	}
	if tu.InputTokens != 0 {
		t.Errorf("expected zero tokens for missing usageMetadata, got %+v", tu)
	}
	if model != "gemini-2.5-pro" {
		t.Errorf("model = %q, want gemini-2.5-pro", model)
	}
}

func TestParseTranscript_UnreadableFile(t *testing.T) {
	protected := filepath.Join(t.TempDir(), "no-read.jsonl")
	os.WriteFile(protected, []byte(`line`), 0o000)
	_, err := ParseTranscript(protected)
	if err == nil {
		t.Error("expected error for unreadable file")
	}
}

func TestParseAntigravityTranscript_UnreadableFile(t *testing.T) {
	protected := filepath.Join(t.TempDir(), "no-read.jsonl")
	os.WriteFile(protected, []byte(`line`), 0o000)
	_, _, err := ParseAntigravityTranscript(protected)
	if err == nil {
		t.Error("expected error for unreadable file")
	}
}
