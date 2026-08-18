package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mcpOneiroiTestRepo creates a temp git repo and chdirs into it, mirroring
// oneiroiGitRepo (oneiroi_test.go) — the MCP oneiroi_seed handler resolves its repo
// root the same way the CLI's runOneiroiSeed does (config.FindRepoRoot against cwd),
// per decision 8 in design.md ("delegating to identical Go logic").
func mcpOneiroiTestRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git", "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	orig, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	stubRunCmd(t, func(_ string, _ ...string) (string, error) { return "", nil })

	return root
}

// connectInMemoryOneiroiMCPClient wires a fake MCP client to newOneiroiMCPServer's
// server (the expected testable seam for cmd/mcp_serve.go, task 8.2 — a small factory
// extracted from runMcpServe so callers other than the stdio transport, like this test,
// can obtain the configured *mcp.Server directly) over an in-process pipe
// (mcp.NewInMemoryTransports), matching task 8.3's "fake MCP client ... over an
// in-process pipe" requirement.
func connectInMemoryOneiroiMCPClient(t *testing.T, repoRoot string) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()

	server, err := newOneiroiMCPServer(repoRoot)
	if err != nil {
		t.Fatalf("newOneiroiMCPServer: %v", err)
	}

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("server.Connect: %v", err)
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "oneiroi-test-client", Version: "v0.0.1"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

// TestMCPOneiroiSeed_ToolsListIncludesOneiroiTools covers "CLI seed command available"'s
// MCP counterpart: oneiroi_seed, oneiroi_revise, and oneiroi_fork must be discoverable
// via a real tools/list round trip, not just callable if the name is guessed.
func TestMCPOneiroiSeed_ToolsListIncludesOneiroiTools(t *testing.T) {
	root := mcpOneiroiTestRepo(t)
	session := connectInMemoryOneiroiMCPClient(t, root)

	result, err := session.ListTools(context.Background(), &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	want := map[string]bool{"oneiroi_seed": false, "oneiroi_revise": false, "oneiroi_fork": false}
	for _, tool := range result.Tools {
		if _, ok := want[tool.Name]; ok {
			want[tool.Name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("tools/list missing %q", name)
		}
	}
}

// TestMCPOneiroiSeed_ToolCallMatchesDirectCLIInvocation covers the "MCP tool produces
// the same result as the equivalent CLI invocation" scenario (oneiroi-seed-naming
// capability): a fake MCP client's tools/call for oneiroi_seed with {"role": "example"}
// over an in-process pipe must produce a registry entry, stub files, and slash command
// indistinguishable in shape from `dreamland oneiroi seed --role "example"` run
// directly — not byte-identical (the generated family name is drawn independently in
// each of the two repos), but the same structural shape: a single 2-word, no-parent,
// full-edit-tier entry, stub files on all six platforms, and a slash command file.
func TestMCPOneiroiSeed_ToolCallMatchesDirectCLIInvocation(t *testing.T) {
	mcpRoot := mcpOneiroiTestRepo(t)
	session := connectInMemoryOneiroiMCPClient(t, mcpRoot)

	callResult, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "oneiroi_seed",
		Arguments: map[string]any{"role": "example"},
	})
	if err != nil {
		t.Fatalf("CallTool oneiroi_seed: %v", err)
	}
	if callResult.IsError {
		t.Fatalf("oneiroi_seed tool call reported an error result: %+v", callResult.Content)
	}

	mcpReg := readOneiroiRegistry(t, mcpRoot)
	if len(mcpReg.Agents) != 1 {
		t.Fatalf("expected exactly one registry entry via MCP, got %d: %+v", len(mcpReg.Agents), mcpReg.Agents)
	}
	mcpEntry := mcpReg.Agents[0]

	// Direct CLI invocation in a separate repo, for shape comparison.
	cliRoot := oneiroiGitRepo(t)
	if _, _, err := execCLI(t, "oneiroi", "seed", "--role", "example"); err != nil {
		t.Fatalf("oneiroi seed (direct CLI): %v", err)
	}
	cliReg := readOneiroiRegistry(t, cliRoot)
	if len(cliReg.Agents) != 1 {
		t.Fatalf("expected exactly one registry entry via direct CLI, got %d: %+v", len(cliReg.Agents), cliReg.Agents)
	}
	cliEntry := cliReg.Agents[0]

	if len(mcpEntry.Words) != len(cliEntry.Words) {
		t.Errorf("word count differs: MCP=%v, CLI=%v", mcpEntry.Words, cliEntry.Words)
	}
	if mcpEntry.Role != cliEntry.Role {
		t.Errorf("role differs: MCP=%q, CLI=%q", mcpEntry.Role, cliEntry.Role)
	}
	if mcpEntry.ToolTier != cliEntry.ToolTier {
		t.Errorf("tool_tier differs: MCP=%q, CLI=%q", mcpEntry.ToolTier, cliEntry.ToolTier)
	}
	if (mcpEntry.Parent == nil) != (cliEntry.Parent == nil) {
		t.Errorf("parent-nilness differs: MCP=%v, CLI=%v", mcpEntry.Parent, cliEntry.Parent)
	}

	// Stub files on every supported platform, for the MCP-driven repo.
	for platform, path := range stubAgentPathsForCmdTest(mcpRoot, mcpEntry.Name) {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s: missing stub agent file %s (via MCP): %v", platform, path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(mcpRoot, ".claude", "commands", "drmlnd", mcpEntry.Name+".md")); err != nil {
		t.Errorf("missing slash command file for %s (via MCP): %v", mcpEntry.Name, err)
	}
}
