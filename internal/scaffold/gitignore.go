package scaffold

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnsureGitignoreEntry appends entry to .gitignore at repoRoot if it is not already present.
// Creates .gitignore if absent. The check is line-exact.
func EnsureGitignoreEntry(repoRoot, entry string) error {
	target := filepath.Join(repoRoot, ".gitignore")

	// Check whether entry already exists.
	f, err := os.Open(target)
	if err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			if strings.TrimRight(scanner.Text(), "\r\n") == entry {
				return nil // already present
			}
		}
		f.Close()
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read .gitignore: %w", err)
	}

	file, err := os.OpenFile(target, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open .gitignore: %w", err)
	}
	defer file.Close()
	_, err = fmt.Fprintln(file, entry)
	return err
}
