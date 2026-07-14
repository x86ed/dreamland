//go:build !windows

package cmd

import (
	"os/exec"
	"testing"
)

func TestDetachProcess_SetsSid(t *testing.T) {
	cmd := exec.Command("true")
	detachProcess(cmd)
	if cmd.SysProcAttr == nil {
		t.Fatal("expected SysProcAttr to be set")
	}
	if !cmd.SysProcAttr.Setsid {
		t.Error("expected Setsid to be true")
	}
}
