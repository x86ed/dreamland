//go:build windows

package cmd

import (
	"errors"
	"os/exec"
	"syscall"
)

// detachProcess configures cmd to run detached from the parent's console, so the parent
// can exit without the child being torn down alongside it.
func detachProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000008} // DETACHED_PROCESS
}

// STUBS (nyx, TDD red phase): morpheus implements these per task 4.2.

func processAlive(pid int) bool { return false }

func terminateProcess(pid int) error { return errors.New("terminateProcess: not implemented") }

func lookupListenerPID(port int) ([]int, error) {
	return nil, errors.New("lookupListenerPID: not implemented")
}

func processCommandLine(pid int) (string, []string, error) {
	return "", nil, errors.New("processCommandLine: not implemented")
}
