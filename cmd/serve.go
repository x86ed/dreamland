package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"dreamland/internal/telemetry"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MCP server",
	RunE:  runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

type helloInput struct {
	Name string `json:"name" description:"Name to greet"`
}

type helloOutput struct {
	Message string `json:"message"`
}

type telemetryWriteInput struct {
	Tool    string `json:"tool" description:"Tool name (claude-code, codex, cursor, kiro, antigravity, github-copilot)"`
	Payload string `json:"payload" description:"JSON hook payload string"`
}

type telemetryWriteOutput struct {
	Written bool   `json:"written"`
	Message string `json:"message,omitempty"`
}

var mcpTracer trace.Tracer

// runServe starts the MCP server over stdio.
func runServe(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	tp, err := telemetry.NewTracerProvider(ctx)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "OTEL init warning: %v\n", err)
	} else {
		otel.SetTracerProvider(tp)
		defer func() { _ = tp.Shutdown(ctx) }()
	}
	mcpTracer = otel.Tracer("dreamland/mcp")

	s := mcp.NewServer(&mcp.Implementation{
		Name:    "dreamland",
		Version: "0.1.0",
	}, nil)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "hello",
		Description: "Say hello",
	}, makeSpanHandler("hello", helloHandler))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "telemetry_write",
		Description: "Write an AI session telemetry snapshot from a hook payload JSON string",
	}, makeSpanHandler("telemetry_write", telemetryWriteHandler))

	fmt.Fprintln(cmd.ErrOrStderr(), "Starting MCP server (stdio)...")
	return s.Run(ctx, &mcp.StdioTransport{})
}

// makeSpanHandler wraps a typed MCP handler in a mcp.tool_call span.
func makeSpanHandler[I, O any](
	toolName string,
	h func(context.Context, *mcp.CallToolRequest, I) (*mcp.CallToolResult, O, error),
) func(context.Context, *mcp.CallToolRequest, I) (*mcp.CallToolResult, O, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input I) (*mcp.CallToolResult, O, error) {
		cfg := GetConfig()
		attrs := []attribute.KeyValue{
			attribute.String("ai.tool", toolName),
		}
		if cfg != nil {
			attrs = append(attrs, attribute.String("ai.model", cfg.ModelID))
		}
		ctx, span := mcpTracer.Start(ctx, "mcp.tool_call", trace.WithAttributes(attrs...))
		defer span.End()
		return h(ctx, req, input)
	}
}

// helloHandler responds to hello tool requests.
func helloHandler(_ context.Context, _ *mcp.CallToolRequest, input helloInput) (*mcp.CallToolResult, helloOutput, error) {
	msg := fmt.Sprintf("Hello, %s!", input.Name)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: msg}},
	}, helloOutput{Message: msg}, nil
}

// telemetryWriteHandler allows MCP-capable tools to write session telemetry directly.
func telemetryWriteHandler(_ context.Context, _ *mcp.CallToolRequest, input telemetryWriteInput) (*mcp.CallToolResult, telemetryWriteOutput, error) {
	collector, ok := telemetry.Registry[input.Tool]
	if !ok {
		msg := fmt.Sprintf("unknown tool %q", input.Tool)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: msg}},
		}, telemetryWriteOutput{Written: false, Message: msg}, nil
	}

	cfg := GetConfig()
	result, err := collector.Collect(strings.NewReader(input.Payload), cfg)
	if err != nil || result == nil {
		msg := "collect error"
		if err != nil {
			msg = err.Error()
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: msg}},
		}, telemetryWriteOutput{Written: false, Message: msg}, nil
	}

	repoRoot := "."
	if cfg != nil && cfg.RepoRoot != "" {
		repoRoot = cfg.RepoRoot
	}
	if err := telemetry.Write(repoRoot, result); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
		}, telemetryWriteOutput{Written: false, Message: err.Error()}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: "telemetry written"}},
	}, telemetryWriteOutput{Written: true}, nil
}
