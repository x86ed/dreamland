package cmd

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"dreamland/internal/config"
)

var transitionLogCmd = &cobra.Command{
	Use:   "transition-log",
	Short: "Append a timestamped turn-complete entry to .dreamland/transition.log",
	RunE:  runTransitionLog,
}

func init() {
	rootCmd.AddCommand(transitionLogCmd)
}

func runTransitionLog(_ *cobra.Command, _ []string) error {
	cwd, err := osGetwd()
	if err != nil {
		return err
	}
	repoRoot, err := config.FindRepoRoot(cwd)
	if err != nil {
		return err
	}

	sessionID := resolveSessionID()
	line := fmt.Sprintf("%s [%s] turn complete\n", time.Now().UTC().Format(time.RFC3339), sessionID)

	logDir := filepath.Join(repoRoot, ".dreamland")
	// Suppress errors — transition-log is always a no-op on failure.
	_ = os.MkdirAll(logDir, 0o755)

	f, err := os.OpenFile(filepath.Join(logDir, "transition.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil // suppress
	}
	defer f.Close()
	_, _ = f.WriteString(line)
	return nil
}

func resolveSessionID() string {
	for _, env := range []string{
		"CLAUDE_SESSION_ID",
		"CODEX_SESSION_ID",
		"CURSOR_SESSION_ID",
		"KIRO_SESSION_ID",
	} {
		if v := os.Getenv(env); v != "" {
			return v
		}
	}
	return randomHex(4)
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
