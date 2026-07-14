//go:build !windows

package cmd

import (
	"os/exec"
	"syscall"
)

// detachProcess configures cmd to run in its own session, detached from the parent's
// controlling terminal/process group, so the parent can exit without the child receiving
// a SIGHUP or being reaped as part of the parent's process group.
func detachProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
