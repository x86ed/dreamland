package cmd

import (
	"bytes"
	"testing"
)

func TestRunVersionPrintsBuildCommit(t *testing.T) {
	orig := buildCommit
	buildCommit = "test-sha"
	t.Cleanup(func() { buildCommit = orig })

	var out bytes.Buffer
	versionCmd.SetOut(&out)
	t.Cleanup(func() { versionCmd.SetOut(nil) })

	if err := runVersion(versionCmd, nil); err != nil {
		t.Fatalf("runVersion: %v", err)
	}
	if got, want := out.String(), "dreamland build test-sha\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}
