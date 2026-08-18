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

// --- SkillAttachment: persist-skill-attachment-edges ------------------------
//
// EdgeAttachment (Skill.available_to -> Agent.skills) is the same kind of
// graph-only state AgentPosition already is: nothing on disk represents it,
// so Import can never re-derive it, and it needs the identical
// save/load-as-a-local-cache treatment. See
// openspec/changes/persist-skill-attachment-edges/design.md.

func TestLoadSkillAttachmentsMissingFileReturnsNilNotError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workflow-skill-attachments.json")

	attachments, err := LoadSkillAttachments(path)
	if err != nil {
		t.Fatalf("LoadSkillAttachments on missing file: unexpected error %v", err)
	}
	if attachments != nil {
		t.Fatalf("LoadSkillAttachments on missing file: expected nil slice, got %+v", attachments)
	}
}

func TestSaveSkillAttachmentsCreatesParentDir(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".dreamland", "workflow-skill-attachments.json")

	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Fatalf("test setup: .dreamland should not exist yet, stat err = %v", err)
	}

	if err := SaveSkillAttachments(path, New(root)); err != nil {
		t.Fatalf("SaveSkillAttachments: unexpected error %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected the skill-attachments file to exist: %v", err)
	}
}

func TestSaveLoadSkillAttachmentsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workflow-skill-attachments.json")

	g := New("/repo")
	g.Edges = []Edge{
		{Kind: EdgeAttachment, From: "openspec-propose", To: "hypnos"},
		{Kind: EdgeAttachment, From: "openspec-apply", To: "morpheus"},
		{Kind: EdgeRouting, From: "hypnos", To: "morpheus"}, // must not round-trip as an attachment
	}

	if err := SaveSkillAttachments(path, g); err != nil {
		t.Fatalf("SaveSkillAttachments: unexpected error %v", err)
	}

	loaded, err := LoadSkillAttachments(path)
	if err != nil {
		t.Fatalf("LoadSkillAttachments: unexpected error %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 attachments to round-trip, got %d: %+v", len(loaded), loaded)
	}

	want := map[string]string{
		"openspec-propose": "hypnos",
		"openspec-apply":   "morpheus",
	}
	for _, a := range loaded {
		agentID, ok := want[a.SkillID]
		if !ok {
			t.Errorf("unexpected skill id in loaded attachments: %q", a.SkillID)
			continue
		}
		if a.AgentID != agentID {
			t.Errorf("attachment for skill %q: AgentID = %q, want %q", a.SkillID, a.AgentID, agentID)
		}
	}
}

// TestSaveSkillAttachmentsIsDeterministic asserts two saves of the same edge
// set, added in different orders, produce byte-identical output — required
// so repeated saves of an unchanged attachment set don't spuriously dirty the
// cache file (tasks.md 1.4).
func TestSaveSkillAttachmentsIsDeterministic(t *testing.T) {
	pathA := filepath.Join(t.TempDir(), "workflow-skill-attachments.json")
	pathB := filepath.Join(t.TempDir(), "workflow-skill-attachments.json")

	gA := New("/repo")
	gA.Edges = []Edge{
		{Kind: EdgeAttachment, From: "openspec-propose", To: "hypnos"},
		{Kind: EdgeAttachment, From: "openspec-apply", To: "morpheus"},
		{Kind: EdgeAttachment, From: "openspec-apply", To: "hypnos"},
	}

	gB := New("/repo")
	gB.Edges = []Edge{
		{Kind: EdgeAttachment, From: "openspec-apply", To: "hypnos"},
		{Kind: EdgeAttachment, From: "openspec-apply", To: "morpheus"},
		{Kind: EdgeAttachment, From: "openspec-propose", To: "hypnos"},
	}

	if err := SaveSkillAttachments(pathA, gA); err != nil {
		t.Fatalf("SaveSkillAttachments(A): unexpected error %v", err)
	}
	if err := SaveSkillAttachments(pathB, gB); err != nil {
		t.Fatalf("SaveSkillAttachments(B): unexpected error %v", err)
	}

	dataA, err := os.ReadFile(pathA)
	if err != nil {
		t.Fatal(err)
	}
	dataB, err := os.ReadFile(pathB)
	if err != nil {
		t.Fatal(err)
	}
	if string(dataA) != string(dataB) {
		t.Errorf("expected identical bytes for the same edge set saved in different orders:\nA:\n%s\nB:\n%s", dataA, dataB)
	}
}

func TestSaveSkillAttachmentsMkdirAllError(t *testing.T) {
	root := t.TempDir()
	blocker := filepath.Join(root, "blocker")
	if err := os.WriteFile(blocker, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	g := New("/repo")
	err := SaveSkillAttachments(filepath.Join(blocker, "attachments.json"), g)
	if err == nil {
		t.Error("expected an error when the parent path is blocked by a file")
	}
}

func TestLoadSkillAttachmentsParseError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "attachments.json")
	if err := os.WriteFile(path, []byte("{not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSkillAttachments(path); err == nil {
		t.Error("expected an error parsing malformed skill attachments JSON")
	}
}

func TestLoadSkillAttachmentsReadError(t *testing.T) {
	// Path is a directory, not a file — ReadFile fails with a non-NotExist error.
	path := filepath.Join(t.TempDir(), "attachments-dir")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSkillAttachments(path); err == nil {
		t.Error("expected an error reading a path that is a directory")
	}
}

func TestSavePositionsMkdirAllError(t *testing.T) {
	root := t.TempDir()
	blocker := filepath.Join(root, "blocker")
	if err := os.WriteFile(blocker, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	g := New("/repo")
	err := SavePositions(filepath.Join(blocker, "positions.json"), g)
	if err == nil {
		t.Error("expected an error when the parent path is blocked by a file")
	}
}

func TestLoadPositionsParseError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "positions.json")
	if err := os.WriteFile(path, []byte("{not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPositions(path); err == nil {
		t.Error("expected an error parsing malformed positions JSON")
	}
}

func TestLoadPositionsReadError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "positions-dir")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPositions(path); err == nil {
		t.Error("expected an error reading a path that is a directory")
	}
}
