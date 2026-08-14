package otelreceiver

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

func strAttr(key, val string) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: key, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: val}}}
}

func intAttr(key string, val int64) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: key, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: val}}}
}

func sampleExportRequest(conversationID string, input, output int64) *coltracepb.ExportTraceServiceRequest {
	span := &tracepb.Span{
		Name: "invoke_agent",
		Attributes: []*commonpb.KeyValue{
			strAttr("gen_ai.conversation.id", conversationID),
			strAttr("gen_ai.response.model", "gpt-4o"),
			intAttr("gen_ai.usage.input_tokens", input),
			intAttr("gen_ai.usage.output_tokens", output),
			intAttr("gen_ai.usage.cache_read.input_tokens", 5),
		},
	}
	return &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{},
				ScopeSpans: []*tracepb.ScopeSpans{
					{Spans: []*tracepb.Span{span}},
				},
			},
		},
	}
}

func TestHandler_ProtobufTraceExport_WritesSessionUsage(t *testing.T) {
	root := t.TempDir()
	req := sampleExportRequest("sess-123", 1000, 200)
	body, err := proto.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(Handler(root))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/v1/traces", "application/x-protobuf", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	usage, err := ReadSessionUsage(root, "sess-123")
	if err != nil {
		t.Fatal(err)
	}
	if usage == nil {
		t.Fatal("expected session usage to be written")
	}
	if usage.InputTokens != 1000 || usage.OutputTokens != 200 || usage.CachedTokens != 5 {
		t.Errorf("got %+v, want input=1000 output=200 cached=5", usage)
	}
	if usage.Model != "gpt-4o" {
		t.Errorf("Model = %q, want gpt-4o", usage.Model)
	}
}

func TestHandler_JSONTraceExport_WritesSessionUsage(t *testing.T) {
	root := t.TempDir()
	req := sampleExportRequest("sess-json", 50, 10)
	body, err := protojsonMarshal(req)
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(Handler(root))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/v1/traces", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	usage, err := ReadSessionUsage(root, "sess-json")
	if err != nil {
		t.Fatal(err)
	}
	if usage == nil || usage.InputTokens != 50 {
		t.Fatalf("got %+v, want input=50", usage)
	}
}

func TestHandler_LogsEveryRequest(t *testing.T) {
	root := t.TempDir()
	srv := httptest.NewServer(Handler(root))
	defer srv.Close()

	// A successful traces export.
	req := sampleExportRequest("sess-log", 10, 5)
	body, _ := proto.Marshal(req)
	resp, err := http.Post(srv.URL+"/v1/traces", "application/x-protobuf", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// An unhandled path (e.g. Copilot exporting metrics/logs instead of traces).
	resp2, err := http.Post(srv.URL+"/v1/metrics", "application/x-protobuf", bytes.NewReader([]byte("x")))
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()

	data, err := os.ReadFile(filepath.Join(root, ".dreamland", "otel-receiver.log"))
	if err != nil {
		t.Fatalf("log file not written: %v", err)
	}
	log := string(data)
	if !strings.Contains(log, "/v1/traces") {
		t.Errorf("log missing /v1/traces entry, got:\n%s", log)
	}
	if !strings.Contains(log, "sess-log") {
		t.Errorf("log missing conversation.id, got:\n%s", log)
	}
	if !strings.Contains(log, "/v1/metrics") {
		t.Errorf("log missing unhandled-path entry, got:\n%s", log)
	}
}

func TestHandler_SpanWithoutConversationID_Skipped(t *testing.T) {
	root := t.TempDir()
	span := &tracepb.Span{
		Name: "invoke_agent",
		Attributes: []*commonpb.KeyValue{
			intAttr("gen_ai.usage.input_tokens", 100),
		},
	}
	req := &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{ScopeSpans: []*tracepb.ScopeSpans{{Spans: []*tracepb.Span{span}}}},
		},
	}
	body, _ := proto.Marshal(req)

	srv := httptest.NewServer(Handler(root))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/v1/traces", "application/x-protobuf", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if _, err := os.Stat(filepath.Join(root, ".dreamland", "otel-sessions")); !os.IsNotExist(err) {
		t.Error("expected no otel-sessions dir when no conversation.id present")
	}
}

func TestReadSessionUsage_NotFound(t *testing.T) {
	usage, err := ReadSessionUsage(t.TempDir(), "nonexistent")
	if err != nil {
		t.Fatalf("expected nil error for missing session, got: %v", err)
	}
	if usage != nil {
		t.Errorf("expected nil usage, got %+v", usage)
	}
}

func TestReadSessionUsage_EmptyConversationID(t *testing.T) {
	usage, err := ReadSessionUsage(t.TempDir(), "")
	if err != nil || usage != nil {
		t.Errorf("expected (nil, nil) for empty conversationID, got (%+v, %v)", usage, err)
	}
}

func TestHandler_InvalidProtobuf_Returns400(t *testing.T) {
	srv := httptest.NewServer(Handler(t.TempDir()))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/v1/traces", "application/x-protobuf", bytes.NewReader([]byte("not protobuf")))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestHandler_JSONTraceExport_MalformedJSON_Returns400(t *testing.T) {
	srv := httptest.NewServer(Handler(t.TempDir()))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/v1/traces", "application/json", bytes.NewReader([]byte("{not json")))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestAttrInt_Float64AndDefault(t *testing.T) {
	m := map[string]any{
		"f": float64(42),
		"s": "not a number",
	}
	if got := attrInt(m, "f"); got != 42 {
		t.Errorf("float64 case: got %d, want 42", got)
	}
	if got := attrInt(m, "s"); got != 0 {
		t.Errorf("non-numeric case: got %d, want 0", got)
	}
	if got := attrInt(m, "missing"); got != 0 {
		t.Errorf("missing key: got %d, want 0", got)
	}
}

func TestAttrMap_DoubleBoolAndNilValues(t *testing.T) {
	kvs := []*commonpb.KeyValue{
		{Key: "d", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_DoubleValue{DoubleValue: 3.14}}},
		{Key: "b", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_BoolValue{BoolValue: true}}},
		nil,
		{Key: "novalue", Value: nil},
	}
	m := attrMap(kvs)
	if m["d"] != 3.14 {
		t.Errorf("double value: got %v, want 3.14", m["d"])
	}
	if m["b"] != true {
		t.Errorf("bool value: got %v, want true", m["b"])
	}
	if _, ok := m["novalue"]; ok {
		t.Error("expected key with nil AnyValue to be skipped")
	}
}

func TestProcessSpan_RequestModelFallback(t *testing.T) {
	root := t.TempDir()
	span := &tracepb.Span{
		Attributes: []*commonpb.KeyValue{
			strAttr("gen_ai.conversation.id", "sess-fallback"),
			strAttr("gen_ai.request.model", "gpt-fallback"),
			intAttr("gen_ai.usage.input_tokens", 10),
		},
	}
	processSpan(root, span)

	usage, err := ReadSessionUsage(root, "sess-fallback")
	if err != nil {
		t.Fatal(err)
	}
	if usage == nil || usage.Model != "gpt-fallback" {
		t.Fatalf("got %+v, want Model=gpt-fallback", usage)
	}
}

func TestProcessSpan_NoUsageData_Skipped(t *testing.T) {
	root := t.TempDir()
	span := &tracepb.Span{
		Attributes: []*commonpb.KeyValue{
			strAttr("gen_ai.conversation.id", "sess-nousage"),
		},
	}
	processSpan(root, span)

	usage, err := ReadSessionUsage(root, "sess-nousage")
	if err != nil {
		t.Fatal(err)
	}
	if usage != nil {
		t.Errorf("expected no usage written for a span with no token data, got %+v", usage)
	}
}

func TestWriteSessionUsage_MkdirFails(t *testing.T) {
	root := t.TempDir()
	// Create a regular file where the ".dreamland" directory needs to go, so
	// MkdirAll fails.
	blocker := filepath.Join(root, ".dreamland")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Must not panic; writeSessionUsage is best-effort and swallows the error.
	writeSessionUsage(root, "sess-blocked", SessionUsage{InputTokens: 1})

	if _, err := ReadSessionUsage(root, "sess-blocked"); err == nil {
		t.Error("expected ReadSessionUsage to report an error reading through a blocked directory")
	}
}

func TestWriteSessionUsage_WriteFileFails(t *testing.T) {
	root := t.TempDir()
	path := sessionPath(root, "sess-tmp-blocked")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	// Make the ".tmp" write target a directory so os.WriteFile fails.
	if err := os.MkdirAll(path+".tmp", 0o755); err != nil {
		t.Fatal(err)
	}

	writeSessionUsage(root, "sess-tmp-blocked", SessionUsage{InputTokens: 1})

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected no session file to be written when the tmp write fails")
	}
}

func TestLogRequest_MkdirFails(t *testing.T) {
	root := t.TempDir()
	blocker := filepath.Join(root, ".dreamland")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Must not panic; logRequest is best-effort and swallows the error.
	srv := httptest.NewServer(Handler(root))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/traces")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}

func TestLogRequest_OpenFileFails(t *testing.T) {
	root := t.TempDir()
	// Pre-create the log file's path as a directory, so MkdirAll(parent) succeeds
	// (already exists) but OpenFile on the log path itself fails (EISDIR).
	logPath := filepath.Join(root, ".dreamland", "otel-receiver.log")
	if err := os.MkdirAll(logPath, 0o755); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/traces", nil)
	// Must not panic; logRequest is best-effort and swallows the error.
	logRequest(root, req, "note", 0, 0)
}

func TestReadSessionUsage_UnmarshalError(t *testing.T) {
	root := t.TempDir()
	path := sessionPath(root, "sess-corrupt")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := ReadSessionUsage(root, "sess-corrupt"); err == nil {
		t.Error("expected error unmarshalling corrupt session file")
	}
}

func TestHandler_GetMethod_NotAllowed(t *testing.T) {
	srv := httptest.NewServer(Handler(t.TempDir()))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/traces")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", resp.StatusCode)
	}
}
