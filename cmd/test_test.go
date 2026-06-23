package cmd

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"dreamland/internal/config"
)

func makeTestRepo(t *testing.T, cfg config.Config) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(cfg)
	if err := os.WriteFile(filepath.Join(root, ".dreamland.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	orig := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = orig })
	return root
}

func TestRunTest_SourceChanged_RunsCommand(t *testing.T) {
	makeTestRepo(t, config.Config{Language: "Go", TestCommand: "echo ok"})

	origRunCmd := runCmd
	runCmd = func(_ string, args ...string) (string, error) {
		return "M  main.go\n", nil // git status shows .go change
	}
	t.Cleanup(func() { runCmd = origRunCmd })

	if err := runTest(nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunTest_NoSourceChanged_Silent(t *testing.T) {
	makeTestRepo(t, config.Config{Language: "Go", TestCommand: "false"})

	origRunCmd := runCmd
	runCmd = func(_ string, _ ...string) (string, error) {
		return "M  README.md\n", nil // only non-Go file changed
	}
	t.Cleanup(func() { runCmd = origRunCmd })

	// If hasMatchingFiles returns false, runTest returns nil without running "false".
	if err := runTest(nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunTest_NoConfig_Silent(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	// No .dreamland.json written — Load returns nil cfg.
	orig := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = orig })

	if err := runTest(nil, nil); err != nil {
		t.Fatalf("expected nil for missing config, got: %v", err)
	}
}

func TestRunTest_GitUnavailable_Silent(t *testing.T) {
	makeTestRepo(t, config.Config{Language: "Go", TestCommand: "false"})

	origRunCmd := runCmd
	runCmd = func(_ string, _ ...string) (string, error) {
		return "", os.ErrNotExist // git not found
	}
	t.Cleanup(func() { runCmd = origRunCmd })

	if err := runTest(nil, nil); err != nil {
		t.Fatalf("expected nil when git unavailable, got: %v", err)
	}
}

func TestRunTest_UnknownLanguage_Silent(t *testing.T) {
	makeTestRepo(t, config.Config{Language: "COBOL", TestCommand: "false"})

	origRunCmd := runCmd
	runCmd = func(_ string, _ ...string) (string, error) { return "M main.cob\n", nil }
	t.Cleanup(func() { runCmd = origRunCmd })

	if err := runTest(nil, nil); err != nil {
		t.Fatalf("expected nil for unknown language, got: %v", err)
	}
}

func TestHasMatchingFiles_Go(t *testing.T) {
	status := " M cmd/foo.go\n M README.md\n"
	if !hasMatchingFiles(status, []string{".go"}) {
		t.Error("expected true for .go file")
	}
	if hasMatchingFiles(" M README.md\n", []string{".go"}) {
		t.Error("expected false when no .go files")
	}
}

func TestHasMatchingFiles_TypeScript(t *testing.T) {
	exts := sourceExtensions["Node/TypeScript"]
	cases := []struct {
		line string
		want bool
	}{
		{" M src/app.ts\n", true},
		{" M src/comp.tsx\n", true},
		{" M src/util.js\n", true},
		{" M src/mod.mts\n", true},
		{" M README.md\n", false},
		{" M styles.css\n", false},
	}
	for _, tc := range cases {
		got := hasMatchingFiles(tc.line, exts)
		if got != tc.want {
			t.Errorf("hasMatchingFiles(%q) = %v, want %v", tc.line, got, tc.want)
		}
	}
}

func TestHasMatchingFiles_Rust(t *testing.T) {
	exts := sourceExtensions["Rust"]
	if !hasMatchingFiles(" M src/main.rs\n", exts) {
		t.Error("expected true for .rs file")
	}
	if hasMatchingFiles(" M Cargo.toml\n", exts) {
		t.Error("expected false for Cargo.toml")
	}
}

func TestHasMatchingFiles_Python(t *testing.T) {
	exts := sourceExtensions["Python"]
	if !hasMatchingFiles(" M app.py\n", exts) {
		t.Error("expected true for .py file")
	}
	if hasMatchingFiles(" M requirements.txt\n", exts) {
		t.Error("expected false for .txt file")
	}
}

func TestHasMatchingFiles_Empty(t *testing.T) {
	if hasMatchingFiles("", []string{".go"}) {
		t.Error("expected false for empty status")
	}
}

func TestHasMatchingFiles_MultipleFiles(t *testing.T) {
	status := " M README.md\n M styles.css\n M main.go\n"
	if !hasMatchingFiles(status, []string{".go"}) {
		t.Error("expected true when one of multiple files matches")
	}
}

// --- Additional tests to improve coverage ---

func TestRunTest_OsGetwdError(t *testing.T) {
	orig := osGetwd
	osGetwd = func() (string, error) { return "", errors.New("getwd failed") }
	t.Cleanup(func() { osGetwd = orig })

	if err := runTest(nil, nil); err == nil || err.Error() != "getwd failed" {
		t.Fatalf("expected getwd error, got: %v", err)
	}
}

func TestRunTest_ConfigLoadError(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Write invalid JSON so config.Load fails.
	if err := os.WriteFile(filepath.Join(root, ".dreamland.json"), []byte("not-json"), 0o644); err != nil {
		t.Fatal(err)
	}
	orig := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = orig })

	if err := runTest(nil, nil); err == nil {
		t.Fatal("expected error from config.Load with invalid JSON")
	}
}

func TestRunTest_WhitespaceTestCommand(t *testing.T) {
	// TestCommand is all whitespace → strings.Fields returns empty slice → return nil
	makeTestRepo(t, config.Config{Language: "Go", TestCommand: "   "})

	origRunCmd := runCmd
	runCmd = func(_ string, _ ...string) (string, error) {
		return "M  main.go\n", nil // has source changes
	}
	t.Cleanup(func() { runCmd = origRunCmd })

	if err := runTest(nil, nil); err != nil {
		t.Fatalf("expected nil for whitespace TestCommand, got: %v", err)
	}
}
