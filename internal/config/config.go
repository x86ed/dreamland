package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const filename = ".dreamland.json"


// Config holds the project-level settings persisted by `dreamland init`.
type Config struct {
	CodingTool     string `json:"coding_tool"`
	Language       string `json:"language"`
	TestCommand    string `json:"test_command"`
	DocCommand     string `json:"doc_command"`
	VersionCommand string `json:"version_command"`
}

// FindRepoRoot walks from dir toward the filesystem root, returning the first
// directory that contains a ".git" entry. Returns an error if none is found.
func FindRepoRoot(dir string) (string, error) {
	current := filepath.Clean(dir)
	for {
		if _, err := os.Stat(filepath.Join(current, ".git")); err == nil {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", errors.New("no git repository found in any parent directory")
		}
		current = parent
	}
}

// Load reads .dreamland.json from the repository root containing dir.
// Returns nil, nil if the file does not exist (not an error).
func Load(dir string) (*Config, error) {
	root, err := FindRepoRoot(dir)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(root, filename))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Save writes cfg as JSON to .dreamland.json at the repository root containing dir.
func Save(dir string, cfg *Config) error {
	root, err := FindRepoRoot(dir)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, filename), append(data, '\n'), 0o644)
}
