package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// writeLiveDashState records the test process as a running dashboard, so
// zhougongDashRunning reports it alive without spawning a real child.
func writeLiveDashState(t *testing.T, repo, url string) {
	t.Helper()
	b, err := json.Marshal(zhougongDashState{PID: os.Getpid(), URL: url})
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(repo, zhougongDashStateFile)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func connectDashboardMCP(t *testing.T, repo string) *mcp.ClientSession {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	serverT, clientT := mcp.NewInMemoryTransports()
	done := make(chan error, 1)
	go func() { done <- runDashboardMCP(ctx, repo, serverT) }()
	client := mcp.NewClient(&mcp.Implementation{Name: "dash-test", Version: "v0.0.1"}, nil)
	session, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = session.Close()
		cancel()
		<-done
	})
	return session
}

func toolText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	var sb strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			sb.WriteString(tc.Text)
		}
	}
	return sb.String()
}

func promptText(t *testing.T, res *mcp.GetPromptResult) string {
	t.Helper()
	var sb strings.Builder
	for _, m := range res.Messages {
		if tc, ok := m.Content.(*mcp.TextContent); ok {
			sb.WriteString(tc.Text)
		}
	}
	return sb.String()
}

func TestDashboardMCPStatusStartStop(t *testing.T) {
	repo := t.TempDir()
	ctx := context.Background()
	s := connectDashboardMCP(t, repo)

	res, err := s.CallTool(ctx, &mcp.CallToolParams{Name: "dashboard_status"})
	if err != nil || res.IsError {
		t.Fatalf("status: %v %+v", err, res)
	}
	if !strings.Contains(toolText(t, res), `"running":false`) {
		t.Errorf("idle status = %s", toolText(t, res))
	}

	writeLiveDashState(t, repo, "http://127.0.0.1:1234")
	res, err = s.CallTool(ctx, &mcp.CallToolParams{Name: "dashboard_status"})
	if err != nil || !strings.Contains(toolText(t, res), "http://127.0.0.1:1234") {
		t.Fatalf("running status: %v %s", err, toolText(t, res))
	}

	// A live dashboard is reused, never respawned.
	res, err = s.CallTool(ctx, &mcp.CallToolParams{Name: "dashboard_start"})
	if err != nil || res.IsError || !strings.Contains(toolText(t, res), "http://127.0.0.1:1234") {
		t.Fatalf("start: %v %s", err, toolText(t, res))
	}
	p, err := s.GetPrompt(ctx, &mcp.GetPromptParams{Name: "start"})
	if err != nil || !strings.Contains(promptText(t, p), "http://127.0.0.1:1234") {
		t.Fatalf("start prompt: %v", err)
	}

	// No state file: stop is a no-op success for both tool and prompt.
	if err := os.Remove(filepath.Join(repo, zhougongDashStateFile)); err != nil {
		t.Fatal(err)
	}
	res, err = s.CallTool(ctx, &mcp.CallToolParams{Name: "dashboard_stop"})
	if err != nil || res.IsError {
		t.Fatalf("stop: %v %+v", err, res)
	}
	p, err = s.GetPrompt(ctx, &mcp.GetPromptParams{Name: "stop"})
	if err != nil || !strings.Contains(promptText(t, p), "stopped") {
		t.Fatalf("stop prompt: %v", err)
	}
}

func TestDashboardMCPStartFailureReported(t *testing.T) {
	orig := osExecutable
	osExecutable = func() (string, error) { return "", errors.New("no exe") }
	t.Cleanup(func() { osExecutable = orig })

	ctx := context.Background()
	s := connectDashboardMCP(t, t.TempDir())

	res, err := s.CallTool(ctx, &mcp.CallToolParams{Name: "dashboard_start"})
	if err != nil || !res.IsError || !strings.Contains(toolText(t, res), "no exe") {
		t.Fatalf("start failure: %v %+v", err, res)
	}
	p, err := s.GetPrompt(ctx, &mcp.GetPromptParams{Name: "start"})
	if err != nil || !strings.Contains(promptText(t, p), "failed") {
		t.Fatalf("start prompt: %v", err)
	}
}

func TestWatchDashboardAnnouncesLifecycle(t *testing.T) {
	orig := dashboardWatchInterval
	dashboardWatchInterval = 5 * time.Millisecond
	t.Cleanup(func() { dashboardWatchInterval = orig })

	repo := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	serverT, clientT := mcp.NewInMemoryTransports()
	done := make(chan error, 1)
	go func() { done <- runDashboardMCP(ctx, repo, serverT) }()

	msgs := make(chan string, 16)
	client := mcp.NewClient(&mcp.Implementation{Name: "watch-test", Version: "v0.0.1"}, &mcp.ClientOptions{
		LoggingMessageHandler: func(_ context.Context, r *mcp.LoggingMessageRequest) {
			msgs <- string(r.Params.Level) + ": " + r.Params.Data.(string)
		},
	})
	session, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = session.Close()
		cancel()
		<-done
	})
	if err := session.SetLoggingLevel(ctx, &mcp.SetLoggingLevelParams{Level: "info"}); err != nil {
		t.Fatal(err)
	}

	want := func(sub string) {
		t.Helper()
		select {
		case m := <-msgs:
			if !strings.Contains(m, sub) {
				t.Fatalf("message %q, want substring %q", m, sub)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("no announcement containing %q", sub)
		}
	}

	writeLiveDashState(t, repo, "http://127.0.0.1:9")
	want("started at http://127.0.0.1:9")

	if err := os.Remove(filepath.Join(repo, zhougongDashStateFile)); err != nil {
		t.Fatal(err)
	}
	want("stopped")

	// A state file whose process is gone reads as an unexpected death.
	writeDeadDashState(t, repo)
	writeLiveDashState(t, repo, "http://127.0.0.1:9")
	want("started at")
	writeDeadDashState(t, repo)
	want("died unexpectedly")
}

func writeDeadDashState(t *testing.T, repo string) {
	t.Helper()
	b, _ := json.Marshal(zhougongDashState{PID: 1 << 30, URL: "http://127.0.0.1:9"})
	p := filepath.Join(repo, zhougongDashStateFile)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveZhougongDashStateKeepsForeignPID(t *testing.T) {
	repo := t.TempDir()
	writeLiveDashState(t, repo, "http://127.0.0.1:1")
	removeZhougongDashState(repo, os.Getpid()+1)
	if _, ok := readZhougongDashState(repo); !ok {
		t.Fatal("state owned by another pid was removed")
	}
	removeZhougongDashState(repo, os.Getpid())
	if _, ok := readZhougongDashState(repo); ok {
		t.Fatal("state owned by pid was not removed")
	}
}

func TestStopZhougongDashboardCleansDeadState(t *testing.T) {
	repo := t.TempDir()
	writeDeadDashState(t, repo)
	if err := stopZhougongDashboard(repo); err != nil {
		t.Fatal(err)
	}
	if _, ok := readZhougongDashState(repo); ok {
		t.Fatal("dead dashboard state was left behind")
	}
}
