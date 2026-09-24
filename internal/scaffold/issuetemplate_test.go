package scaffold

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestInstallIssueTemplate(t *testing.T) {
	root := fakeGitRepo(t)
	if _, err := InstallIssueTemplate(root, false); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".github", "ISSUE_TEMPLATE", "new-agent.yml")
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var form struct {
		Labels []string `yaml:"labels"`
		Body   []struct {
			ID         string `yaml:"id"`
			Attributes struct {
				Options []string `yaml:"options"`
			} `yaml:"attributes"`
			Validations struct {
				Required bool `yaml:"required"`
			} `yaml:"validations"`
		} `yaml:"body"`
	}
	if err := yaml.Unmarshal(first, &form); err != nil {
		t.Fatalf("invalid YAML: %v", err)
	}
	if len(form.Labels) != 1 || form.Labels[0] != "new-agent" {
		t.Errorf("labels = %v", form.Labels)
	}
	want := []string{"name", "role", "rationale", "tool_tier", "routing", "acceptance_criteria"}
	if len(form.Body) != len(want) {
		t.Fatalf("fields = %d, want %d", len(form.Body), len(want))
	}
	for i, f := range form.Body {
		if f.ID != want[i] || !f.Validations.Required {
			t.Errorf("field %d = %q required=%v", i, f.ID, f.Validations.Required)
		}
	}
	if got := form.Body[3].Attributes.Options; len(got) != 4 || got[0] != "router" || got[3] != "write-only-no-edit" {
		t.Errorf("tier options = %v", got)
	}

	r, err := InstallIssueTemplate(root, false)
	if err != nil || r.Action != "unchanged" {
		t.Errorf("second run: %v %v", r, err)
	}
	if second, _ := os.ReadFile(path); string(second) != string(first) {
		t.Error("not idempotent")
	}
}

func TestInstallIssueTemplateSkipsModifiedUnlessForced(t *testing.T) {
	root := fakeGitRepo(t)
	path := filepath.Join(root, ".github", "ISSUE_TEMPLATE", "new-agent.yml")
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, []byte("custom"), 0o644)
	if r, _ := InstallIssueTemplate(root, false); r.Action != "skipped (already exists)" {
		t.Errorf("action = %q", r.Action)
	}
	if b, _ := os.ReadFile(path); string(b) != "custom" {
		t.Error("overwritten without force")
	}
	if _, err := InstallIssueTemplate(root, true); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); string(b) == "custom" {
		t.Error("force did not overwrite")
	}
}
