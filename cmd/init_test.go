package cmd

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/huh"

	"dreamland/internal/config"
)

// makeGitRepo creates a temp dir with a .git subdirectory, changes into it,
// and restores the original working directory on cleanup.
func makeGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	return root
}

// stubWizard returns a wizardRunner that injects the given result without a TTY.
func stubWizard(res *wizardResult, err error) func(*config.Config, io.Writer) (*wizardResult, error) {
	return func(_ *config.Config, _ io.Writer) (*wizardResult, error) {
		return res, err
	}
}

// runInitWithBuf calls runInit with a captured output buffer.
func runInitWithBuf(t *testing.T) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	initCmd.SetOut(&buf)
	t.Cleanup(func() { initCmd.SetOut(nil) })
	err := runInit(initCmd, nil)
	return buf.String(), err
}

func TestInitCmdMetadata(t *testing.T) {
	if initCmd.Use != "init" {
		t.Errorf("Use = %q, want %q", initCmd.Use, "init")
	}
	if initCmd.Short == "" {
		t.Error("Short description should not be empty")
	}
	if initCmd.RunE == nil {
		t.Error("RunE should be set")
	}
}

func TestInitSuccess(t *testing.T) {
	makeGitRepo(t)

	orig := wizardRunner
	wizardRunner = stubWizard(&wizardResult{
		tool:           "Claude Code",
		language:       "Go",
		testCommand:    "go test ./...",
		docCommand:     "godoc",
		versionCommand: "go version",
	}, nil)
	t.Cleanup(func() { wizardRunner = orig })

	out, err := runInitWithBuf(t)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, ".dreamland.json") {
		t.Errorf("expected success message containing .dreamland.json, got: %q", out)
	}

	cwd, _ := os.Getwd()
	cfg, err := config.Load(cwd)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected config, got nil")
	}
	if cfg.CodingTool != "Claude Code" || cfg.Language != "Go" || cfg.TestCommand != "go test ./..." {
		t.Errorf("unexpected config: %+v", cfg)
	}
}

func TestInitDocCommandSkipped(t *testing.T) {
	makeGitRepo(t)

	orig := wizardRunner
	wizardRunner = stubWizard(&wizardResult{
		tool:           "Kiro",
		language:       "Rust",
		testCommand:    "cargo test",
		docCommand:     "",
		versionCommand: "rustc --version",
	}, nil)
	t.Cleanup(func() { wizardRunner = orig })

	_, err := runInitWithBuf(t)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cwd, _ := os.Getwd()
	cfg, _ := config.Load(cwd)
	if cfg == nil {
		t.Fatal("expected config, got nil")
	}
	if cfg.DocCommand != "" {
		t.Errorf("expected empty doc_command, got %q", cfg.DocCommand)
	}
}

func TestInitVersionCommandOverride(t *testing.T) {
	makeGitRepo(t)

	orig := wizardRunner
	wizardRunner = stubWizard(&wizardResult{
		tool:           "Cursor",
		language:       "Python",
		testCommand:    "pytest",
		docCommand:     "sphinx-build",
		versionCommand: "python3.11 --version",
	}, nil)
	t.Cleanup(func() { wizardRunner = orig })

	_, err := runInitWithBuf(t)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cwd, _ := os.Getwd()
	cfg, _ := config.Load(cwd)
	if cfg == nil {
		t.Fatal("expected config, got nil")
	}
	if cfg.VersionCommand != "python3.11 --version" {
		t.Errorf("expected custom version command, got %q", cfg.VersionCommand)
	}
}

func TestInitReInitDecline(t *testing.T) {
	makeGitRepo(t)

	cwd, _ := os.Getwd()
	existing := &config.Config{CodingTool: "Kiro", Language: "Go", TestCommand: "go test", VersionCommand: "go version"}
	if err := config.Save(cwd, existing); err != nil {
		t.Fatal(err)
	}

	orig := wizardRunner
	wizardRunner = func(ex *config.Config, w io.Writer) (*wizardResult, error) {
		if ex == nil {
			t.Error("expected existing config to be passed to wizard runner")
		}
		return nil, errAborted
	}
	t.Cleanup(func() { wizardRunner = orig })

	_, err := runInitWithBuf(t)
	if err != nil {
		t.Fatalf("expected no error on decline, got: %v", err)
	}

	cfg, _ := config.Load(cwd)
	if cfg == nil || cfg.CodingTool != "Kiro" {
		t.Errorf("config was modified after decline, got %+v", cfg)
	}
}

func TestInitReInitConfirm(t *testing.T) {
	makeGitRepo(t)

	cwd, _ := os.Getwd()
	if err := config.Save(cwd, &config.Config{CodingTool: "Kiro", Language: "Go", TestCommand: "go test", VersionCommand: "go version"}); err != nil {
		t.Fatal(err)
	}

	orig := wizardRunner
	wizardRunner = stubWizard(&wizardResult{
		tool:           "Claude Code",
		language:       "Go",
		testCommand:    "go test ./...",
		versionCommand: "go version",
	}, nil)
	t.Cleanup(func() { wizardRunner = orig })

	_, err := runInitWithBuf(t)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg, _ := config.Load(cwd)
	if cfg == nil || cfg.CodingTool != "Claude Code" {
		t.Errorf("expected updated config, got %+v", cfg)
	}
}

func TestInitWizardError(t *testing.T) {
	makeGitRepo(t)

	orig := wizardRunner
	wizardRunner = stubWizard(nil, errors.New("user closed terminal"))
	t.Cleanup(func() { wizardRunner = orig })

	_, err := runInitWithBuf(t)
	if err == nil {
		t.Fatal("expected error when wizard fails, got nil")
	}
}

func TestInitSaveError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses permission checks")
	}
	root := makeGitRepo(t)

	orig := wizardRunner
	wizardRunner = stubWizard(&wizardResult{
		tool: "Kiro", language: "Go", testCommand: "go test", versionCommand: "go version",
	}, nil)
	t.Cleanup(func() { wizardRunner = orig })

	if err := os.Chmod(root, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o755) })

	_, err := runInitWithBuf(t)
	if err == nil {
		t.Fatal("expected error when save fails, got nil")
	}
}

func TestInitNoGitRepo(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	origW := wizardRunner
	wizardRunner = stubWizard(&wizardResult{
		tool: "Kiro", language: "Go", testCommand: "go test", versionCommand: "go version",
	}, nil)
	t.Cleanup(func() { wizardRunner = origW })

	_, err := runInitWithBuf(t)
	if err == nil {
		t.Fatal("expected error when no git repo, got nil")
	}
}

func TestInitGetCwdError(t *testing.T) {
	orig := osGetwd
	osGetwd = func() (string, error) { return "", errors.New("getwd failed") }
	t.Cleanup(func() { osGetwd = orig })

	_, err := runInitWithBuf(t)
	if err == nil {
		t.Fatal("expected error when Getwd fails, got nil")
	}
}

func TestInitLoadError(t *testing.T) {
	// Create a git repo where .dreamland.json is a directory (causes Load to fail
	// with a non-ErrNotExist error, triggering the isNoGitErr guard).
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Place a directory at the config path to force a read error.
	if err := os.Mkdir(filepath.Join(root, ".dreamland.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	orig, _ := os.Getwd()
	_ = os.Chdir(root)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	origW := wizardRunner
	wizardRunner = stubWizard(&wizardResult{tool: "Kiro", language: "Go", testCommand: "go test", versionCommand: "go version"}, nil)
	t.Cleanup(func() { wizardRunner = origW })

	_, err := runInitWithBuf(t)
	if err == nil {
		t.Fatal("expected error for bad config file, got nil")
	}
}

// --- defaultWizardRunner tests (stub huhFormRunner to avoid TTY) ---

func stubHuhFormRunner(err error) func() {
	orig := huhFormRunner
	huhFormRunner = func(_ *huh.Form) error { return err }
	return func() { huhFormRunner = orig }
}

func TestDefaultWizardRunner_NoExisting(t *testing.T) {
	restore := stubHuhFormRunner(nil)
	defer restore()

	var buf bytes.Buffer
	res, err := defaultWizardRunner(nil, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestDefaultWizardRunner_ExistingDeclined(t *testing.T) {
	// huhFormRunner returns nil and confirmed stays false → errAborted.
	restore := stubHuhFormRunner(nil)
	defer restore()

	var buf bytes.Buffer
	existing := &config.Config{CodingTool: "Kiro"}
	_, err := defaultWizardRunner(existing, &buf)
	if !errors.Is(err, errAborted) {
		t.Fatalf("expected errAborted, got %v", err)
	}
	if !strings.Contains(buf.String(), "Aborted") {
		t.Errorf("expected Aborted message, got %q", buf.String())
	}
}

func TestDefaultWizardRunner_ConfirmFormError(t *testing.T) {
	restore := stubHuhFormRunner(errors.New("ctrl-c"))
	defer restore()

	var buf bytes.Buffer
	existing := &config.Config{CodingTool: "Kiro"}
	_, err := defaultWizardRunner(existing, &buf)
	if err == nil {
		t.Fatal("expected error from confirm form, got nil")
	}
}

func TestDefaultWizardRunner_Step1Error(t *testing.T) {
	calls := 0
	orig := huhFormRunner
	huhFormRunner = func(_ *huh.Form) error {
		calls++
		if calls == 1 {
			return errors.New("step 1 error")
		}
		return nil
	}
	defer func() { huhFormRunner = orig }()

	var buf bytes.Buffer
	_, err := defaultWizardRunner(nil, &buf)
	if err == nil {
		t.Fatal("expected error from step 1 form, got nil")
	}
}

func TestDefaultWizardRunner_Step23Error(t *testing.T) {
	calls := 0
	orig := huhFormRunner
	huhFormRunner = func(_ *huh.Form) error {
		calls++
		if calls == 2 {
			return errors.New("step 2-3 error")
		}
		return nil
	}
	defer func() { huhFormRunner = orig }()

	var buf bytes.Buffer
	_, err := defaultWizardRunner(nil, &buf)
	if err == nil {
		t.Fatal("expected error from step 2-3 form, got nil")
	}
}

func TestDefaultWizardRunner_Step45Error(t *testing.T) {
	calls := 0
	orig := huhFormRunner
	huhFormRunner = func(_ *huh.Form) error {
		calls++
		if calls == 3 {
			return errors.New("step 4-5 error")
		}
		return nil
	}
	defer func() { huhFormRunner = orig }()

	var buf bytes.Buffer
	_, err := defaultWizardRunner(nil, &buf)
	if err == nil {
		t.Fatal("expected error from step 4-5 form, got nil")
	}
}

func TestDefaultWizardRunner_Step6Error(t *testing.T) {
	calls := 0
	orig := huhFormRunner
	huhFormRunner = func(_ *huh.Form) error {
		calls++
		if calls == 4 {
			return errors.New("step 6 error")
		}
		return nil
	}
	defer func() { huhFormRunner = orig }()

	var buf bytes.Buffer
	_, err := defaultWizardRunner(nil, &buf)
	if err == nil {
		t.Fatal("expected error from step 6 form, got nil")
	}
}

func TestValidateNonEmpty(t *testing.T) {
	if err := validateNonEmpty("go test ./..."); err != nil {
		t.Errorf("expected nil for non-empty input, got %v", err)
	}
	if err := validateNonEmpty(""); err == nil {
		t.Error("expected error for empty input, got nil")
	}
}

func TestVersionDefaults(t *testing.T) {
	cases := map[string]string{
		"Go":              "go version",
		"Node/TypeScript": "node --version",
		"Rust":            "rustc --version",
		"Python":          "python3 --version",
	}
	for lang, want := range cases {
		if got := versionDefaults[lang]; got != want {
			t.Errorf("versionDefaults[%q] = %q, want %q", lang, got, want)
		}
	}
}

func TestIsNoGitErr(t *testing.T) {
	if !isNoGitErr(errors.New("no git repository found in any parent directory")) {
		t.Error("expected true for no-git error")
	}
	if isNoGitErr(errors.New("some other error")) {
		t.Error("expected false for unrelated error")
	}
	if isNoGitErr(nil) {
		t.Error("expected false for nil error")
	}
}

func TestInitScaffoldOutput(t *testing.T) {
	root := makeGitRepo(t)

	orig := wizardRunner
	wizardRunner = stubWizard(&wizardResult{
		repoRoot:       root,
		tool:           "Claude Code",
		language:       "Go",
		testCommand:    "go test ./...",
		versionCommand: "go version",
	}, nil)
	t.Cleanup(func() { wizardRunner = orig })

	out, err := runInitWithBuf(t)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "installed") {
		t.Errorf("expected scaffold 'installed' lines in output, got:\n%s", out)
	}
	if !strings.Contains(out, "PATH") {
		t.Errorf("expected PATH reminder in output, got:\n%s", out)
	}
}

func TestInitForceFlag(t *testing.T) {
	root := makeGitRepo(t)

	// Pre-create an agent file.
	agentDir := filepath.Join(root, ".claude", "agents")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, "orchestrator.md"), []byte("old content"), 0o644); err != nil {
		t.Fatal(err)
	}

	orig := wizardRunner
	wizardRunner = stubWizard(&wizardResult{
		repoRoot:       root,
		tool:           "Claude Code",
		language:       "Go",
		testCommand:    "go test ./...",
		versionCommand: "go version",
	}, nil)
	t.Cleanup(func() { wizardRunner = orig })

	origForce := forceFlag
	forceFlag = true
	t.Cleanup(func() { forceFlag = origForce })

	out, err := runInitWithBuf(t)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "installed (forced)") {
		t.Errorf("expected 'installed (forced)' in output, got:\n%s", out)
	}

	data, _ := os.ReadFile(filepath.Join(agentDir, "orchestrator.md"))
	if string(data) == "old content" {
		t.Error("orchestrator.md was not overwritten with --force")
	}
}

func TestInitEmailSuffix(t *testing.T) {
	root := makeGitRepo(t)

	orig := wizardRunner
	wizardRunner = stubWizard(&wizardResult{
		repoRoot:       root,
		tool:           "Kiro",
		language:       "Go",
		testCommand:    "go test ./...",
		versionCommand: "go version",
	}, nil)
	t.Cleanup(func() { wizardRunner = orig })

	origSuffix := emailSuffixFlag
	emailSuffixFlag = "@myorg.com"
	t.Cleanup(func() { emailSuffixFlag = origSuffix })

	if _, err := runInitWithBuf(t); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cwd, _ := os.Getwd()
	cfg, _ := config.Load(cwd)
	if cfg == nil {
		t.Fatal("expected config")
	}
	if cfg.EmailSuffix != "@myorg.com" {
		t.Errorf("EmailSuffix = %q, want @myorg.com", cfg.EmailSuffix)
	}
}

func TestInitVersionBumpCommandSet(t *testing.T) {
	root := makeGitRepo(t)

	orig := wizardRunner
	wizardRunner = stubWizard(&wizardResult{
		repoRoot:       root,
		tool:           "Codex CLI",
		language:       "Node/TypeScript",
		testCommand:    "npm test",
		versionCommand: "node --version",
	}, nil)
	t.Cleanup(func() { wizardRunner = orig })

	if _, err := runInitWithBuf(t); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cwd, _ := os.Getwd()
	cfg, _ := config.Load(cwd)
	if cfg == nil {
		t.Fatal("expected config")
	}
	if cfg.VersionBumpCommand != "npm version" {
		t.Errorf("VersionBumpCommand = %q, want npm version", cfg.VersionBumpCommand)
	}
	if cfg.ModelID != "codex-1" {
		t.Errorf("ModelID = %q, want codex-1", cfg.ModelID)
	}
}

func TestValidatePathExists_Valid(t *testing.T) {
	dir := t.TempDir()
	if err := validatePathExists(dir); err != nil {
		t.Errorf("expected nil for existing path, got: %v", err)
	}
}

func TestValidatePathExists_Invalid(t *testing.T) {
	if err := validatePathExists(filepath.Join(t.TempDir(), "nonexistent")); err == nil {
		t.Error("expected error for non-existent path, got nil")
	}
}
