// Package otelreceiver implements a minimal OTLP/HTTP trace receiver used to capture
// GitHub Copilot's native OpenTelemetry export (github.copilot.chat.otel.*), since its
// SubagentStop hook payload and transcript file expose no token-usage data at all —
// confirmed empirically by grepping a real captured transcript. Copilot follows the OTel
// GenAI Semantic Conventions: token usage lives on "invoke_agent" span attributes
// (gen_ai.usage.input_tokens, gen_ai.usage.output_tokens, gen_ai.usage.cache_read.input_tokens),
// correlated to a session via gen_ai.conversation.id — which matches the "session_id" field
// already present in the hook payload the CopilotCollector reads.
package otelreceiver

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"google.golang.org/protobuf/proto"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

// SessionUsage is the token-usage mailbox written per gen_ai.conversation.id, consumed by
// the GitHub Copilot telemetry collector at SubagentStop.
type SessionUsage struct {
	Model        string `json:"model"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	CachedTokens int64  `json:"cached_tokens"`
	CapturedAt   string `json:"captured_at"`
}

// Handler returns an http.Handler implementing the OTLP/HTTP trace-export endpoint
// (POST /v1/traces) needed to receive GitHub Copilot's exported spans. Every request,
// regardless of whether it yields usable token data, is answered with a valid empty
// OTLP export response so the exporter never treats the send as failed.
func Handler(repoRoot string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/traces", func(w http.ResponseWriter, r *http.Request) {
		handleTraces(repoRoot, w, r)
	})
	return mux
}

func handleTraces(repoRoot string, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	isJSON := r.Header.Get("Content-Type") == "application/json"

	var req coltracepb.ExportTraceServiceRequest
	if isJSON {
		if err := protojsonUnmarshal(body, &req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		if err := proto.Unmarshal(body, &req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	for _, rs := range req.ResourceSpans {
		for _, ss := range rs.ScopeSpans {
			for _, span := range ss.Spans {
				processSpan(repoRoot, span)
			}
		}
	}

	resp := &coltracepb.ExportTraceServiceResponse{}
	if isJSON {
		w.Header().Set("Content-Type", "application/json")
		data, _ := protojsonMarshal(resp)
		w.Write(data)
		return
	}
	w.Header().Set("Content-Type", "application/x-protobuf")
	data, _ := proto.Marshal(resp)
	w.Write(data)
}

func processSpan(repoRoot string, span *tracepb.Span) {
	attrs := attrMap(span.GetAttributes())

	conversationID, _ := attrs["gen_ai.conversation.id"].(string)
	if conversationID == "" {
		return // can't correlate to a hook-payload session_id without this
	}

	usage := SessionUsage{
		CapturedAt:   time.Now().UTC().Format(time.RFC3339),
		InputTokens:  attrInt(attrs, "gen_ai.usage.input_tokens"),
		OutputTokens: attrInt(attrs, "gen_ai.usage.output_tokens"),
		CachedTokens: attrInt(attrs, "gen_ai.usage.cache_read.input_tokens"),
	}
	if model, ok := attrs["gen_ai.response.model"].(string); ok && model != "" {
		usage.Model = model
	} else if model, ok := attrs["gen_ai.request.model"].(string); ok {
		usage.Model = model
	}

	if usage.InputTokens == 0 && usage.OutputTokens == 0 && usage.CachedTokens == 0 {
		return // this span carries no usage data — nothing worth recording
	}

	writeSessionUsage(repoRoot, conversationID, usage)
}

func attrMap(kvs []*commonpb.KeyValue) map[string]any {
	m := make(map[string]any, len(kvs))
	for _, kv := range kvs {
		if kv == nil || kv.GetValue() == nil {
			continue
		}
		switch val := kv.GetValue().GetValue().(type) {
		case *commonpb.AnyValue_StringValue:
			m[kv.GetKey()] = val.StringValue
		case *commonpb.AnyValue_IntValue:
			m[kv.GetKey()] = val.IntValue
		case *commonpb.AnyValue_DoubleValue:
			m[kv.GetKey()] = val.DoubleValue
		case *commonpb.AnyValue_BoolValue:
			m[kv.GetKey()] = val.BoolValue
		}
	}
	return m
}

func attrInt(m map[string]any, key string) int64 {
	switch v := m[key].(type) {
	case int64:
		return v
	case float64:
		return int64(v)
	}
	return 0
}

func sessionPath(repoRoot, conversationID string) string {
	return filepath.Join(repoRoot, ".dreamland", "otel-sessions", conversationID+".json")
}

// writeSessionUsage best-effort persists usage for conversationID. Failures are silent —
// this must never break the HTTP response to the exporter.
func writeSessionUsage(repoRoot, conversationID string, usage SessionUsage) {
	path := sessionPath(repoRoot, conversationID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	data, err := json.Marshal(usage)
	if err != nil {
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

// ReadSessionUsage reads the mailbox for conversationID, if present. Returns
// (nil, nil) — not an error — when no data has been captured for that session yet.
func ReadSessionUsage(repoRoot, conversationID string) (*SessionUsage, error) {
	if conversationID == "" {
		return nil, nil
	}
	data, err := os.ReadFile(sessionPath(repoRoot, conversationID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var usage SessionUsage
	if err := json.Unmarshal(data, &usage); err != nil {
		return nil, err
	}
	return &usage, nil
}
