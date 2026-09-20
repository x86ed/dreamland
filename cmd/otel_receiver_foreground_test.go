package cmd

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"dreamland/internal/config"
	"dreamland/internal/telemetry/otelreceiver"
)

type foregroundRun struct {
	addr     string
	done     chan error
	stopped  bool
	health   map[string]any
	stateDir string
}

// stop delivers an interrupt to this process and waits for the foreground receiver to
// return. A no-op signal handler is installed first so a mis-ordered signal can never kill
// the test binary.
func (f *foregroundRun) stop(t *testing.T) error {
	t.Helper()
	f.stopped = true
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Signal(os.Interrupt); err != nil {
		t.Fatalf("signal self: %v", err)
	}
	select {
	case err := <-f.done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("foreground receiver did not shut down within 5 s of SIGINT")
		return nil
	}
}

// startForeground runs `dreamland otel-receiver --foreground --addr addr` in-process from a
// cwd that is not inside any git repository, and waits until its health endpoint answers
// with a document. It returns without Fatal-ing before the signal shield is in place.
func startForeground(t *testing.T, h *recvHarness, addr string) *foregroundRun {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("SIGINT to self is not supported on Windows")
	}
	sink := make(chan os.Signal, 4)
	signal.Notify(sink, os.Interrupt)
	t.Cleanup(func() { signal.Stop(sink) })

	origBuild := otelreceiver.Build
	t.Cleanup(func() { otelreceiver.Build = origBuild })

	// Not a repository, and t.Chdir restores the original cwd after the receiver has
	// os.Chdir'd into the state dir.
	t.Chdir(t.TempDir())
	osGetwd = func() (string, error) { return os.Getwd() }

	otelReceiverForeground = true
	otelReceiverAddrFlag = addr

	run := &foregroundRun{addr: addr, done: make(chan error, 1), stateDir: h.stateDir}
	go func() { run.done <- runOtelReceiver(nil, nil) }()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-run.done:
			t.Fatalf("foreground receiver returned early: %v", err)
		default:
		}
		resp, err := http.Get("http://" + addr + "/.dreamland/health")
		if err == nil {
			var doc map[string]any
			decodeErr := json.NewDecoder(resp.Body).Decode(&doc)
			resp.Body.Close()
			if decodeErr == nil && doc["service"] == "dreamland-otel-receiver" {
				run.health = doc
				return run
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("foreground receiver never served a health document")
	return nil
}

func TestForeground_RequiresAddr(t *testing.T) {
	h := newRecvHarness(t)
	// Hold the endpoint the config points at so that nothing can bind or hang if the
	// implementation (wrongly) falls back to the configured endpoint.
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("loopback unavailable: %v", err)
	}
	t.Cleanup(func() { occupied.Close() })
	root := makeCoauthorRepo(t, config.Config{CodingTool: "GitHub Copilot", OtelEndpoint: "http://" + occupied.Addr().String()})
	osGetwd = func() (string, error) { return root, nil }
	otelReceiverForeground = true
	otelReceiverAddrFlag = ""

	err = runOtelReceiver(nil, nil)
	if err == nil {
		t.Fatal("--foreground without --addr must be an error")
	}
	_ = h
}

func TestForeground_AddressInUseExitsZeroSilently(t *testing.T) {
	h := newRecvHarness(t)
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("loopback unavailable: %v", err)
	}
	t.Cleanup(func() { occupied.Close() })
	addr := occupied.Addr().String()
	root := makeCoauthorRepo(t, config.Config{CodingTool: "GitHub Copilot", OtelEndpoint: "http://" + addr})
	osGetwd = func() (string, error) { return root, nil }
	otelReceiverForeground = true
	otelReceiverAddrFlag = addr

	done := make(chan error, 1)
	go func() { done <- runOtelReceiver(nil, nil) }()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("EADDRINUSE must exit 0 (lost the start race), got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("foreground receiver did not return on EADDRINUSE")
	}
	if _, err := os.Stat(h.pidFile(portOf(addr))); !os.IsNotExist(err) {
		t.Errorf("pid file must only be written after a successful bind (err=%v)", err)
	}
	if lines := h.stderr.lines(); len(lines) != 0 {
		t.Errorf("EADDRINUSE must be silent, got %v", lines)
	}
}

func TestForeground_WorksOutsideARepoPinsStateDirAndCleansUp(t *testing.T) {
	h := newRecvHarness(t)
	addr := freeAddr(t)

	// A stale mailbox that the startup GC must remove.
	sessions := filepath.Join(h.stateDir, "sessions")
	if err := os.MkdirAll(sessions, 0o755); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(sessions, "old.json")
	if err := os.WriteFile(old, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	ts := time.Now().Add(-8 * 24 * time.Hour)
	if err := os.Chtimes(old, ts, ts); err != nil {
		t.Fatal(err)
	}

	origCommit := buildCommit
	buildCommit = "deadbeef"
	t.Cleanup(func() { buildCommit = origCommit })

	run := startForeground(t, h, addr)

	// Health reports this process, the state dir and the injected build.
	if run.health["build"] != "deadbeef" {
		t.Errorf("health build = %v, want buildCommit (deadbeef) injected via otelreceiver.Build", run.health["build"])
	}
	if run.health["revision"] != float64(2) {
		t.Errorf("health revision = %v, want 2", run.health["revision"])
	}
	if run.health["state_dir"] != h.stateDir {
		t.Errorf("health state_dir = %v, want %q", run.health["state_dir"], h.stateDir)
	}

	// The process working directory is the state dir, not a repo/worktree.
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	gotCwd, _ := filepath.EvalSymlinks(cwd)
	wantCwd, _ := filepath.EvalSymlinks(h.stateDir)
	if gotCwd != wantCwd {
		t.Errorf("cwd = %q, want the state dir %q", gotCwd, wantCwd)
	}

	// Pid file written after bind, with the documented fields.
	data, err := os.ReadFile(h.pidFile(portOf(addr)))
	if err != nil {
		t.Fatalf("pid file receiver-<port>.json not written after bind: %v", err)
	}
	var pf map[string]any
	if err := json.Unmarshal(data, &pf); err != nil {
		t.Fatalf("pid file is not JSON: %v", err)
	}
	if pf["pid"] != float64(os.Getpid()) {
		t.Errorf("pid file pid = %v, want %d", pf["pid"], os.Getpid())
	}
	if pf["addr"] != addr {
		t.Errorf("pid file addr = %v, want %s", pf["addr"], addr)
	}
	for _, k := range []string{"revision", "build", "started_at"} {
		if _, ok := pf[k]; !ok {
			t.Errorf("pid file missing %q: %s", k, data)
		}
	}

	// StartGC ran immediately.
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(old); os.IsNotExist(err) {
			break
		}
		if time.Now().After(deadline) {
			t.Error("startup GC did not delete the 8-day-old mailbox")
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if err := run.stop(t); err != nil {
		t.Errorf("graceful shutdown must return nil, got %v", err)
	}
	if _, err := os.Stat(h.pidFile(portOf(addr))); !os.IsNotExist(err) {
		t.Errorf("pid file must be removed on graceful shutdown (err=%v)", err)
	}
	if _, err := http.Get("http://" + addr + "/.dreamland/health"); err == nil {
		t.Error("receiver still answering after shutdown")
	}
}
