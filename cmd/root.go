package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"dreamland/internal/config"
)

var (
	rootCmd = &cobra.Command{
		Use:              "dreamland",
		Short:            "Dreamland CLI",
		PersistentPreRunE: loadConfig,
	}
	currentConfig *config.Config
)

// loadConfig is rootCmd's PersistentPreRunE: loads .dreamland.json if present
// and performs a binary freshness check.
func loadConfig(_ *cobra.Command, _ []string) error {
	cwd, err := osGetwd()
	if err != nil {
		return err
	}

	// Check for binary staleness (advisory, non-blocking)
	checkBinaryFreshness(cwd)

	cfg, err := config.Load(cwd)
	if err != nil {
		// No git repo or other load error — treat as no config.
		currentConfig = nil
		return nil
	}
	currentConfig = cfg
	return nil
}

// GetConfig returns the project config loaded at startup, or nil if absent.
func GetConfig() *config.Config {
	return currentConfig
}

// Execute runs the root CLI command and exits appropriately based on error type.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		if IsBlocking(err) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}
