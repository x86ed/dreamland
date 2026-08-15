package workflowgraph

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadPositionsMissingFileReturnsNilNotError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workflow-positions.json")

	positions, err := LoadPositions(path)
	if err != nil {
		t.Fatalf("LoadPositions on missing file: unexpected error %v", err)
	}
	if positions != nil {
		t.Fatalf("LoadPositions on missing file: expected nil map, got %+v", positions)
	}
}

// TestSavePositionsCreatesParentDir is a regression test: the original Save
// (before this file only persisted positions at all) called os.WriteFile
// directly with no os.MkdirAll first, so it errored on a fresh repo where
// .dreamland/ doesn't exist yet — caught via a real browser test against a
// brand-new temp repo (existing repos happened to already have .dreamland/
// from other files, masking the gap).
func TestSavePositionsCreatesParentDir(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".dreamland", "workflow-positions.json")

	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Fatalf("test setup: .dreamland should not exist yet, stat err = %v", err)
	}

	if err := SavePositions(path, New(root)); err != nil {
		t.Fatalf("SavePositions: unexpected error %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected the positions file to exist: %v", err)
	}
}

func TestSaveLoadPositionsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workflow-positions.json")

	g := New("/repo")
	g.Agents["hypnos"] = &AgentNode{ID: "hypnos", PosX: 120, PosY: 340}
	g.Agents["mengpo"] = &AgentNode{ID: "mengpo", PosX: -50, PosY: 0}

	if err := SavePositions(path, g); err != nil {
		t.Fatalf("SavePositions: unexpected error %v", err)
	}

	loaded, err := LoadPositions(path)
	if err != nil {
		t.Fatalf("LoadPositions: unexpected error %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadPositions: expected a non-nil map")
	}

	hypnos, ok := loaded["hypnos"]
	if !ok {
		t.Fatal("expected \"hypnos\" position to round-trip")
	}
	if hypnos.PosX != 120 || hypnos.PosY != 340 {
		t.Errorf("hypnos position = (%v, %v), want (120, 340)", hypnos.PosX, hypnos.PosY)
	}

	mengpo, ok := loaded["mengpo"]
	if !ok {
		t.Fatal("expected \"mengpo\" position to round-trip")
	}
	if mengpo.PosX != -50 || mengpo.PosY != 0 {
		t.Errorf("mengpo position = (%v, %v), want (-50, 0)", mengpo.PosX, mengpo.PosY)
	}
}

// TestSavePositionsOnlyPersistsPosition confirms nothing beyond position is
// written — the whole point of narrowing this from a full-graph cache to a
// positions file: everything else is always freshly re-derivable from the
// six live platform directories and shouldn't need local persistence at all.
func TestSavePositionsOnlyPersistsPosition(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workflow-positions.json")

	g := New("/repo")
	g.Agents["hypnos"] = &AgentNode{
		ID:              "hypnos",
		Description:     "Should not be persisted.",
		Tier:            TierFullEdit,
		InstructionBody: "Should not be persisted either.",
		PosX:            10,
		PosY:            20,
	}

	if err := SavePositions(path, g); err != nil {
		t.Fatalf("SavePositions: unexpected error %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, unwanted := range []string{"Description", "InstructionBody", "Tier", "Should not be persisted"} {
		if strings.Contains(content, unwanted) {
			t.Errorf("positions file unexpectedly contains %q:\n%s", unwanted, content)
		}
	}
}
