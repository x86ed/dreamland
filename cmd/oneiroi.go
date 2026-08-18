package cmd

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"dreamland/internal/config"
	"dreamland/internal/oneiroi"
	"dreamland/internal/oneiroi/seedwords"
	"dreamland/internal/scaffold"
)

var oneiroiCmd = &cobra.Command{
	Use:   "oneiroi",
	Short: "Seed, revise, or fork a dynamically-named oneiroi agent",
}

var (
	oneiroiSeedRole     string
	oneiroiSeedToolTier string

	oneiroiReviseAgent  string
	oneiroiReviseReason string

	oneiroiForkAgent    string
	oneiroiForkRole     string
	oneiroiForkToolTier string
)

func init() {
	oneiroi.GitExec = func(args ...string) (string, error) {
		return runCmd("git", args...)
	}

	rootCmd.AddCommand(oneiroiCmd)

	seedCmd := &cobra.Command{
		Use:   "seed",
		Short: "Generate a new collision-free 2-word oneiroi family name and scaffold it",
		RunE:  runOneiroiSeed,
	}
	seedCmd.Flags().StringVar(&oneiroiSeedRole, "role", "", "one-line role description")
	seedCmd.Flags().StringVar(&oneiroiSeedToolTier, "tool-tier", "full-edit", "router, read-dispatch-only, full-edit, or write-only-no-edit")
	oneiroiCmd.AddCommand(seedCmd)

	reviseCmd := &cobra.Command{
		Use:   "revise",
		Short: "Draw a fresh third word for an existing oneiroi family",
		RunE:  runOneiroiRevise,
	}
	reviseCmd.Flags().StringVar(&oneiroiReviseAgent, "agent", "", "existing agent name")
	reviseCmd.Flags().StringVar(&oneiroiReviseReason, "reason", "", "reason for the revision")
	oneiroiCmd.AddCommand(reviseCmd)

	forkCmd := &cobra.Command{
		Use:   "fork",
		Short: "Create a sibling agent sharing an existing family's word1/word2 pair",
		RunE:  runOneiroiFork,
	}
	forkCmd.Flags().StringVar(&oneiroiForkAgent, "agent", "", "parent agent name")
	forkCmd.Flags().StringVar(&oneiroiForkRole, "role", "", "one-line role description")
	forkCmd.Flags().StringVar(&oneiroiForkToolTier, "tool-tier", "", "inherited from parent unless overridden")
	oneiroiCmd.AddCommand(forkCmd)
}

// validOneiroiToolTiers is the closed set --tool-tier accepts, per the tool-binding
// matrix (agent-scaffolding capability).
var validOneiroiToolTiers = map[string]bool{
	"router":             true,
	"read-dispatch-only": true,
	"full-edit":          true,
	"write-only-no-edit": true,
}

func oneiroiRepoRoot() (string, error) {
	cwd, err := osGetwd()
	if err != nil {
		return "", err
	}
	return config.FindRepoRoot(cwd)
}

// resetFlagsToDefaults restores every flag on cmd to its registered default value and
// clears its Changed state. pflag's FlagSet.Parse only ever sets Changed to true, never
// resets it — so across repeated cobra Execute() calls in the same process (every CLI
// test in this package invokes execCLI this way), a package-level flag var and its
// Changed bit otherwise leak an earlier invocation's explicit value into a later one
// that omits the flag. Deferred at the top of each oneiroi RunE so the *next*
// invocation starts pristine.
func resetFlagsToDefaults(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		_ = f.Value.Set(f.DefValue)
		f.Changed = false
	})
}

func runOneiroiSeed(cmd *cobra.Command, args []string) error {
	defer resetFlagsToDefaults(cmd)

	repoRoot, err := oneiroiRepoRoot()
	if err != nil {
		return Blocking(err)
	}
	name, err := oneiroiSeedCore(repoRoot, oneiroiSeedRole, oneiroiSeedToolTier)
	if err != nil {
		return Blocking(err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), name)
	return nil
}

func runOneiroiRevise(cmd *cobra.Command, args []string) error {
	defer resetFlagsToDefaults(cmd)

	repoRoot, err := oneiroiRepoRoot()
	if err != nil {
		return Blocking(err)
	}
	name, err := oneiroiReviseCore(repoRoot, oneiroiReviseAgent, oneiroiReviseReason)
	if err != nil {
		return Blocking(err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), name)
	return nil
}

func runOneiroiFork(cmd *cobra.Command, args []string) error {
	defer resetFlagsToDefaults(cmd)

	repoRoot, err := oneiroiRepoRoot()
	if err != nil {
		return Blocking(err)
	}
	name, _, err := oneiroiForkCore(repoRoot, oneiroiForkAgent, oneiroiForkRole, oneiroiForkToolTier)
	if err != nil {
		return Blocking(err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), name)
	return nil
}

// oneiroiSeedCore implements `dreamland oneiroi seed`'s full behavior — name
// generation, registry write, scaffold, routing-table stub edges, and the
// self-authored commit — independent of the Cobra command layer, so both
// runOneiroiSeed (CLI) and the oneiroi_seed MCP tool handler (cmd/mcp_serve.go) call
// identical logic (design.md decision 8).
func oneiroiSeedCore(repoRoot, role, toolTier string) (string, error) {
	if !validOneiroiToolTiers[toolTier] {
		return "", fmt.Errorf("invalid --tool-tier %q; must be one of router, read-dispatch-only, full-edit, write-only-no-edit", toolTier)
	}

	pool, err := seedwords.Load()
	if err != nil {
		return "", fmt.Errorf("load seed words: %w", err)
	}

	reg, err := oneiroi.Load(repoRoot)
	if err != nil {
		return "", fmt.Errorf("load registry: %w", err)
	}

	word1, word2, err := oneiroi.GenerateFamily(pool, reg)
	if err != nil {
		return "", err
	}
	name := word1 + "-" + word2

	entry := oneiroi.Entry{
		Name:      name,
		Words:     []string{word1, word2},
		Role:      role,
		ToolTier:  toolTier,
		Parent:    nil,
		Created:   time.Now().UTC(),
		Revisions: []oneiroi.Revision{},
	}
	reg.Agents = append(reg.Agents, entry)

	paths, err := scaffoldOneiroiAgent(repoRoot, name, role, toolTier, reg)
	if err != nil {
		return "", err
	}

	subject := fmt.Sprintf("oneiroi: seed %s (%s tier)", name, toolTier)
	if err := oneiroi.CommitScaffold(repoRoot, paths, subject); err != nil {
		return "", err
	}

	return name, nil
}

// oneiroiReviseCore implements `dreamland oneiroi revise`'s full behavior.
func oneiroiReviseCore(repoRoot, agent, reason string) (string, error) {
	if agent == "" {
		return "", fmt.Errorf("--agent is required")
	}

	pool, err := seedwords.Load()
	if err != nil {
		return "", fmt.Errorf("load seed words: %w", err)
	}

	reg, err := oneiroi.Load(repoRoot)
	if err != nil {
		return "", fmt.Errorf("load registry: %w", err)
	}

	idx := -1
	for i, e := range reg.Agents {
		if e.Name == agent {
			idx = i
			break
		}
	}
	if idx == -1 {
		return "", fmt.Errorf("no registered oneiroi named %q", agent)
	}
	entry := &reg.Agents[idx]

	familyWords := entry.Words[:2]
	exclude := make([]string, 0, 1+len(entry.Revisions))
	if len(entry.Words) >= 3 {
		exclude = append(exclude, entry.Words[2])
	}
	for _, rev := range entry.Revisions {
		exclude = append(exclude, rev.Word)
	}

	word3, err := oneiroi.GenerateThirdWord(pool, familyWords, exclude)
	if err != nil {
		return "", err
	}

	oldName := entry.Name
	if len(entry.Words) >= 3 {
		entry.Revisions = append(entry.Revisions, oneiroi.Revision{
			Word:   entry.Words[2],
			Reason: reason,
			At:     time.Now().UTC(),
		})
		entry.Words[2] = word3
	} else {
		entry.Words = append(entry.Words, word3)
	}
	newName := entry.Words[0] + "-" + entry.Words[1] + "-" + entry.Words[2]
	entry.Name = newName

	paths, err := renameOneiroiAgentFiles(repoRoot, oldName, newName, reg)
	if err != nil {
		return "", err
	}

	subject := fmt.Sprintf("oneiroi: revise %s -> %s", oldName, newName)
	if err := oneiroi.CommitScaffold(repoRoot, paths, subject); err != nil {
		return "", err
	}

	return newName, nil
}

// oneiroiForkCore implements `dreamland oneiroi fork`'s full behavior, returning the
// new fork's name and its parent's name.
func oneiroiForkCore(repoRoot, agent, role, toolTierOverride string) (name, parentName string, err error) {
	if agent == "" {
		return "", "", fmt.Errorf("--agent is required")
	}

	pool, err := seedwords.Load()
	if err != nil {
		return "", "", fmt.Errorf("load seed words: %w", err)
	}

	reg, err := oneiroi.Load(repoRoot)
	if err != nil {
		return "", "", fmt.Errorf("load registry: %w", err)
	}

	var parent *oneiroi.Entry
	for i, e := range reg.Agents {
		if e.Name == agent {
			parent = &reg.Agents[i]
			break
		}
	}
	if parent == nil {
		return "", "", fmt.Errorf("no registered oneiroi named %q", agent)
	}
	if len(parent.Words) < 2 {
		return "", "", fmt.Errorf("parent oneiroi %q has fewer than 2 family words", agent)
	}

	familyWords := parent.Words[:2]
	var exclude []string
	if len(parent.Words) >= 3 {
		exclude = append(exclude, parent.Words[2])
	}
	for _, sibling := range reg.Agents {
		if sibling.Parent != nil && *sibling.Parent == parent.Name && len(sibling.Words) >= 3 {
			exclude = append(exclude, sibling.Words[2])
		}
	}

	word3, err := oneiroi.GenerateThirdWord(pool, familyWords, exclude)
	if err != nil {
		return "", "", err
	}

	toolTier := toolTierOverride
	if toolTier == "" {
		toolTier = parent.ToolTier
	}
	if !validOneiroiToolTiers[toolTier] {
		return "", "", fmt.Errorf("invalid --tool-tier %q; must be one of router, read-dispatch-only, full-edit, write-only-no-edit", toolTier)
	}

	forkName := familyWords[0] + "-" + familyWords[1] + "-" + word3
	parentName = parent.Name

	entry := oneiroi.Entry{
		Name:      forkName,
		Words:     []string{familyWords[0], familyWords[1], word3},
		Role:      role,
		ToolTier:  toolTier,
		Parent:    &parentName,
		Created:   time.Now().UTC(),
		Revisions: []oneiroi.Revision{},
	}
	reg.Agents = append(reg.Agents, entry)

	paths, err := scaffoldOneiroiAgent(repoRoot, forkName, role, toolTier, reg)
	if err != nil {
		return "", "", err
	}

	subject := fmt.Sprintf("oneiroi: fork %s from %s (%s tier)", forkName, parentName, toolTier)
	if err := oneiroi.CommitScaffold(repoRoot, paths, subject); err != nil {
		return "", "", err
	}

	return forkName, parentName, nil
}

// scaffoldOneiroiAgent writes the registry, the six stub agent files, the per-agent
// slash command, and the janus routing-table stub edges for a freshly seeded or
// forked oneiroi, and returns the full list of paths touched for CommitScaffold.
func scaffoldOneiroiAgent(repoRoot, name, role, toolTier string, reg *oneiroi.Registry) ([]string, error) {
	var paths []string

	scaffoldCfg := scaffold.Config{RepoRoot: repoRoot}

	stubResults, err := scaffold.InstallAgentStub(scaffoldCfg, name, role, toolTier)
	if err != nil {
		return nil, fmt.Errorf("install agent stub: %w", err)
	}
	for _, r := range stubResults {
		paths = append(paths, r.Path)
	}

	cmdResults, err := scaffold.InstallAgentCommand(scaffoldCfg, name)
	if err != nil {
		return nil, fmt.Errorf("install agent command: %w", err)
	}
	for _, r := range cmdResults {
		paths = append(paths, r.Path)
	}

	edgePaths, err := oneiroi.AddStubEdges(repoRoot, name)
	if err != nil {
		return nil, fmt.Errorf("add routing-table stub edges: %w", err)
	}
	paths = append(paths, edgePaths...)

	if err := reg.Save(repoRoot); err != nil {
		return nil, fmt.Errorf("save registry: %w", err)
	}
	paths = append(paths, registryFilePath(repoRoot))

	return paths, nil
}

// renameOneiroiAgentFiles rewrites an existing oneiroi's six stub/template files and its
// slash command file's `--agent-name`/frontmatter `name:` references from oldName to
// newName, and returns the full list of paths touched (including the registry file) for
// CommitScaffold.
func renameOneiroiAgentFiles(repoRoot, oldName, newName string, reg *oneiroi.Registry) ([]string, error) {
	paths, err := oneiroi.RenameAgentReferences(repoRoot, oldName, newName)
	if err != nil {
		return nil, fmt.Errorf("rename agent references: %w", err)
	}

	if err := reg.Save(repoRoot); err != nil {
		return nil, fmt.Errorf("save registry: %w", err)
	}
	paths = append(paths, registryFilePath(repoRoot))

	return paths, nil
}

func registryFilePath(repoRoot string) string {
	return filepath.Join(repoRoot, ".dreamland", "oneiroi", "registry.json")
}
