package cmd

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// The command package (including its tests) must type-check for GOOS=windows: the
// platform files define processAlive and terminateProcess there, and no unix-only symbol
// may leak into portable code.
func TestWindowsBuildTypeChecks(t *testing.T) {
	if testing.Short() {
		t.Skip("cross-compiles for windows; skipped in -short")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go toolchain not on PATH")
	}
	c := exec.Command(goBin, "vet", "./cmd/", "./internal/telemetry/...")
	c.Dir = ".."
	c.Env = append(os.Environ(), "GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=0")
	out, err := c.CombinedOutput()
	if err != nil {
		t.Errorf("GOOS=windows go vet failed: %v\n%s", err, strings.TrimSpace(string(out)))
	}
}
