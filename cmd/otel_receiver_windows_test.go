//go:build windows

package cmd

import (
	"os"
	"os/exec"
	"testing"
)

func TestDetachProcess_WindowsCreationFlags(t *testing.T) {
	cmd := exec.Command("cmd", "/c", "exit", "0")
	detachProcess(cmd)
	if cmd.SysProcAttr == nil {
		t.Fatal("expected SysProcAttr to be set")
	}
	const want = 0x00000008 | 0x00000200 // DETACHED_PROCESS | CREATE_NEW_PROCESS_GROUP
	if cmd.SysProcAttr.CreationFlags != want {
		t.Errorf("CreationFlags = %#x, want %#x", cmd.SysProcAttr.CreationFlags, want)
	}
}

func TestProcessAlive_WindowsSelfAndExitedChild(t *testing.T) {
	if !processAlive(os.Getpid()) {
		t.Error("processAlive(self) = false, want true")
	}
	c := exec.Command("cmd", "/c", "exit", "0")
	if err := c.Run(); err != nil {
		t.Skipf("cannot run cmd: %v", err)
	}
	if processAlive(c.Process.Pid) {
		t.Errorf("processAlive(%d) = true for an exited child", c.Process.Pid)
	}
}
