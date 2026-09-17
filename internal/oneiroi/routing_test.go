package oneiroi

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAddStubEdge_UnknownPlatform(t *testing.T) {
	if _, err := AddStubEdge(t.TempDir(), "Not A Platform", "agent"); err == nil {
		t.Error("expected error for unknown platform")
	}
}

func TestAddStubEdge_MissingFileIsNoop(t *testing.T) {
	touched, err := AddStubEdge(t.TempDir(), "Claude Code", "agent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if touched {
		t.Error("expected no-op when janus file does not exist")
	}
}

func TestAddStubEdge_AppendsMarkerThenIsIdempotent(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".claude", "agents", "janus.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("# janus\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	touched, err := AddStubEdge(root, "Claude Code", "hermes")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !touched {
		t.Fatal("expected file to be touched on first add")
	}

	touchedAgain, err := AddStubEdge(root, "Claude Code", "hermes")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if touchedAgain {
		t.Error("expected AddStubEdge to be idempotent once marker exists")
	}
}

func TestAddStubEdges(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".claude", "agents", "janus.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("# janus"), 0o644); err != nil {
		t.Fatal(err)
	}

	touchedPaths, err := AddStubEdges(root, "hermes")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(touchedPaths) != 1 || touchedPaths[0] != path {
		t.Fatalf("expected only %s to be touched, got %v", path, touchedPaths)
	}
}
