package otelreceiver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

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

// agentSpan builds an invoke_agent span carrying usage for one conversation.
func agentSpan(conversationID, spanID string, input, output, cached int64, model string) *tracepb.Span {
	attrs := []*commonpb.KeyValue{
		strAttr("gen_ai.operation.name", "invoke_agent"),
		strAttr("gen_ai.conversation.id", conversationID),
		intAttr("gen_ai.usage.input_tokens", input),
		intAttr("gen_ai.usage.output_tokens", output),
		intAttr("gen_ai.usage.cache_read.input_tokens", cached),
	}
	if model != "" {
		attrs = append(attrs, strAttr("gen_ai.response.model", model))
	}
	return &tracepb.Span{Name: "invoke_agent", SpanId: []byte(spanID), Attributes: attrs}
}

func exportRequest(spans ...*tracepb.Span) *coltracepb.ExportTraceServiceRequest {
	return &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{{
			Resource:   &resourcepb.Resource{},
			ScopeSpans: []*tracepb.ScopeSpans{{Spans: spans}},
		}},
	}
}

func sampleExportRequest(conversationID string, input, output int64) *coltracepb.ExportTraceServiceRequest {
	return exportRequest(agentSpan(conversationID, "span-1", input, output, 5, "gpt-4o"))
}

func postExport(t testing.TB, url string, req *coltracepb.ExportTraceServiceRequest) *http.Response {
	t.Helper()
	body, err := proto.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Post(url+"/v1/traces", "application/x-protobuf", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return resp
}

func mustUsage(t *testing.T, stateDir, cid string) *SessionUsage {
	t.Helper()
	u, err := ReadSessionUsage(stateDir, cid)
	if err != nil {
		t.Fatalf("ReadSessionUsage(%q): %v", cid, err)
	}
	if u == nil {
		t.Fatalf("no mailbox for %q", cid)
	}
	return u
}

func listFiles(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(root, p)
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	return out
}

func TestReceiverRevisionIsTwo(t *testing.T) {
	if ReceiverRevision != 2 {
		t.Errorf("ReceiverRevision = %d, want 2 (pre-handshake receivers are revision 1)", ReceiverRevision)
	}
}

func TestHandler_ProtobufTraceExport_WritesSessionUsage(t *testing.T) {
	stateDir := t.TempDir()
	srv := httptest.NewServer(Handler(stateDir))
	defer srv.Close()

	resp := postExport(t, srv.URL, sampleExportRequest("sess-123", 1000, 200))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	usage := mustUsage(t, stateDir, "sess-123")
	if usage.InputTokens != 1000 || usage.OutputTokens != 200 || usage.CachedTokens != 5 {
		t.Errorf("got %+v, want input=1000 output=200 cached=5", usage)
	}
	if usage.Model != "gpt-4o" {
		t.Errorf("Model = %q, want gpt-4o", usage.Model)
	}

	// The mailbox lives at <state-dir>/sessions/<cid>.json and carries the v2 fields.
	data, err := os.ReadFile(filepath.Join(stateDir, "sessions", "sess-123.json"))
	if err != nil {
		t.Fatalf("mailbox not at <state-dir>/sessions/<cid>.json: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if raw["version"] != float64(2) {
		t.Errorf("version = %v, want 2", raw["version"])
	}
	if raw["span_count"] != float64(1) {
		t.Errorf("span_count = %v, want 1", raw["span_count"])
	}
	for _, k := range []string{"model", "input_tokens", "output_tokens", "cached_tokens", "captured_at"} {
		if _, ok := raw[k]; !ok {
			t.Errorf("mailbox missing %q field: %s", k, data)
		}
	}
}

func TestHandler_JSONTraceExport_WritesSessionUsage(t *testing.T) {
	stateDir := t.TempDir()
	body, err := protojsonMarshal(sampleExportRequest("sess-json", 50, 10))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(Handler(stateDir))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/v1/traces", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if usage := mustUsage(t, stateDir, "sess-json"); usage.InputTokens != 50 {
		t.Fatalf("got %+v, want input=50", usage)
	}
}

func TestHandler_MultipleSpansForOneSessionAreSummed(t *testing.T) {
	stateDir := t.TempDir()
	srv := httptest.NewServer(Handler(stateDir))
	defer srv.Close()

	postExport(t, srv.URL, exportRequest(agentSpan("sess-sum", "s1", 100, 10, 0, "")))
	postExport(t, srv.URL, exportRequest(agentSpan("sess-sum", "s2", 40, 5, 0, "")))

	u := mustUsage(t, stateDir, "sess-sum")
	if u.InputTokens != 140 || u.OutputTokens != 15 || u.SpanCount != 2 {
		t.Errorf("got %+v, want input=140 output=15 span_count=2", u)
	}
	if u.Version != 2 {
		t.Errorf("Version = %d, want 2", u.Version)
	}
}

func TestHandler_SpansInOneRequestAreSummed(t *testing.T) {
	stateDir := t.TempDir()
	srv := httptest.NewServer(Handler(stateDir))
	defer srv.Close()

	postExport(t, srv.URL, exportRequest(
		agentSpan("sess-one-req", "s1", 100, 10, 1, ""),
		agentSpan("sess-one-req", "s2", 40, 5, 2, ""),
	))
	u := mustUsage(t, stateDir, "sess-one-req")
	if u.InputTokens != 140 || u.OutputTokens != 15 || u.CachedTokens != 3 || u.SpanCount != 2 {
		t.Errorf("got %+v, want 140/15/3 span_count=2", u)
	}
}

func TestHandler_RedeliveredSpanIsNotDoubleCounted(t *testing.T) {
	stateDir := t.TempDir()
	srv := httptest.NewServer(Handler(stateDir))
	defer srv.Close()

	req := exportRequest(agentSpan("sess-dup", "same-span", 100, 10, 0, ""))
	postExport(t, srv.URL, req)
	postExport(t, srv.URL, req)

	u := mustUsage(t, stateDir, "sess-dup")
	if u.InputTokens != 100 || u.OutputTokens != 10 || u.SpanCount != 1 {
		t.Errorf("got %+v, want a single delivery's totals (100/10, span_count=1)", u)
	}
}

func TestHandler_SpanIDDedupeIsPerConversation(t *testing.T) {
	stateDir := t.TempDir()
	srv := httptest.NewServer(Handler(stateDir))
	defer srv.Close()

	postExport(t, srv.URL, exportRequest(
		agentSpan("conv-a", "shared-id", 10, 1, 0, ""),
		agentSpan("conv-b", "shared-id", 20, 2, 0, ""),
	))
	if a := mustUsage(t, stateDir, "conv-a"); a.InputTokens != 10 {
		t.Errorf("conv-a = %+v, want input 10", a)
	}
	if b := mustUsage(t, stateDir, "conv-b"); b.InputTokens != 20 {
		t.Errorf("conv-b = %+v, want input 20 (same span id in another conversation is not a duplicate)", b)
	}
}

func TestHandler_SpanIDSetIsBoundedAndEvictsOldest(t *testing.T) {
	stateDir := t.TempDir()
	srv := httptest.NewServer(Handler(stateDir))
	defer srv.Close()

	// 4097 distinct span ids: the bounded per-conversation set (4096) has evicted the oldest.
	var spans []*tracepb.Span
	for i := 1; i <= 4097; i++ {
		spans = append(spans, agentSpan("sess-bound", fmt.Sprintf("id-%05d", i), 1, 0, 0, ""))
	}
	postExport(t, srv.URL, exportRequest(spans...))
	if u := mustUsage(t, stateDir, "sess-bound"); u.InputTokens != 4097 || u.SpanCount != 4097 {
		t.Fatalf("after 4097 unique spans: %+v, want 4097/4097", u)
	}

	// The newest id is still remembered, the oldest was evicted.
	postExport(t, srv.URL, exportRequest(agentSpan("sess-bound", "id-04097", 1, 0, 0, "")))
	if u := mustUsage(t, stateDir, "sess-bound"); u.InputTokens != 4097 {
		t.Errorf("replay of the newest span id was counted again: %+v", u)
	}
	postExport(t, srv.URL, exportRequest(agentSpan("sess-bound", "id-00001", 1, 0, 0, "")))
	if u := mustUsage(t, stateDir, "sess-bound"); u.InputTokens != 4098 {
		t.Errorf("replay of the evicted oldest span id should be counted: %+v, want input 4098", u)
	}
}

func TestHandler_NonAgentSpansAreNotCounted(t *testing.T) {
	usageAttrs := func(extra ...*commonpb.KeyValue) []*commonpb.KeyValue {
		return append(extra,
			strAttr("gen_ai.conversation.id", "sess-chat"),
			intAttr("gen_ai.usage.input_tokens", 100),
			intAttr("gen_ai.usage.output_tokens", 10))
	}
	tests := []struct {
		name string
		span *tracepb.Span
	}{
		{"operation chat", &tracepb.Span{Name: "chat gpt-4o", SpanId: []byte("c1"),
			Attributes: usageAttrs(strAttr("gen_ai.operation.name", "chat"))}},
		{"operation attr wins over invoke_agent-looking name", &tracepb.Span{Name: "invoke_agent x", SpanId: []byte("c2"),
			Attributes: usageAttrs(strAttr("gen_ai.operation.name", "chat"))}},
		{"no operation attr and non-invoke_agent name", &tracepb.Span{Name: "chat gpt-4o", SpanId: []byte("c3"),
			Attributes: usageAttrs()}},
		{"execute_tool", &tracepb.Span{Name: "execute_tool", SpanId: []byte("c4"),
			Attributes: usageAttrs(strAttr("gen_ai.operation.name", "execute_tool"))}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateDir := t.TempDir()
			srv := httptest.NewServer(Handler(stateDir))
			defer srv.Close()

			if resp := postExport(t, srv.URL, exportRequest(tt.span)); resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want 200", resp.StatusCode)
			}
			if u, err := ReadSessionUsage(stateDir, "sess-chat"); err != nil || u != nil {
				t.Errorf("non-agent span created/changed a mailbox: (%+v, %v)", u, err)
			}
			if _, err := os.Stat(filepath.Join(stateDir, "sessions", "sess-chat.json")); !os.IsNotExist(err) {
				t.Errorf("mailbox file exists for a non-agent span (err=%v)", err)
			}
		})
	}
}

func TestHandler_InvokeAgentNamePrefixCountsWhenOperationAttrAbsent(t *testing.T) {
	stateDir := t.TempDir()
	srv := httptest.NewServer(Handler(stateDir))
	defer srv.Close()

	span := &tracepb.Span{Name: "invoke_agent copilot", SpanId: []byte("p1"), Attributes: []*commonpb.KeyValue{
		strAttr("gen_ai.conversation.id", "sess-prefix"),
		intAttr("gen_ai.usage.input_tokens", 7),
	}}
	postExport(t, srv.URL, exportRequest(span))
	if u := mustUsage(t, stateDir, "sess-prefix"); u.InputTokens != 7 {
		t.Errorf("got %+v, want input=7", u)
	}
}

func TestHandler_ModelIsLatestNonEmpty(t *testing.T) {
	stateDir := t.TempDir()
	srv := httptest.NewServer(Handler(stateDir))
	defer srv.Close()

	postExport(t, srv.URL, exportRequest(agentSpan("sess-model", "m1", 1, 1, 0, "gpt-4o")))
	postExport(t, srv.URL, exportRequest(agentSpan("sess-model", "m2", 1, 1, 0, "")))
	if u := mustUsage(t, stateDir, "sess-model"); u.Model != "gpt-4o" {
		t.Errorf("Model = %q after a span with no model, want gpt-4o kept", u.Model)
	}
	postExport(t, srv.URL, exportRequest(agentSpan("sess-model", "m3", 1, 1, 0, "claude-sonnet")))
	if u := mustUsage(t, stateDir, "sess-model"); u.Model != "claude-sonnet" {
		t.Errorf("Model = %q, want the most recent non-empty model claude-sonnet", u.Model)
	}
}

func TestHandler_InterleavedConversationsStaySeparate(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	srv := httptest.NewServer(Handler(stateDir))
	defer srv.Close()

	postExport(t, srv.URL, exportRequest(agentSpan("A", "a1", 100, 10, 0, "")))
	postExport(t, srv.URL, exportRequest(agentSpan("B", "b1", 7, 1, 0, "")))
	postExport(t, srv.URL, exportRequest(agentSpan("A", "a2", 50, 5, 0, ""), agentSpan("B", "b2", 3, 1, 0, "")))

	a, b := mustUsage(t, stateDir, "A"), mustUsage(t, stateDir, "B")
	if a.InputTokens != 150 || a.OutputTokens != 15 || a.SpanCount != 2 {
		t.Errorf("A = %+v, want 150/15 span_count=2", a)
	}
	if b.InputTokens != 10 || b.OutputTokens != 2 || b.SpanCount != 2 {
		t.Errorf("B = %+v, want 10/2 span_count=2", b)
	}
	for _, f := range listFiles(t, root) {
		if f != "state/sessions/A.json" && f != "state/sessions/B.json" && f != "state/receiver.log" {
			t.Errorf("unexpected file %q outside the mailboxes and log", f)
		}
	}
}

func TestHandler_UnsafeConversationIDsWriteNothing(t *testing.T) {
	ids := []string{"../../evil", "", ".", "..", "a/b", `a\b`, "a:b", strings.Repeat("a", 129)}
	for _, id := range ids {
		name := id
		if name == "" {
			name = "empty"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			// Nest the state dir so a traversal escape would land inside root and be seen.
			stateDir := filepath.Join(root, "a", "state")
			if err := os.MkdirAll(stateDir, 0o755); err != nil {
				t.Fatal(err)
			}
			srv := httptest.NewServer(Handler(stateDir))
			defer srv.Close()

			resp := postExport(t, srv.URL, exportRequest(agentSpan(id, "u1", 100, 10, 0, "")))
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want a valid OTLP success response", resp.StatusCode)
			}
			for _, f := range listFiles(t, root) {
				if f != "a/state/receiver.log" {
					t.Errorf("unsafe id %q caused a file to be written: %s", id, f)
				}
			}
			if id != "" {
				logData, _ := os.ReadFile(filepath.Join(stateDir, "receiver.log"))
				if !strings.Contains(string(logData), "invalid conversation id") {
					t.Errorf("log should note 'invalid conversation id', got:\n%s", logData)
				}
			}
		})
	}
}

func TestHandler_HealthEndpoint(t *testing.T) {
	stateDir := t.TempDir()
	origBuild := Build
	Build = "abc1234"
	t.Cleanup(func() { Build = origBuild })

	srv := httptest.NewServer(Handler(stateDir))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/.dreamland/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var h struct {
		Service  string `json:"service"`
		Revision int    `json:"revision"`
		Build    string `json:"build"`
		PID      int    `json:"pid"`
		StateDir string `json:"state_dir"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&h); err != nil {
		t.Fatalf("health body is not the documented JSON: %v", err)
	}
	if h.Service != "dreamland-otel-receiver" {
		t.Errorf("service = %q, want dreamland-otel-receiver", h.Service)
	}
	if h.Revision != 2 {
		t.Errorf("revision = %d, want 2", h.Revision)
	}
	if h.Build != "abc1234" {
		t.Errorf("build = %q, want the injected Build abc1234", h.Build)
	}
	if h.PID != os.Getpid() {
		t.Errorf("pid = %d, want %d", h.PID, os.Getpid())
	}
	if h.StateDir != stateDir {
		t.Errorf("state_dir = %q, want %q", h.StateDir, stateDir)
	}
}

func TestHandler_HealthReportsUnknownBuildWhenUnset(t *testing.T) {
	origBuild := Build
	Build = ""
	t.Cleanup(func() { Build = origBuild })

	srv := httptest.NewServer(Handler(t.TempDir()))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/.dreamland/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var h map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&h); err != nil {
		t.Fatal(err)
	}
	if h["build"] != "unknown" {
		t.Errorf("build = %v, want \"unknown\" when Build is unset", h["build"])
	}
}

func TestHandler_HasNoShutdownEndpoint(t *testing.T) {
	srv := httptest.NewServer(Handler(t.TempDir()))
	defer srv.Close()
	for _, p := range []string{"/shutdown", "/.dreamland/shutdown", "/.dreamland/stop"} {
		resp, err := http.Post(srv.URL+p, "text/plain", nil)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		// The catch-all answers 200; the point is that the server is still up afterwards.
		if _, err := http.Get(srv.URL + "/.dreamland/health"); err != nil {
			t.Fatalf("receiver stopped serving after POST %s: %v", p, err)
		}
	}
}

func TestHandler_LogsEveryRequest(t *testing.T) {
	stateDir := t.TempDir()
	srv := httptest.NewServer(Handler(stateDir))
	defer srv.Close()

	postExport(t, srv.URL, sampleExportRequest("sess-log", 10, 5))
	resp2, err := http.Post(srv.URL+"/v1/metrics", "application/x-protobuf", bytes.NewReader([]byte("x")))
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()

	data, err := os.ReadFile(filepath.Join(stateDir, "receiver.log"))
	if err != nil {
		t.Fatalf("log file not written to <state-dir>/receiver.log: %v", err)
	}
	log := string(data)
	for _, want := range []string{"/v1/traces", "sess-log", "/v1/metrics"} {
		if !strings.Contains(log, want) {
			t.Errorf("log missing %q, got:\n%s", want, log)
		}
	}
}

func TestLogRotation_RotatesAtFiveMiB(t *testing.T) {
	stateDir := t.TempDir()
	logPath := filepath.Join(stateDir, "receiver.log")
	big := bytes.Repeat([]byte("x"), 5*1024*1024+1)
	if err := os.WriteFile(logPath, big, 0o644); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(Handler(stateDir))
	defer srv.Close()
	postExport(t, srv.URL, sampleExportRequest("sess-rot", 1, 1))

	rotated, err := os.ReadFile(logPath + ".1")
	if err != nil {
		t.Fatalf("expected receiver.log.1 after exceeding 5 MiB: %v", err)
	}
	if len(rotated) != len(big) {
		t.Errorf("receiver.log.1 has %d bytes, want the old log (%d)", len(rotated), len(big))
	}
	cur, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(cur) >= 1024 || !strings.Contains(string(cur), "/v1/traces") {
		t.Errorf("new receiver.log should be small and hold the new request; len=%d", len(cur))
	}
}

func TestLogRotation_NotBelowThreshold(t *testing.T) {
	stateDir := t.TempDir()
	logPath := filepath.Join(stateDir, "receiver.log")
	if err := os.WriteFile(logPath, bytes.Repeat([]byte("x"), 1024), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(Handler(stateDir))
	defer srv.Close()
	postExport(t, srv.URL, sampleExportRequest("sess-norot", 1, 1))
	if _, err := os.Stat(logPath + ".1"); !os.IsNotExist(err) {
		t.Errorf("log below 5 MiB must not rotate (err=%v)", err)
	}
}

func TestHandler_ConcurrentPostsAcrossConversationsAreExact(t *testing.T) {
	stateDir := t.TempDir()
	srv := httptest.NewServer(Handler(stateDir))
	defer srv.Close()

	const posts, convs = 50, 5
	wantIn := make(map[string]int64)
	var wg sync.WaitGroup
	for i := 0; i < posts; i++ {
		cid := fmt.Sprintf("conv-%d", i%convs)
		in := int64(i + 1)
		wantIn[cid] += in
		wg.Add(1)
		go func(i int, cid string, in int64) {
			defer wg.Done()
			body, _ := proto.Marshal(exportRequest(agentSpan(cid, fmt.Sprintf("span-%d", i), in, 1, 0, "")))
			resp, err := http.Post(srv.URL+"/v1/traces", "application/x-protobuf", bytes.NewReader(body))
			if err != nil {
				t.Errorf("post %d: %v", i, err)
				return
			}
			resp.Body.Close()
		}(i, cid, in)
	}
	wg.Wait()

	for cid, want := range wantIn {
		u := mustUsage(t, stateDir, cid)
		if u.InputTokens != want || u.OutputTokens != posts/convs || u.SpanCount != posts/convs {
			t.Errorf("%s = %+v, want input=%d output=%d span_count=%d", cid, u, want, posts/convs, posts/convs)
		}
	}
}

func TestMailboxWrite_RetriesFailedRename(t *testing.T) {
	stateDir := t.TempDir()
	orig := osRename
	calls := 0
	osRename = func(oldpath, newpath string) error {
		calls++
		if calls <= 2 { // e.g. a Windows sharing violation
			return errors.New("sharing violation")
		}
		return orig(oldpath, newpath)
	}
	t.Cleanup(func() { osRename = orig })

	processSpan(stateDir, agentSpan("sess-retry", "r1", 11, 2, 0, ""))

	u := mustUsage(t, stateDir, "sess-retry")
	if u.InputTokens != 11 {
		t.Errorf("mailbox not updated after rename retries: %+v", u)
	}
	if calls != 3 {
		t.Errorf("rename attempts = %d, want 3 (two failures, then success)", calls)
	}
}

func TestMailboxWrite_PersistentRenameFailureIsSwallowed(t *testing.T) {
	stateDir := t.TempDir()
	orig := osRename
	calls := 0
	osRename = func(string, string) error { calls++; return errors.New("always fails") }
	t.Cleanup(func() { osRename = orig })

	processSpan(stateDir, agentSpan("sess-fail", "f1", 1, 1, 0, "")) // must not panic
	if calls > 3 {
		t.Errorf("rename attempted %d times, want at most 3", calls)
	}
}

func TestGC_DeletesOldMailboxesAndTempFilesOnly(t *testing.T) {
	stateDir := t.TempDir()
	sessions := filepath.Join(stateDir, "sessions")
	if err := os.MkdirAll(sessions, 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	touch := func(path string, age time.Duration) {
		t.Helper()
		if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
		ts := now.Add(-age)
		if err := os.Chtimes(path, ts, ts); err != nil {
			t.Fatal(err)
		}
	}
	touch(filepath.Join(sessions, "old.json"), 8*24*time.Hour)
	touch(filepath.Join(sessions, "new.json"), 24*time.Hour)
	touch(filepath.Join(sessions, "old.json.tmp"), 2*time.Hour)
	touch(filepath.Join(sessions, "fresh.json.tmp"), 10*time.Minute)
	// Files outside sessions/ are never garbage collected, however old.
	touch(filepath.Join(stateDir, "receiver.log"), 30*24*time.Hour)
	touch(filepath.Join(stateDir, "receiver-4318.json"), 30*24*time.Hour)

	GC(stateDir, now)

	gone := []string{"sessions/old.json", "sessions/old.json.tmp"}
	kept := []string{"sessions/new.json", "sessions/fresh.json.tmp", "receiver.log", "receiver-4318.json"}
	for _, f := range gone {
		if _, err := os.Stat(filepath.Join(stateDir, f)); !os.IsNotExist(err) {
			t.Errorf("%s should have been deleted (err=%v)", f, err)
		}
	}
	for _, f := range kept {
		if _, err := os.Stat(filepath.Join(stateDir, f)); err != nil {
			t.Errorf("%s should have been kept: %v", f, err)
		}
	}
}

func TestGC_MissingDirIsNoop(t *testing.T) {
	GC(filepath.Join(t.TempDir(), "does-not-exist"), time.Now()) // must not panic
}

func TestStartGC_RunsOnceImmediately(t *testing.T) {
	stateDir := t.TempDir()
	sessions := filepath.Join(stateDir, "sessions")
	if err := os.MkdirAll(sessions, 0o755); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(sessions, "old.json")
	if err := os.WriteFile(old, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	ts := time.Now().Add(-8 * 24 * time.Hour)
	if err := os.Chtimes(old, ts, ts); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	StartGC(ctx, stateDir)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(old); os.IsNotExist(err) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Error("StartGC did not run a collection immediately; old.json is still present")
}

func TestHandler_SpanWithoutConversationID_Skipped(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	span := &tracepb.Span{
		Name:       "invoke_agent",
		SpanId:     []byte("n1"),
		Attributes: []*commonpb.KeyValue{intAttr("gen_ai.usage.input_tokens", 100)},
	}
	srv := httptest.NewServer(Handler(stateDir))
	defer srv.Close()
	postExport(t, srv.URL, exportRequest(span))

	if _, err := os.Stat(filepath.Join(stateDir, "sessions")); !os.IsNotExist(err) {
		t.Error("expected no sessions dir content when no conversation.id present")
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

func TestReadSessionUsage_UnsafeIDReadsNothing(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	if err := os.MkdirAll(filepath.Join(stateDir, "sessions"), 0o755); err != nil {
		t.Fatal(err)
	}
	// <state>/sessions/../x.json == <state>/x.json: must not be reachable via "../x".
	if err := os.WriteFile(filepath.Join(stateDir, "x.json"), []byte(`{"input_tokens":9}`), 0o644); err != nil {
		t.Fatal(err)
	}
	usage, err := ReadSessionUsage(stateDir, "../x")
	if err != nil || usage != nil {
		t.Errorf("ReadSessionUsage(../x) = (%+v, %v), want (nil, nil)", usage, err)
	}
}

func TestReadSessionUsage_UnmarshalError(t *testing.T) {
	stateDir := t.TempDir()
	path := sessionPath(stateDir, "sess-corrupt")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadSessionUsage(stateDir, "sess-corrupt"); err == nil {
		t.Error("expected error unmarshalling corrupt session file")
	}
}

func TestSessionPath_IsUnderStateDirSessions(t *testing.T) {
	got := sessionPath("/s", "abc")
	want := filepath.Join("/s", "sessions", "abc.json")
	if got != want {
		t.Errorf("sessionPath = %q, want %q", got, want)
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

func TestAttrInt_Float64AndDefault(t *testing.T) {
	m := map[string]any{"f": float64(42), "s": "not a number"}
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
	stateDir := t.TempDir()
	span := &tracepb.Span{
		Name:   "invoke_agent",
		SpanId: []byte("rm1"),
		Attributes: []*commonpb.KeyValue{
			strAttr("gen_ai.conversation.id", "sess-fallback"),
			strAttr("gen_ai.request.model", "gpt-fallback"),
			intAttr("gen_ai.usage.input_tokens", 10),
		},
	}
	processSpan(stateDir, span)
	if u := mustUsage(t, stateDir, "sess-fallback"); u.Model != "gpt-fallback" {
		t.Fatalf("got %+v, want Model=gpt-fallback", u)
	}
}

func TestProcessSpan_NoUsageData_Skipped(t *testing.T) {
	stateDir := t.TempDir()
	span := &tracepb.Span{
		Name:       "invoke_agent",
		SpanId:     []byte("nu1"),
		Attributes: []*commonpb.KeyValue{strAttr("gen_ai.conversation.id", "sess-nousage")},
	}
	processSpan(stateDir, span)
	if u, err := ReadSessionUsage(stateDir, "sess-nousage"); err != nil || u != nil {
		t.Errorf("expected no usage written for a span with no token data, got (%+v, %v)", u, err)
	}
}

func TestProcessSpan_SessionsDirBlocked_DoesNotPanic(t *testing.T) {
	stateDir := t.TempDir()
	// A regular file where the sessions directory needs to be: MkdirAll fails.
	if err := os.WriteFile(filepath.Join(stateDir, "sessions"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	processSpan(stateDir, agentSpan("sess-blocked", "b1", 1, 1, 0, "")) // best-effort: no panic
	if _, err := ReadSessionUsage(stateDir, "sess-blocked"); err == nil {
		t.Error("expected ReadSessionUsage to report an error reading through a blocked directory")
	}
}

func TestLogRequest_StateDirParentBlocked_DoesNotPanic(t *testing.T) {
	root := t.TempDir()
	blocker := filepath.Join(root, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(Handler(filepath.Join(blocker, "state")))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/v1/traces")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}

func TestLogRequest_OpenFileFails(t *testing.T) {
	stateDir := t.TempDir()
	// receiver.log as a directory: OpenFile fails (EISDIR); logRequest is best-effort.
	if err := os.MkdirAll(filepath.Join(stateDir, "receiver.log"), 0o755); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/traces", nil)
	logRequest(stateDir, req, "note", 0, 0)
}
