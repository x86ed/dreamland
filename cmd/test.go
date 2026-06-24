package cmd

import (
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"dreamland/internal/config"
)

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Run tests if source files changed since last commit",
	RunE:  runTest,
}

func init() {
	rootCmd.AddCommand(testCmd)
}

var sourceExtensions = map[string][]string{
	"Go":              {".go"},
	"Node/TypeScript": {".ts", ".tsx", ".js", ".jsx", ".mts", ".cts"},
	"Rust":            {".rs"},
	"Python":          {".py"},
}

func runTest(_ *cobra.Command, _ []string) error {
	cwd, err := osGetwd()
	if err != nil {
		return err
	}
	cfg, err := config.Load(cwd)
	if err != nil {
		return err
	}
	if cfg == nil || cfg.TestCommand == "" {
		return nil
	}

	exts, ok := sourceExtensions[cfg.Language]
	if !ok {
		return nil
	}

	out, err := runCmd("git", "status", "--porcelain")
	if err != nil {
		return nil // git unavailable → skip
	}

	if !hasMatchingFiles(out, exts) {
		return nil
	}

	parts := strings.Fields(cfg.TestCommand)
	if len(parts) == 0 {
		return nil
	}
	c := exec.Command(parts[0], parts[1:]...)
	c.Stdout = nil
	c.Stderr = nil
	return c.Run()
}

func hasMatchingFiles(gitStatus string, exts []string) bool {
	for _, line := range strings.Split(gitStatus, "\n") {
		if len(line) < 3 {
			continue
		}
		// git status --porcelain: "XY filename" (first 2 chars = status, then space, then path)
		path := strings.TrimSpace(line[2:])
		for _, ext := range exts {
			if strings.HasSuffix(path, ext) {
				return true
			}
		}
	}
	return false
}
