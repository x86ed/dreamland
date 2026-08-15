package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadCurrentAgent(t *testing.T) {
	t.Run("present and valid", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, ".dreamland-session.json"), []byte(`{"tool":"claude-code","agent":"morpheus"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := readCurrentAgent(root); got != "morpheus" {
			t.Errorf("readCurrentAgent = %q, want %q", got, "morpheus")
		}
	})

	t.Run("missing file", func(t *testing.T) {
		if got := readCurrentAgent(t.TempDir()); got != "" {
			t.Errorf("readCurrentAgent = %q, want \"\"", got)
		}
	})

	t.Run("malformed json", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, ".dreamland-session.json"), []byte(`not json`), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := readCurrentAgent(root); got != "" {
			t.Errorf("readCurrentAgent = %q, want \"\" on malformed JSON", got)
		}
	})
}

func TestParseChangeStatuses(t *testing.T) {
	t.Run("real shape", func(t *testing.T) {
		data := []byte(`{"changes":[{"name":"hypnos-litegraph-editor","completedTasks":67,"totalTasks":68,"lastModified":"2026-08-15T07:31:26.997Z","status":"in-progress"}]}`)
		got := parseChangeStatuses(data)
		if len(got) != 1 {
			t.Fatalf("got %d changes, want 1", len(got))
		}
		if got[0].Name != "hypnos-litegraph-editor" || got[0].CompletedTasks != 67 || got[0].TotalTasks != 68 || got[0].Status != "in-progress" {
			t.Errorf("got %+v", got[0])
		}
	})

	t.Run("malformed json returns empty not nil", func(t *testing.T) {
		got := parseChangeStatuses([]byte("not json"))
		if got == nil {
			t.Fatal("parseChangeStatuses returned nil, want an empty slice")
		}
		if len(got) != 0 {
			t.Errorf("got %d changes, want 0", len(got))
		}
	})

	t.Run("no changes key returns empty not nil", func(t *testing.T) {
		got := parseChangeStatuses([]byte(`{}`))
		if got == nil {
			t.Fatal("parseChangeStatuses returned nil, want an empty slice")
		}
	})
}

func TestListChangeStatusesToleratesMissingOpenspecBinary(t *testing.T) {
	t.Setenv("PATH", "") // guarantee `openspec` is not found
	got := listChangeStatuses(t.TempDir())
	if got == nil {
		t.Fatal("listChangeStatuses returned nil, want an empty slice on missing binary")
	}
	if len(got) != 0 {
		t.Errorf("got %d changes, want 0", len(got))
	}
}

func TestCurrentStatusAssemblesBothSources(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".dreamland-session.json"), []byte(`{"agent":"hypnos"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", "") // no openspec available — changes should just be empty

	got := currentStatus(root)
	if got.CurrentAgent != "hypnos" {
		t.Errorf("CurrentAgent = %q, want %q", got.CurrentAgent, "hypnos")
	}
	if got.Changes == nil || len(got.Changes) != 0 {
		t.Errorf("Changes = %+v, want an empty slice", got.Changes)
	}
}
