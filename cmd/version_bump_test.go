package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"dreamland/internal/config"
)

// stubRunCmd replaces runCmd for the duration of the test and restores it on cleanup.
func stubRunCmd(t *testing.T, fn func(name string, args ...string) (string, error)) {
	t.Helper()
	orig := runCmd
	runCmd = fn
	t.Cleanup(func() { runCmd = orig })
}

// makeVersionBumpRepo sets up a temp dir with a .git dir, a .dreamland.json, and
// stubs osGetwd to return it.
func makeVersionBumpRepo(t *testing.T, cfg config.Config) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(cfg)
	if err := os.WriteFile(filepath.Join(root, ".dreamland.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	orig := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = orig })
	return root
}

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

func TestGitLastTag_NoTags(t *testing.T) {
	stubRunCmd(t, func(_ string, _ ...string) (string, error) {
		return "", errors.New("no tags")
	})
	got, err := gitLastTag()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "v0.0.0" {
		t.Errorf("got %q, want v0.0.0", got)
	}
}

func TestGitLastTag_WithTag(t *testing.T) {
	stubRunCmd(t, func(_ string, _ ...string) (string, error) {
		return "v1.2.3\n", nil
	})
	got, err := gitLastTag()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "v1.2.3" {
		t.Errorf("got %q, want v1.2.3", got)
	}
}

func TestHasNoChanges_NoChanges(t *testing.T) {
	stubRunCmd(t, func(_ string, _ ...string) (string, error) {
		return "", nil
	})
	if !hasNoChanges("v1.0.0") {
		t.Error("expected true when no diff output")
	}
}

func TestHasNoChanges_WithChanges(t *testing.T) {
	stubRunCmd(t, func(_ string, _ ...string) (string, error) {
		return "M  main.go\n", nil
	})
	if hasNoChanges("v1.0.0") {
		t.Error("expected false when diff output present")
	}
}

func TestHasNoChanges_GitError(t *testing.T) {
	stubRunCmd(t, func(_ string, _ ...string) (string, error) {
		return "", errors.New("git error")
	})
	if hasNoChanges("v1.0.0") {
		t.Error("expected false on git error")
	}
}

func TestGitCurrentBranch_Success(t *testing.T) {
	stubRunCmd(t, func(_ string, _ ...string) (string, error) {
		return "feature-branch\n", nil
	})
	got, err := gitCurrentBranch()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "feature-branch" {
		t.Errorf("got %q, want feature-branch", got)
	}
}

func TestGitCurrentBranch_Error(t *testing.T) {
	stubRunCmd(t, func(_ string, _ ...string) (string, error) {
		return "", errors.New("detached HEAD")
	})
	_, err := gitCurrentBranch()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestHasUpstream_True(t *testing.T) {
	stubRunCmd(t, func(_ string, _ ...string) (string, error) {
		return "origin/main\n", nil
	})
	if !hasUpstream() {
		t.Error("expected true when rev-parse succeeds")
	}
}

func TestHasUpstream_False(t *testing.T) {
	stubRunCmd(t, func(_ string, _ ...string) (string, error) {
		return "", errors.New("no upstream")
	})
	if hasUpstream() {
		t.Error("expected false when rev-parse fails")
	}
}

func TestIsNotFound_True(t *testing.T) {
	err := &exec.Error{Name: "missing-tool", Err: exec.ErrNotFound}
	if !isNotFound(err) {
		t.Error("expected true for ErrNotFound")
	}
}

func TestIsNotFound_False(t *testing.T) {
	if isNotFound(fmt.Errorf("some other error")) {
		t.Error("expected false for non-ErrNotFound error")
	}
}

func TestPerformBump_GoPath(t *testing.T) {
	var tagged string
	stubRunCmd(t, func(_ string, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "tag" {
			tagged = args[2] // "git tag -a <version> -m <version>"
		}
		return "", nil
	})
	cfg := &config.Config{} // empty VersionBumpCommand → Go path
	if err := performBump(nil, cfg, t.TempDir(), "v1.0.0", "minor", ""); err != nil {
		t.Fatalf("performBump: %v", err)
	}
	if tagged != "v1.1.0" {
		t.Errorf("tagged %q, want v1.1.0", tagged)
	}
}

func TestPerformBump_GoPath_ExplicitVersion(t *testing.T) {
	var tagged string
	stubRunCmd(t, func(_ string, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "tag" {
			tagged = args[2]
		}
		return "", nil
	})
	cfg := &config.Config{}
	if err := performBump(nil, cfg, t.TempDir(), "v1.0.0", "", "v3.0.0"); err != nil {
		t.Fatalf("performBump: %v", err)
	}
	if tagged != "v3.0.0" {
		t.Errorf("tagged %q, want v3.0.0", tagged)
	}
}

func TestPerformBump_DelegatedPath(t *testing.T) {
	var delegatedArgs []string
	stubRunCmd(t, func(name string, args ...string) (string, error) {
		delegatedArgs = append([]string{name}, args...)
		return "", nil
	})
	cfg := &config.Config{VersionBumpCommand: "npm version"}
	if err := performBump(nil, cfg, t.TempDir(), "v1.0.0", "minor", ""); err != nil {
		t.Fatalf("performBump: %v", err)
	}
	if len(delegatedArgs) < 3 || delegatedArgs[0] != "npm" || delegatedArgs[1] != "version" || delegatedArgs[2] != "minor" {
		t.Errorf("unexpected delegated command: %v", delegatedArgs)
	}
}

func TestRunVersionBump_IdempotentBranch(t *testing.T) {
	root := makeVersionBumpRepo(t, config.Config{})

	// Pre-populate branch-bumps for "my-branch".
	bumpsFile := filepath.Join(root, ".dreamland", "branch-bumps")
	if err := os.MkdirAll(filepath.Dir(bumpsFile), 0o755); err != nil {
		t.Fatal(err)
	}
	bumps := map[string]branchBumpEntry{"my-branch": {Version: "v1.0.0"}}
	data, _ := json.Marshal(bumps)
	os.WriteFile(bumpsFile, data, 0o644)

	stubRunCmd(t, func(_ string, args ...string) (string, error) {
		switch {
		case strings.Contains(strings.Join(args, " "), "describe"):
			return "v1.0.0\n", nil
		case strings.Contains(strings.Join(args, " "), "diff"):
			return "M main.go\n", nil // has changes, so not skipped by no-changes check
		case strings.Contains(strings.Join(args, " "), "abbrev-ref") && !strings.Contains(strings.Join(args, " "), "symbolic"):
			return "my-branch\n", nil
		}
		return "", nil
	})

	origFlags := [4]interface{}{vbMajor, vbMinor, vbPatch, vbVersion}
	vbMajor, vbMinor, vbPatch, vbVersion = false, false, false, ""
	t.Cleanup(func() { vbMajor, vbMinor, vbPatch, vbVersion = origFlags[0].(bool), origFlags[1].(bool), origFlags[2].(bool), origFlags[3].(string) })

	if err := runVersionBump(versionBumpCmd, nil); err != nil {
		t.Fatalf("runVersionBump: %v", err)
	}
}

func TestRunVersionBump_PatchMode_NoChanges(t *testing.T) {
	makeVersionBumpRepo(t, config.Config{})

	stubRunCmd(t, func(_ string, args ...string) (string, error) {
		switch {
		case strings.Contains(strings.Join(args, " "), "describe"):
			return "v1.0.0\n", nil
		case strings.Contains(strings.Join(args, " "), "diff"):
			return "", nil // no changes
		}
		return "", nil
	})

	origFlags := [4]interface{}{vbMajor, vbMinor, vbPatch, vbVersion}
	vbMajor, vbMinor, vbPatch, vbVersion = false, false, true, ""
	t.Cleanup(func() { vbMajor, vbMinor, vbPatch, vbVersion = origFlags[0].(bool), origFlags[1].(bool), origFlags[2].(bool), origFlags[3].(string) })

	if err := runVersionBump(versionBumpCmd, nil); err != nil {
		t.Fatalf("expected silent exit, got: %v", err)
	}
}

func TestRunVersionBump_NewBranch_MinorBump(t *testing.T) {
	root := makeVersionBumpRepo(t, config.Config{})

	var tagCreated string
	stubRunCmd(t, func(_ string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "describe"):
			if tagCreated != "" {
				return tagCreated + "\n", nil
			}
			return "v1.0.0\n", nil
		case strings.Contains(joined, "diff"):
			return "M main.go\n", nil
		case args[0] == "tag" && len(args) >= 3:
			tagCreated = args[2]
		case strings.Contains(joined, "abbrev-ref") && strings.Contains(joined, "symbolic"):
			return "", errors.New("no upstream") // triggers push
		case strings.Contains(joined, "abbrev-ref"):
			return "new-feature\n", nil
		case args[0] == "push":
			// accept push
		}
		return "", nil
	})

	origFlags := [4]interface{}{vbMajor, vbMinor, vbPatch, vbVersion}
	vbMajor, vbMinor, vbPatch, vbVersion = false, false, false, ""
	t.Cleanup(func() { vbMajor, vbMinor, vbPatch, vbVersion = origFlags[0].(bool), origFlags[1].(bool), origFlags[2].(bool), origFlags[3].(string) })

	if err := runVersionBump(versionBumpCmd, nil); err != nil {
		t.Fatalf("runVersionBump: %v", err)
	}

	// branch-bumps should be written.
	bumpsFile := filepath.Join(root, ".dreamland", "branch-bumps")
	data, err := os.ReadFile(bumpsFile)
	if err != nil {
		t.Fatalf("branch-bumps not written: %v", err)
	}
	var bumps map[string]branchBumpEntry
	if err := json.Unmarshal(data, &bumps); err != nil {
		t.Fatalf("branch-bumps invalid JSON: %v", err)
	}
	if _, ok := bumps["new-feature"]; !ok {
		t.Error("branch-bumps missing new-feature entry")
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

// --- Additional tests to improve coverage ---

func TestVersionBumpFlags_VersionAndPatch_TooMany(t *testing.T) {
	// --version counts as explicit; --patch also counts → explicit > 1 → error
	origMajor, origMinor, origPatch, origVersion := vbMajor, vbMinor, vbPatch, vbVersion
	t.Cleanup(func() { vbMajor, vbMinor, vbPatch, vbVersion = origMajor, origMinor, origPatch, origVersion })

	vbMajor = false
	vbMinor = false
	vbPatch = true
	vbVersion = "v9.0.0"

	err := runVersionBump(versionBumpCmd, nil)
	if err == nil {
		t.Fatal("expected error when both --patch and --version set")
	}
}

func TestRunVersionBump_OsGetwdError(t *testing.T) {
	orig := osGetwd
	osGetwd = func() (string, error) { return "", errors.New("getwd failed") }
	t.Cleanup(func() { osGetwd = orig })

	if err := runVersionBump(versionBumpCmd, nil); err == nil || err.Error() != "getwd failed" {
		t.Fatalf("expected getwd error, got: %v", err)
	}
}

func TestRunVersionBump_ConfigLoadError(t *testing.T) {
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

	if err := runVersionBump(versionBumpCmd, nil); err == nil {
		t.Fatal("expected error from config.Load with invalid JSON")
	}
}

func TestRunVersionBump_FindRepoRootError(t *testing.T) {
	// Use a temp dir WITHOUT a .git directory so FindRepoRoot fails.
	root := t.TempDir()
	orig := osGetwd
	osGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { osGetwd = orig })

	if err := runVersionBump(versionBumpCmd, nil); err == nil {
		t.Fatal("expected error from FindRepoRoot when no .git dir")
	}
}

func TestRunVersionBump_GitCurrentBranchError(t *testing.T) {
	root := makeVersionBumpRepo(t, config.Config{})

	origFlags := [4]interface{}{vbMajor, vbMinor, vbPatch, vbVersion}
	vbMajor, vbMinor, vbPatch, vbVersion = false, false, false, ""
	t.Cleanup(func() { vbMajor, vbMinor, vbPatch, vbVersion = origFlags[0].(bool), origFlags[1].(bool), origFlags[2].(bool), origFlags[3].(string) })

	callCount := 0
	stubRunCmd(t, func(_ string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		callCount++
		if strings.Contains(joined, "describe") {
			return "v1.0.0\n", nil
		}
		if strings.Contains(joined, "diff") {
			return "M main.go\n", nil // has changes
		}
		// gitCurrentBranch calls rev-parse --abbrev-ref HEAD (without --symbolic-full-name)
		if strings.Contains(joined, "abbrev-ref") && !strings.Contains(joined, "symbolic") {
			return "", errors.New("detached HEAD")
		}
		return "", nil
	})

	if err := runVersionBump(versionBumpCmd, nil); err == nil {
		t.Fatal("expected gitCurrentBranch error to be propagated")
	}
	_ = root
}

func TestRunVersionBump_MajorLevel(t *testing.T) {
	root := makeVersionBumpRepo(t, config.Config{})

	origFlags := [5]interface{}{vbMajor, vbMinor, vbPatch, vbVersion, vbBreaking}
	vbMajor, vbMinor, vbPatch, vbVersion, vbBreaking = true, false, false, "", false
	t.Cleanup(func() {
		vbMajor, vbMinor, vbPatch, vbVersion, vbBreaking = origFlags[0].(bool), origFlags[1].(bool), origFlags[2].(bool), origFlags[3].(string), origFlags[4].(bool)
	})

	var taggedVersion string
	stubRunCmd(t, func(_ string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "describe"):
			if taggedVersion != "" {
				return taggedVersion + "\n", nil
			}
			return "v1.0.0\n", nil
		case strings.Contains(joined, "diff"):
			return "M main.go\n", nil
		case len(args) > 0 && args[0] == "tag":
			taggedVersion = args[2]
		case strings.Contains(joined, "abbrev-ref") && strings.Contains(joined, "symbolic"):
			return "", errors.New("no upstream")
		case strings.Contains(joined, "abbrev-ref"):
			return "feature-major\n", nil
		case len(args) > 0 && args[0] == "push":
			// accept
		}
		return "", nil
	})

	if err := runVersionBump(versionBumpCmd, nil); err != nil {
		t.Fatalf("runVersionBump with --major: %v", err)
	}
	if taggedVersion != "v2.0.0" {
		t.Errorf("expected v2.0.0 major bump, got %q", taggedVersion)
	}
	_ = root
}

func TestRunVersionBump_MinorFlagLevel(t *testing.T) {
	root := makeVersionBumpRepo(t, config.Config{})

	origFlags := [4]interface{}{vbMajor, vbMinor, vbPatch, vbVersion}
	vbMajor, vbMinor, vbPatch, vbVersion = false, true, false, ""
	t.Cleanup(func() { vbMajor, vbMinor, vbPatch, vbVersion = origFlags[0].(bool), origFlags[1].(bool), origFlags[2].(bool), origFlags[3].(string) })

	var taggedVersion string
	stubRunCmd(t, func(_ string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "describe"):
			if taggedVersion != "" {
				return taggedVersion + "\n", nil
			}
			return "v1.2.0\n", nil
		case strings.Contains(joined, "diff"):
			return "M main.go\n", nil
		case len(args) > 0 && args[0] == "tag":
			taggedVersion = args[2]
		case strings.Contains(joined, "abbrev-ref") && strings.Contains(joined, "symbolic"):
			return "", errors.New("no upstream")
		case strings.Contains(joined, "abbrev-ref"):
			return "feature-minor\n", nil
		case len(args) > 0 && args[0] == "push":
			// accept
		}
		return "", nil
	})

	if err := runVersionBump(versionBumpCmd, nil); err != nil {
		t.Fatalf("runVersionBump with --minor: %v", err)
	}
	if taggedVersion != "v1.3.0" {
		t.Errorf("expected v1.3.0 minor bump, got %q", taggedVersion)
	}
	_ = root
}

func TestRunVersionBump_ExplicitVersionLevel(t *testing.T) {
	root := makeVersionBumpRepo(t, config.Config{})

	origFlags := [4]interface{}{vbMajor, vbMinor, vbPatch, vbVersion}
	vbMajor, vbMinor, vbPatch, vbVersion = false, false, false, "v5.0.0"
	t.Cleanup(func() { vbMajor, vbMinor, vbPatch, vbVersion = origFlags[0].(bool), origFlags[1].(bool), origFlags[2].(bool), origFlags[3].(string) })

	var taggedVersion string
	stubRunCmd(t, func(_ string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "describe"):
			if taggedVersion != "" {
				return taggedVersion + "\n", nil
			}
			return "v1.0.0\n", nil
		case strings.Contains(joined, "diff"):
			return "M main.go\n", nil
		case len(args) > 0 && args[0] == "tag":
			taggedVersion = args[2]
		case strings.Contains(joined, "abbrev-ref") && strings.Contains(joined, "symbolic"):
			return "", errors.New("no upstream")
		case strings.Contains(joined, "abbrev-ref"):
			return "feature-explicit\n", nil
		case len(args) > 0 && args[0] == "push":
			// accept
		}
		return "", nil
	})

	if err := runVersionBump(versionBumpCmd, nil); err != nil {
		t.Fatalf("runVersionBump with --version: %v", err)
	}
	if taggedVersion != "v5.0.0" {
		t.Errorf("expected v5.0.0 explicit version, got %q", taggedVersion)
	}
	_ = root
}

func TestRunVersionBump_PerformBumpError(t *testing.T) {
	root := makeVersionBumpRepo(t, config.Config{})

	origFlags := [4]interface{}{vbMajor, vbMinor, vbPatch, vbVersion}
	vbMajor, vbMinor, vbPatch, vbVersion = false, false, false, ""
	t.Cleanup(func() { vbMajor, vbMinor, vbPatch, vbVersion = origFlags[0].(bool), origFlags[1].(bool), origFlags[2].(bool), origFlags[3].(string) })

	stubRunCmd(t, func(_ string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "describe"):
			return "v1.0.0\n", nil
		case strings.Contains(joined, "diff"):
			return "M main.go\n", nil
		case strings.Contains(joined, "abbrev-ref") && strings.Contains(joined, "symbolic"):
			return "", errors.New("no upstream")
		case strings.Contains(joined, "abbrev-ref"):
			return "feature-bump-error\n", nil
		case len(args) > 0 && args[0] == "tag":
			return "", errors.New("tag failed")
		}
		return "", nil
	})

	if err := runVersionBump(versionBumpCmd, nil); err == nil {
		t.Fatal("expected performBump error to be propagated")
	}
	_ = root
}

func TestRunVersionBump_GitLastTagErrorAfterBump(t *testing.T) {
	// Cover the "newTag = lastTag" fallback path (gitLastTag error after performBump)
	root := makeVersionBumpRepo(t, config.Config{})

	origFlags := [4]interface{}{vbMajor, vbMinor, vbPatch, vbVersion}
	vbMajor, vbMinor, vbPatch, vbVersion = false, false, false, ""
	t.Cleanup(func() { vbMajor, vbMinor, vbPatch, vbVersion = origFlags[0].(bool), origFlags[1].(bool), origFlags[2].(bool), origFlags[3].(string) })

	tagCount := 0
	stubRunCmd(t, func(_ string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "describe"):
			tagCount++
			if tagCount == 1 {
				return "v1.0.0\n", nil // first call: lastTag
			}
			return "", errors.New("no tags found") // second call: after bump → fallback
		case strings.Contains(joined, "diff"):
			return "M main.go\n", nil
		case strings.Contains(joined, "abbrev-ref") && strings.Contains(joined, "symbolic"):
			return "", errors.New("no upstream")
		case strings.Contains(joined, "abbrev-ref"):
			return "feature-tag-fallback\n", nil
		case len(args) > 0 && args[0] == "tag":
			return "", nil // tag succeeds
		case len(args) > 0 && args[0] == "push":
			return "", nil
		}
		return "", nil
	})

	// Should succeed — gitLastTag error after bump causes fallback to lastTag, not a return err
	if err := runVersionBump(versionBumpCmd, nil); err != nil {
		t.Fatalf("expected nil (gitLastTag error after bump is suppressed): %v", err)
	}
	_ = root
}

func TestRunVersionBump_WriteBranchBumpsError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root; skip permission test")
	}

	root := makeVersionBumpRepo(t, config.Config{})

	// Make .dreamland dir exist but unwritable.
	dreamlandDir := filepath.Join(root, ".dreamland")
	if err := os.MkdirAll(dreamlandDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dreamlandDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dreamlandDir, 0o755) })

	origFlags := [4]interface{}{vbMajor, vbMinor, vbPatch, vbVersion}
	vbMajor, vbMinor, vbPatch, vbVersion = false, false, false, ""
	t.Cleanup(func() { vbMajor, vbMinor, vbPatch, vbVersion = origFlags[0].(bool), origFlags[1].(bool), origFlags[2].(bool), origFlags[3].(string) })

	stubRunCmd(t, func(_ string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "describe"):
			return "v1.0.0\n", nil
		case strings.Contains(joined, "diff"):
			return "M main.go\n", nil
		case strings.Contains(joined, "abbrev-ref") && strings.Contains(joined, "symbolic"):
			return "", errors.New("no upstream")
		case strings.Contains(joined, "abbrev-ref"):
			return "write-bumps-error-branch\n", nil
		case len(args) > 0 && args[0] == "tag":
			return "", nil
		}
		return "", nil
	})

	if err := runVersionBump(versionBumpCmd, nil); err == nil {
		t.Fatal("expected writeBranchBumps error from unwritable dir")
	}
}

func TestRunVersionBump_GitPushError(t *testing.T) {
	root := makeVersionBumpRepo(t, config.Config{})

	origFlags := [4]interface{}{vbMajor, vbMinor, vbPatch, vbVersion}
	vbMajor, vbMinor, vbPatch, vbVersion = false, false, false, ""
	t.Cleanup(func() { vbMajor, vbMinor, vbPatch, vbVersion = origFlags[0].(bool), origFlags[1].(bool), origFlags[2].(bool), origFlags[3].(string) })

	taggedVersion := ""
	stubRunCmd(t, func(_ string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "describe"):
			if taggedVersion != "" {
				return taggedVersion + "\n", nil
			}
			return "v1.0.0\n", nil
		case strings.Contains(joined, "diff"):
			return "M main.go\n", nil
		case strings.Contains(joined, "abbrev-ref") && strings.Contains(joined, "symbolic"):
			return "", errors.New("no upstream") // trigger push
		case strings.Contains(joined, "abbrev-ref"):
			return "push-error-branch\n", nil
		case len(args) > 0 && args[0] == "tag":
			taggedVersion = args[2]
			return "", nil
		case len(args) > 0 && args[0] == "push":
			return "push output", errors.New("push failed")
		}
		return "", nil
	})

	if err := runVersionBump(versionBumpCmd, nil); err == nil {
		t.Fatal("expected git push error to be propagated")
	}
	_ = root
}

func TestPerformBump_GoPath_BumpSemverError(t *testing.T) {
	cfg := &config.Config{} // empty VersionBumpCommand → Go path
	// "bogus" level causes bumpSemver to return an error.
	if err := performBump(nil, cfg, t.TempDir(), "v1.0.0", "bogus", ""); err == nil {
		t.Fatal("expected error from bumpSemver with unknown level")
	}
}

func TestPerformBump_GoPath_GitTagError(t *testing.T) {
	stubRunCmd(t, func(_ string, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "tag" {
			return "", errors.New("tag already exists")
		}
		return "", nil
	})
	cfg := &config.Config{}
	if err := performBump(nil, cfg, t.TempDir(), "v1.0.0", "minor", ""); err == nil {
		t.Fatal("expected error from git tag failure")
	}
}

func TestPerformBump_DelegatedPath_ExplicitArg(t *testing.T) {
	// When explicit != "", arg = explicit (not level)
	var delegatedArg string
	stubRunCmd(t, func(name string, args ...string) (string, error) {
		if len(args) > 0 {
			delegatedArg = args[len(args)-1]
		}
		return "", nil
	})
	cfg := &config.Config{VersionBumpCommand: "npm version"}
	if err := performBump(nil, cfg, t.TempDir(), "v1.0.0", "minor", "v2.0.0"); err != nil {
		t.Fatalf("performBump: %v", err)
	}
	if delegatedArg != "v2.0.0" {
		t.Errorf("expected delegated arg to be v2.0.0, got %q", delegatedArg)
	}
}

func TestPerformBump_DelegatedPath_NotFoundError(t *testing.T) {
	stubRunCmd(t, func(_ string, _ ...string) (string, error) {
		return "", &exec.Error{Name: "missing-cmd", Err: exec.ErrNotFound}
	})
	cfg := &config.Config{VersionBumpCommand: "missing-cmd"}
	err := performBump(nil, cfg, t.TempDir(), "v1.0.0", "minor", "")
	if err == nil {
		t.Fatal("expected not-found error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error message, got: %v", err)
	}
}

func TestPerformBump_DelegatedPath_GenericError(t *testing.T) {
	stubRunCmd(t, func(_ string, _ ...string) (string, error) {
		return "output line", errors.New("command failed")
	})
	cfg := &config.Config{VersionBumpCommand: "bump-tool"}
	err := performBump(nil, cfg, t.TempDir(), "v1.0.0", "minor", "")
	if err == nil {
		t.Fatal("expected generic command error")
	}
	if !strings.Contains(err.Error(), "command failed") {
		t.Errorf("expected 'command failed' in error, got: %v", err)
	}
}

func TestParseIntOrZero_NonDigitBreak(t *testing.T) {
	// A string with non-digit chars should stop at first non-digit.
	if got := parseIntOrZero("12abc"); got != 12 {
		t.Errorf("parseIntOrZero(\"12abc\") = %d, want 12", got)
	}
	if got := parseIntOrZero("abc"); got != 0 {
		t.Errorf("parseIntOrZero(\"abc\") = %d, want 0", got)
	}
}

func TestReadBranchBumps_CorruptJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "branch-bumps")
	if err := os.WriteFile(path, []byte("not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}
	bumps, err := readBranchBumps(path)
	if err != nil {
		t.Fatalf("expected nil error for corrupt JSON, got: %v", err)
	}
	if len(bumps) != 0 {
		t.Errorf("expected empty map for corrupt JSON, got: %v", bumps)
	}
}

func TestReadBranchBumps_ReadError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root; skip permission test")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "branch-bumps")
	// Create file then make it unreadable.
	if err := os.WriteFile(path, []byte(`{"branch":{"version":"v1.0.0"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(path, 0o644) })

	_, err := readBranchBumps(path)
	if err == nil {
		t.Fatal("expected error when reading unreadable file")
	}
}

func TestWriteBranchBumps_WriteError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root; skip permission test")
	}
	dir := t.TempDir()
	dreamlandDir := filepath.Join(dir, ".dreamland")
	if err := os.MkdirAll(dreamlandDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Make the dreamland dir unwritable so WriteFile fails.
	if err := os.Chmod(dreamlandDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dreamlandDir, 0o755) })

	path := filepath.Join(dreamlandDir, "branch-bumps")
	bumps := map[string]branchBumpEntry{"main": {Version: "v1.0.0"}}
	if err := writeBranchBumps(path, bumps); err == nil {
		t.Fatal("expected error when writing to unwritable directory")
	}
}
