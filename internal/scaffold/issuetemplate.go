package scaffold

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
)

const issueTemplateSrc = "templates/github/ISSUE_TEMPLATE/new-agent.yml"

// IssueTemplateRelPath is the repo-relative location of the new-agent issue form.
const IssueTemplateRelPath = ".github/ISSUE_TEMPLATE/new-agent.yml"

// InstallIssueTemplate writes the new-agent issue form. An existing file is
// left alone unless force is set; identical content is always a no-op.
func InstallIssueTemplate(repoRoot string, force bool) (Result, error) {
	target := filepath.Join(repoRoot, filepath.FromSlash(IssueTemplateRelPath))
	data, err := fs.ReadFile(TemplateFS, issueTemplateSrc)
	if err != nil {
		return Result{}, err
	}
	if existing, err := os.ReadFile(target); err == nil {
		if bytes.Equal(existing, data) {
			return Result{Path: target, Action: "unchanged"}, nil
		}
		if !force {
			return Result{Path: target, Action: "skipped (already exists)"}, nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return Result{}, err
	}
	if err := os.WriteFile(target, data, 0o644); err != nil {
		return Result{}, err
	}
	return Result{Path: target, Action: "installed"}, nil
}
