package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"dreamland/internal/config"
)

var coauthorCmd = &cobra.Command{
	Use:   "coauthor",
	Short: "Set agent git identity and install prepare-commit-msg hook",
	RunE:  runCoauthor,
}

var coauthorTrailer string

func init() {
	rootCmd.AddCommand(coauthorCmd)
	coauthorCmd.Flags().StringVar(&coauthorTrailer, "trailer", "", "commit message file path (prepare-commit-msg delegation mode)")
}

func runCoauthor(cmd *cobra.Command, args []string) error {
	cwd, err := osGetwd()
	if err != nil {
		return err
	}
	cfg, err := config.Load(cwd)
	if err != nil {
		return err
	}
	if cfg == nil {
		cfg = &config.Config{}
	}

	suffix := cfg.EmailSuffix
	if suffix == "" {
		suffix = "@github.com"
	}

	if coauthorTrailer != "" {
		// --trailer mode: invoked by prepare-commit-msg git hook.
		// args[0] (via --trailer flag value) is the commit message file path.
		return appendCoauthorTrailer(coauthorTrailer, cfg.ModelID, suffix)
	}

	// Default mode: set agent git identity and install the hook.
	agentName := resolveAgentName(cfg.CodingTool)
	agentEmail := config.EmailClean(agentName) + suffix

	if _, err := gitExec("config", "--local", "user.name", agentName); err != nil {
		return fmt.Errorf("git config user.name: %w", err)
	}
	if _, err := gitExec("config", "--local", "user.email", agentEmail); err != nil {
		return fmt.Errorf("git config user.email: %w", err)
	}

	return installPrepareCommitMsgHook(cwd)
}

// resolveAgentName returns the agent name from platform env vars or falls back to the coding tool name.
func resolveAgentName(codingTool string) string {
	for _, env := range []string{
		"CLAUDE_AGENT_ID",
		"CODEX_AGENT_ID",
		"CURSOR_AGENT_ID",
		"KIRO_AGENT_ID",
	} {
		if v := os.Getenv(env); v != "" {
			return v
		}
	}
	if codingTool != "" {
		return codingTool
	}
	return "dreamland"
}

const prepareCommitMsgContent = "#!/bin/sh\ndreamland coauthor --trailer \"$1\" \"$2\" \"$3\"\n"

func installPrepareCommitMsgHook(repoDir string) error {
	root, err := config.FindRepoRoot(repoDir)
	if err != nil {
		return err
	}
	hookPath := filepath.Join(root, ".git", "hooks", "prepare-commit-msg")

	// Idempotent: skip if already contains our delegation line.
	existing, readErr := os.ReadFile(hookPath)
	if readErr == nil && strings.Contains(string(existing), "dreamland coauthor --trailer") {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(hookPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(hookPath, []byte(prepareCommitMsgContent), 0o755)
}

// appendCoauthorTrailer appends a Co-authored-by trailer to the commit message file if not already present.
func appendCoauthorTrailer(msgFile, modelID, suffix string) error {
	if modelID == "" {
		return nil // no model configured, nothing to append
	}

	// Extract model name (text before first space).
	modelName := modelID
	if idx := strings.Index(modelID, " "); idx >= 0 {
		modelName = modelID[:idx]
	}
	modelEmail := config.EmailClean(modelName) + suffix
	trailer := fmt.Sprintf("Co-authored-by: %s <%s>", modelName, modelEmail)

	data, err := os.ReadFile(msgFile)
	if err != nil {
		return err
	}

	// Idempotent: do not append if trailer already present.
	if strings.Contains(string(data), "Co-authored-by: "+modelName) {
		return nil
	}

	content := string(data)
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += trailer + "\n"
	return os.WriteFile(msgFile, []byte(content), 0o644)
}
