package cmd

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"dreamland/internal/agentissue"
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

type zhougongSnapshotInput struct {
	Branches []string `json:"branches,omitempty" description:"branches or archived record names (max 8); empty means every cached branch"`
	Baseline string   `json:"baseline,omitempty" description:"name to compute deltas against; defaults to the first branch"`
}

type zhougongSnapshotBranch struct {
	Name        string                   `json:"name"`
	Source      string                   `json:"source,omitempty"`
	Missing     bool                     `json:"missing,omitempty"`
	Stale       bool                     `json:"stale"`
	HeadSha     string                   `json:"headSha,omitempty"`
	CollectedAt string                   `json:"collectedAt,omitempty"`
	Summary     *zhougongdata.Summary    `json:"summary,omitempty"`
	Agents      []zhougongdata.AgentStat `json:"agents,omitempty"`
	Runs        []zhougongdata.Run       `json:"runs,omitempty"`
}

type zhougongSnapshotOutput struct {
	Branches    []zhougongSnapshotBranch    `json:"branches"`
	Comparison  *zhougongdata.CompareResult `json:"comparison,omitempty"`
	Attribution string                      `json:"attribution"`
}

type zhougongStartInput struct {
	Port int `json:"port,omitempty" description:"port to bind on 127.0.0.1; 0 picks a free port"`
}

type zhougongURLOutput struct {
	URL string `json:"url"`
}

type zhougongEmpty struct{}

type zhougongNewAgentIssueInput struct {
	Name      string `json:"name" description:"agent name (oneiroi-style)"`
	Role      string `json:"role" description:"one-line role"`
	Rationale string `json:"rationale" description:"rationale and evidence (report link, metrics)"`
	Tier      string `json:"tier" description:"router, read-dispatch-only, full-edit or write-only-no-edit"`
	Routing   string `json:"routing" description:"receives from / hands off to"`
	Criteria  string `json:"criteria" description:"acceptance criteria"`
	Confirm   bool   `json:"confirm,omitempty" description:"false (default) returns a preview and previewId only; true creates the issue and requires the previewId, after the user has approved the preview"`
	PreviewID string `json:"previewId,omitempty" description:"previewId returned by the confirm=false call"`
}

type zhougongNewAgentIssueOutput struct {
	Preview   string `json:"preview,omitempty"`
	PreviewID string `json:"previewId,omitempty"`
	URL       string `json:"url,omitempty"`
}

func toolError(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}}, IsError: true}
}

// newZhougongMCPServer builds the *mcp.Server exposing zhougong_collect,
// zhougong_snapshot, zhougong_dashboard_start, zhougong_dashboard_stop and zhougong_new_agent_issue, bound to repoRoot. Extracted
// as a testable seam like newOneiroiMCPServer.
func newZhougongMCPServer(repoRoot string) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "dreamland-zhougong", Version: "0.1.0"}, nil)
	store := zhougongdash.NewStore(repoRoot)

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
		var toWrite []zhougongdata.Entry
		for _, b := range in.Branches {
			sha, err := zhougongdata.HeadSha(repoRoot, b)
			if err != nil {
				return toolError(err), zhougongCollectOutput{}, nil
			}
			if !in.Refresh {
				if e, ok, err := zhougongdata.Read(repoRoot, b); err == nil && ok && !zhougongdata.IsStale(e, sha) {
					ds := e.Dataset
					ds.Name, ds.Source = e.Branch, "live"
					parsed = append(parsed, ds)
					continue
				}
			}
			ds, err := zhougongdata.ParseBranch(repoRoot, b)
			if err != nil {
				return toolError(err), zhougongCollectOutput{}, nil
			}
			parsed = append(parsed, ds)
			toWrite = append(toWrite, zhougongdata.Entry{Dataset: ds, HeadSha: sha})
		}
		for _, e := range toWrite {
			if err := zhougongdata.Write(repoRoot, e); err != nil {
				return toolError(err), zhougongCollectOutput{}, nil
			}
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
		Name:        "zhougong_snapshot",
		Description: "Return cached per-branch metrics (with collectedAt, stale, missing) and a precomputed comparison; call before answering any report or diff question",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in zhougongSnapshotInput) (*mcp.CallToolResult, zhougongSnapshotOutput, error) {
		if err := zhougongdata.CheckMaxBranches(len(in.Branches)); err != nil {
			return toolError(err), zhougongSnapshotOutput{}, nil
		}
		names := in.Branches
		if len(names) == 0 {
			for _, e := range zhougongdata.ReadAll(repoRoot) {
				names = append(names, e.Branch)
			}
			if err := zhougongdata.CheckMaxBranches(len(names)); err != nil {
				return toolError(err), zhougongSnapshotOutput{}, nil
			}
		}
		archived, err := zhougongdata.LoadArchived(repoRoot)
		if err != nil {
			return toolError(err), zhougongSnapshotOutput{}, nil
		}
		out := zhougongSnapshotOutput{Branches: []zhougongSnapshotBranch{}, Attribution: zhougongdash.AttributionNote}
		var sets []zhougongdata.Dataset
		for _, n := range names {
			var ds zhougongdata.Dataset
			sb := zhougongSnapshotBranch{Name: n}
			if e, ok, err := zhougongdata.Read(repoRoot, n); err == nil && ok {
				sha, shaErr := zhougongdata.HeadSha(repoRoot, n)
				ds = e.Dataset
				ds.Name, ds.Source = e.Branch, "live"
				sb.Source, sb.HeadSha, sb.CollectedAt = "live", e.HeadSha, e.CollectedAt
				sb.Stale = shaErr != nil || zhougongdata.IsStale(e, sha)
			} else if a, ok := findArchived(archived, n); ok {
				ds = a
				sb.Source, sb.CollectedAt = "archived", a.MergedAt
			} else {
				sb.Missing = true
				out.Branches = append(out.Branches, sb)
				continue
			}
			sum := zhougongdata.Summarize(ds)
			sb.Summary, sb.Agents, sb.Runs = &sum, zhougongdata.AgentStats(ds.Runs), ds.Runs
			out.Branches = append(out.Branches, sb)
			sets = append(sets, ds)
		}
		if len(sets) > 0 {
			cmp := zhougongdata.Compare(sets, in.Baseline)
			out.Comparison = &cmp
		}
		return nil, out, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "zhougong_dashboard_start",
		Description: "Start the localhost metrics dashboard (127.0.0.1 only) in a detached process that outlives this agent, and return its URL",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in zhougongStartInput) (*mcp.CallToolResult, zhougongURLOutput, error) {
		url, err := startZhougongDashboard(repoRoot, in.Port)
		if err != nil {
			return toolError(err), zhougongURLOutput{}, nil
		}
		return nil, zhougongURLOutput{URL: url}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "zhougong_dashboard_stop",
		Description: "Stop the localhost metrics dashboard and release its port",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ zhougongEmpty) (*mcp.CallToolResult, zhougongEmpty, error) {
		if err := stopZhougongDashboard(repoRoot); err != nil {
			return toolError(err), zhougongEmpty{}, nil
		}
		return nil, zhougongEmpty{}, nil
	})

	issues := agentissue.NewPreviewStore()
	mcp.AddTool(server, &mcp.Tool{
		Name:        "zhougong_new_agent_issue",
		Description: "Two-phase: confirm=false returns the rendered new-agent issue preview and a previewId without creating anything; only after the user explicitly approves the preview, call again with confirm=true and the same fields and previewId to create the GitHub issue",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in zhougongNewAgentIssueInput) (*mcp.CallToolResult, zhougongNewAgentIssueOutput, error) {
		f := agentissue.Fields{Name: in.Name, Role: in.Role, Rationale: in.Rationale, Tier: in.Tier, Routing: in.Routing, Criteria: in.Criteria}
		if !in.Confirm {
			id, err := issues.Put(f)
			if err != nil {
				return toolError(err), zhougongNewAgentIssueOutput{}, nil
			}
			return nil, zhougongNewAgentIssueOutput{Preview: f.Preview(), PreviewID: id}, nil
		}
		if err := issues.Take(in.PreviewID, f); err != nil {
			return toolError(err), zhougongNewAgentIssueOutput{}, nil
		}
		url, err := agentissue.Create(f)
		if err != nil {
			return toolError(err), zhougongNewAgentIssueOutput{}, nil
		}
		return nil, zhougongNewAgentIssueOutput{URL: url}, nil
	})

	return server
}

func findArchived(archived []zhougongdata.Dataset, name string) (zhougongdata.Dataset, bool) {
	for _, a := range archived {
		if a.Name == name {
			return a, true
		}
	}
	return zhougongdata.Dataset{}, false
}
