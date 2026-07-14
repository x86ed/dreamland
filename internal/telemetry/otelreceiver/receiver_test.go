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
