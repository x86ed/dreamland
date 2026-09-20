//go:build windows

package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

// receiverStopSignals: only os.Interrupt exists on Windows.
var receiverStopSignals = []os.Signal{os.Interrupt}

const toolTimeout = 5 * time.Second

const (
	detachedProcess       = 0x00000008
	createNewProcessGroup = 0x00000200
	wsaeAddrInUse         = syscall.Errno(10048)
	processQueryLimited   = 0x1000
	stillActive           = 259
)

// detachProcess configures cmd to run detached from the parent's console and in its own
// process group, so neither the parent exiting nor a Ctrl+C in the parent console reaches
// the child.
func detachProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: detachedProcess | createNewProcessGroup}
}

func isAddrInUse(err error) bool {
	return errors.Is(err, syscall.EADDRINUSE) || errors.Is(err, wsaeAddrInUse)
}

// processAlive reports whether pid names a running process.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	h, err := syscall.OpenProcess(processQueryLimited, false, uint32(pid))
	if err != nil {
		// Access denied means the process exists but belongs to someone else.
		return errors.Is(err, syscall.ERROR_ACCESS_DENIED)
	}
	defer syscall.CloseHandle(h)
	var code uint32
	if err := syscall.GetExitCodeProcess(h, &code); err != nil {
		return false
	}
	return code == stillActive
}

// terminateProcess kills pid. Callers must have verified the process with
// isDreamlandReceiver first. Windows has no graceful signal for a detached process, so the
// receiver's pid file may be left behind; staleness is decided by liveness.
func terminateProcess(pid int) error {
	if pid <= 0 {
		return errors.New("invalid pid")
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Kill()
}

// lookupListenerPID returns the distinct pids of processes listening on TCP port, via
// `netstat -ano`.
func lookupListenerPID(port int) ([]int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), toolTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "netstat", "-ano").Output()
	if err != nil {
		return nil, err
	}
	return parseNetstatPIDs(string(out), port), nil
}

// processCommandLine returns pid's executable path and arguments via PowerShell's
// Get-CimInstance Win32_Process.
func processCommandLine(pid int) (string, []string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), toolTimeout)
	defer cancel()
	script := fmt.Sprintf(
		"Get-CimInstance Win32_Process -Filter 'ProcessId=%s' | Select-Object ExecutablePath,CommandLine | ConvertTo-Json -Compress",
		strconv.Itoa(pid))
	out, err := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", script).Output()
	if err != nil {
		return "", nil, err
	}
	var doc struct {
		ExecutablePath string
		CommandLine    string
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		return "", nil, err
	}
	if doc.ExecutablePath == "" {
		return "", nil, errors.New("executable path unavailable")
	}
	return doc.ExecutablePath, splitWindowsCommandLine(doc.CommandLine), nil
}
