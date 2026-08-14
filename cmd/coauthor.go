package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"dreamland/internal/agentidentity"
	"dreamland/internal/config"
	"dreamland/internal/telemetry"
)

var coauthorCmd = &cobra.Command{
	Use:   "coauthor",
	Short: "Set agent git identity and install prepare-commit-msg hook",
	RunE:  runCoauthor,
}

var (
	coauthorTrailer   string
	coauthorHook      bool
	coauthorAgentName string
)

func init() {
	rootCmd.AddCommand(coauthorCmd)
	coauthorCmd.Flags().StringVar(&coauthorTrailer, "trailer", "", "commit message file path (prepare-commit-msg delegation mode)")
	coauthorCmd.Flags().BoolVar(&coauthorHook, "hook", false, "set only by dreamland's own hook-binding templates; gates stdin read for hook payload")
	coauthorCmd.Flags().StringVar(&coauthorAgentName, "agent-name", "", "explicit agent name, takes precedence over env var / hook payload lookup")
}

func runCoauthor(cmd *cobra.Command, args []string) error {
	cwd, err := osGetwd()
	if err != nil {
		return err
	}
	cfg, err := config.Load(cwd)
	if err != nil {
		if errors.Is(err, config.ErrNoGitRepo) {
			return nil // skip outside git repo
		}
		return Blocking(err)
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
		if err := appendCoauthorTrailer(coauthorTrailer, cfg.ModelID, suffix); err != nil {
			return err
		}
		if err := appendCodingToolTrailer(coauthorTrailer, cfg.CodingTool, suffix); err != nil {
			return err
		}
		if repoRoot, rrErr := config.FindRepoRoot(cwd); rrErr == nil {
			return appendTokensReport(coauthorTrailer, repoRoot)
		}
		return nil
	}

	// Default mode: set agent git identity and install the hook. --agent-name is an
	// explicit override (from the agent-scoped Stop hook, which knows its own agent
	// identity statically) and takes precedence over the env/stdin agent_type lookup.
	agentName := coauthorAgentName
	if agentName == "" {
		agentName = resolveEnforcedAgentName(cfg)
		if coauthorHook {
			// --hook flag set: read hook payload from stdin (only when invoked by hook templates)
			if hookAgent := agentNameFromHookPayloadFrom(os.Stdin); hookAgent != "" && isRegisteredAgent(hookAgent) {
				agentName = hookAgent
			}
		}
	}
	agentEmail := config.EmailClean(agentName) + suffix

	if _, err := gitExec("config", "--local", "user.name", agentName); err != nil {
		return Blocking(fmt.Errorf("git config user.name: %w", err))
	}
	if _, err := gitExec("config", "--local", "user.email", agentEmail); err != nil {
		return Blocking(fmt.Errorf("git config user.email: %w", err))
	}

	if err := installPrepareCommitMsgHook(cwd); err != nil {
		return Blocking(err)
	}
	return nil
}

// isRegisteredAgent reports whether name is one of the ten registered dreamland
// agents — see the session-agent-identity capability for why a candidate identity
// resolved from a hook payload that isn't in this set must be treated as unresolved.
func isRegisteredAgent(name string) bool {
	return agentidentity.IsRegistered(name)
}

// resolveEnforcedAgentName returns the correct agent name using the full resolution
// sequence: hook payload (if --hook set), env vars, coding tool fallback, or janus for Claude Code.
// Extracted so it can be shared between coauthor and commit.
func resolveEnforcedAgentName(cfg *config.Config) string {
	agentName := resolveAgentName(cfg.CodingTool)
	// Claude Code has no per-agent env var and no sub-agent identifier on its
	// SessionStart/Stop/SubagentStop payloads (only on PreToolUse/PostToolUse for the
	// Task/Agent tool call itself) — so absent a valid hook-resolved identity, the coding
	// tool name is not a real agent and must not become the git identity. Other platforms
	// keep the coding-tool-name fallback unchanged.
	if cfg.CodingTool == "Claude Code" && !isRegisteredAgent(agentName) {
		agentName = "janus"
	}
	return agentName
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

// agentNameFromHookPayloadFrom reads a JSON hook payload from the provided reader and
// extracts the acting sub-agent's identity if present. It does a synchronous read to EOF
// with no timeout (correct because hook-binding callers always write and close promptly).
// Confirmed from live GitHub Copilot SubagentStart/SubagentStop hook payloads: the field
// is "agent_type" (e.g. "morpheus", "iktomi") — undocumented but consistently present.
// Claude Code carries no top-level "agent_type" at all; the sub-agent identifier only
// appears as "tool_input.subagent_type" on the PreToolUse/PostToolUse payload for the
// Task/Agent tool call itself. SessionStart/Stop/SubagentStop payloads on Claude Code
// (which aren't about a specific sub-agent, or don't carry the tool call's input) don't
// carry either field, so the existing env-var/coding-tool fallback in resolveAgentName
// still applies for those. Returns "" whenever no matching payload is found.
func agentNameFromHookPayloadFrom(r io.Reader) string {
	data, err := io.ReadAll(io.LimitReader(r, 1<<16))
	if err != nil || len(data) == 0 {
		return ""
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return ""
	}
	return agentidentity.FromPayload(payload)
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

// appendCoauthorTrailer appends a Co-authored-by trailer for the model to the commit
// message file if not already present.
func appendCoauthorTrailer(msgFile, modelID, suffix string) error {
	if modelID == "" {
		return nil // no model configured, nothing to append
	}

	// Extract model name (text before first space) — model IDs sometimes carry a
	// trailing description (e.g. "claude-sonnet-5 (preview)"), so this is deliberate
	// truncation, unlike appendCodingToolTrailer's verbatim name.
	modelName := modelID
	if idx := strings.Index(modelID, " "); idx >= 0 {
		modelName = modelID[:idx]
	}
	modelEmail := config.EmailClean(modelName) + suffix
	return appendTrailerLine(msgFile, modelName, modelEmail)
}

// appendCodingToolTrailer appends a second Co-authored-by trailer naming the coding
// tool itself (e.g. "Claude Code", "GitHub Copilot") to the commit message file if
// not already present. Unlike the model trailer, the name is used verbatim — coding
// tool names routinely contain a space and must not be truncated at it.
func appendCodingToolTrailer(msgFile, codingTool, suffix string) error {
	if codingTool == "" {
		return nil // no coding tool configured, nothing to append
	}
	toolEmail := config.EmailClean(codingTool) + suffix
	return appendTrailerLine(msgFile, codingTool, toolEmail)
}

// appendTrailerLine appends "Co-authored-by: <name> <email>" to the commit message
// file if a line naming <name> is not already present. Idempotent per name, so the
// model and coding-tool trailers are independently deduped against each other.
func appendTrailerLine(msgFile, name, email string) error {
	trailer := fmt.Sprintf("Co-authored-by: %s <%s>", name, email)

	data, err := os.ReadFile(msgFile)
	if err != nil {
		return err
	}

	// Idempotent: do not append if a trailer for this name is already present.
	if strings.Contains(string(data), "Co-authored-by: "+name) {
		return nil
	}

	content := string(data)
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += trailer + "\n"
	return os.WriteFile(msgFile, []byte(content), 0o644)
}

// appendTokensReport appends a Tokens: report line to the commit message file,
// sourced from the current turn's telemetry snapshot. Silently omitted (not a
// failure) when no telemetry data is available.
func appendTokensReport(msgFile, repoRoot string) error {
	snap, err := telemetry.Read(repoRoot)
	if err != nil || snap == nil {
		return nil
	}
	// All-zero is indistinguishable from "no data" for platforms whose collector has no
	// real source for token counts (e.g. GitHub Copilot's transcript exposes no usage
	// field at all) — a fake "input=0 output=0..." line is worse than omitting it.
	if snap.InputTokens == 0 && snap.OutputTokens == 0 && snap.CachedTokens == 0 && snap.TotalTokens == 0 {
		return nil
	}

	data, err := os.ReadFile(msgFile)
	if err != nil {
		return err
	}
	content := string(data)
	if strings.Contains(content, "Tokens: ") {
		return nil // idempotent
	}

	line := fmt.Sprintf("Tokens: input=%d output=%d cached=%d total=%d",
		snap.InputTokens, snap.OutputTokens, snap.CachedTokens, snap.TotalTokens)

	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += line + "\n"
	return os.WriteFile(msgFile, []byte(content), 0o644)
}
