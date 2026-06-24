package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"dreamland/internal/config"
	"dreamland/internal/scaffold"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize dreamland for this repository",
	Long:  "Run an interactive wizard to configure the AI coding tool, language, test command, doc command, and version command for this repository.",
	RunE:  runInit,
}

var (
	forceFlag       bool
	emailSuffixFlag string
)

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolVar(&forceFlag, "force", false, "overwrite existing scaffold files")
	initCmd.Flags().StringVar(&emailSuffixFlag, "email-suffix", "@github.com", "email domain suffix for agent and model emails (e.g. @myorg.com)")
}

// versionDefaults maps language identifiers to their default version commands.
var versionDefaults = map[string]string{
	"Go":              "go version",
	"Node/TypeScript": "node --version",
	"Rust":            "rustc --version",
	"Python":          "python3 --version",
}

// versionBumpDefaults maps language to the language-tool version bump command.
var versionBumpDefaults = map[string]string{
	"Go":              "",
	"Node/TypeScript": "npm version",
	"Rust":            "cargo bump",
	"Python":          "bump-my-version bump",
}

// modelIDDefaults maps coding tool to the default model ID string.
var modelIDDefaults = map[string]string{
	"Claude Code":    "claude-sonnet-4-6",
	"Codex CLI":      "codex-1",
	"Cursor":         "cursor-default",
	"GitHub Copilot": "gpt-4o",
	"Antigravity":    "gemini-2.5-pro",
	"Kiro":           "kiro-default",
}

// wizardResult holds the values collected by the wizard.
type wizardResult struct {
	repoRoot        string
	tool            string
	language        string
	testCommand     string
	docCommand      string
	versionCommand  string
	otelEndpoint    string
	bedrockLogGroup string
}

// errAborted is returned by the wizard runner when the user declines re-init.
var errAborted = errors.New("aborted")

// wizardRunner runs the interactive wizard and returns collected values.
// Replaced in tests to avoid requiring a TTY.
var wizardRunner = defaultWizardRunner

// huhFormRunner executes a huh form. Replaced in tests to avoid requiring a TTY.
var huhFormRunner = func(f *huh.Form) error { return f.Run() }

// osGetwd returns the current working directory. Replaced in tests.
var osGetwd = os.Getwd

// validateNonEmpty is a huh validator that rejects empty strings.
func validateNonEmpty(s string) error {
	if s == "" {
		return fmt.Errorf("cannot be empty")
	}
	return nil
}

// validatePathExists is a huh validator that requires the path to exist on disk.
func validatePathExists(s string) error {
	if s == "" {
		return fmt.Errorf("cannot be empty")
	}
	if _, err := os.Stat(s); err != nil {
		return fmt.Errorf("path does not exist")
	}
	return nil
}

func defaultWizardRunner(existing *config.Config, out io.Writer) (*wizardResult, error) {
	// Re-init guard.
	if existing != nil {
		var confirmed bool
		confirm := huh.NewConfirm().
			Title("A .dreamland.json already exists. Re-initialize?").
			Value(&confirmed)
		if err := huhFormRunner(huh.NewForm(huh.NewGroup(confirm))); err != nil {
			return nil, err
		}
		if !confirmed {
			fmt.Fprintln(out, "Aborted.")
			return nil, errAborted
		}
	}

	var res wizardResult

	// Step 1: repo root (pre-filled with detected git root).
	cwd, _ := os.Getwd()
	detectedRoot, err := config.FindRepoRoot(cwd)
	if err != nil {
		detectedRoot = cwd
	}
	res.repoRoot = detectedRoot

	repoRootInput := huh.NewInput().
		Title("Step 1/7 — Repository root (where agents and hooks will be installed)?").
		Validate(validatePathExists).
		Value(&res.repoRoot)

	if err := huhFormRunner(huh.NewForm(huh.NewGroup(repoRootInput))); err != nil {
		return nil, err
	}

	// Step 2 & 3: coding tool and language.
	toolSelect := huh.NewSelect[string]().
		Title("Step 2/7 — Which AI coding tool are you using?").
		Options(
			huh.NewOption("Claude Code", "Claude Code"),
			huh.NewOption("Codex CLI", "Codex CLI"),
			huh.NewOption("Cursor", "Cursor"),
			huh.NewOption("GitHub Copilot", "GitHub Copilot"),
			huh.NewOption("Antigravity", "Antigravity"),
			huh.NewOption("Kiro", "Kiro"),
		).
		Value(&res.tool)

	langSelect := huh.NewSelect[string]().
		Title("Step 3/7 — Primary language?").
		Options(
			huh.NewOption("Go", "Go"),
			huh.NewOption("Node/TypeScript", "Node/TypeScript"),
			huh.NewOption("Rust", "Rust"),
			huh.NewOption("Python", "Python"),
		).
		Value(&res.language)

	if err := huhFormRunner(huh.NewForm(huh.NewGroup(toolSelect, langSelect))); err != nil {
		return nil, err
	}

	// Kiro: Bedrock logging notice and bedrock_log_group prompt.
	if res.tool == "Kiro" {
		fmt.Fprintln(out, "\nKiro telemetry uses AWS Bedrock model invocation logging.")
		fmt.Fprintln(out, "Enable it at: AWS Console → Bedrock → Settings → Model invocation logging → CloudWatch Logs")
		fmt.Fprintln(out, "Required IAM permission: logs:FilterLogEvents on the log group.")
		res.bedrockLogGroup = "aws/bedrock/modelinvocations"
		logGroupInput := huh.NewInput().
			Title("Bedrock CloudWatch log group name?").
			Value(&res.bedrockLogGroup)
		if err := huhFormRunner(huh.NewForm(huh.NewGroup(logGroupInput))); err != nil {
			return nil, err
		}
	}

	// Steps 4 & 5: test command (required) and doc command (optional).
	testInput := huh.NewInput().
		Title("Step 4/7 — Test command (e.g. go test ./...)?").
		Validate(validateNonEmpty).
		Value(&res.testCommand)

	docInput := huh.NewInput().
		Title("Step 5/7 — Doc generation command (optional, press Enter to skip)?").
		Value(&res.docCommand)

	if err := huhFormRunner(huh.NewForm(huh.NewGroup(testInput, docInput))); err != nil {
		return nil, err
	}

	// Step 6: version command (pre-filled from language).
	res.versionCommand = versionDefaults[res.language]
	versionInput := huh.NewInput().
		Title("Step 6/7 — Version command?").
		Value(&res.versionCommand)

	if err := huhFormRunner(huh.NewForm(huh.NewGroup(versionInput))); err != nil {
		return nil, err
	}

	// Step 7: OTEL endpoint (optional).
	res.otelEndpoint = "http://localhost:4317"
	otelInput := huh.NewInput().
		Title("Step 7/7 — OpenTelemetry endpoint (press Enter for default http://localhost:4317)?").
		Value(&res.otelEndpoint)

	if err := huhFormRunner(huh.NewForm(huh.NewGroup(otelInput))); err != nil {
		return nil, err
	}

	return &res, nil
}

// runInit is the cobra RunE handler for `dreamland init`.
func runInit(cmd *cobra.Command, _ []string) error {
	cwd, err := osGetwd()
	if err != nil {
		return err
	}

	existing, err := config.Load(cwd)
	if err != nil && !isNoGitErr(err) {
		return err
	}

	res, err := wizardRunner(existing, cmd.OutOrStdout())
	if errors.Is(err, errAborted) {
		return nil
	}
	if err != nil {
		return err
	}

	suffix := emailSuffixFlag
	if suffix == "" {
		suffix = "@github.com"
	}

	otelEndpoint := res.otelEndpoint
	if strings.TrimSpace(otelEndpoint) == "" {
		otelEndpoint = "http://localhost:4317"
	}

	cfg := &config.Config{
		CodingTool:         res.tool,
		Language:           res.language,
		TestCommand:        res.testCommand,
		DocCommand:         res.docCommand,
		VersionCommand:     res.versionCommand,
		RepoRoot:           res.repoRoot,
		VersionBumpCommand: versionBumpDefaults[res.language],
		ModelID:            modelIDDefaults[res.tool],
		EmailSuffix:        suffix,
		OtelEndpoint:       otelEndpoint,
		BedrockLogGroup:    res.bedrockLogGroup,
	}

	if err := config.Save(cwd, cfg); err != nil {
		return err
	}

	// Run base scaffold installer (agents + hook bindings).
	repoRoot := res.repoRoot
	if repoRoot == "" {
		repoRoot, _ = config.FindRepoRoot(cwd)
	}
	results, err := scaffold.Install(scaffold.Config{
		RepoRoot:   repoRoot,
		CodingTool: res.tool,
		Force:      forceFlag,
	})
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "scaffold error: %v\n", err)
		return err
	}
	for _, r := range results {
		fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", r.Action, r.Path)
	}

	// Post-save: OTEL env, telemetry hook binding, commit-msg hook, .gitignore.
	if err := scaffold.InstallOtelEnv(repoRoot, cfg); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "OTEL env setup warning: %v\n", err)
	}
	if err := scaffold.ScaffoldTelemetry(repoRoot, res.tool); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "telemetry scaffold warning: %v\n", err)
	}
	if err := scaffold.InstallCommitMsgHook(repoRoot); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "commit-msg hook warning: %v\n", err)
	}
	if err := scaffold.EnsureGitignoreEntry(repoRoot, ".dreamland-session.json"); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), ".gitignore warning: %v\n", err)
	}

	// Codex: prompt to write ~/.codex/config.toml.
	if res.tool == "Codex CLI" {
		writeCodexOtelConfig(cmd, otelEndpoint)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Initialized! Config written to %s/.dreamland.json\n", repoRoot)
	fmt.Fprintln(cmd.OutOrStdout(), "Ensure 'dreamland' is in your PATH — hook bindings call it directly.")
	return nil
}

// confirmCodexOtel prompts the user to confirm writing the OTEL block. Swappable in tests.
var confirmCodexOtel = func() bool {
	var confirmed bool
	confirm := huh.NewConfirm().
		Title("Write OTEL config to ~/.codex/config.toml?").
		Value(&confirmed)
	if err := huhFormRunner(huh.NewForm(huh.NewGroup(confirm))); err != nil {
		return false
	}
	return confirmed
}

// writeCodexOtelConfig prompts the user and writes the [otel] block to ~/.codex/config.toml.
func writeCodexOtelConfig(cmd *cobra.Command, endpoint string) {
	if !confirmCodexOtel() {
		fmt.Fprintln(cmd.OutOrStdout(), "Skipped ~/.codex/config.toml update. Merge manually from .codex/otel-config.example.toml.")
		return
	}

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "codex OTEL: cannot find home dir: %v\n", err)
		return
	}
	target := filepath.Join(home, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "codex OTEL: %v\n", err)
		return
	}

	// Read existing to check if [otel] block already present.
	existing, _ := os.ReadFile(target)
	if strings.Contains(string(existing), "[otel]") {
		fmt.Fprintln(cmd.OutOrStdout(), "~/.codex/config.toml already has [otel] block — skipped.")
		return
	}

	block := fmt.Sprintf("\n[otel]\notlp_endpoint = %q\notlp_protocol = \"grpc\"\nservice_name   = \"dreamland\"\n", endpoint)
	f, err := os.OpenFile(target, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "codex OTEL: %v\n", err)
		return
	}
	defer f.Close()
	if _, err := f.WriteString(block); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "codex OTEL: %v\n", err)
		return
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Written OTEL config to %s\n", target)
}

// isNoGitErr reports whether err is the "no git repository" sentinel.
func isNoGitErr(err error) bool {
	return err != nil && err.Error() == "no git repository found in any parent directory"
}
