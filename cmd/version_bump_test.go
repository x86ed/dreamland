package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBumpSemver_NoTag(t *testing.T) {
	cases := []struct {
		level string
		want  string
	}{
		{"minor", "v0.1.0"},
		{"major", "v1.0.0"},
		{"patch", "v0.0.1"},
	}
	for _, tc := range cases {
		got, err := bumpSemver("v0.0.0", tc.level, "")
		if err != nil {
			t.Errorf("bumpSemver(%q): %v", tc.level, err)
		}
		if got != tc.want {
			t.Errorf("bumpSemver(v0.0.0, %q) = %q, want %q", tc.level, got, tc.want)
		}
	}
}

func TestBumpSemver_FromExisting(t *testing.T) {
	cases := []struct {
		base  string
		level string
		want  string
	}{
		{"v1.2.3", "patch", "v1.2.4"},
		{"v1.2.3", "minor", "v1.3.0"},
		{"v1.2.3", "major", "v2.0.0"},
	}
	for _, tc := range cases {
		got, err := bumpSemver(tc.base, tc.level, "")
		if err != nil {
			t.Errorf("bumpSemver(%q, %q): %v", tc.base, tc.level, err)
		}
		if got != tc.want {
			t.Errorf("bumpSemver(%q, %q) = %q, want %q", tc.base, tc.level, got, tc.want)
		}
	}
}

func TestBumpSemver_ExplicitOverride(t *testing.T) {
	got, err := bumpSemver("v1.0.0", "minor", "v2.5.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "v2.5.0" {
		t.Errorf("got %q, want v2.5.0", got)
	}
}

func TestBumpSemver_UnknownLevel(t *testing.T) {
	_, err := bumpSemver("v1.0.0", "bogus", "")
	if err == nil {
		t.Fatal("expected error for unknown level, got nil")
	}
}

func TestReadBranchBumps_Absent(t *testing.T) {
	dir := t.TempDir()
	bumps, err := readBranchBumps(filepath.Join(dir, "branch-bumps"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bumps) != 0 {
		t.Errorf("expected empty map, got %v", bumps)
	}
}

func TestWriteAndReadBranchBumps(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".dreamland", "branch-bumps")

	bumps := map[string]branchBumpEntry{
		"feature-xyz": {Version: "v1.2.0", InitializedAt: "2026-06-23T12:00:00Z"},
	}
	if err := writeBranchBumps(path, bumps); err != nil {
		t.Fatalf("writeBranchBumps: %v", err)
	}

	got, err := readBranchBumps(path)
	if err != nil {
		t.Fatalf("readBranchBumps: %v", err)
	}
	entry, ok := got["feature-xyz"]
	if !ok {
		t.Fatal("expected feature-xyz entry")
	}
	if entry.Version != "v1.2.0" {
		t.Errorf("version = %q, want v1.2.0", entry.Version)
	}
}

func TestWriteBranchBumps_Atomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".dreamland", "branch-bumps")

	// Write initial content.
	initial := map[string]branchBumpEntry{"main": {Version: "v1.0.0"}}
	if err := writeBranchBumps(path, initial); err != nil {
		t.Fatal(err)
	}

	// Write again with additional entry.
	initial["feature"] = branchBumpEntry{Version: "v1.1.0"}
	if err := writeBranchBumps(path, initial); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]branchBumpEntry
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}
	if _, ok := m["main"]; !ok {
		t.Error("expected main entry preserved")
	}
	if _, ok := m["feature"]; !ok {
		t.Error("expected feature entry added")
	}
}

func TestVersionBumpFlags_AtMostOne(t *testing.T) {
	// Save and restore global flag state.
	origMajor, origMinor, origPatch, origVersion := vbMajor, vbMinor, vbPatch, vbVersion
	defer func() {
		vbMajor, vbMinor, vbPatch, vbVersion = origMajor, origMinor, origPatch, origVersion
	}()

	vbMajor = true
	vbMinor = true
	vbPatch = false
	vbVersion = ""

	err := runVersionBump(versionBumpCmd, nil)
	if err == nil {
		t.Fatal("expected error when both --major and --minor set")
	}
}
