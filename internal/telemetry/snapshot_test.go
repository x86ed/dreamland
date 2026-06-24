package telemetry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestComputeTotals(t *testing.T) {
	s := &SnapshotResult{InputTokens: 1500, OutputTokens: 300}
	s.ComputeTotals()
	if s.TotalTokens != 1800 {
		t.Errorf("TotalTokens = %d, want 1800", s.TotalTokens)
	}
}

func TestComputeTotals_NoOverwrite(t *testing.T) {
	s := &SnapshotResult{InputTokens: 100, OutputTokens: 50, TotalTokens: 999}
	s.ComputeTotals()
	if s.TotalTokens != 999 {
		t.Errorf("TotalTokens = %d, want 999 (should not overwrite)", s.TotalTokens)
	}
}

func TestReadWrite(t *testing.T) {
	root := t.TempDir()

	// Read from empty dir returns nil.
	got, err := Read(root)
	if err != nil {
		t.Fatalf("Read empty: %v", err)
	}
	if got != nil {
		t.Fatalf("Read empty: want nil, got %+v", got)
	}

	// First write.
	if err := Write(root, &SnapshotResult{
		Tool:         "claude-code",
		Model:        "claude-sonnet-4-6",
		InputTokens:  1000,
		OutputTokens: 200,
		CachedTokens: 300,
	}); err != nil {
		t.Fatalf("Write 1: %v", err)
	}

	got, err = Read(root)
	if err != nil {
		t.Fatalf("Read after first write: %v", err)
	}
	if got.InputTokens != 1000 || got.OutputTokens != 200 || got.CachedTokens != 300 {
		t.Errorf("after first write: %+v", got)
	}
	if got.TotalTokens != 1200 {
		t.Errorf("TotalTokens = %d, want 1200", got.TotalTokens)
	}

	// Second write accumulates.
	if err := Write(root, &SnapshotResult{
		Tool:         "claude-code",
		Model:        "claude-sonnet-4-6",
		InputTokens:  500,
		OutputTokens: 100,
	}); err != nil {
		t.Fatalf("Write 2: %v", err)
	}

	got, err = Read(root)
	if err != nil {
		t.Fatalf("Read after second write: %v", err)
	}
	if got.InputTokens != 1500 {
		t.Errorf("accumulated InputTokens = %d, want 1500", got.InputTokens)
	}
	if got.OutputTokens != 300 {
		t.Errorf("accumulated OutputTokens = %d, want 300", got.OutputTokens)
	}
}

func TestWriteSetsTimestamp(t *testing.T) {
	root := t.TempDir()
	if err := Write(root, &SnapshotResult{Tool: "codex", Model: "o4-mini"}); err != nil {
		t.Fatal(err)
	}
	got, _ := Read(root)
	if got.CapturedAt == "" {
		t.Error("CapturedAt should be set after Write")
	}
}

func TestRead_CorruptJSON(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, sessionFile), []byte("{bad json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Read(root)
	if err == nil {
		t.Error("expected error for corrupt session file")
	}
}

func TestWrite_CreateTempError(t *testing.T) {
	// Read-only root: Read returns nil (no existing file), then CreateTemp fails.
	root := t.TempDir()
	if err := os.Chmod(root, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(root, 0o755) })
	err := Write(root, &SnapshotResult{Tool: "test"})
	if err == nil {
		t.Error("expected error when directory is read-only")
	}
}

func TestWrite_ReadError(t *testing.T) {
	// Use a file as repoRoot so that Read fails (can't join path into a file).
	notADir := filepath.Join(t.TempDir(), "notadir")
	if err := os.WriteFile(notADir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := Write(notADir, &SnapshotResult{Tool: "test"})
	if err == nil {
		t.Error("expected error when repoRoot is a regular file")
	}
}

func TestRegister(t *testing.T) {
	const name = "test-register-tool"
	orig := Registry[name]
	Register(name, nil)
	if _, ok := Registry[name]; !ok {
		t.Errorf("Register did not add %q to Registry", name)
	}
	t.Cleanup(func() {
		if orig == nil {
			delete(Registry, name)
		} else {
			Registry[name] = orig
		}
	})
}
