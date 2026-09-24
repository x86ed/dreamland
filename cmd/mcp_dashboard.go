package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

)

const (
	dashboardMCPName   = "dreamland-dashboard"
	channelMethod      = "notifications/claude/channel"
	channelCapability  = "claude/channel"
	dashboardMCPLogger = "dreamland-dashboard"
)

// dashboardWatchInterval is how often the server re-reads the dashboard state file.
var dashboardWatchInterval = 500 * time.Millisecond

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run a dreamland MCP server (stdio)",
}

var mcpDashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Start the dashboard-lifecycle MCP server (stdio)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		root, err := zhougongDashRepoRoot()
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.ErrOrStderr(), "Starting dashboard MCP server (stdio)...")
		return runDashboardMCP(context.Background(), root, &mcp.StdioTransport{})
	},
}

func init() {
	mcpCmd.AddCommand(mcpDashboardCmd)
	rootCmd.AddCommand(mcpCmd)
}


type dashboardStartInput struct {
	Port int `json:"port,omitempty" description:"port to bind on 127.0.0.1; 0 picks a free port"`
}

type dashboardStatusOutput struct {
	Running bool   `json:"running"`
	URL     string `json:"url,omitempty"`
}

// channelConn wraps a Connection so the server can also emit raw JSON-RPC notifications
// (the go-sdk exposes no public API for custom notification methods).
type channelConn struct {
	mcp.Connection
}

func (c *channelConn) notify(ctx context.Context, method string, params any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return c.Write(ctx, &jsonrpc.Request{Method: method, Params: raw})
}

type channelTransport struct {
	inner mcp.Transport
	mu    sync.Mutex
	conn  *channelConn
}

func (t *channelTransport) Connect(ctx context.Context) (mcp.Connection, error) {
	c, err := t.inner.Connect(ctx)
	if err != nil {
		return nil, err
	}
	cc := &channelConn{Connection: c}
	t.mu.Lock()
	t.conn = cc
	t.mu.Unlock()
	return cc, nil
}

func (t *channelTransport) current() *channelConn {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.conn
}

func newDashboardMCPServer(repoRoot string) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: dashboardMCPName, Version: "0.1.0"}, &mcp.ServerOptions{
		Capabilities: &mcp.ServerCapabilities{
			Logging:      &mcp.LoggingCapabilities{},
			Experimental: map[string]any{channelCapability: map[string]any{}},
		},
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "dashboard_start",
		Description: "Start the zhougong metrics dashboard (127.0.0.1 only) in a detached process and return its URL; reuses a running one",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in dashboardStartInput) (*mcp.CallToolResult, zhougongURLOutput, error) {
		url, err := startZhougongDashboard(repoRoot, in.Port)
		if err != nil {
			return toolError(err), zhougongURLOutput{}, nil
		}
		return nil, zhougongURLOutput{URL: url}, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "dashboard_stop",
		Description: "Stop the zhougong metrics dashboard and release its port",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ zhougongEmpty) (*mcp.CallToolResult, zhougongEmpty, error) {
		if err := stopZhougongDashboard(repoRoot); err != nil {
			return toolError(err), zhougongEmpty{}, nil
		}
		return nil, zhougongEmpty{}, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "dashboard_status",
		Description: "Report whether the zhougong metrics dashboard is running and its URL",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ zhougongEmpty) (*mcp.CallToolResult, dashboardStatusOutput, error) {
		url, ok := zhougongDashRunning(repoRoot)
		return nil, dashboardStatusOutput{Running: ok, URL: url}, nil
	})

	text := func(s string) *mcp.GetPromptResult {
		return &mcp.GetPromptResult{Messages: []*mcp.PromptMessage{{Role: "user", Content: &mcp.TextContent{Text: s}}}}
	}
	server.AddPrompt(&mcp.Prompt{Name: "start", Description: "Start the zhougong metrics dashboard and show its URL"},
		func(context.Context, *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			url, err := startZhougongDashboard(repoRoot, 0)
			if err != nil {
				return text("Starting the zhougong dashboard failed: " + err.Error() + "\nReport this error to the user."), nil
			}
			return text("The zhougong dashboard is running at " + url + ". Tell the user this URL and nothing else."), nil
		})
	server.AddPrompt(&mcp.Prompt{Name: "stop", Description: "Stop the zhougong metrics dashboard"},
		func(context.Context, *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			if err := stopZhougongDashboard(repoRoot); err != nil {
				return text("Stopping the zhougong dashboard failed: " + err.Error() + "\nReport this error to the user."), nil
			}
			return text("The zhougong dashboard is stopped. Tell the user that and nothing else."), nil
		})
	return server
}

// runDashboardMCP serves the dashboard-lifecycle MCP server over t and, while it runs, watches
// the dashboard state file so start/stop/death is announced even when caused outside this server.
func runDashboardMCP(ctx context.Context, repoRoot string, t mcp.Transport) error {
	server := newDashboardMCPServer(repoRoot)
	ct := &channelTransport{inner: t}
	wctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go watchDashboard(wctx, repoRoot, server, ct)
	return server.Run(ctx, ct)
}

func watchDashboard(ctx context.Context, repoRoot string, server *mcp.Server, ct *channelTransport) {
	prevURL, prevRunning := zhougongDashRunning(repoRoot)
	tick := time.NewTicker(dashboardWatchInterval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		url, running := zhougongDashRunning(repoRoot)
		var level mcp.LoggingLevel
		var msg string
		switch {
		case running && (!prevRunning || url != prevURL):
			level, msg = "info", "zhougong dashboard started at "+url
		case !running && prevRunning:
			if _, present := readZhougongDashState(repoRoot); present {
				level, msg = "warning", "zhougong dashboard died unexpectedly (was "+prevURL+")"
			} else {
				level, msg = "info", "zhougong dashboard stopped (was "+prevURL+")"
			}
		}
		prevURL, prevRunning = url, running
		if msg != "" {
			announceDashboard(ctx, server, ct, level, msg)
		}
	}
}

func announceDashboard(ctx context.Context, server *mcp.Server, ct *channelTransport, level mcp.LoggingLevel, msg string) {
	for ss := range server.Sessions() {
		_ = ss.Log(ctx, &mcp.LoggingMessageParams{Level: level, Logger: dashboardMCPLogger, Data: msg})
	}
	if c := ct.current(); c != nil {
		_ = c.notify(ctx, channelMethod, map[string]any{
			"content": msg,
			"meta":    map[string]string{"source": dashboardMCPName, "level": string(level)},
		})
	}
}
