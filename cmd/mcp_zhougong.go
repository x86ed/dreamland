package cmd

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"dreamland/internal/config"
	"dreamland/internal/zhougongdash"
	"dreamland/internal/zhougongdata"
)

// mcpZhougongCmd runs the zhougong-only stdio MCP server. It is declared solely in
// zhougong's agent frontmatter (mcpServers), never in .mcp.json, so other agents
// cannot see its tools.
var mcpZhougongCmd = &cobra.Command{
	Use:   "mcp-zhougong",
	Short: "Start the zhougong metrics MCP server (stdio)",
	RunE:  runMcpZhougong,
}

func init() {
	rootCmd.AddCommand(mcpZhougongCmd)
}

func runMcpZhougong(cmd *cobra.Command, args []string) error {
	cwd, err := osGetwd()
	if err != nil {
		return err
	}
	repoRoot, err := config.FindRepoRoot(cwd)
	if err != nil {
		return fmt.Errorf("not in a git repository: %w", err)
	}
	fmt.Fprintln(cmd.ErrOrStderr(), "Starting zhougong MCP server (stdio)...")
	return newZhougongMCPServer(repoRoot).Run(context.Background(), &mcp.StdioTransport{})
}

type zhougongCollectInput struct {
	Branches []string `json:"branches" description:"branches to collect (max 8)"`
	Refresh  bool     `json:"refresh,omitempty" description:"re-parse branches that were already collected"`
}

type zhougongCollectOutput struct {
	Datasets        []zhougongdata.Summary `json:"datasets"`
	ComparisonTable string                 `json:"comparisonTable,omitempty"`
}

type zhougongStartInput struct {
	Port int `json:"port,omitempty" description:"port to bind on 127.0.0.1; 0 picks a free port"`
}

type zhougongURLOutput struct {
	URL string `json:"url"`
}

type zhougongEmpty struct{}

func toolError(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}}, IsError: true}
}

// newZhougongMCPServer builds the *mcp.Server exposing zhougong_collect,
// zhougong_dashboard_start and zhougong_dashboard_stop, bound to repoRoot. Extracted
// as a testable seam like newOneiroiMCPServer.
func newZhougongMCPServer(repoRoot string) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "dreamland-zhougong", Version: "0.1.0"}, nil)
	store := zhougongdash.NewStore(repoRoot)
	dash := zhougongdash.New(store)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "zhougong_collect",
		Description: "Parse per-run agent metrics (tokens, code lines, flow) from git for the named branches",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in zhougongCollectInput) (*mcp.CallToolResult, zhougongCollectOutput, error) {
		if err := zhougongdata.CheckMaxBranches(len(in.Branches)); err != nil {
			return toolError(err), zhougongCollectOutput{}, nil
		}
		if len(in.Branches) == 0 {
			return toolError(fmt.Errorf("at least one branch is required")), zhougongCollectOutput{}, nil
		}
		var parsed []zhougongdata.Dataset
		for _, b := range in.Branches {
			if !in.Refresh && store.Has(b) {
				parsed = append(parsed, store.Resolve(b))
				continue
			}
			ds, err := zhougongdata.ParseBranch(repoRoot, b)
			if err != nil {
				return toolError(err), zhougongCollectOutput{}, nil
			}
			parsed = append(parsed, ds)
		}
		out := zhougongCollectOutput{}
		for _, ds := range parsed {
			store.Put(ds)
			out.Datasets = append(out.Datasets, zhougongdata.Summarize(ds))
		}
		if len(parsed) > 1 {
			table, err := zhougongdata.MarkdownTable(parsed, "")
			if err != nil {
				return toolError(err), zhougongCollectOutput{}, nil
			}
			out.ComparisonTable = table
		}
		return nil, out, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "zhougong_dashboard_start",
		Description: "Start the localhost metrics dashboard (127.0.0.1 only) and return its URL",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in zhougongStartInput) (*mcp.CallToolResult, zhougongURLOutput, error) {
		url, err := dash.Start(in.Port)
		if err != nil {
			return toolError(err), zhougongURLOutput{}, nil
		}
		return nil, zhougongURLOutput{URL: url}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "zhougong_dashboard_stop",
		Description: "Stop the localhost metrics dashboard and release its port",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ zhougongEmpty) (*mcp.CallToolResult, zhougongEmpty, error) {
		if err := dash.Stop(); err != nil {
			return toolError(err), zhougongEmpty{}, nil
		}
		return nil, zhougongEmpty{}, nil
	})

	return server
}
