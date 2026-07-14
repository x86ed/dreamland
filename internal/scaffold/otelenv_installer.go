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
		// Required (preview) for the agent-scoped `hooks:` frontmatter field on
		// .github/agents/*.agent.md to actually fire — without this, only the
		// workspace-level .github/hooks/*.json bindings run.
		"chat.useCustomAgentHooks": true,
	}
	if err := MergeVscodeSettings(repoRoot, patch); err != nil {
		fmt.Fprintf(os.Stderr, "dreamland: OTEL env warning (copilot): %v\n", err)
	}
}

// ScaffoldTelemetry is a no-op: the telemetry write command is included directly
// in each platform's base hook binding template (see bindHooks in scaffold.go),
// GitHub Copilot included (templates/hooks/bindings/github-copilot/hooks.json).
func ScaffoldTelemetry(_, _ string) error {
	return nil
}
