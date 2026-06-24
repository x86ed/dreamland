package scaffold

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// MergeVscodeSettings reads .vscode/settings.json (creates empty {} if absent),
// deep-merges patch into it, and writes the result atomically.
func MergeVscodeSettings(repoRoot string, patch map[string]any) error {
	target := filepath.Join(repoRoot, ".vscode", "settings.json")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create .vscode dir: %w", err)
	}

	var existing map[string]any
	raw, err := os.ReadFile(target)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("read settings.json: %w", err)
		}
		existing = map[string]any{}
	} else {
		if err := json.Unmarshal(raw, &existing); err != nil {
			existing = map[string]any{} // reset on parse error
		}
	}

	deepMerge(existing, patch)

	merged, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(target, append(merged, '\n'), 0o644)
}
