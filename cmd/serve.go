package cmd

import (
	"context"
	"fmt"

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

type transitionLogInput struct{}
type transitionLogOutput struct {
	OK bool `json:"ok"`
}

type versionBumpInput struct {
	Major    bool   `json:"major,omitempty" description:"bump major version"`
	Minor    bool   `json:"minor,omitempty" description:"bump minor version"`
	Patch    bool   `json:"patch,omitempty" description:"bump patch version (end-of-turn mode)"`
	Breaking bool   `json:"breaking,omitempty" description:"breaking change: bump major instead of minor"`
	Version  string `json:"version,omitempty" description:"set explicit version (e.g. v1.2.3)"`
}
type versionBumpOutput struct {
	OK bool `json:"ok"`
}

type runTestsInput struct{}
type runTestsOutput struct {
	OK bool `json:"ok"`
}

type coauthorInput struct {
	Trailer string `json:"trailer,omitempty" description:"commit message file path (prepare-commit-msg delegation mode)"`
}
type coauthorOutput struct {
	OK bool `json:"ok"`
}

var mcpTracer trace.Tracer

// runServe starts the MCP server over stdio.
func runServe(cmd *cobra.Command, _ []string) error {
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
		Name:        "transition_log",
		Description: "Append a timestamped turn-complete entry to .dreamland/transition.log",
	}, makeSpanHandler("transition_log", transitionLogHandler))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "version_bump",
		Description: "Bump the project version (session-start: minor/major; end-of-turn: --patch)",
	}, makeSpanHandler("version_bump", versionBumpHandler))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "run_tests",
		Description: "Run tests if source files changed since last commit",
	}, makeSpanHandler("run_tests", runTestsHandler))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "coauthor",
		Description: "Set agent git identity and install prepare-commit-msg hook",
	}, makeSpanHandler("coauthor", coauthorMCPHandler))

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

func transitionLogHandler(_ context.Context, _ *mcp.CallToolRequest, _ transitionLogInput) (*mcp.CallToolResult, transitionLogOutput, error) {
	if err := execTransitionLog(); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
		}, transitionLogOutput{}, err
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: "transition log written"}},
	}, transitionLogOutput{OK: true}, nil
}

func versionBumpHandler(_ context.Context, _ *mcp.CallToolRequest, input versionBumpInput) (*mcp.CallToolResult, versionBumpOutput, error) {
	if err := execVersionBump(input.Major, input.Minor, input.Patch, input.Breaking, input.Version); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
		}, versionBumpOutput{}, err
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: "version bumped"}},
	}, versionBumpOutput{OK: true}, nil
}

func runTestsHandler(_ context.Context, _ *mcp.CallToolRequest, _ runTestsInput) (*mcp.CallToolResult, runTestsOutput, error) {
	if err := execTest(); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
		}, runTestsOutput{}, err
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: "tests passed"}},
	}, runTestsOutput{OK: true}, nil
}

func coauthorMCPHandler(_ context.Context, _ *mcp.CallToolRequest, input coauthorInput) (*mcp.CallToolResult, coauthorOutput, error) {
	if err := execCoauthor(input.Trailer); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
		}, coauthorOutput{}, err
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: "coauthor configured"}},
	}, coauthorOutput{OK: true}, nil
}
