package oneiroi

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// janusPaths returns each supported platform's janus routing-table file path, relative
// to repoRoot, mirroring internal/scaffold's platformAgentSpec convention.
func janusPaths(repoRoot string) map[string]string {
	return map[string]string{
		"Claude Code":    filepath.Join(repoRoot, ".claude", "agents", "janus.md"),
		"Codex CLI":      filepath.Join(repoRoot, ".codex", "agents", "janus.toml"),
		"Cursor":         filepath.Join(repoRoot, ".cursor", "rules", "janus.mdc"),
		"Kiro":           filepath.Join(repoRoot, ".kiro", "steering", "janus.md"),
		"Antigravity":    filepath.Join(repoRoot, ".agents", "skills", "janus", "SKILL.md"),
		"GitHub Copilot": filepath.Join(repoRoot, ".github", "agents", "janus.agent.md"),
	}
}

// AddStubEdge appends a marked TODO line to platform's janus routing-table file,
// naming agentName as a delegation target for hypnos to finalize the tier/placement of
// (per design.md decision 5's "routing-table placement decision" non-goal). If the
// repository has not been scaffolded for platform (its janus file doesn't exist),
// AddStubEdge is a no-op — not an error — since there is nothing to add the agent to
// yet. If the marker for agentName is already present, AddStubEdge is idempotent.
// Returns whether the file was modified, so callers can build an exact commit path list.
func AddStubEdge(repoRoot, platform, agentName string) (touched bool, err error) {
	path, ok := janusPaths(repoRoot)[platform]
	if !ok {
		return false, fmt.Errorf("oneiroi: unknown platform %q", platform)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	marker := fmt.Sprintf("<!-- TODO(hypnos): add %s as a routing-table delegation target and finalize its tier/placement -->\n", agentName)
	content := string(data)
	if containsMarker(content, agentName) {
		return false, nil
	}
	if len(content) > 0 && content[len(content)-1] != '\n' {
		content += "\n"
	}
	content += marker

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

func containsMarker(content, agentName string) bool {
	want := fmt.Sprintf("TODO(hypnos): add %s as a routing-table delegation target", agentName)
	return strings.Contains(content, want)
}

// AddStubEdges calls AddStubEdge for every supported platform and returns the list of
// janus routing-table file paths that were actually modified (for the caller's commit
// path list).
func AddStubEdges(repoRoot, agentName string) ([]string, error) {
	var touchedPaths []string
	for platform, path := range janusPaths(repoRoot) {
		touched, err := AddStubEdge(repoRoot, platform, agentName)
		if err != nil {
			return touchedPaths, err
		}
		if touched {
			touchedPaths = append(touchedPaths, path)
		}
	}
	return touchedPaths, nil
}
