package cmd

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"

	"dreamland/internal/config"
	"dreamland/internal/telemetry"
	"dreamland/internal/telemetry/tools"
)

func e2eSpan(cid, spanID string, in, out int64) *tracepb.Span {
	str := func(k, v string) *commonpb.KeyValue {
		return &commonpb.KeyValue{Key: k, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: v}}}
	}
	num := func(k string, v int64) *commonpb.KeyValue {
		return &commonpb.KeyValue{Key: k, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: v}}}
	}
	return &tracepb.Span{
		Name:   "invoke_agent",
		SpanId: []byte(spanID),
		Attributes: []*commonpb.KeyValue{
			str("gen_ai.operation.name", "invoke_agent"),
			str("gen_ai.conversation.id", cid),
			str("gen_ai.response.model", "gpt-4o"),
			num("gen_ai.usage.input_tokens", in),
			num("gen_ai.usage.output_tokens", out),
		},
	}
}

func postSpans(t *testing.T, addr string, spans ...*tracepb.Span) {
	t.Helper()
	req := &coltracepb.ExportTraceServiceRequest{ResourceSpans: []*tracepb.ResourceSpans{{
		ScopeSpans: []*tracepb.ScopeSpans{{Spans: spans}},
	}}}
	body, err := proto.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Post("http://"+addr+"/v1/traces", "application/x-protobuf", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("export status = %d, want 200", resp.StatusCode)
	}
}

func e2eGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// End to end: one shared foreground receiver on an ephemeral port with a temp state dir,
// sessions from two repositories, and each repository's collector run. Each repo must end
// up with only its own session's totals and contain no receiver artifacts.
func TestE2E_OneSharedReceiverTwoRepos(t *testing.T) {
	h := newRecvHarness(t) // sets DREAMLAND_STATE_DIR to a temp dir and stubs process seams
	t.Setenv("HOME", t.TempDir())
	t.Setenv("LOCALAPPDATA", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())
	addr := freeAddr(t)
	repoX, repoY := e2eGitRepo(t), e2eGitRepo(t)

	run := startForeground(t, h, addr)

	postSpans(t, addr, e2eSpan("A", "a1", 1000, 100))
	postSpans(t, addr, e2eSpan("B", "b1", 20, 2))
	postSpans(t, addr, e2eSpan("A", "a2", 300, 30), e2eSpan("B", "b2", 5, 1))
	// An exporter retry of an already delivered batch must not change anything.
	postSpans(t, addr, e2eSpan("A", "a2", 300, 30))

	collect := func(repo, sid string) {
		t.Helper()
		cfg := &config.Config{ModelID: "default-model", RepoRoot: repo}
		stdin := strings.NewReader(`{"hook_event_name":"Stop","session_id":"` + sid + `","transcript_path":"/nonexistent.jsonl"}`)
		res, err := (&tools.CopilotCollector{}).Collect(stdin, cfg)
		if err != nil {
			t.Fatalf("collect %s: %v", sid, err)
		}
		if err := telemetry.Write(repo, res); err != nil {
			t.Fatalf("write %s: %v", sid, err)
		}
	}
	collect(repoX, "A")
	collect(repoY, "B")
	collect(repoX, "A") // second Stop with no new span: must not grow

	x, err := telemetry.Read(repoX)
	if err != nil || x == nil {
		t.Fatalf("repo X snapshot = (%v, %v)", x, err)
	}
	y, err := telemetry.Read(repoY)
	if err != nil || y == nil {
		t.Fatalf("repo Y snapshot = (%v, %v)", y, err)
	}
	if x.InputTokens != 1300 || x.OutputTokens != 130 {
		t.Errorf("repo X = %+v, want only session A's 1300/130", x)
	}
	if y.InputTokens != 25 || y.OutputTokens != 3 {
		t.Errorf("repo Y = %+v, want only session B's 25/3", y)
	}

	for name, repo := range map[string]string{"X": repoX, "Y": repoY} {
		for _, rel := range []string{".dreamland/otel-sessions", ".dreamland/otel-receiver.log"} {
			if _, err := os.Stat(filepath.Join(repo, rel)); !os.IsNotExist(err) {
				t.Errorf("repo %s contains %s; the shared receiver must not write under a repository (err=%v)", name, rel, err)
			}
		}
	}
	for _, f := range []string{"sessions/A.json", "sessions/B.json", "receiver.log"} {
		if _, err := os.Stat(filepath.Join(h.stateDir, f)); err != nil {
			t.Errorf("state dir is missing %s: %v", f, err)
		}
	}

	if err := run.stop(t); err != nil {
		t.Errorf("shutdown: %v", err)
	}
}
