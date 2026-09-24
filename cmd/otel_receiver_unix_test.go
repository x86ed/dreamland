//go:build !windows

package cmd

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
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

func TestProcessAlive_SelfIsAlive(t *testing.T) {
	if !processAlive(os.Getpid()) {
		t.Error("processAlive(self) = false, want true")
	}
}

func TestProcessAlive_ExitedChildIsDead(t *testing.T) {
	c := exec.Command("true")
	if err := c.Run(); err != nil {
		t.Skipf("cannot run true: %v", err)
	}
	if processAlive(c.Process.Pid) {
		t.Errorf("processAlive(%d) = true for a reaped, exited child", c.Process.Pid)
	}
}

func TestProcessAlive_InvalidPids(t *testing.T) {
	for _, pid := range []int{0, -1} {
		if processAlive(pid) {
			t.Errorf("processAlive(%d) = true, want false (never signal a process group)", pid)
		}
	}
}

func TestTerminateProcess_StopsASpawnedSleep(t *testing.T) {
	c := exec.Command("sleep", "30")
	if err := c.Start(); err != nil {
		t.Skipf("cannot start sleep: %v", err)
	}
	waited := make(chan error, 1)
	go func() { waited <- c.Wait() }()
	t.Cleanup(func() { c.Process.Kill() }) // best effort; the goroutine above reaps it

	if err := terminateProcess(c.Process.Pid); err != nil {
		t.Fatalf("terminateProcess: %v", err)
	}
	select {
	case <-waited:
	case <-time.After(3 * time.Second):
		t.Fatal("sleep still running 3 s after terminateProcess (SIGTERM)")
	}
}

func TestTerminateProcess_DeadPidReturnsErrorWithoutPanic(t *testing.T) {
	c := exec.Command("true")
	if err := c.Run(); err != nil {
		t.Skipf("cannot run true: %v", err)
	}
	_ = terminateProcess(c.Process.Pid) // must not panic; an error is fine
}

func TestLookupListenerPID_FindsOwnListener(t *testing.T) {
	if _, err := exec.LookPath("lsof"); err != nil {
		t.Skip("lsof not installed")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("loopback unavailable: %v", err)
	}
	defer ln.Close()
	port, _ := strconv.Atoi(portOf(ln.Addr().String()))

	pids, err := lookupListenerPID(port)
	if err != nil {
		t.Fatalf("lookupListenerPID: %v", err)
	}
	if len(pids) != 1 || pids[0] != os.Getpid() {
		t.Errorf("pids = %v, want [%d]", pids, os.Getpid())
	}
}

func TestLookupListenerPID_MissingToolIsAnErrorNotAPanic(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // no lsof reachable
	if _, err := lookupListenerPID(4318); err == nil {
		t.Error("expected an error when lsof is unavailable")
	}
}

func TestProcessCommandLine_ReadsOwnCommandLine(t *testing.T) {
	if _, err := exec.LookPath("ps"); err != nil {
		t.Skip("ps not installed")
	}
	exe, _, err := processCommandLine(os.Getpid())
	if err != nil {
		t.Fatalf("processCommandLine(self): %v", err)
	}
	if exe == "" {
		t.Fatal("empty executable")
	}
	self, _ := os.Executable()
	if filepath.Base(exe) != filepath.Base(self) && len(filepath.Base(exe)) < 10 {
		t.Errorf("executable %q does not look like the test binary %q", exe, self)
	}
}

func TestProcessCommandLine_MissingToolIsAnErrorNotAPanic(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, _, err := processCommandLine(os.Getpid()); err == nil {
		t.Error("expected an error when ps is unavailable")
	}
}

func TestProcessCommandLine_UnknownPidIsAnError(t *testing.T) {
	if _, err := exec.LookPath("ps"); err != nil {
		t.Skip("ps not installed")
	}
	c := exec.Command("true")
	if err := c.Run(); err != nil {
		t.Skipf("cannot run true: %v", err)
	}
	if _, _, err := processCommandLine(c.Process.Pid); err == nil {
		t.Error("expected an error for a pid that no longer exists")
	}
}
