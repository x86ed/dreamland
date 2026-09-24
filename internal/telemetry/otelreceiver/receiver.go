// Package otelreceiver implements a minimal OTLP/HTTP trace receiver used to capture
// GitHub Copilot's native OpenTelemetry export (github.copilot.chat.otel.*), since its
// SubagentStop hook payload and transcript file expose no token-usage data at all —
// confirmed empirically by grepping a real captured transcript. Copilot follows the OTel
// GenAI Semantic Conventions: token usage lives on "invoke_agent" span attributes
// (gen_ai.usage.input_tokens, gen_ai.usage.output_tokens, gen_ai.usage.cache_read.input_tokens),
// correlated to a session via gen_ai.conversation.id — which matches the "session_id" field
// already present in the hook payload the CopilotCollector reads.
//
// The receiver is one shared, repo-agnostic process per listen address: it never reads or
// writes anything under a repository. Mailboxes (cumulative per-conversation totals), the
// request log and the pid/lock files all live in the per-user state directory (StateDir).
// Attribution to a repo happens in the consumer, which knows its own session_id.
package otelreceiver

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

// SessionUsage is the cumulative token-usage mailbox written per gen_ai.conversation.id,
// consumed by the GitHub Copilot telemetry collector.
type SessionUsage struct {
	Version      int    `json:"version"`
	SpanCount    int    `json:"span_count"`
	Model        string `json:"model"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	CachedTokens int64  `json:"cached_tokens"`
	CapturedAt   string `json:"captured_at"`
}

// ReceiverRevision identifies the receiver's behavior and mailbox format. Bump it whenever
// handler behavior or the mailbox format changes so a newer binary replaces a running,
// older receiver. Pre-handshake receivers (no health endpoint) are revision 1 by definition.
const ReceiverRevision = 2

const (
	mailboxVersion = 2
	// agentOperation is the gen_ai.operation.name of the spans whose usage is counted.
	// Unverified against a real Copilot capture (openspec task 0.1).
	agentOperation   = "invoke_agent"
	maxSpanIDsPerCID = 4096
	logRotateBytes   = 5 * 1024 * 1024
	renameAttempts   = 3
	renameBackoff    = 20 * time.Millisecond
	sessionMaxAge    = 7 * 24 * time.Hour
	tmpMaxAge        = time.Hour
)

// Build is injected by cmd from buildCommit and reported by the health endpoint.
var Build string

// osRename is the rename seam used by the mailbox writer.
var osRename = os.Rename

// mailboxMu serializes every mailbox read-modify-write and guards seenSpans.
var (
	mailboxMu sync.Mutex
	seenSpans = map[string]*spanIDSet{}
	logMu     sync.Mutex
)

// spanIDSet is a bounded FIFO set of span ids already counted for one conversation.
type spanIDSet struct {
	ids   map[string]struct{}
	order []string
}

func (s *spanIDSet) has(id string) bool { _, ok := s.ids[id]; return ok }

func (s *spanIDSet) add(id string) {
	if len(s.order) >= maxSpanIDsPerCID {
		delete(s.ids, s.order[0])
		s.order = s.order[1:]
	}
	s.ids[id] = struct{}{}
	s.order = append(s.order, id)
}

// GC deletes sessions/*.json older than 7 days and leftover sessions/*.tmp older than 1 h.
func GC(stateDir string, now time.Time) {
	dir := SessionsDir(stateDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		var maxAge time.Duration
		switch {
		case strings.HasSuffix(e.Name(), ".tmp"):
			maxAge = tmpMaxAge
		case strings.HasSuffix(e.Name(), ".json"):
			maxAge = sessionMaxAge
		default:
			continue
		}
		info, err := e.Info()
		if err != nil || now.Sub(info.ModTime()) <= maxAge {
			continue
		}
		os.Remove(filepath.Join(dir, e.Name()))
	}
}

// StartGC runs GC once immediately and hourly until ctx is done.
func StartGC(ctx context.Context, stateDir string) {
	GC(stateDir, time.Now())
	go func() {
		t := time.NewTicker(time.Hour)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-t.C:
				GC(stateDir, now)
			}
		}
	}()
}

// Handler returns an http.Handler implementing the OTLP/HTTP trace-export endpoint
// (POST /v1/traces) needed to receive GitHub Copilot's exported spans, plus
// GET /.dreamland/health for the start-up handshake. Every trace request, regardless of
// whether it yields usable token data, is answered with a valid empty OTLP export response
// so the exporter never treats the send as failed. Every other request to any path is
// logged to <stateDir>/receiver.log — since the receiver normally runs detached with its
// own stdout/stderr discarded, this is the only way to confirm whether an exporter is
// reaching it at all, and on which path, without guessing.
func Handler(stateDir string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/traces", func(w http.ResponseWriter, r *http.Request) {
		handleTraces(stateDir, w, r)
	})
	mux.HandleFunc("/.dreamland/health", func(w http.ResponseWriter, r *http.Request) {
		build := Build
		if build == "" {
			build = "unknown"
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"service":   "dreamland-otel-receiver",
			"revision":  ReceiverRevision,
			"build":     build,
			"pid":       os.Getpid(),
			"state_dir": stateDir,
		})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		logRequest(stateDir, r, "unhandled path", 0, 0)
		w.WriteHeader(http.StatusOK)
	})
	return mux
}

func handleTraces(stateDir string, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		logRequest(stateDir, r, "method not allowed", 0, 0)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		logRequest(stateDir, r, "read body error: "+err.Error(), 0, 0)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	isJSON := r.Header.Get("Content-Type") == "application/json"

	var req coltracepb.ExportTraceServiceRequest
	if isJSON {
		if err := protojsonUnmarshal(body, &req); err != nil {
			logRequest(stateDir, r, "json decode error: "+err.Error(), len(body), 0)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		if err := proto.Unmarshal(body, &req); err != nil {
			logRequest(stateDir, r, "protobuf decode error: "+err.Error(), len(body), 0)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	spanCount := 0
	var spanNames []string
	var conversationIDs []string
	var notes []string
	for _, rs := range req.ResourceSpans {
		for _, ss := range rs.ScopeSpans {
			for _, span := range ss.Spans {
				spanCount++
				spanNames = append(spanNames, span.GetName())
				if cid, ok := attrMap(span.GetAttributes())["gen_ai.conversation.id"].(string); ok && cid != "" {
					conversationIDs = append(conversationIDs, cid)
				}
				if note := processSpan(stateDir, span); note != "" {
					notes = append(notes, note)
				}
			}
		}
	}
	note := fmt.Sprintf("ok spans=%v conversation.ids=%v", spanNames, conversationIDs)
	if len(notes) > 0 {
		note += " " + strings.Join(notes, "; ")
	}
	logRequest(stateDir, r, note, len(body), spanCount)

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

// logRequest appends a one-line record of every request the receiver sees, regardless of
// outcome, to <stateDir>/receiver.log, rotating it to receiver.log.1 above 5 MiB.
// Best-effort: failures are silent.
func logRequest(stateDir string, r *http.Request, note string, bodyLen, spanCount int) {
	logMu.Lock()
	defer logMu.Unlock()
	path := LogPath(stateDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	if info, err := os.Stat(path); err == nil && info.Size() > logRotateBytes {
		_ = os.Rename(path, path+".1")
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s %s content-type=%q body_bytes=%d spans=%d note=%s\n",
		time.Now().UTC().Format(time.RFC3339), r.Method, r.URL.Path, r.Header.Get("Content-Type"), bodyLen, spanCount, note)
}

func isAgentSpan(span *tracepb.Span, attrs map[string]any) bool {
	if op, ok := attrs["gen_ai.operation.name"].(string); ok {
		return op == agentOperation
	}
	return strings.HasPrefix(span.GetName(), agentOperation)
}

// processSpan adds one qualifying span's usage to its conversation's mailbox. It returns a
// short note for the request log when the span was rejected for a reason worth recording.
func processSpan(stateDir string, span *tracepb.Span) string {
	attrs := attrMap(span.GetAttributes())

	if !isAgentSpan(span, attrs) {
		return ""
	}

	conversationID, _ := attrs["gen_ai.conversation.id"].(string)
	if conversationID == "" {
		return "" // can't correlate to a hook-payload session_id without this
	}
	if !ValidConversationID(conversationID) {
		return "invalid conversation id"
	}

	usage := SessionUsage{
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
		return "" // this span carries no usage data — nothing worth recording
	}

	writeSessionUsage(stateDir, conversationID, hex.EncodeToString(span.GetSpanId()), usage)
	return ""
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

func sessionPath(stateDir, conversationID string) string {
	return filepath.Join(SessionsDir(stateDir), conversationID+".json")
}

func readMailbox(path string) (*SessionUsage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var usage SessionUsage
	if err := json.Unmarshal(data, &usage); err != nil {
		return nil, err
	}
	return &usage, nil
}

// writeSessionUsage best-effort adds usage to the cumulative mailbox for conversationID,
// skipping a span id already counted. Failures are silent — this must never break the HTTP
// response to the exporter.
func writeSessionUsage(stateDir, conversationID, spanID string, usage SessionUsage) {
	mailboxMu.Lock()
	defer mailboxMu.Unlock()

	seenKey := stateDir + "\x00" + conversationID
	seen := seenSpans[seenKey]
	if seen == nil {
		seen = &spanIDSet{ids: map[string]struct{}{}}
	}
	if spanID != "" && seen.has(spanID) {
		return
	}

	path := sessionPath(stateDir, conversationID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}

	total := SessionUsage{}
	if existing, err := readMailbox(path); err == nil {
		total = *existing
	}
	total.Version = mailboxVersion
	total.SpanCount++
	total.InputTokens += usage.InputTokens
	total.OutputTokens += usage.OutputTokens
	total.CachedTokens += usage.CachedTokens
	if usage.Model != "" {
		total.Model = usage.Model
	}
	total.CapturedAt = time.Now().UTC().Format(time.RFC3339)

	data, err := json.Marshal(total)
	if err != nil {
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	var renameErr error
	for attempt := 0; attempt < renameAttempts; attempt++ {
		if renameErr = osRename(tmp, path); renameErr == nil {
			break
		}
		if attempt < renameAttempts-1 {
			time.Sleep(renameBackoff)
		}
	}
	if renameErr != nil {
		os.Remove(tmp)
		return
	}

	if spanID != "" {
		seen.add(spanID)
		seenSpans[seenKey] = seen
	}
}

// ReadSessionUsage reads the mailbox for conversationID, if present. Returns
// (nil, nil) — not an error — when no data has been captured for that session yet or the
// id is not a valid conversation id.
func ReadSessionUsage(stateDir, conversationID string) (*SessionUsage, error) {
	if !ValidConversationID(conversationID) {
		return nil, nil
	}
	usage, err := readMailbox(sessionPath(stateDir, conversationID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return usage, nil
}
