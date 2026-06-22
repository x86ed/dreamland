package cmd

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
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

func runServe(cmd *cobra.Command, args []string) error {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "dreamland",
		Version: "0.1.0",
	}, nil)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "hello",
		Description: "Say hello",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input helloInput) (*mcp.CallToolResult, helloOutput, error) {
		msg := fmt.Sprintf("Hello, %s!", input.Name)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: msg}},
		}, helloOutput{Message: msg}, nil
	})

	fmt.Fprintln(cmd.ErrOrStderr(), "Starting MCP server (stdio)...")
	return s.Run(context.Background(), &mcp.StdioTransport{})
}
