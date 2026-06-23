package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// makeGitRepo creates a temp dir with a .git subdirectory and returns the path.
func makeGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// TestFindRepoRoot_FromRoot finds .git when starting at the repo root.
func TestFindRepoRoot_FromRoot(t *testing.T) {
	root := makeGitRepo(t)
	got, err := FindRepoRoot(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != root {
		t.Errorf("got %q, want %q", got, root)
	}
}

// TestFindRepoRoot_FromSubdir finds .git when starting from a nested directory.
func TestFindRepoRoot_FromSubdir(t *testing.T) {
	root := makeGitRepo(t)
	sub := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := FindRepoRoot(sub)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != root {
		t.Errorf("got %q, want %q", got, root)
	}
}

// TestFindRepoRoot_NoGit returns an error when no .git is found.
func TestFindRepoRoot_NoGit(t *testing.T) {
	dir := t.TempDir()
	_, err := FindRepoRoot(dir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// TestLoad_Absent returns nil config (not an error) when the file is missing.
func TestLoad_Absent(t *testing.T) {
	root := makeGitRepo(t)
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Errorf("expected nil config, got %+v", cfg)
	}
}

// TestLoad_Present parses a valid .dreamland.json.
func TestLoad_Present(t *testing.T) {
	root := makeGitRepo(t)
	want := &Config{
		CodingTool:     "claude-code",
		Language:       "go",
		TestCommand:    "go test ./...",
		DocCommand:     "godoc",
		VersionCommand: "go version",
	}
	data, _ := json.Marshal(want)
	if err := os.WriteFile(filepath.Join(root, filename), data, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *got != *want {
		t.Errorf("got %+v, want %+v", *got, *want)
	}
}

// TestLoad_Invalid returns an error for malformed JSON.
func TestLoad_Invalid(t *testing.T) {
	root := makeGitRepo(t)
	if err := os.WriteFile(filepath.Join(root, filename), []byte("not json{"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(root)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

// TestLoad_NoGit returns an error when there is no git repo.
func TestLoad_NoGit(t *testing.T) {
	dir := t.TempDir()
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error when no git repo, got nil")
	}
}

// TestSave_Success writes a config and reads it back.
func TestSave_Success(t *testing.T) {
	root := makeGitRepo(t)
	cfg := &Config{
		CodingTool:     "kiro",
		Language:       "rust",
		TestCommand:    "cargo test",
		DocCommand:     "cargo doc",
		VersionCommand: "rustc --version",
	}
	if err := Save(root, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(root)
	if err != nil {
		t.Fatalf("Load after Save: %v", err)
	}
	if *got != *cfg {
		t.Errorf("got %+v, want %+v", *got, *cfg)
	}
}

// TestSave_Overwrites verifies a second Save replaces the first.
func TestSave_Overwrites(t *testing.T) {
	root := makeGitRepo(t)
	first := &Config{CodingTool: "ghcp", Language: "go", TestCommand: "go test ./...", VersionCommand: "go version"}
	second := &Config{CodingTool: "claude-code", Language: "python", TestCommand: "pytest", VersionCommand: "python3 --version"}
	if err := Save(root, first); err != nil {
		t.Fatal(err)
	}
	if err := Save(root, second); err != nil {
		t.Fatal(err)
	}
	got, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if *got != *second {
		t.Errorf("got %+v, want %+v", *got, *second)
	}
}

// TestSave_NoGit returns an error when there is no git repo.
func TestSave_NoGit(t *testing.T) {
	dir := t.TempDir()
	err := Save(dir, &Config{})
	if err == nil {
		t.Fatal("expected error when no git repo, got nil")
	}
}

// TestLoad_UnreadableFile returns an error when the config file exists but
// cannot be read (simulated by placing a directory at the config path).
func TestLoad_UnreadableFile(t *testing.T) {
	root := makeGitRepo(t)
	if err := os.Mkdir(filepath.Join(root, filename), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := Load(root)
	if err == nil {
		t.Fatal("expected error reading directory as file, got nil")
	}
}

// TestSave_CreateTempFails returns an error when the repo root is read-only.
func TestSave_CreateTempFails(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root can write to read-only directories")
	}
	root := makeGitRepo(t)
	if err := os.Chmod(root, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o755) })
	err := Save(root, &Config{})
	if err == nil {
		t.Fatal("expected error with read-only root, got nil")
	}
}

// TestSave_RenameFails returns an error when os.Rename fails.
func TestSave_RenameFails(t *testing.T) {
	root := makeGitRepo(t)
	orig := renameFunc
	renameFunc = func(_, _ string) error { return errors.New("rename failed") }
	t.Cleanup(func() { renameFunc = orig })

	err := Save(root, &Config{VersionCommand: "go version"})
	if err == nil {
		t.Fatal("expected error from rename, got nil")
	}
}

// TestSave_WriteFails returns an error when the temp file write fails.
func TestSave_WriteFails(t *testing.T) {
	root := makeGitRepo(t)
	orig := writeFunc
	writeFunc = func(f *os.File, _ []byte) (int, error) {
		_ = f.Close()
		return 0, errors.New("write failed")
	}
	t.Cleanup(func() { writeFunc = orig })

	err := Save(root, &Config{VersionCommand: "go version"})
	if err == nil {
		t.Fatal("expected error from write, got nil")
	}
}

// TestSave_CloseFails returns an error when the temp file close fails.
func TestSave_CloseFails(t *testing.T) {
	root := makeGitRepo(t)
	origClose := closeFunc
	calls := 0
	closeFunc = func(f *os.File) error {
		calls++
		if calls == 1 {
			_ = f.Close()
			return errors.New("close failed")
		}
		return f.Close()
	}
	t.Cleanup(func() { closeFunc = origClose })

	err := Save(root, &Config{VersionCommand: "go version"})
	if err == nil {
		t.Fatal("expected error from close, got nil")
	}
}

// TestSave_ValidJSON verifies that a successful Save produces a
// valid file (covers the success path of the atomic write).
func TestSave_ValidJSON(t *testing.T) {
	root := makeGitRepo(t)
	cfg := &Config{VersionCommand: "go version", Language: "go", TestCommand: "go test ./..."}
	if err := Save(root, cfg); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, filename))
	if err != nil {
		t.Fatal(err)
	}
	var out Config
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Errorf("produced invalid JSON: %v", err)
	}
}
