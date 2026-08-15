package workflowgraph

import (
	"fmt"
	"os"
	"path/filepath"
)

// skillCapablePlatforms are the platforms with a directory-per-skill
// convention this writer can target (see platformSkillDirs, import.go).
// Cursor/Kiro/GitHub Copilot have no general-purpose skill concept — only a
// fixed slash-command mechanism — so they're not included here.
var skillCapablePlatforms = map[string]bool{
	"claude-code": true,
	"codex":       true,
	"antigravity": true,
}

// installedSkillPlatforms returns the installed platforms (agent directory
// present) that also support skills — creating a skill doesn't require the
// platform's skills/ directory to already exist, only the platform itself.
func installedSkillPlatforms(repoRoot string) []string {
	var out []string
	for _, p := range installedPlatforms(repoRoot) {
		if skillCapablePlatforms[p] {
			out = append(out, p)
		}
	}
	return out
}

// renderSkillFile renders a dreamland-authored SKILL.md: name/description
// frontmatter plus DreamlandManagedMarker, so a future live-rebuild import
// recognizes it as owner: dreamland rather than owner: external. Deliberately
// does not claim the openspec CLI's own metadata.generatedBy/license/
// compatibility fields — those describe openspec's skills, not dreamland's.
func renderSkillFile(id, description string) string {
	return fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n\n%s\n", id, description, DreamlandManagedMarker)
}

// CreateSkill writes a new SKILL.md for id on every installed skill-capable
// platform and adds the corresponding SkillNode (owner: dreamland). On
// Antigravity, id must not collide with an existing agent id — that platform
// shares one directory (.agents/skills/) between agent personas and skills.
func CreateSkill(repoRoot string, g *Graph, id, description string) error {
	if _, exists := g.Skills[id]; exists {
		return fmt.Errorf("CreateSkill: skill %q already exists", id)
	}
	if _, isAgent := g.Agents[id]; isAgent {
		return fmt.Errorf("CreateSkill: id %q collides with an existing agent id", id)
	}

	content := renderSkillFile(id, description)
	for _, platform := range installedSkillPlatforms(repoRoot) {
		dir := filepath.Join(repoRoot, platformSkillDirs[platform], id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
			return err
		}
	}

	g.Skills[id] = &SkillNode{ID: id, Description: description, Owner: OwnerDreamland}
	return nil
}

// DeleteSkill removes a dreamland-owned skill's files from every installed
// skill-capable platform and drops it (and any attachment edges to it) from
// the graph. Refuses to delete an owner: external skill — those files aren't
// dreamland's to remove (mirrors the read/attach-only boundary on attach).
func DeleteSkill(repoRoot string, g *Graph, id string) error {
	skill, ok := g.Skills[id]
	if !ok {
		return fmt.Errorf("DeleteSkill: unknown skill %q", id)
	}
	if skill.Owner != OwnerDreamland {
		return fmt.Errorf("DeleteSkill: skill %q is owner:%s, not dreamland's to delete", id, skill.Owner)
	}

	for _, platform := range installedSkillPlatforms(repoRoot) {
		dir := filepath.Join(repoRoot, platformSkillDirs[platform], id)
		if err := os.RemoveAll(dir); err != nil {
			return fmt.Errorf("remove %s skill dir: %w", platform, err)
		}
	}

	var remaining []Edge
	for _, e := range g.Edges {
		if e.Kind == EdgeAttachment && e.From == id {
			continue
		}
		remaining = append(remaining, e)
	}
	g.Edges = remaining
	delete(g.Skills, id)
	return nil
}
