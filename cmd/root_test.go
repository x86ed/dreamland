package cmd

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"dreamland/internal/config"
)

// makeRootGitRepo creates a temp dir with .git, cds into it, and restores on cleanup.
func makeRootGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		currentConfig = nil
		_ = os.Chdir(orig)
	})
	return root
}

// TestLoadConfig_SetsCurrentConfig verifies PersistentPreRunE populates currentConfig.
func TestLoadConfig_SetsCurrentConfig(t *testing.T) {
	root := makeRootGitRepo(t)

	want := &config.Config{
		CodingTool:     "claude-code",
		Language:       "go",
		TestCommand:    "go test ./...",
		DocCommand:     "godoc",
		VersionCommand: "go version",
	}
	data, _ := json.MarshalIndent(want, "", "  ")
	if err := os.WriteFile(filepath.Join(root, ".dreamland.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := loadConfig(nil, nil); err != nil {
		t.Fatalf("loadConfig: %v", err)
	}

	got := GetConfig()
	if got == nil {
		t.Fatal("expected non-nil config after loadConfig")
	}
	if *got != *want {
		t.Errorf("got %+v, want %+v", *got, *want)
	}
}

// TestLoadConfig_AbsentConfig verifies currentConfig is nil when no config file exists.
func TestLoadConfig_AbsentConfig(t *testing.T) {
	makeRootGitRepo(t)

	if err := loadConfig(nil, nil); err != nil {
		t.Fatalf("loadConfig: %v", err)
	}

	if got := GetConfig(); got != nil {
		t.Errorf("expected nil config, got %+v", got)
	}
}

// TestLoadConfig_NoGitRepo verifies loadConfig succeeds (nil config) outside a git repo.
func TestLoadConfig_NoGitRepo(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() {
		currentConfig = nil
		_ = os.Chdir(orig)
	})

	if err := loadConfig(nil, nil); err != nil {
		t.Fatalf("expected no error outside git repo, got: %v", err)
	}
	if got := GetConfig(); got != nil {
		t.Errorf("expected nil config outside git repo, got %+v", got)
	}
}

// TestLoadConfig_GetCwdError verifies loadConfig returns an error when os.Getwd fails.
func TestLoadConfig_GetCwdError(t *testing.T) {
	orig := osGetwd
	osGetwd = func() (string, error) { return "", errors.New("getwd failed") }
	t.Cleanup(func() { osGetwd = orig })

	if err := loadConfig(nil, nil); err == nil {
		t.Fatal("expected error when Getwd fails, got nil")
	}
}

// TestGetConfig_ReturnsCurrentConfig verifies GetConfig returns the package-level value.
func TestGetConfig_ReturnsCurrentConfig(t *testing.T) {
	orig := currentConfig
	t.Cleanup(func() { currentConfig = orig })

	currentConfig = &config.Config{CodingTool: "kiro"}
	if got := GetConfig(); got == nil || got.CodingTool != "kiro" {
		t.Errorf("GetConfig() = %v, want CodingTool=kiro", got)
	}
}
