//go:build windows

package cmd

import (
	"os/exec"
	"syscall"
)

// detachProcess configures cmd to run detached from the parent's console, so the parent
// can exit without the child being torn down alongside it.
func detachProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000008} // DETACHED_PROCESS
}
