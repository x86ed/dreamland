package oneiroi

import (
	"os"
	"path/filepath"
	"strings"
)

// agentStubPaths mirrors internal/scaffold's platformAgentSpec per-platform path
// convention (duplicated here rather than imported, matching the existing pattern in
// this repo's own tests, so this package has no dependency on internal/scaffold).
func agentStubPaths(repoRoot, name string) map[string]string {
	return map[string]string{
		"Claude Code":    filepath.Join(repoRoot, ".claude", "agents", name+".md"),
		"Codex CLI":      filepath.Join(repoRoot, ".codex", "agents", name+".toml"),
		"Cursor":         filepath.Join(repoRoot, ".cursor", "rules", name+".mdc"),
		"Kiro":           filepath.Join(repoRoot, ".kiro", "steering", name+".md"),
		"Antigravity":    filepath.Join(repoRoot, ".agents", "skills", name, "SKILL.md"),
		"GitHub Copilot": filepath.Join(repoRoot, ".github", "agents", name+".agent.md"),
	}
}

// agentCommandPaths mirrors internal/scaffold's platformCommandSpec per-agent slash
// command path convention.
func agentCommandPaths(repoRoot, name string) map[string]string {
	return map[string]string{
		"Claude Code":    filepath.Join(repoRoot, ".claude", "commands", "drmlnd", name+".md"),
		"Cursor":         filepath.Join(repoRoot, ".cursor", "commands", name+".md"),
		"GitHub Copilot": filepath.Join(repoRoot, ".github", "prompts", name+".prompt.md"),
		"Kiro":           filepath.Join(repoRoot, ".kiro", "steering", "drmlnd-"+name+".md"),
		"Antigravity":    filepath.Join(repoRoot, ".agents", "skills", "drmlnd-"+name+".md"),
		"Codex CLI":      filepath.Join(repoRoot, ".codex", "skills", "drmlnd-"+name, "SKILL.md"),
	}
}

// RenameAgentReferences rewrites an existing oneiroi's six stub/template files and its
// per-platform slash command files: each file at oldName's path is moved to newName's
// path (per platform's own naming convention — the command paths' drmlnd-prefixed
// filenames included), and every literal occurrence of oldName within the file content
// (--agent-name references, frontmatter name: fields) is replaced with newName. A
// platform whose file doesn't exist for oldName (never scaffolded in this repo) is
// skipped, not an error. Returns every new path written, for CommitScaffold's path list.
func RenameAgentReferences(repoRoot, oldName, newName string) ([]string, error) {
	var touched []string

	oldStubPaths := agentStubPaths(repoRoot, oldName)
	newStubPaths := agentStubPaths(repoRoot, newName)
	for platform, oldPath := range oldStubPaths {
		newPath := newStubPaths[platform]
		ok, err := renameAndRewrite(oldPath, newPath, oldName, newName)
		if err != nil {
			return touched, err
		}
		if ok {
			touched = append(touched, newPath)
			if newPath != oldPath {
				touched = append(touched, oldPath) // stage the deletion at the old path
			}
		}
	}

	oldCmdPaths := agentCommandPaths(repoRoot, oldName)
	newCmdPaths := agentCommandPaths(repoRoot, newName)
	for platform, oldPath := range oldCmdPaths {
		newPath := newCmdPaths[platform]
		ok, err := renameAndRewrite(oldPath, newPath, oldName, newName)
		if err != nil {
			return touched, err
		}
		if ok {
			touched = append(touched, newPath)
			if newPath != oldPath {
				touched = append(touched, oldPath) // stage the deletion at the old path
			}
		}
	}

	return touched, nil
}

// renameAndRewrite moves oldPath to newPath (if oldPath exists) and replaces every
// literal occurrence of oldName with newName in the moved file's content.
func renameAndRewrite(oldPath, newPath, oldName, newName string) (bool, error) {
	data, err := os.ReadFile(oldPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	content := strings.ReplaceAll(string(data), oldName, newName)

	if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
		return false, err
	}
	if err := os.WriteFile(newPath, []byte(content), 0o644); err != nil {
		return false, err
	}
	if newPath != oldPath {
		if err := os.Remove(oldPath); err != nil {
			return false, err
		}
	}
	return true, nil
}
