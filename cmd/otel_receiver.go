package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"dreamland/internal/config"
	"dreamland/internal/telemetry/otelreceiver"
)

var otelReceiverCmd = &cobra.Command{
	Use:   "otel-receiver",
	Short: "Start a local OTLP/HTTP trace receiver for GitHub Copilot token usage (no-op if a current one is running)",
	RunE:  runOtelReceiver,
}

var (
	otelReceiverForeground bool
	otelReceiverAddrFlag   string
	otelReceiverReplace    bool
)

// osExecutable is a seam over os.Executable for tests: spawning the real test binary as
// the detached child would recursively re-run the whole test suite.
var osExecutable = os.Executable

// Seams so tests never signal, inspect or spawn a real process. Each *Fn var is the
// indirection over the platform function of the same name (otel_receiver_unix.go /
// otel_receiver_windows.go).
var (
	// otelReceiverStderr receives the one-line user-facing messages of the start algorithm.
	otelReceiverStderr io.Writer = os.Stderr

	// startDetachedFn starts (and releases) the detached foreground child.
	startDetachedFn = func(c *exec.Cmd) error {
		if err := c.Start(); err != nil {
			return err
		}
		return c.Process.Release()
	}

	processAliveFn       = processAlive
	terminateProcessFn   = terminateProcess
	lookupListenerPIDFn  = lookupListenerPID
	processCommandLineFn = processCommandLine
)

const (
	healthService      = "dreamland-otel-receiver"
	healthProbeTimeout = 500 * time.Millisecond
	portFreeTimeout    = 3 * time.Second
	lockStaleAfter     = 15 * time.Second
)

func init() {
	rootCmd.AddCommand(otelReceiverCmd)
	otelReceiverCmd.Flags().BoolVar(&otelReceiverForeground, "foreground", false,
		"run the receiver loop in this process instead of spawning a detached background process (internal use)")
	otelReceiverCmd.Flags().StringVar(&otelReceiverAddrFlag, "addr", "",
		"listen address host:port (required with --foreground)")
	otelReceiverCmd.Flags().BoolVar(&otelReceiverReplace, "replace", false,
		"replace the dreamland receiver holding the port regardless of its revision")
}

func runOtelReceiver(_ *cobra.Command, _ []string) error {
	if otelReceiverForeground {
		return runReceiverForeground(otelReceiverAddrFlag)
	}
	return runReceiverStart()
}

// runReceiverForeground is the detached child: one shared, repo-agnostic receiver that
// keeps its state in the per-user state directory and never touches a repository.
func runReceiverForeground(addr string) error {
	if addr == "" {
		return errors.New("--foreground requires --addr host:port")
	}
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid --addr %q: %w", addr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("invalid --addr %q: %w", addr, err)
	}

	stateDir := otelreceiver.StateDir()
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}
	otelreceiver.Build = buildCommit

	ctx, stop := signal.NotifyContext(context.Background(), receiverStopSignals...)
	defer stop()

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		if isAddrInUse(err) {
			return nil // lost the start race to another receiver
		}
		return err
	}
	// Do not pin a repository or worktree directory (notably on Windows). Only after a
	// successful bind, so a starter that lost the race leaves its cwd alone.
	if err := os.Chdir(stateDir); err != nil {
		ln.Close()
		return err
	}

	pidFile := otelreceiver.PidFilePath(stateDir, port)
	writePidFile(pidFile, addr)
	defer os.Remove(pidFile)

	otelreceiver.StartGC(ctx, stateDir)

	srv := &http.Server{Handler: otelreceiver.Handler(stateDir)}
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()

	select {
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return nil
	}
}

func writePidFile(path, addr string) {
	data, err := json.Marshal(map[string]any{
		"pid":        os.Getpid(),
		"addr":       addr,
		"revision":   otelreceiver.ReceiverRevision,
		"build":      buildCommit,
		"started_at": time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o644)
}

// runReceiverStart is the SessionStart entry point. Every failure below the working
// directory lookup is best effort: it always returns nil so the hook chain never fails.
func runReceiverStart() error {
	cwd, err := osGetwd()
	if err != nil {
		return err
	}
	// The repo is only needed for cfg.OtelEndpoint; outside a repo (or with an unreadable
	// config) the default endpoint is used.
	var endpoint string
	if cfg, cerr := config.Load(cwd); cerr == nil && cfg != nil {
		endpoint = cfg.OtelEndpoint
	}
	addr := otelReceiverAddr(endpoint)
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return nil
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil
	}

	stateDir := otelreceiver.StateDir()
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return nil
	}
	release, ok := acquireStartLock(otelreceiver.LockPath(stateDir, port))
	if !ok {
		return nil // another start is in progress
	}
	defer release()

	startReceiver(stateDir, addr, port)
	return nil
}

// acquireStartLock takes the O_EXCL start lock. A lock older than 15 s is stale and is
// removed once. ok is false only when a fresh lock is held by another starter.
func acquireStartLock(path string) (release func(), ok bool) {
	for attempt := 0; attempt < 2; attempt++ {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			fmt.Fprintf(f, "%d\n", os.Getpid())
			f.Close()
			return func() { os.Remove(path) }, true
		}
		if !errors.Is(err, os.ErrExist) {
			return func() {}, true // cannot lock (read-only dir, ...): proceed best-effort
		}
		if info, serr := os.Stat(path); serr == nil && time.Since(info.ModTime()) <= lockStaleAfter {
			return nil, false
		}
		os.Remove(path)
	}
	return nil, false
}

type healthDoc struct {
	Service  string `json:"service"`
	Revision int    `json:"revision"`
	PID      int    `json:"pid"`
}

// probeHealth returns (doc, true) when a dreamland receiver answers the handshake.
func probeHealth(addr string) (healthDoc, bool) {
	client := &http.Client{Timeout: healthProbeTimeout}
	resp, err := client.Get("http://" + addr + "/.dreamland/health")
	if err != nil {
		return healthDoc{}, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return healthDoc{}, false
	}
	var doc healthDoc
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&doc); err != nil {
		return healthDoc{}, false
	}
	if doc.Service != healthService {
		return healthDoc{}, false
	}
	return doc, true
}

func portOpen(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, healthProbeTimeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func waitPortFree(addr string) bool {
	deadline := time.Now().Add(portFreeTimeout)
	for {
		if !portOpen(addr) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func startReceiver(stateDir, addr string, port int) {
	pidFile := otelreceiver.PidFilePath(stateDir, port)

	if !portOpen(addr) {
		os.Remove(pidFile) // a pid file with nothing listening is stale by definition
		spawnReceiver(stateDir, addr)
		return
	}

	doc, identified := probeHealth(addr)
	if identified && !otelReceiverReplace && doc.Revision >= otelreceiver.ReceiverRevision {
		return
	}
	replaceReceiver(stateDir, addr, port, doc.PID, !identified)
}

// replaceReceiver terminates the dreamland receiver holding addr and spawns the current
// one. knownPID is the pid reported by the receiver's health response, or 0 when it must
// be resolved through the operating system (preHandshake: the holder gave no valid health
// document). A process is signalled only after isDreamlandReceiver has just accepted its
// command line, whatever the source of the pid.
func replaceReceiver(stateDir, addr string, port, knownPID int, preHandshake bool) {
	pid := knownPID
	if pid == 0 {
		pids, err := lookupListenerPIDFn(port)
		if err != nil {
			refuseToSignal(port)
			return
		}
		pids = distinctPIDs(pids)
		if len(pids) != 1 {
			refuseToSignal(port)
			return
		}
		pid = pids[0]
	}

	exe, args, err := processCommandLineFn(pid)
	if err != nil {
		refuseToSignal(port)
		return
	}
	if !isDreamlandReceiver(exe, args) {
		refuseToSignal(port)
		return
	}

	if err := terminateProcessFn(pid); err != nil {
		fmt.Fprintf(otelReceiverStderr, "dreamland: could not terminate receiver pid %d on port %d: %v\n", pid, port, err)
		return
	}
	if !waitPortFree(addr) {
		fmt.Fprintf(otelReceiverStderr, "dreamland: terminated receiver pid %d but port %d was not released within %s; not starting a new one\n", pid, port, portFreeTimeout)
		return
	}
	spawnReceiver(stateDir, addr)
	if preHandshake {
		fmt.Fprintf(otelReceiverStderr, "dreamland: evicted pre-handshake dreamland receiver (pid %d) on port %d\n", pid, port)
	}
}

// refuseToSignal writes the single "left alone" line for a listener that failed the
// identification or the isDreamlandReceiver check.
func refuseToSignal(port int) {
	if otelReceiverReplace {
		fmt.Fprintf(otelReceiverStderr, "dreamland: port %d is held by a process that is not a dreamland receiver or could not be identified; --replace will not signal it (stop it manually if it should go)\n", port)
		return
	}
	fmt.Fprintf(otelReceiverStderr, "dreamland: port %d is held by a process that is not a dreamland receiver or could not be identified; leaving it alone (use 'dreamland otel-receiver --replace' to override for a dreamland receiver)\n", port)
}

func distinctPIDs(pids []int) []int {
	seen := map[int]bool{}
	var out []int
	for _, p := range pids {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

func spawnReceiver(stateDir, addr string) {
	exe, err := osExecutable()
	if err != nil {
		exe = "dreamland"
	}
	child := exec.Command(exe, "otel-receiver", "--foreground", "--addr", addr)
	child.Dir = stateDir
	detachProcess(child)
	// Best effort: a missing receiver must never fail the hook chain.
	_ = startDetachedFn(child)
}

// isDreamlandReceiver reports whether exe/args describe a `dreamland otel-receiver`
// process: the executable's basename (case-insensitive, .exe stripped) is exactly
// "dreamland" and "otel-receiver" is one of the arguments as a whole argument.
func isDreamlandReceiver(exe string, args []string) bool {
	base := exe
	if i := strings.LastIndexAny(base, `/\`); i >= 0 {
		base = base[i+1:]
	}
	base = strings.ToLower(base)
	base = strings.TrimSuffix(base, ".exe")
	if base != "dreamland" {
		return false
	}
	for _, a := range args {
		if a == "otel-receiver" {
			return true
		}
	}
	return false
}

// otelReceiverAddr derives the listen address (host:port) from the configured OTLP
// endpoint, applying the same gRPC(4317)->HTTP(4318) port translation used when writing
// github.copilot.chat.otel.otlpEndpoint (see copilotOtelEndpoint in internal/scaffold).
func otelReceiverAddr(otelEndpoint string) string {
	endpoint := otelEndpoint
	if endpoint == "" {
		endpoint = "http://localhost:4317"
	}
	endpoint = strings.ReplaceAll(endpoint, ":4317", ":4318")

	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" {
		return "localhost:4318"
	}
	return u.Host
}
