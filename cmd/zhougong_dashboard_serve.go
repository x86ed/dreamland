package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"dreamland/internal/config"
	"dreamland/internal/zhougongdash"
)

const (
	zhougongDashStateFile = ".dreamland/zhougong-dashboard.json"
	zhougongDashLogFile   = ".dreamland/zhougong-dashboard.log"
)

var (
	zhougongDashReadyTimeout = 5 * time.Second
	zhougongDashStopTimeout  = 3 * time.Second
)

type zhougongDashState struct {
	PID int    `json:"pid"`
	URL string `json:"url"`
}

var zhougongDashServePort int

// zhougongDashboardServeCmd runs the dashboard in the foreground. mcp-zhougong spawns it
// detached so the dashboard outlives the short-lived zhougong agent process.
var zhougongDashboardServeCmd = &cobra.Command{
	Use:    "zhougong-dashboard-serve",
	Short:  "Serve the zhougong metrics dashboard in the foreground (internal)",
	Hidden: true,
	RunE:   runZhougongDashboardServe,
}

func init() {
	zhougongDashboardServeCmd.Flags().IntVar(&zhougongDashServePort, "port", 0, "port to bind on 127.0.0.1; 0 picks a free port")
	rootCmd.AddCommand(zhougongDashboardServeCmd)
}

func runZhougongDashboardServe(cmd *cobra.Command, _ []string) error {
	cwd, err := osGetwd()
	if err != nil {
		return err
	}
	repoRoot, err := config.FindRepoRoot(cwd)
	if err != nil {
		return fmt.Errorf("not in a git repository: %w", err)
	}
	dash := zhougongdash.New(zhougongdash.NewStore(repoRoot))
	url, err := dash.Start(zhougongDashServePort)
	if err != nil {
		return err
	}
	if err := writeZhougongDashState(repoRoot, zhougongDashState{PID: os.Getpid(), URL: url}); err != nil {
		_ = dash.Stop()
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), receiverStopSignals...)
	defer stop()
	<-ctx.Done()
	err = dash.Stop()
	removeZhougongDashState(repoRoot, os.Getpid())
	// A background collect may still be running; exit now rather than wait on it.
	if err != nil {
		fmt.Fprintln(cmd.ErrOrStderr(), err)
	}
	os.Exit(0)
	return nil
}

func writeZhougongDashState(repoRoot string, st zhougongDashState) error {
	path := filepath.Join(repoRoot, zhougongDashStateFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(st)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readZhougongDashState(repoRoot string) (zhougongDashState, bool) {
	b, err := os.ReadFile(filepath.Join(repoRoot, zhougongDashStateFile))
	if err != nil {
		return zhougongDashState{}, false
	}
	var st zhougongDashState
	if json.Unmarshal(b, &st) != nil || st.PID <= 0 || st.URL == "" {
		return zhougongDashState{}, false
	}
	return st, true
}

// removeZhougongDashState deletes the state file only if it still belongs to pid.
func removeZhougongDashState(repoRoot string, pid int) {
	if st, ok := readZhougongDashState(repoRoot); ok && st.PID != pid {
		return
	}
	_ = os.Remove(filepath.Join(repoRoot, zhougongDashStateFile))
}

// startZhougongDashboard returns the URL of the running detached dashboard, spawning one if needed.
func startZhougongDashboard(repoRoot string, port int) (string, error) {
	if st, ok := readZhougongDashState(repoRoot); ok {
		if processAlive(st.PID) {
			return st.URL, nil
		}
		removeZhougongDashState(repoRoot, st.PID)
	}
	exe, err := osExecutable()
	if err != nil {
		return "", err
	}
	logPath := filepath.Join(repoRoot, zhougongDashLogFile)
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return "", err
	}
	logf, err := os.Create(logPath)
	if err != nil {
		return "", err
	}
	defer logf.Close()
	child := exec.Command(exe, "zhougong-dashboard-serve", "--port", strconv.Itoa(port))
	child.Dir = repoRoot
	child.Stdout, child.Stderr = logf, logf
	detachProcess(child)
	if err := child.Start(); err != nil {
		return "", err
	}
	exited := make(chan struct{})
	go func() { _ = child.Wait(); close(exited) }()

	deadline := time.After(zhougongDashReadyTimeout)
	for {
		if st, ok := readZhougongDashState(repoRoot); ok && st.PID == child.Process.Pid {
			return st.URL, nil
		}
		select {
		case <-exited:
			msg, _ := os.ReadFile(logPath)
			return "", fmt.Errorf("dashboard failed to start: %s", strings.TrimSpace(string(msg)))
		case <-deadline:
			_ = terminateProcess(child.Process.Pid)
			return "", fmt.Errorf("dashboard did not become ready within %s", zhougongDashReadyTimeout)
		case <-time.After(20 * time.Millisecond):
		}
	}
}

// stopZhougongDashboard terminates the detached dashboard (SIGTERM, then SIGKILL if it lingers)
// and removes its state file only once the process is gone; a no-op when none runs.
func stopZhougongDashboard(repoRoot string) error {
	st, ok := readZhougongDashState(repoRoot)
	if !ok {
		return nil
	}
	if processAlive(st.PID) {
		if err := terminateProcess(st.PID); err != nil && processAlive(st.PID) {
			return err
		}
		if !waitProcessGone(st.PID, zhougongDashStopTimeout) {
			if err := killProcess(st.PID); err != nil && processAlive(st.PID) {
				return err
			}
			if !waitProcessGone(st.PID, zhougongDashStopTimeout) {
				return fmt.Errorf("dashboard (pid %d) did not exit after SIGKILL", st.PID)
			}
		}
	}
	removeZhougongDashState(repoRoot, st.PID)
	return nil
}

func waitProcessGone(pid int, d time.Duration) bool {
	deadline := time.Now().Add(d)
	for processAlive(pid) {
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(20 * time.Millisecond)
	}
	return true
}
