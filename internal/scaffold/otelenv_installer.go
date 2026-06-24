package scaffold

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"dreamland/internal/config"
)

// InstallOtelEnv orchestrates per-tool OTEL environment setup.
// Per-file failures are logged to stderr but do not return an error.
func InstallOtelEnv(repoRoot string, cfg *config.Config) error {
	endpoint := cfg.OtelEndpoint
	if endpoint == "" {
		endpoint = "http://localhost:4317"
	}

	switch cfg.CodingTool {
	case "Claude Code":
		installClaudeOtelEnv(repoRoot, endpoint)
	case "Cursor":
		installCursorOtelEnv(repoRoot, endpoint)
	case "Kiro":
		installKiroOtelEnv(repoRoot, endpoint)
	case "Antigravity":
		installAntigravityOtelEnv(repoRoot, endpoint)
	case "GitHub Copilot":
		installCopilotOtelEnv(repoRoot, endpoint)
	case "Codex CLI":
		// Codex OTEL is written to ~/.codex/config.toml with a confirmation prompt
		// handled in cmd/init.go; no session-scoped env script needed here.
	}
	return nil
}

func installClaudeOtelEnv(repoRoot, endpoint string) {
	script, err := RenderOtelEnvScript("Claude Code", endpoint)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dreamland: OTEL env warning (claude): %v\n", err)
		return
	}
	scriptPath := filepath.Join(repoRoot, ".claude", "scripts", "dreamland-otel-env.sh")
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "dreamland: OTEL env warning (claude): %v\n", err)
		return
	}
	if err := atomicWrite(scriptPath, script, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "dreamland: OTEL env warning (claude): %v\n", err)
		return
	}
	patch := map[string]any{
		"hooks": map[string]any{
			"SessionStart": []any{
				map[string]any{
					"matcher": "",
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": "bash .claude/scripts/dreamland-otel-env.sh",
						},
					},
				},
			},
		},
	}
	patchBytes, _ := json.Marshal(patch)
	if err := atomicJSONMerge(filepath.Join(repoRoot, ".claude", "settings.json"), patchBytes); err != nil {
		fmt.Fprintf(os.Stderr, "dreamland: OTEL env warning (claude settings): %v\n", err)
	}
}

func installCursorOtelEnv(repoRoot, endpoint string) {
	script, err := RenderOtelEnvScript("Cursor", endpoint)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dreamland: OTEL env warning (cursor): %v\n", err)
		return
	}
	scriptPath := filepath.Join(repoRoot, ".cursor", "hooks", "dreamland-otel-env.sh")
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "dreamland: OTEL env warning (cursor): %v\n", err)
		return
	}
	if err := atomicWrite(scriptPath, script, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "dreamland: OTEL env warning (cursor): %v\n", err)
		return
	}
	patch := map[string]any{
		"version": 1,
		"hooks": map[string]any{
			"sessionStart": []any{
				map[string]any{
					"type":    "command",
					"command": "bash .cursor/hooks/dreamland-otel-env.sh",
				},
			},
		},
	}
	patchBytes, _ := json.Marshal(patch)
	if err := atomicJSONMerge(filepath.Join(repoRoot, ".cursor", "hooks.json"), patchBytes); err != nil {
		fmt.Fprintf(os.Stderr, "dreamland: OTEL env warning (cursor hooks): %v\n", err)
	}
}

func installKiroOtelEnv(repoRoot, endpoint string) {
	script, err := RenderOtelEnvScript("Kiro", endpoint)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dreamland: OTEL env warning (kiro): %v\n", err)
		return
	}
	scriptPath := filepath.Join(repoRoot, ".kiro", "hooks", "dreamland-otel-env.sh")
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "dreamland: OTEL env warning (kiro): %v\n", err)
		return
	}
	if err := atomicWrite(scriptPath, script, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "dreamland: OTEL env warning (kiro): %v\n", err)
	}
}

func installAntigravityOtelEnv(repoRoot, endpoint string) {
	agentsHooks := map[string]any{
		"hooks": map[string]any{
			"SessionStart": []any{
				map[string]any{
					"command": fmt.Sprintf(
						"export OTEL_EXPORTER_OTLP_ENDPOINT='%s' OTEL_EXPORTER_OTLP_PROTOCOL='grpc' OTEL_SERVICE_NAME='dreamland' IDE_OTEL_IDE_NAME='antigravity'",
						endpoint,
					),
				},
			},
		},
	}
	patchBytes, _ := json.Marshal(agentsHooks)
	target := filepath.Join(repoRoot, ".agents", "hooks.json")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "dreamland: OTEL env warning (antigravity): %v\n", err)
		return
	}
	if err := atomicJSONMerge(target, patchBytes); err != nil {
		fmt.Fprintf(os.Stderr, "dreamland: OTEL env warning (antigravity): %v\n", err)
	}
}

func installCopilotOtelEnv(repoRoot, endpoint string) {
	patch := map[string]any{
		"github.copilot.chat.otel.enabled":      true,
		"github.copilot.chat.otel.exporterType": "otlp-http",
		"github.copilot.chat.otel.otlpEndpoint": copilotOtelEndpoint(endpoint),
	}
	if err := MergeVscodeSettings(repoRoot, patch); err != nil {
		fmt.Fprintf(os.Stderr, "dreamland: OTEL env warning (copilot): %v\n", err)
	}
}

// ScaffoldTelemetry installs the Copilot agentStop hook binding file.
// Other tools include the telemetry write command in their base binding templates.
func ScaffoldTelemetry(repoRoot, tool string) error {
	if tool == "GitHub Copilot" {
		return installCopilotHookBinding(repoRoot)
	}
	return nil
}

func installCopilotHookBinding(repoRoot string) error {
	target := filepath.Join(repoRoot, ".github", "hooks", "dreamland-telemetry.json")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(target); err == nil {
		return nil // already installed
	}
	data, err := json.MarshalIndent(map[string]any{
		"version": 1,
		"hooks": map[string]any{
			"agentStop": []any{
				map[string]any{
					"type":       "bash",
					"bash":       "dreamland telemetry write --tool github-copilot",
					"timeoutSec": 30,
				},
			},
		},
	}, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(target, append(data, '\n'), 0o644)
}
