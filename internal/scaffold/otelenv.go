package scaffold

import (
	"fmt"
	"io/fs"
	"strings"
)

// RenderOtelEnvScript reads the OTEL env shell script template for the given platform,
// substitutes {{OTEL_ENDPOINT}} with endpoint, and returns the rendered bytes.
// Returns an error if no template exists for the platform.
func RenderOtelEnvScript(platform, endpoint string) ([]byte, error) {
	templatePaths := map[string]string{
		"Claude Code": "templates/hooks/bindings/claude-code/dreamland-otel-env.sh",
		"Cursor":      "templates/hooks/bindings/cursor/dreamland-otel-env.sh",
		"Kiro":        "templates/hooks/bindings/kiro/dreamland-otel-env.sh",
	}
	path, ok := templatePaths[platform]
	if !ok {
		return nil, fmt.Errorf("no OTEL env script template for platform %q", platform)
	}
	raw, err := fs.ReadFile(TemplateFS, path)
	if err != nil {
		return nil, fmt.Errorf("read OTEL env template: %w", err)
	}
	rendered := strings.ReplaceAll(string(raw), "{{OTEL_ENDPOINT}}", endpoint)
	return []byte(rendered), nil
}

// copilotOtelEndpoint derives the OTLP HTTP endpoint from an OTLP gRPC endpoint.
// Copilot's native OTel uses OTLP HTTP (port 4318); replace 4317 with 4318.
func copilotOtelEndpoint(grpcEndpoint string) string {
	return strings.ReplaceAll(grpcEndpoint, ":4317", ":4318")
}
