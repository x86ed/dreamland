package cmd

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

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

func runOneiroiSeed(cmd *cobra.Command, args []string) error {
	if !validOneiroiToolTiers[oneiroiSeedToolTier] {
		return Blocking(fmt.Errorf("invalid --tool-tier %q; must be one of router, read-dispatch-only, full-edit, write-only-no-edit", oneiroiSeedToolTier))
	}

	repoRoot, err := oneiroiRepoRoot()
	if err != nil {
		return Blocking(err)
	}

	pool, err := seedwords.Load()
	if err != nil {
		return Blocking(fmt.Errorf("load seed words: %w", err))
	}

	reg, err := oneiroi.Load(repoRoot)
	if err != nil {
		return Blocking(fmt.Errorf("load registry: %w", err))
	}

	word1, word2, err := oneiroi.GenerateFamily(pool, reg)
	if err != nil {
		return Blocking(err)
	}
	name := word1 + "-" + word2

	entry := oneiroi.Entry{
		Name:      name,
		Words:     []string{word1, word2},
		Role:      oneiroiSeedRole,
		ToolTier:  oneiroiSeedToolTier,
		Parent:    nil,
		Created:   time.Now().UTC(),
		Revisions: []oneiroi.Revision{},
	}
	reg.Agents = append(reg.Agents, entry)

	paths, err := scaffoldOneiroiAgent(repoRoot, name, oneiroiSeedRole, oneiroiSeedToolTier, reg)
	if err != nil {
		return Blocking(err)
	}

	subject := fmt.Sprintf("oneiroi: seed %s (%s tier)", name, oneiroiSeedToolTier)
	if err := oneiroi.CommitScaffold(repoRoot, paths, subject); err != nil {
		return Blocking(err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), name)
	return nil
}

func runOneiroiRevise(cmd *cobra.Command, args []string) error {
	if oneiroiReviseAgent == "" {
		return Blocking(fmt.Errorf("--agent is required"))
	}

	repoRoot, err := oneiroiRepoRoot()
	if err != nil {
		return Blocking(err)
	}

	pool, err := seedwords.Load()
	if err != nil {
		return Blocking(fmt.Errorf("load seed words: %w", err))
	}

	reg, err := oneiroi.Load(repoRoot)
	if err != nil {
		return Blocking(fmt.Errorf("load registry: %w", err))
	}

	idx := -1
	for i, e := range reg.Agents {
		if e.Name == oneiroiReviseAgent {
			idx = i
			break
		}
	}
	if idx == -1 {
		return Blocking(fmt.Errorf("no registered oneiroi named %q", oneiroiReviseAgent))
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
		return Blocking(err)
	}

	oldName := entry.Name
	if len(entry.Words) >= 3 {
		entry.Revisions = append(entry.Revisions, oneiroi.Revision{
			Word:   entry.Words[2],
			Reason: oneiroiReviseReason,
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
		return Blocking(err)
	}

	subject := fmt.Sprintf("oneiroi: revise %s -> %s", oldName, newName)
	if err := oneiroi.CommitScaffold(repoRoot, paths, subject); err != nil {
		return Blocking(err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), newName)
	return nil
}

func runOneiroiFork(cmd *cobra.Command, args []string) error {
	if oneiroiForkAgent == "" {
		return Blocking(fmt.Errorf("--agent is required"))
	}

	repoRoot, err := oneiroiRepoRoot()
	if err != nil {
		return Blocking(err)
	}

	pool, err := seedwords.Load()
	if err != nil {
		return Blocking(fmt.Errorf("load seed words: %w", err))
	}

	reg, err := oneiroi.Load(repoRoot)
	if err != nil {
		return Blocking(fmt.Errorf("load registry: %w", err))
	}

	var parent *oneiroi.Entry
	for i, e := range reg.Agents {
		if e.Name == oneiroiForkAgent {
			parent = &reg.Agents[i]
			break
		}
	}
	if parent == nil {
		return Blocking(fmt.Errorf("no registered oneiroi named %q", oneiroiForkAgent))
	}
	if len(parent.Words) < 2 {
		return Blocking(fmt.Errorf("parent oneiroi %q has fewer than 2 family words", oneiroiForkAgent))
	}

	familyWords := parent.Words[:2]
	exclude := []string{}
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
		return Blocking(err)
	}

	toolTier := oneiroiForkToolTier
	if toolTier == "" {
		toolTier = parent.ToolTier
	}
	if !validOneiroiToolTiers[toolTier] {
		return Blocking(fmt.Errorf("invalid --tool-tier %q; must be one of router, read-dispatch-only, full-edit, write-only-no-edit", toolTier))
	}

	role := oneiroiForkRole
	name := familyWords[0] + "-" + familyWords[1] + "-" + word3
	parentName := parent.Name

	entry := oneiroi.Entry{
		Name:      name,
		Words:     []string{familyWords[0], familyWords[1], word3},
		Role:      role,
		ToolTier:  toolTier,
		Parent:    &parentName,
		Created:   time.Now().UTC(),
		Revisions: []oneiroi.Revision{},
	}
	reg.Agents = append(reg.Agents, entry)

	paths, err := scaffoldOneiroiAgent(repoRoot, name, role, toolTier, reg)
	if err != nil {
		return Blocking(err)
	}

	subject := fmt.Sprintf("oneiroi: fork %s from %s (%s tier)", name, parentName, toolTier)
	if err := oneiroi.CommitScaffold(repoRoot, paths, subject); err != nil {
		return Blocking(err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), name)
	return nil
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
