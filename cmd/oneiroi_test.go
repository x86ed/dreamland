package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// oneiroiRegistryEntry mirrors the .dreamland/oneiroi/registry.json entry shape
// documented in design.md decision 4 (task 2.1's Entry struct), decoded independently
// of internal/oneiroi's Go types so these CLI-level tests don't couple to that
// package's exact exported names.
type oneiroiRegistryEntry struct {
	Name      string   `json:"name"`
	Words     []string `json:"words"`
	Role      string   `json:"role"`
	ToolTier  string   `json:"tool_tier"`
	Parent    *string  `json:"parent"`
	Created   string   `json:"created"`
	Revisions []struct {
		Word   string `json:"word"`
		Reason string `json:"reason"`
		At     string `json:"at"`
	} `json:"revisions"`
}

type oneiroiRegistryFile struct {
	Agents []oneiroiRegistryEntry `json:"agents"`
}

// oneiroiGitRepo creates a temp git repo, chdirs into it (execCLI/osGetwd resolve
// against cwd, matching the pattern in telemetry_test.go), and stubs runCmd so the
// git plumbing CommitScaffold (task 6.1) shells out to never touches a real git
// binary — these are unit tests of the registry/scaffold effects, not of the commit
// step itself (see oneiroi_integration_test.go, task 6.5, for the real-git case).
func oneiroiGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git", "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	orig, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	stubRunCmd(t, func(_ string, _ ...string) (string, error) { return "", nil })

	return root
}

// readOneiroiRegistry reads and decodes .dreamland/oneiroi/registry.json.
func readOneiroiRegistry(t *testing.T, root string) oneiroiRegistryFile {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".dreamland", "oneiroi", "registry.json"))
	if err != nil {
		t.Fatalf("reading registry.json: %v", err)
	}
	var reg oneiroiRegistryFile
	if err := json.Unmarshal(data, &reg); err != nil {
		t.Fatalf("parsing registry.json: %v\n%s", err, data)
	}
	return reg
}

func findOneiroiEntry(reg oneiroiRegistryFile, name string) *oneiroiRegistryEntry {
	for i := range reg.Agents {
		if reg.Agents[i].Name == name {
			return &reg.Agents[i]
		}
	}
	return nil
}

// TestOneiroiSeed_HappyPath covers the "Fresh family name generated with no collision"
// and "Stub files exist on all six platforms after seeding" scenarios end to end via
// the CLI entry point.
func TestOneiroiSeed_HappyPath(t *testing.T) {
	root := oneiroiGitRepo(t)

	if _, _, err := execCLI(t, "oneiroi", "seed", "--role", "example role"); err != nil {
		t.Fatalf("oneiroi seed: %v", err)
	}

	reg := readOneiroiRegistry(t, root)
	if len(reg.Agents) != 1 {
		t.Fatalf("expected exactly one registry entry, got %d: %+v", len(reg.Agents), reg.Agents)
	}
	entry := reg.Agents[0]

	if len(entry.Words) != 2 {
		t.Errorf("expected exactly 2 words for a fresh seed, got %v", entry.Words)
	}
	if entry.Role != "example role" {
		t.Errorf("role = %q, want %q", entry.Role, "example role")
	}
	if entry.ToolTier != "full-edit" {
		t.Errorf("tool_tier = %q, want default %q", entry.ToolTier, "full-edit")
	}
	if entry.Parent != nil {
		t.Errorf("expected nil parent for a fresh seed, got %v", *entry.Parent)
	}
	if entry.Name == "" {
		t.Fatal("expected a non-empty generated name")
	}

	// Stub files on every supported platform.
	for platform, path := range stubAgentPathsForCmdTest(root, entry.Name) {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s: missing stub agent file %s: %v", platform, path, err)
		}
	}

	// Slash command on Claude Code.
	if _, err := os.Stat(filepath.Join(root, ".claude", "commands", "drmlnd", entry.Name+".md")); err != nil {
		t.Errorf("missing slash command file for %s: %v", entry.Name, err)
	}
}

// TestOneiroiSeed_ToolTierFlag covers "--tool-tier SHALL accept one of ... when
// omitted, it SHALL default to full-edit" — the non-default branch.
func TestOneiroiSeed_ToolTierFlag(t *testing.T) {
	root := oneiroiGitRepo(t)

	if _, _, err := execCLI(t, "oneiroi", "seed", "--role", "example role", "--tool-tier", "write-only-no-edit"); err != nil {
		t.Fatalf("oneiroi seed: %v", err)
	}

	reg := readOneiroiRegistry(t, root)
	if len(reg.Agents) != 1 {
		t.Fatalf("expected exactly one registry entry, got %d", len(reg.Agents))
	}
	if got := reg.Agents[0].ToolTier; got != "write-only-no-edit" {
		t.Errorf("tool_tier = %q, want %q", got, "write-only-no-edit")
	}
}

// TestOneiroiSeed_InvalidToolTier covers the tool-binding matrix's closed tier set:
// an unrecognized --tool-tier value must error and must not write a registry entry.
func TestOneiroiSeed_InvalidToolTier(t *testing.T) {
	root := oneiroiGitRepo(t)

	if _, _, err := execCLI(t, "oneiroi", "seed", "--role", "example role", "--tool-tier", "not-a-real-tier"); err == nil {
		t.Fatal("expected error for an invalid --tool-tier value")
	}

	if data, err := os.ReadFile(filepath.Join(root, ".dreamland", "oneiroi", "registry.json")); err == nil {
		var reg oneiroiRegistryFile
		if jsonErr := json.Unmarshal(data, &reg); jsonErr == nil && len(reg.Agents) != 0 {
			t.Errorf("expected no registry entry written for an invalid tool tier, got: %+v", reg.Agents)
		}
	}
}

// TestOneiroiRevise_AddsThirdWord covers "Revision adds a third word to an existing
// 2-word family".
func TestOneiroiRevise_AddsThirdWord(t *testing.T) {
	root := oneiroiGitRepo(t)

	if _, _, err := execCLI(t, "oneiroi", "seed", "--role", "example role"); err != nil {
		t.Fatalf("oneiroi seed: %v", err)
	}
	seeded := readOneiroiRegistry(t, root).Agents[0]

	if _, _, err := execCLI(t, "oneiroi", "revise", "--agent", seeded.Name, "--reason", "model swap"); err != nil {
		t.Fatalf("oneiroi revise: %v", err)
	}

	reg := readOneiroiRegistry(t, root)
	revised := findOneiroiEntry(reg, seeded.Name)
	if revised == nil {
		t.Fatalf("revise must not create a new registry entry; %s not found in %+v", seeded.Name, reg.Agents)
	}
	if len(revised.Words) != 3 {
		t.Errorf("expected 3 words after revision, got %v", revised.Words)
	}
	if revised.Words[0] != seeded.Words[0] || revised.Words[1] != seeded.Words[1] {
		t.Errorf("expected the original two family words preserved, got %v (was %v)", revised.Words, seeded.Words)
	}
}

// TestOneiroiRevise_ReplacesNotAppendsThirdWord covers "Re-revision replaces, not
// appends, the third word": a second revise call must keep exactly 3 live words, and
// the entry's revisions history must record the replaced word.
func TestOneiroiRevise_ReplacesNotAppendsThirdWord(t *testing.T) {
	root := oneiroiGitRepo(t)

	if _, _, err := execCLI(t, "oneiroi", "seed", "--role", "example role"); err != nil {
		t.Fatalf("oneiroi seed: %v", err)
	}
	seeded := readOneiroiRegistry(t, root).Agents[0]

	if _, _, err := execCLI(t, "oneiroi", "revise", "--agent", seeded.Name, "--reason", "first revision"); err != nil {
		t.Fatalf("first oneiroi revise: %v", err)
	}
	firstRevision := findOneiroiEntry(readOneiroiRegistry(t, root), seeded.Name)
	firstThirdWord := firstRevision.Words[2]

	if _, _, err := execCLI(t, "oneiroi", "revise", "--agent", seeded.Name, "--reason", "second revision"); err != nil {
		t.Fatalf("second oneiroi revise: %v", err)
	}
	reg := readOneiroiRegistry(t, root)
	secondRevision := findOneiroiEntry(reg, seeded.Name)
	if secondRevision == nil {
		t.Fatalf("entry disappeared after second revise: %+v", reg.Agents)
	}
	if len(secondRevision.Words) != 3 {
		t.Fatalf("expected exactly 3 live words after a second revision, got %v", secondRevision.Words)
	}

	foundPriorInHistory := false
	for _, rev := range secondRevision.Revisions {
		if rev.Word == firstThirdWord {
			foundPriorInHistory = true
		}
	}
	if !foundPriorInHistory {
		t.Errorf("expected the replaced third word %q preserved in revisions history, got %+v", firstThirdWord, secondRevision.Revisions)
	}
}

// TestOneiroiFork_CreatesSiblingSharingFamilyPair covers "Fork creates a sibling
// sharing the family pair with its own third word" and "Neither operation ... modifies
// the entry it does not target".
func TestOneiroiFork_CreatesSiblingSharingFamilyPair(t *testing.T) {
	root := oneiroiGitRepo(t)

	if _, _, err := execCLI(t, "oneiroi", "seed", "--role", "example role"); err != nil {
		t.Fatalf("oneiroi seed: %v", err)
	}
	parent := readOneiroiRegistry(t, root).Agents[0]

	if _, _, err := execCLI(t, "oneiroi", "fork", "--agent", parent.Name, "--role", "variant role"); err != nil {
		t.Fatalf("oneiroi fork: %v", err)
	}

	reg := readOneiroiRegistry(t, root)
	if len(reg.Agents) != 2 {
		t.Fatalf("expected 2 registry entries after fork, got %d: %+v", len(reg.Agents), reg.Agents)
	}

	// Parent entry is untouched.
	stillParent := findOneiroiEntry(reg, parent.Name)
	if stillParent == nil {
		t.Fatalf("parent entry %s missing after fork", parent.Name)
	}
	if len(stillParent.Words) != 2 || stillParent.Words[0] != parent.Words[0] || stillParent.Words[1] != parent.Words[1] {
		t.Errorf("parent entry mutated by fork: was %v, now %v", parent.Words, stillParent.Words)
	}
	if stillParent.Role != parent.Role {
		t.Errorf("parent role mutated by fork: was %q, now %q", parent.Role, stillParent.Role)
	}

	// New fork entry.
	var forkEntry *oneiroiRegistryEntry
	for i := range reg.Agents {
		if reg.Agents[i].Name != parent.Name {
			forkEntry = &reg.Agents[i]
		}
	}
	if forkEntry == nil {
		t.Fatal("expected a new fork entry distinct from the parent")
	}
	if len(forkEntry.Words) != 3 {
		t.Fatalf("expected fork entry to have 3 words, got %v", forkEntry.Words)
	}
	if forkEntry.Words[0] != parent.Words[0] || forkEntry.Words[1] != parent.Words[1] {
		t.Errorf("fork entry does not inherit parent's word1/word2 verbatim: got %v, parent %v", forkEntry.Words, parent.Words)
	}
	if forkEntry.Parent == nil || *forkEntry.Parent != parent.Name {
		t.Errorf("expected fork entry's parent field to be %q, got %v", parent.Name, forkEntry.Parent)
	}
	if forkEntry.Role != "variant role" {
		t.Errorf("fork entry role = %q, want %q", forkEntry.Role, "variant role")
	}
}

// TestOneiroiFork_SiblingThirdWordsNeverCollide covers "Fork's third word never
// collides with a sibling fork's third word".
func TestOneiroiFork_SiblingThirdWordsNeverCollide(t *testing.T) {
	root := oneiroiGitRepo(t)

	if _, _, err := execCLI(t, "oneiroi", "seed", "--role", "example role"); err != nil {
		t.Fatalf("oneiroi seed: %v", err)
	}
	parent := readOneiroiRegistry(t, root).Agents[0]

	if _, _, err := execCLI(t, "oneiroi", "fork", "--agent", parent.Name, "--role", "variant role one"); err != nil {
		t.Fatalf("first oneiroi fork: %v", err)
	}
	if _, _, err := execCLI(t, "oneiroi", "fork", "--agent", parent.Name, "--role", "variant role two"); err != nil {
		t.Fatalf("second oneiroi fork: %v", err)
	}

	reg := readOneiroiRegistry(t, root)
	var thirdWords []string
	for _, e := range reg.Agents {
		if e.Name == parent.Name {
			continue
		}
		if len(e.Words) != 3 {
			t.Fatalf("fork entry %s does not have 3 words: %v", e.Name, e.Words)
		}
		thirdWords = append(thirdWords, e.Words[2])
	}
	if len(thirdWords) != 2 {
		t.Fatalf("expected 2 fork entries, got %d", len(thirdWords))
	}
	if thirdWords[0] == thirdWords[1] {
		t.Errorf("sibling forks share the same third word %q", thirdWords[0])
	}
}

// stubAgentPathsForCmdTest mirrors internal/scaffold's platformAgentSpec path
// convention (see internal/scaffold/scaffold_stub_test.go's stubAgentPaths, duplicated
// here rather than imported so this file compiles against the cmd package alone).
func stubAgentPathsForCmdTest(root, name string) map[string]string {
	return map[string]string{
		"Claude Code":    filepath.Join(root, ".claude", "agents", name+".md"),
		"Codex CLI":      filepath.Join(root, ".codex", "agents", name+".toml"),
		"Cursor":         filepath.Join(root, ".cursor", "rules", name+".mdc"),
		"Kiro":           filepath.Join(root, ".kiro", "steering", name+".md"),
		"Antigravity":    filepath.Join(root, ".agents", "skills", name, "SKILL.md"),
		"GitHub Copilot": filepath.Join(root, ".github", "agents", name+".agent.md"),
	}
}
