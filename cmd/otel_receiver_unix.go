//go:build !windows

package cmd

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

// receiverStopSignals are the signals that trigger a graceful foreground shutdown.
var receiverStopSignals = []os.Signal{syscall.SIGTERM, os.Interrupt}

const toolTimeout = 2 * time.Second

// detachProcess configures cmd to run in its own session, detached from the parent's
// controlling terminal/process group, so the parent can exit without the child receiving
// a SIGHUP or being reaped as part of the parent's process group.
func detachProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

func isAddrInUse(err error) bool { return errors.Is(err, syscall.EADDRINUSE) }

// processAlive reports whether pid names a live process. EPERM means it exists but belongs
// to someone else, which is still alive.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false // kill(0) / kill(-1) would address a process group or everything
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// terminateProcess asks pid to exit (SIGTERM). Callers must have verified the process
// with isDreamlandReceiver first.
func terminateProcess(pid int) error {
	if pid <= 0 {
		return errors.New("invalid pid")
	}
	return syscall.Kill(pid, syscall.SIGTERM)
}

// lookupListenerPID returns the distinct pids of processes listening on TCP port, via lsof.
func lookupListenerPID(port int) ([]int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), toolTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "lsof", "-nP", "-iTCP:"+strconv.Itoa(port), "-sTCP:LISTEN", "-t").Output()
	if err != nil {
		// lsof exits 1 with no output when nothing matches.
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() == 1 && len(out) == 0 {
			return nil, nil
		}
		return nil, err
	}
	return parseLsofPIDs(string(out))
}

// processCommandLine returns pid's executable and arguments. `ps -o comm=` supplies the
// executable and `ps -o args=` the arguments, so paths with spaces are handled.
func processCommandLine(pid int) (string, []string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), toolTimeout)
	defer cancel()
	pidArg := strconv.Itoa(pid)
	comm, err := exec.CommandContext(ctx, "ps", "-o", "comm=", "-p", pidArg).Output()
	if err != nil {
		return "", nil, err
	}
	argsLine, err := exec.CommandContext(ctx, "ps", "-o", "args=", "-p", pidArg).Output()
	if err != nil {
		return "", nil, err
	}
	return parsePSCommandLine(string(comm), string(argsLine))
}

// killProcess force-kills pid (SIGKILL).
func killProcess(pid int) error {
	if pid <= 0 {
		return errors.New("invalid pid")
	}
	return syscall.Kill(pid, syscall.SIGKILL)
}
