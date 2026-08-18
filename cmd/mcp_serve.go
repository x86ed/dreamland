package cmd

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"dreamland/internal/config"
)

// mcpServeCmd runs a thin, optional stdio MCP server exposing dreamland oneiroi's
// seed/revise/fork operations as MCP tools — see design.md decision 8: no new business
// logic, every tool handler delegates to the exact same internal/oneiroi functions the
// `dreamland oneiroi` Cobra subcommands call.
var mcpServeCmd = &cobra.Command{
	Use:   "mcp-serve",
	Short: "Start the oneiroi MCP server (stdio)",
	RunE:  runMcpServe,
}

func init() {
	rootCmd.AddCommand(mcpServeCmd)
}

func runMcpServe(cmd *cobra.Command, args []string) error {
	cwd, err := osGetwd()
	if err != nil {
		return err
	}
	repoRoot, err := config.FindRepoRoot(cwd)
	if err != nil {
		return fmt.Errorf("not in a git repository: %w", err)
	}

	server, err := newOneiroiMCPServer(repoRoot)
	if err != nil {
		return err
	}

	fmt.Fprintln(cmd.ErrOrStderr(), "Starting oneiroi MCP server (stdio)...")
	return server.Run(context.Background(), &mcp.StdioTransport{})
}

// newOneiroiMCPServer builds the *mcp.Server exposing oneiroi_seed/oneiroi_revise/
// oneiroi_fork, bound to repoRoot. Extracted as its own testable seam so tests can
// connect an in-process client without going through the stdio transport.
func newOneiroiMCPServer(repoRoot string) (*mcp.Server, error) {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "dreamland-oneiroi",
		Version: "0.1.0",
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "oneiroi_seed",
		Description: "Generate a new collision-free 2-word oneiroi family name and scaffold it",
	}, oneiroiSeedToolHandler(repoRoot))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "oneiroi_revise",
		Description: "Draw a fresh third word for an existing oneiroi family",
	}, oneiroiReviseToolHandler(repoRoot))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "oneiroi_fork",
		Description: "Create a sibling agent sharing an existing family's word1/word2 pair",
	}, oneiroiForkToolHandler(repoRoot))

	return server, nil
}

type oneiroiSeedToolInput struct {
	Role     string `json:"role" description:"one-line role description"`
	ToolTier string `json:"tool_tier,omitempty" description:"router, read-dispatch-only, full-edit, or write-only-no-edit"`
}

type oneiroiToolOutput struct {
	Name   string `json:"name"`
	Parent string `json:"parent,omitempty"`
}

func oneiroiSeedToolHandler(repoRoot string) func(context.Context, *mcp.CallToolRequest, oneiroiSeedToolInput) (*mcp.CallToolResult, oneiroiToolOutput, error) {
	return func(_ context.Context, _ *mcp.CallToolRequest, input oneiroiSeedToolInput) (*mcp.CallToolResult, oneiroiToolOutput, error) {
		toolTier := input.ToolTier
		if toolTier == "" {
			toolTier = "full-edit"
		}
		name, err := oneiroiSeedCore(repoRoot, input.Role, toolTier)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
				IsError: true,
			}, oneiroiToolOutput{}, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: name}},
		}, oneiroiToolOutput{Name: name}, nil
	}
}

type oneiroiReviseToolInput struct {
	Agent  string `json:"agent" description:"existing agent name"`
	Reason string `json:"reason" description:"reason for the revision"`
}

func oneiroiReviseToolHandler(repoRoot string) func(context.Context, *mcp.CallToolRequest, oneiroiReviseToolInput) (*mcp.CallToolResult, oneiroiToolOutput, error) {
	return func(_ context.Context, _ *mcp.CallToolRequest, input oneiroiReviseToolInput) (*mcp.CallToolResult, oneiroiToolOutput, error) {
		name, err := oneiroiReviseCore(repoRoot, input.Agent, input.Reason)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
				IsError: true,
			}, oneiroiToolOutput{}, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: name}},
		}, oneiroiToolOutput{Name: name}, nil
	}
}

type oneiroiForkToolInput struct {
	Agent    string `json:"agent" description:"parent agent name"`
	Role     string `json:"role" description:"one-line role description"`
	ToolTier string `json:"tool_tier,omitempty" description:"inherited from parent unless overridden"`
}

func oneiroiForkToolHandler(repoRoot string) func(context.Context, *mcp.CallToolRequest, oneiroiForkToolInput) (*mcp.CallToolResult, oneiroiToolOutput, error) {
	return func(_ context.Context, _ *mcp.CallToolRequest, input oneiroiForkToolInput) (*mcp.CallToolResult, oneiroiToolOutput, error) {
		name, parent, err := oneiroiForkCore(repoRoot, input.Agent, input.Role, input.ToolTier)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
				IsError: true,
			}, oneiroiToolOutput{}, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: name}},
		}, oneiroiToolOutput{Name: name, Parent: parent}, nil
	}
}
