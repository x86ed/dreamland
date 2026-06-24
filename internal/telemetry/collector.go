package telemetry

import (
	"io"

	"dreamland/internal/config"
)

// Collector reads a tool-specific hook payload from stdin and returns a normalized snapshot.
type Collector interface {
	Collect(stdin io.Reader, cfg *config.Config) (*SnapshotResult, error)
}

// Registry maps tool name strings to their Collector implementations.
// Populated by cmd/telemetry.go at startup.
var Registry = map[string]Collector{}

// Register adds a collector to the Registry under the given tool name.
func Register(name string, c Collector) {
	Registry[name] = c
}
