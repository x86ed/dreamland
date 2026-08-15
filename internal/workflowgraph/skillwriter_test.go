package workflowgraph

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateSkillWritesFileAndOwnerDreamland(t *testing.T) {
	root := newClaudeRepo(t)
	g := New(root)

	if err := CreateSkill(root, g, "my-skill", "Does a thing."); err != nil {
		t.Fatalf("CreateSkill: unexpected error %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".claude", "skills", "my-skill", "SKILL.md"))
	if err != nil {
		t.Fatalf("expected SKILL.md to be written: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "name: my-skill") || !strings.Contains(content, "Does a thing.") {
		t.Errorf("rendered skill file missing expected fields:\n%s", content)
	}
	if !strings.Contains(content, DreamlandManagedMarker) {
		t.Errorf("rendered skill file missing dreamland-managed marker:\n%s", content)
	}

	skill, ok := g.Skills["my-skill"]
	if !ok {
		t.Fatal("expected \"my-skill\" in g.Skills")
	}
	if skill.Owner != OwnerDreamland {
		t.Errorf("Owner = %q, want %q", skill.Owner, OwnerDreamland)
	}

	// Round-trip: re-importing must recognize it as owner: dreamland via the marker.
	reimported, err := Import(root)
	if err != nil {
		t.Fatalf("Import: unexpected error %v", err)
	}
	if s := reimported.Skills["my-skill"]; s == nil || s.Owner != OwnerDreamland {
		t.Errorf("re-imported skill = %+v, want Owner=dreamland", s)
	}
}

func TestCreateSkillDuplicateAndAgentCollisionErrors(t *testing.T) {
	root := newClaudeRepo(t)
	g := New(root)
	if err := CreateAgent(root, g, "hypnos", "desc", TierFullEdit); err != nil {
		t.Fatal(err)
	}

	if err := CreateSkill(root, g, "hypnos", "collides with an agent"); err == nil {
		t.Error("expected an error creating a skill whose id collides with an existing agent, got nil")
	}

	if err := CreateSkill(root, g, "ok-skill", "fine"); err != nil {
		t.Fatal(err)
	}
	if err := CreateSkill(root, g, "ok-skill", "duplicate"); err == nil {
		t.Error("expected an error creating a duplicate skill id, got nil")
	}
}

func TestDeleteSkillRefusesExternalOwner(t *testing.T) {
	root := newClaudeRepo(t)
	skillDir := filepath.Join(root, ".claude", "skills", "openspec-propose")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: openspec-propose\ndescription: x\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	g := New(root)
	if err := importSkills(root, g); err != nil {
		t.Fatal(err)
	}

	if err := DeleteSkill(root, g, "openspec-propose"); err == nil {
		t.Error("expected DeleteSkill to refuse an owner:external skill, got nil error")
	}
	if _, err := os.Stat(filepath.Join(skillDir, "SKILL.md")); err != nil {
		t.Errorf("expected external skill file untouched, stat err = %v", err)
	}
}

func TestDeleteSkillRemovesDreamlandOwnedSkill(t *testing.T) {
	root := newClaudeRepo(t)
	g := New(root)
	if err := CreateSkill(root, g, "removable", "desc"); err != nil {
		t.Fatal(err)
	}
	if err := CreateAgent(root, g, "user", "desc", TierFullEdit); err != nil {
		t.Fatal(err)
	}
	if err := AttachSkill(g, "user", "removable"); err != nil {
		t.Fatal(err)
	}

	if err := DeleteSkill(root, g, "removable"); err != nil {
		t.Fatalf("DeleteSkill: unexpected error %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "skills", "removable")); !os.IsNotExist(err) {
		t.Errorf("expected skill directory removed, stat err = %v", err)
	}
	if _, ok := g.Skills["removable"]; ok {
		t.Error("expected \"removable\" gone from g.Skills")
	}
	for _, e := range g.Edges {
		if e.Kind == EdgeAttachment && e.From == "removable" {
			t.Errorf("expected attachment edge cleaned up, found %+v", e)
		}
	}
}
