package scaffold

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	hookBegin = "# BEGIN dreamland-telemetry"
	hookEnd   = "# END dreamland-telemetry"
)

// InstallCommitMsgHook installs the dreamland telemetry block into .git/hooks/commit-msg.
// If the file already exists, the block is appended after existing content.
// The operation is idempotent — if the block is already present, the file is unchanged.
func InstallCommitMsgHook(repoRoot string) error {
	hookDir := filepath.Join(repoRoot, ".git", "hooks")
	if err := os.MkdirAll(hookDir, 0o755); err != nil {
		return fmt.Errorf("create hooks dir: %w", err)
	}

	block, err := fs.ReadFile(TemplateFS, "templates/hooks/commit-msg")
	if err != nil {
		return fmt.Errorf("read commit-msg template: %w", err)
	}

	target := filepath.Join(hookDir, "commit-msg")

	existing, err := os.ReadFile(target)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read existing hook: %w", err)
	}

	// Idempotent: skip if block already present.
	if bytes.Contains(existing, []byte(hookBegin)) {
		return nil
	}

	var content []byte
	if len(existing) > 0 {
		content = append(existing, '\n')
	} else {
		content = []byte("#!/bin/sh\n")
	}
	content = append(content, block...)

	return atomicWrite(target, content, 0o755)
}

// UninstallCommitMsgHook removes the dreamland-managed block from .git/hooks/commit-msg.
// Deletes the file if only whitespace remains after removal.
func UninstallCommitMsgHook(repoRoot string) error {
	target := filepath.Join(repoRoot, ".git", "hooks", "commit-msg")
	data, err := os.ReadFile(target)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	content := string(data)
	startIdx := strings.Index(content, hookBegin)
	if startIdx < 0 {
		return nil // block not found, nothing to do
	}
	endIdx := strings.Index(content, hookEnd)
	if endIdx >= 0 {
		endIdx += len(hookEnd)
	} else {
		endIdx = len(content)
	}

	remaining := content[:startIdx] + content[endIdx:]
	remaining = strings.TrimRight(remaining, "\n\r\t ")

	if remaining == "" || remaining == "#!/bin/sh" {
		return os.Remove(target)
	}
	return atomicWrite(target, []byte(remaining+"\n"), 0o755)
}

// atomicWrite writes data to target atomically via temp-file rename and sets the given mode.
func atomicWrite(target string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(target)
	tmp, err := os.CreateTemp(dir, ".dreamland-hook-tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, target)
}
