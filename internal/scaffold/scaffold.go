package scaffold

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Config drives a scaffold installation.
type Config struct {
	RepoRoot   string
	CodingTool string
	Force      bool
}

// Result describes what happened to one file during installation.
type Result struct {
	Path   string
	Action string // "installed", "installed (forced)", "skipped (already exists)", or a note
}

// Install installs agent files and hook bindings for the configured coding tool.
func Install(cfg Config) ([]Result, error) {
	var results []Result

	agentResults, err := installAgents(cfg)
	if err != nil {
		return results, err
	}
	results = append(results, agentResults...)

	hookResults, err := bindHooks(cfg)
	if err != nil {
		return results, err
	}
	results = append(results, hookResults...)

	return results, nil
}

// platformSpec maps a coding tool name to its template and target directories.
// platformSpec maps a coding tool name to its template and target directories.
// skillFile is set for platforms that use a directory-per-skill layout (e.g. Antigravity).
// When skillFile is non-empty, each subdirectory in templateDir becomes a skill directory
// under targetDir, and skillFile is the filename written inside each skill directory.
type platformSpec struct {
	templateDir string
	targetDir   string
	skillFile   string // non-empty → directory-per-skill layout
}

func platformAgentSpec(tool, repoRoot string) (platformSpec, bool) {
	specs := map[string]platformSpec{
		"Claude Code":    {templateDir: "agents/claude-code", targetDir: filepath.Join(repoRoot, ".claude", "agents")},
		"Codex CLI":      {templateDir: "agents/codex", targetDir: filepath.Join(repoRoot, ".codex", "agents")},
		"Cursor":         {templateDir: "agents/cursor", targetDir: filepath.Join(repoRoot, ".cursor", "rules")},
		"Kiro":           {templateDir: "agents/kiro", targetDir: filepath.Join(repoRoot, ".kiro", "steering")},
		"Antigravity":    {templateDir: "agents/antigravity", targetDir: filepath.Join(repoRoot, ".agents", "skills"), skillFile: "SKILL.md"},
		"GitHub Copilot": {templateDir: "agents/github-copilot", targetDir: filepath.Join(repoRoot, ".github", "agents")},
	}
	spec, ok := specs[tool]
	return spec, ok
}

func installAgents(cfg Config) ([]Result, error) {
	spec, ok := platformAgentSpec(cfg.CodingTool, cfg.RepoRoot)
	if !ok {
		return nil, fmt.Errorf("unknown coding tool %q", cfg.CodingTool)
	}

	if spec.skillFile != "" {
		return installSkills(cfg, spec)
	}
	return installFlatAgents(cfg, spec)
}

// installFlatAgents copies each file in templateDir directly into targetDir (all platforms except Antigravity).
func installFlatAgents(cfg Config, spec platformSpec) ([]Result, error) {
	if err := os.MkdirAll(spec.targetDir, 0o755); err != nil {
		return nil, fmt.Errorf("create agent dir: %w", err)
	}

	var results []Result
	templateDir := "templates/" + spec.templateDir
	entries, err := fs.ReadDir(TemplateFS, templateDir)
	if err != nil {
		return nil, fmt.Errorf("read template dir %q: %w", templateDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		targetPath := filepath.Join(spec.targetDir, entry.Name())

		if _, err := os.Stat(targetPath); err == nil && !cfg.Force {
			results = append(results, Result{Path: targetPath, Action: "skipped (already exists)"})
			continue
		}

		data, err := fs.ReadFile(TemplateFS, templateDir+"/"+entry.Name())
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(targetPath, data, 0o644); err != nil {
			return nil, err
		}
		action := "installed"
		if cfg.Force {
			action = "installed (forced)"
		}
		results = append(results, Result{Path: targetPath, Action: action})
	}

	return results, nil
}

// installSkills writes directory-per-skill layout: for each subdirectory in templateDir,
// creates targetDir/<skill-name>/<skillFile> (used by Antigravity).
func installSkills(cfg Config, spec platformSpec) ([]Result, error) {
	if err := os.MkdirAll(spec.targetDir, 0o755); err != nil {
		return nil, fmt.Errorf("create skills dir: %w", err)
	}

	var results []Result
	templateDir := "templates/" + spec.templateDir
	entries, err := fs.ReadDir(TemplateFS, templateDir)
	if err != nil {
		return nil, fmt.Errorf("read template dir %q: %w", templateDir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue // skills are directories; skip stray files
		}
		skillName := entry.Name()
		skillDir := filepath.Join(spec.targetDir, skillName)
		targetPath := filepath.Join(skillDir, spec.skillFile)

		if _, err := os.Stat(targetPath); err == nil && !cfg.Force {
			results = append(results, Result{Path: targetPath, Action: "skipped (already exists)"})
			continue
		}

		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			return nil, err
		}

		data, err := fs.ReadFile(TemplateFS, templateDir+"/"+skillName+"/"+spec.skillFile)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(targetPath, data, 0o644); err != nil {
			return nil, err
		}
		action := "installed"
		if cfg.Force {
			action = "installed (forced)"
		}
		results = append(results, Result{Path: targetPath, Action: action})
	}

	return results, nil
}

func bindHooks(cfg Config) ([]Result, error) {
	type binder func(repoRoot string, patch []byte, force bool) (Result, error)
	binders := map[string]struct {
		templatePath string
		bind         binder
	}{
		"Claude Code":    {"templates/hooks/bindings/claude-code/settings-patch.json", bindClaudeCode},
		"Codex CLI":      {"templates/hooks/bindings/codex/hooks.json", bindCodex},
		"Cursor":         {"templates/hooks/bindings/cursor/hooks.json", bindCursor},
		"Kiro":           {"templates/hooks/bindings/kiro/agent-patch.json", bindKiro},
		"Antigravity":    {"templates/hooks/bindings/antigravity/hooks.json", bindAntigravity},
		"GitHub Copilot": {"templates/hooks/bindings/github-copilot/vscode-tasks.json", bindGitHubCopilot},
	}

	b, ok := binders[cfg.CodingTool]
	if !ok {
		return nil, fmt.Errorf("unknown coding tool %q", cfg.CodingTool)
	}

	patch, err := fs.ReadFile(TemplateFS, b.templatePath)
	if err != nil {
		return nil, err
	}

	result, err := b.bind(cfg.RepoRoot, patch, cfg.Force)
	if err != nil {
		return nil, err
	}
	return []Result{result}, nil
}

// atomicJSONMerge reads existing JSON at target (or starts with {}), deep-merges
// patch into it, and writes the result atomically via temp-file rename.
func atomicJSONMerge(target string, patch []byte) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}

	// Read existing content.
	var existing map[string]interface{}
	raw, err := os.ReadFile(target)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		existing = map[string]interface{}{}
	} else {
		if err := json.Unmarshal(raw, &existing); err != nil {
			existing = map[string]interface{}{}
		}
	}

	// Parse patch.
	var patchMap map[string]interface{}
	if err := json.Unmarshal(patch, &patchMap); err != nil {
		return fmt.Errorf("invalid patch JSON: %w", err)
	}

	// Deep-merge patch into existing.
	deepMerge(existing, patchMap)

	merged, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}

	// Write to temp file in same directory, then rename atomically.
	dir := filepath.Dir(target)
	tmp, err := os.CreateTemp(dir, ".dreamland-tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(append(merged, '\n')); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, target)
}

// deepMerge merges src into dst recursively.
// Maps are merged key-by-key. Arrays are unioned (src items not already in dst are appended,
// compared by JSON encoding). All other values in src overwrite dst.
func deepMerge(dst, src map[string]interface{}) {
	for k, sv := range src {
		dv, exists := dst[k]
		if !exists {
			dst[k] = sv
			continue
		}
		dstMap, dstIsMap := dv.(map[string]interface{})
		srcMap, srcIsMap := sv.(map[string]interface{})
		if dstIsMap && srcIsMap {
			deepMerge(dstMap, srcMap)
			continue
		}
		dstArr, dstIsArr := dv.([]interface{})
		srcArr, srcIsArr := sv.([]interface{})
		if dstIsArr && srcIsArr {
			seen := make(map[string]bool, len(dstArr))
			for _, item := range dstArr {
				b, _ := json.Marshal(item)
				seen[string(b)] = true
			}
			for _, item := range srcArr {
				b, _ := json.Marshal(item)
				if !seen[string(b)] {
					dstArr = append(dstArr, item)
					seen[string(b)] = true
				}
			}
			dst[k] = dstArr
			continue
		}
		dst[k] = sv
	}
}

func bindClaudeCode(repoRoot string, patch []byte, _ bool) (Result, error) {
	target := filepath.Join(repoRoot, ".claude", "settings.json")
	if err := atomicJSONMerge(target, patch); err != nil {
		return Result{}, err
	}
	return Result{Path: target, Action: "merged"}, nil
}

func bindCodex(repoRoot string, patch []byte, _ bool) (Result, error) {
	target := filepath.Join(repoRoot, ".codex", "hooks.json")
	if err := atomicJSONMerge(target, patch); err != nil {
		return Result{}, err
	}
	return Result{Path: target, Action: "merged"}, nil
}

func bindCursor(repoRoot string, patch []byte, _ bool) (Result, error) {
	target := filepath.Join(repoRoot, ".cursor", "hooks.json")
	if err := atomicJSONMerge(target, patch); err != nil {
		return Result{}, err
	}

	// Ensure "version": 1 is present.
	raw, err := os.ReadFile(target)
	if err != nil {
		return Result{}, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return Result{}, err
	}
	if _, ok := m["version"]; !ok {
		m["version"] = float64(1)
		data, _ := json.MarshalIndent(m, "", "  ")
		tmp, err := os.CreateTemp(filepath.Dir(target), ".dreamland-tmp-*")
		if err != nil {
			return Result{}, err
		}
		tmpName := tmp.Name()
		tmp.Write(append(data, '\n'))
		tmp.Close()
		os.Rename(tmpName, target)
	}

	return Result{Path: target, Action: "merged"}, nil
}

func bindKiro(repoRoot string, patch []byte, _ bool) (Result, error) {
	target := filepath.Join(repoRoot, ".kiro", "agent.json")
	if err := atomicJSONMerge(target, patch); err != nil {
		return Result{}, err
	}
	return Result{Path: target, Action: "merged"}, nil
}

func bindAntigravity(_ string, patch []byte, force bool) (Result, error) {
	home, _ := os.UserHomeDir()
	pluginDir := filepath.Join(home, ".gemini", "antigravity-cli", "plugins", "dreamland")
	target := filepath.Join(pluginDir, "hooks.json")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		return Result{}, err
	}

	if _, err := os.Stat(target); err == nil && !force {
		return Result{Path: target, Action: "skipped (already exists)"}, nil
	}

	if err := os.WriteFile(target, patch, 0o644); err != nil {
		return Result{}, err
	}
	action := "installed"
	if force {
		action = "installed (forced)"
	}
	return Result{Path: target, Action: action}, nil
}

func bindGitHubCopilot(repoRoot string, patch []byte, _ bool) (Result, error) {
	target := filepath.Join(repoRoot, ".vscode", "tasks.json")
	if err := atomicJSONMerge(target, patch); err != nil {
		return Result{}, err
	}
	return Result{Path: target, Action: "merged"}, nil
}
