//go:build !windows

package cmd

import (
	"errors"
	"os/exec"
	"syscall"
)

// detachProcess configures cmd to run in its own session, detached from the parent's
// controlling terminal/process group, so the parent can exit without the child receiving
// a SIGHUP or being reaped as part of the parent's process group.
func detachProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

// STUBS (nyx, TDD red phase): morpheus implements these per task 4.1.

func processAlive(pid int) bool { return false }

func terminateProcess(pid int) error { return errors.New("terminateProcess: not implemented") }

func lookupListenerPID(port int) ([]int, error) {
	return nil, errors.New("lookupListenerPID: not implemented")
}

func processCommandLine(pid int) (string, []string, error) {
	return "", nil, errors.New("processCommandLine: not implemented")
}
