package cmd

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// The handoff mechanism must type-check for GOOS=windows: no flock, signals, or
// process inspection, only O_EXCL lock files and os.Rename.
func TestHandoffWindowsBuildTypeChecks(t *testing.T) {
	if testing.Short() {
		t.Skip("cross-compiles for windows; skipped in -short")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go toolchain not on PATH")
	}
	c := exec.Command(goBin, "vet", "./internal/handoff/...", "./cmd/")
	c.Dir = ".."
	c.Env = append(os.Environ(), "GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=0")
	out, err := c.CombinedOutput()
	if err != nil {
		t.Errorf("GOOS=windows go vet failed: %v\n%s", err, strings.TrimSpace(string(out)))
	}
}
