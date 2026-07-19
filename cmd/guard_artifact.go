package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/spf13/cobra"

	"dreamland/internal/config"
)

var guardArtifactCmd = &cobra.Command{
	Use:   "guard-artifact",
	Short: "PreToolUse (Write|Edit) guard enforcing fixed artifact ownership",
	RunE:  runGuardArtifact,
}

func init() {
	rootCmd.AddCommand(guardArtifactCmd)
}

// guardArtifactExit is os.Exit, exposed as a variable so tests can stub it instead of
// terminating the test process.
var guardArtifactExit = os.Exit

// artifactOwner maps a protected path pattern (matched against a repo-root-relative,
// slash-normalized path) to the sole agent allowed to write there.
type artifactOwner struct {
	pattern *regexp.Regexp
	owner   string
}

// artifactOwners is the fixed path -> agent ownership table. Paths not matching any
// entry here are unrestricted, regardless of which agent writes them.
var artifactOwners = []artifactOwner{
	{regexp.MustCompile(`^openspec/changes/[^/]+/(proposal|design|tasks)\.md$`), "phantasos"},
	{regexp.MustCompile(`^openspec/changes/[^/]+/specs/.*\.md$`), "phantasos"},
	{regexp.MustCompile(`^openspec/specs/.*/spec\.md$`), "phantasos"},
	{regexp.MustCompile(`^internal/scaffold/templates/(agents|commands)/`), "hypnos"},
	{regexp.MustCompile(`^\.dreamland/reports/[^/]+\.md$`), "zhougong"},
}

// hookToolInputPayload is the subset of a PreToolUse hook JSON payload guard-artifact
// needs: the acting agent's identity and the target file path of a Write/Edit call.
type hookToolInputPayload struct {
	AgentType string `json:"agent_type"`
	ToolInput struct {
		FilePath string `json:"file_path"`
	} `json:"tool_input"`
}

func runGuardArtifact(cmd *cobra.Command, _ []string) error {
	data, err := io.ReadAll(cmd.InOrStdin())
	if err != nil || len(data) == 0 {
		return nil // no payload at all — nothing to check, fail-open
	}

	var payload hookToolInputPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil // malformed payload — not this command's job to validate hook shape
	}

	repoRoot := ""
	if cwd, gwErr := osGetwd(); gwErr == nil {
		if rr, rrErr := config.FindRepoRoot(cwd); rrErr == nil {
			repoRoot = rr
		}
	}

	if reason := checkArtifactOwnership(payload, repoRoot); reason != "" {
		fmt.Fprintln(cmd.ErrOrStderr(), reason)
		guardArtifactExit(2)
	}
	return nil
}

// checkArtifactOwnership returns a non-empty block reason if payload's agent_type is
// not the declared owner of its tool_input.file_path, or "" if the write is allowed
// (unprotected path, correct owner, or missing agent_type/file_path — fail-open on
// unknown identity, fail-closed only on a known-wrong one).
func checkArtifactOwnership(payload hookToolInputPayload, repoRoot string) string {
	if payload.AgentType == "" || payload.ToolInput.FilePath == "" {
		return ""
	}

	relPath := payload.ToolInput.FilePath
	if repoRoot != "" {
		if rel, err := filepath.Rel(repoRoot, payload.ToolInput.FilePath); err == nil {
			relPath = rel
		}
	}
	relPath = filepath.ToSlash(relPath)

	for _, o := range artifactOwners {
		if o.pattern.MatchString(relPath) && payload.AgentType != o.owner {
			return fmt.Sprintf("blocked: %s is owned by %s, not %s", relPath, o.owner, payload.AgentType)
		}
	}
	return ""
}
