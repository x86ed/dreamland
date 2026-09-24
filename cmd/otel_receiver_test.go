package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"dreamland/internal/config"
	"dreamland/internal/telemetry/otelreceiver"
)

func TestOtelReceiverAddr_Default(t *testing.T) {
	if got := otelReceiverAddr(""); got != "localhost:4318" {
		t.Errorf("got %q, want localhost:4318", got)
	}
}

func TestOtelReceiverAddr_TranslatesGRPCPort(t *testing.T) {
	if got := otelReceiverAddr("http://localhost:4317"); got != "localhost:4318" {
		t.Errorf("got %q, want localhost:4318", got)
	}
}

func TestOtelReceiverAddr_CustomPortPreserved(t *testing.T) {
	if got := otelReceiverAddr("http://localhost:9999"); got != "localhost:9999" {
		t.Errorf("got %q, want localhost:9999", got)
	}
}

func TestOtelReceiverAddr_InvalidURLFallsBack(t *testing.T) {
	if got := otelReceiverAddr("://not a url"); got != "localhost:4318" {
		t.Errorf("got %q, want localhost:4318 fallback", got)
	}
}

// --- test harness --------------------------------------------------------------------

// lockedBuffer is a bytes.Buffer safe for the concurrent-start test.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// lines returns the non-empty lines written.
func (b *lockedBuffer) lines() []string {
	var out []string
	for _, l := range strings.Split(b.String(), "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}

const testDreamlandExe = "/opt/dreamland/bin/dreamland"

// recvHarness replaces every process-touching seam of the receiver start algorithm so a
// test can never signal, inspect or spawn a real process, and never binds a fixed port.
// By default: no process is alive, listener lookup and command-line reads fail, spawn only
// records the *exec.Cmd, and terminate only records the pid.
type recvHarness struct {
	t        *testing.T
	stateDir string
	stderr   *lockedBuffer

	mu         sync.Mutex
	spawned    []*exec.Cmd
	terminated []int
	events     []string

	// onTerminate lets a test simulate the terminated process releasing its port.
	onTerminate func(pid int)
	// onSpawn lets a test simulate the spawned child binding its port.
	onSpawn func(c *exec.Cmd)
}

func newRecvHarness(t *testing.T) *recvHarness {
	t.Helper()
	h := &recvHarness{t: t, stateDir: t.TempDir(), stderr: &lockedBuffer{}}
	t.Setenv("DREAMLAND_STATE_DIR", h.stateDir)

	saved := struct {
		fg      bool
		replace bool
		addr    string
		exe     func() (string, error)
		getwd   func() (string, error)
	}{otelReceiverForeground, otelReceiverReplace, otelReceiverAddrFlag, osExecutable, osGetwd}
	origStderr, origStart := otelReceiverStderr, startDetachedFn
	origAlive, origTerm, origLookup, origCmdline := processAliveFn, terminateProcessFn, lookupListenerPIDFn, processCommandLineFn
	t.Cleanup(func() {
		otelReceiverForeground, otelReceiverReplace, otelReceiverAddrFlag = saved.fg, saved.replace, saved.addr
		osExecutable, osGetwd = saved.exe, saved.getwd
		otelReceiverStderr, startDetachedFn = origStderr, origStart
		processAliveFn, terminateProcessFn, lookupListenerPIDFn, processCommandLineFn = origAlive, origTerm, origLookup, origCmdline
	})

	otelReceiverForeground, otelReceiverReplace, otelReceiverAddrFlag = false, false, ""
	osExecutable = func() (string, error) { return testDreamlandExe, nil }
	otelReceiverStderr = h.stderr
	startDetachedFn = func(c *exec.Cmd) error {
		h.mu.Lock()
		h.spawned = append(h.spawned, c)
		h.events = append(h.events, "spawn")
		hook := h.onSpawn
		h.mu.Unlock()
		if hook != nil {
			hook(c)
		}
		return nil
	}
	processAliveFn = func(int) bool { return false }
	terminateProcessFn = func(pid int) error {
		h.mu.Lock()
		h.terminated = append(h.terminated, pid)
		h.events = append(h.events, fmt.Sprintf("terminate:%d", pid))
		hook := h.onTerminate
		h.mu.Unlock()
		if hook != nil {
			hook(pid)
		}
		return nil
	}
	lookupListenerPIDFn = func(int) ([]int, error) { return nil, errors.New("listener lookup unavailable") }
	processCommandLineFn = func(pid int) (string, []string, error) {
		h.mu.Lock()
		h.events = append(h.events, fmt.Sprintf("cmdline:%d", pid))
		h.mu.Unlock()
		return "", nil, errors.New("command line unavailable")
	}
	return h
}

// cmdlineIs makes processCommandLine report exe/args for pid (and error for other pids).
func (h *recvHarness) cmdlineIs(pid int, exe string, args ...string) {
	processCommandLineFn = func(p int) (string, []string, error) {
		h.mu.Lock()
		h.events = append(h.events, fmt.Sprintf("cmdline:%d", p))
		h.mu.Unlock()
		if p != pid {
			return "", nil, fmt.Errorf("no such pid %d", p)
		}
		return exe, args, nil
	}
}

func (h *recvHarness) listenerIs(pids ...int) {
	lookupListenerPIDFn = func(int) ([]int, error) {
		h.mu.Lock()
		h.events = append(h.events, "lookup")
		h.mu.Unlock()
		return pids, nil
	}
}

func (h *recvHarness) spawnCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.spawned)
}

func (h *recvHarness) terminatedPIDs() []int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]int(nil), h.terminated...)
}

func (h *recvHarness) eventIndex(prefix string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i, e := range h.events {
		if strings.HasPrefix(e, prefix) {
			return i
		}
	}
	return -1
}

func (h *recvHarness) pidFile(port string) string {
	return filepath.Join(h.stateDir, "receiver-"+port+".json")
}

func (h *recvHarness) lockFile(port string) string {
	return filepath.Join(h.stateDir, "receiver-"+port+".lock")
}

// run invokes the non-foreground command against endpoint addr from a repo configured
// with that endpoint and asserts the "always exits 0" contract.
func (h *recvHarness) run(addr string) {
	h.t.Helper()
	root := makeCoauthorRepo(h.t, config.Config{CodingTool: "GitHub Copilot", OtelEndpoint: "http://" + addr})
	osGetwd = func() (string, error) { return root, nil }
	h.runHere()
}

func (h *recvHarness) runHere() {
	h.t.Helper()
	if err := runOtelReceiver(nil, nil); err != nil {
		h.t.Fatalf("dreamland otel-receiver must exit 0 in every case, got error: %v", err)
	}
}

func (h *recvHarness) assertNothingDone(what string) {
	h.t.Helper()
	if got := h.terminatedPIDs(); len(got) != 0 {
		h.t.Errorf("%s: a process was signalled: %v", what, got)
	}
	if n := h.spawnCount(); n != 0 {
		h.t.Errorf("%s: %d receiver(s) spawned, want 0", what, n)
	}
}

func (h *recvHarness) assertOneStderrLine(what string, mustContain ...string) {
	h.t.Helper()
	lines := h.stderr.lines()
	if len(lines) != 1 {
		h.t.Fatalf("%s: stderr has %d lines, want exactly 1:\n%s", what, len(lines), h.stderr.String())
	}
	for _, m := range mustContain {
		if !strings.Contains(lines[0], m) {
			h.t.Errorf("%s: stderr line %q does not contain %q", what, lines[0], m)
		}
	}
}

// freeAddr returns a loopback address that is currently free.
func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("loopback unavailable: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()
	return addr
}

func portOf(addr string) string {
	_, p, _ := net.SplitHostPort(addr)
	return p
}

// fakeReceiver is a loopback HTTP server standing in for whatever holds the port.
type fakeReceiver struct {
	addr string
	srv  *http.Server
	once sync.Once
}

func (f *fakeReceiver) Close() { f.once.Do(func() { f.srv.Close() }) }

func startFake(t *testing.T, handler http.Handler) *fakeReceiver {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("loopback unavailable: %v", err)
	}
	f := &fakeReceiver{addr: ln.Addr().String(), srv: &http.Server{Handler: handler}}
	go f.srv.Serve(ln)
	t.Cleanup(f.Close)
	return f
}

// startFakeOn serves handler on a specific (free) address.
func startFakeOn(t *testing.T, addr string, handler http.Handler) *fakeReceiver {
	t.Helper()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Errorf("fake receiver could not bind %s: %v", addr, err)
		return nil
	}
	f := &fakeReceiver{addr: addr, srv: &http.Server{Handler: handler}}
	go f.srv.Serve(ln)
	t.Cleanup(f.Close)
	return f
}

// healthHandler answers GET /.dreamland/health like a handshake-capable dreamland receiver.
func healthHandler(revision, pid int, build string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/.dreamland/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"service": "dreamland-otel-receiver", "revision": revision, "build": build,
			"pid": pid, "state_dir": "/somewhere",
		})
	})
	return mux
}

// --- existing behavior kept (updated for the state-dir world) ---------------------------

func TestRunOtelReceiver_GetwdError(t *testing.T) {
	h := newRecvHarness(t)
	osGetwd = func() (string, error) { return "", errors.New("getwd failed") }
	if err := runOtelReceiver(nil, nil); err == nil {
		t.Fatal("expected error when osGetwd fails")
	}
	h.assertNothingDone("getwd failure")
}

// Outside any git repository the non-foreground command must not error: the repo is only
// needed for cfg.OtelEndpoint and falls back to the default endpoint.
func TestRunOtelReceiver_NotInGitRepoDoesNotError(t *testing.T) {
	h := newRecvHarness(t)
	osGetwd = func() (string, error) { return t.TempDir(), nil } // no .git, no .dreamland.json
	h.runHere()
	// Whatever answers on the default port (usually nothing), no real process may be touched:
	// every process seam is stubbed, so reaching here without a panic is the assertion.
}

func TestRunOtelReceiver_NilConfig(t *testing.T) {
	h := newRecvHarness(t)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	osGetwd = func() (string, error) { return root, nil } // .git but no .dreamland.json
	h.runHere()
}

func TestRunOtelReceiver_ExecutableLookupFailsFallsBackToDreamland(t *testing.T) {
	h := newRecvHarness(t)
	osExecutable = func() (string, error) { return "", errors.New("no executable") }
	h.run(freeAddr(t))
	if h.spawnCount() != 1 {
		t.Fatalf("spawn count = %d, want 1", h.spawnCount())
	}
	if got := h.spawned[0].Path; !strings.HasSuffix(got, "dreamland") && h.spawned[0].Args[0] != "dreamland" {
		t.Errorf("fallback executable = %q / %q, want dreamland", got, h.spawned[0].Args[0])
	}
}

func TestRunOtelReceiver_SpawnFailureIsSwallowed(t *testing.T) {
	h := newRecvHarness(t)
	startDetachedFn = func(*exec.Cmd) error { return errors.New("exec failed") }
	h.run(freeAddr(t)) // must not fail the hook chain
}

// --- start algorithm: refused port -----------------------------------------------------

func TestStart_RefusedPortSpawnsDetachedForegroundChild(t *testing.T) {
	h := newRecvHarness(t)
	addr := freeAddr(t)
	h.run(addr)

	if h.spawnCount() != 1 {
		t.Fatalf("spawn count = %d, want 1", h.spawnCount())
	}
	c := h.spawned[0]
	if c.Path != testDreamlandExe {
		t.Errorf("child executable = %q, want the osExecutable result %q", c.Path, testDreamlandExe)
	}
	want := []string{testDreamlandExe, "otel-receiver", "--foreground", "--addr", addr}
	if strings.Join(c.Args, "\x00") != strings.Join(want, "\x00") {
		t.Errorf("child args = %q, want %q", c.Args, want)
	}
	if len(h.terminatedPIDs()) != 0 {
		t.Errorf("nothing should be terminated when the port is free: %v", h.terminatedPIDs())
	}
	if lines := h.stderr.lines(); len(lines) != 0 {
		t.Errorf("a clean start must be silent on stderr, got %v", lines)
	}
}

func TestStart_ChildRunsInStateDirNeverTheRepo(t *testing.T) {
	h := newRecvHarness(t)
	addr := freeAddr(t)
	root := makeCoauthorRepo(t, config.Config{CodingTool: "GitHub Copilot", OtelEndpoint: "http://" + addr})
	osGetwd = func() (string, error) { return root, nil }
	h.runHere()

	if h.spawnCount() != 1 {
		t.Fatalf("spawn count = %d, want 1", h.spawnCount())
	}
	dir := h.spawned[0].Dir
	if dir != h.stateDir {
		t.Errorf("child Dir = %q, want the state dir %q", dir, h.stateDir)
	}
	if dir == root || strings.HasPrefix(dir, root+string(filepath.Separator)) {
		t.Errorf("child Dir %q is inside the repo %q; it would pin the worktree", dir, root)
	}
}

func TestStart_RemovesStalePidFileAndStarts(t *testing.T) {
	h := newRecvHarness(t)
	addr := freeAddr(t)
	stale := h.pidFile(portOf(addr))
	if err := os.WriteFile(stale, []byte(fmt.Sprintf(`{"pid":424242,"addr":%q,"revision":1}`, addr)), 0o644); err != nil {
		t.Fatal(err)
	}
	processAliveFn = func(int) bool { return false }

	h.run(addr)

	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("stale pid file was not removed (err=%v)", err)
	}
	if h.spawnCount() != 1 {
		t.Errorf("spawn count = %d, want 1", h.spawnCount())
	}
}

func TestStart_ReleasesLockAfterwards(t *testing.T) {
	h := newRecvHarness(t)
	addr := freeAddr(t)
	h.run(addr)
	if _, err := os.Stat(h.lockFile(portOf(addr))); !os.IsNotExist(err) {
		t.Errorf("start lock must be released when the command finishes (err=%v)", err)
	}
}

// --- start lock ------------------------------------------------------------------------

func TestStart_FreshLockMeansAnotherStartIsInProgress(t *testing.T) {
	h := newRecvHarness(t)
	addr := freeAddr(t)
	lock := h.lockFile(portOf(addr))
	if err := os.WriteFile(lock, []byte("other starter"), 0o644); err != nil {
		t.Fatal(err)
	}

	h.run(addr)

	h.assertNothingDone("fresh lock")
	if _, err := os.Stat(lock); err != nil {
		t.Errorf("a lock held by another starter must be left in place: %v", err)
	}
}

func TestStart_StaleLockIsRemovedAndStartProceeds(t *testing.T) {
	h := newRecvHarness(t)
	addr := freeAddr(t)
	lock := h.lockFile(portOf(addr))
	if err := os.WriteFile(lock, []byte("dead starter"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-20 * time.Second) // older than the 15 s staleness bound
	if err := os.Chtimes(lock, old, old); err != nil {
		t.Fatal(err)
	}

	h.run(addr)

	if h.spawnCount() != 1 {
		t.Errorf("spawn count = %d, want 1 after removing the stale lock", h.spawnCount())
	}
}

func TestStart_ConcurrentStartsSpawnExactlyOnce(t *testing.T) {
	h := newRecvHarness(t)
	addr := freeAddr(t)
	root := makeCoauthorRepo(t, config.Config{CodingTool: "GitHub Copilot", OtelEndpoint: "http://" + addr})
	osGetwd = func() (string, error) { return root, nil }

	// The spawned child takes a moment and then really listens (and answers health), as a
	// real detached receiver would.
	h.onSpawn = func(*exec.Cmd) {
		time.Sleep(100 * time.Millisecond)
		startFakeOn(t, addr, healthHandler(otelreceiver.ReceiverRevision, os.Getpid(), "test"))
	}

	const starters = 8
	var wg sync.WaitGroup
	for i := 0; i < starters; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := runOtelReceiver(nil, nil); err != nil {
				t.Errorf("start returned %v, want nil", err)
			}
		}()
	}
	wg.Wait()

	if n := h.spawnCount(); n != 1 {
		t.Errorf("spawn count = %d, want exactly 1 across %d concurrent starts", n, starters)
	}
	if len(h.terminatedPIDs()) != 0 {
		t.Errorf("no process may be signalled: %v", h.terminatedPIDs())
	}
}

// --- health-identified receivers --------------------------------------------------------

func TestStart_NewerRunningReceiverIsLeftAlone(t *testing.T) {
	h := newRecvHarness(t)
	f := startFake(t, healthHandler(otelreceiver.ReceiverRevision+1, 4242, "newer"))
	h.run(f.addr)
	h.assertNothingDone("newer receiver running")
	if lines := h.stderr.lines(); len(lines) != 0 {
		t.Errorf("no-op must be silent, got %v", lines)
	}
}

func TestStart_SameRevisionDifferentBuildIsNoOp(t *testing.T) {
	h := newRecvHarness(t)
	f := startFake(t, healthHandler(otelreceiver.ReceiverRevision, 4242, "some-other-build"))
	h.run(f.addr)
	h.assertNothingDone("equal revision")
}

func TestStart_OlderReceiverIsVerifiedThenTerminatedThenReplaced(t *testing.T) {
	h := newRecvHarness(t)
	f := startFake(t, healthHandler(1, 4242, "old"))
	h.onTerminate = func(int) { f.Close() } // the old process exits and frees the port
	h.cmdlineIs(4242, "/usr/local/bin/dreamland", "otel-receiver", "--foreground")

	h.run(f.addr)

	if got := h.terminatedPIDs(); len(got) != 1 || got[0] != 4242 {
		t.Fatalf("terminated = %v, want [4242] (the health-reported pid)", got)
	}
	if h.spawnCount() != 1 {
		t.Fatalf("spawn count = %d, want 1 after the port was freed", h.spawnCount())
	}
	ci, ti, si := h.eventIndex("cmdline:4242"), h.eventIndex("terminate:4242"), h.eventIndex("spawn")
	if ci < 0 || ci > ti || ti > si {
		t.Errorf("event order = cmdline %d, terminate %d, spawn %d; want cmdline < terminate < spawn", ci, ti, si)
	}
}

func TestStart_OlderReceiverWhoseReportedPidIsNotDreamlandIsNotSignalled(t *testing.T) {
	h := newRecvHarness(t)
	f := startFake(t, healthHandler(1, 4242, "old"))
	h.cmdlineIs(4242, "/usr/bin/node", "server.js") // a spoofed or stale health pid

	h.run(f.addr)

	h.assertNothingDone("health pid failing isDreamlandReceiver")
	h.assertOneStderrLine("health pid failing isDreamlandReceiver")
}

func TestStart_OlderReceiverWhosePortNeverFreesGivesUpQuietly(t *testing.T) {
	if testing.Short() {
		t.Skip("waits the full 3 s port-release window")
	}
	h := newRecvHarness(t)
	f := startFake(t, healthHandler(1, 4242, "old"))
	h.cmdlineIs(4242, "/usr/local/bin/dreamland", "otel-receiver", "--foreground")
	// terminate "succeeds" but the process keeps the port

	start := time.Now()
	h.run(f.addr)

	if elapsed := time.Since(start); elapsed < 2*time.Second || elapsed > 6*time.Second {
		t.Errorf("waited %v for the port to free, want about 3 s", elapsed)
	}
	if h.spawnCount() != 0 {
		t.Error("must not spawn while the port is still held")
	}
	if len(h.stderr.lines()) != 1 {
		t.Errorf("want exactly one stderr line, got %q", h.stderr.String())
	}
}

// --- unidentified listeners: automatic eviction and its safety rules --------------------

// unidentifiedHandlers are the shapes a listener without a valid health document takes,
// including every pre-handshake dreamland receiver (catch-all 200, no health document).
var unidentifiedHandlers = map[string]http.Handler{
	"404": http.NotFoundHandler(),
	"200 catch-all empty body (pre-handshake dreamland receiver)": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}),
	"200 non-health body": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello, not a health document"))
	}),
	"200 wrong service": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"service":"some-other-thing","revision":99,"pid":1}`))
	}),
}

func TestStart_PreHandshakeDreamlandReceiverIsEvicted(t *testing.T) {
	for name, handler := range unidentifiedHandlers {
		t.Run(name, func(t *testing.T) {
			h := newRecvHarness(t)
			f := startFake(t, handler)
			h.onTerminate = func(int) { f.Close() }
			h.listenerIs(777)
			h.cmdlineIs(777, "/usr/local/bin/dreamland", "otel-receiver", "--foreground")

			h.run(f.addr)

			if got := h.terminatedPIDs(); len(got) != 1 || got[0] != 777 {
				t.Fatalf("terminated = %v, want [777]", got)
			}
			if h.spawnCount() != 1 {
				t.Fatalf("spawn count = %d, want 1", h.spawnCount())
			}
			h.assertOneStderrLine("eviction", "evicted", "777", portOf(f.addr))
			ci, ti, si := h.eventIndex("cmdline:777"), h.eventIndex("terminate:777"), h.eventIndex("spawn")
			if ci < 0 || ci > ti || ti > si {
				t.Errorf("event order cmdline=%d terminate=%d spawn=%d; the command line must be verified before the signal", ci, ti, si)
			}
		})
	}
}

func TestStart_SamePidListedForIPv4AndIPv6IsOneDistinctPid(t *testing.T) {
	h := newRecvHarness(t)
	f := startFake(t, http.NotFoundHandler())
	h.onTerminate = func(int) { f.Close() }
	h.listenerIs(777, 777)
	h.cmdlineIs(777, "/usr/local/bin/dreamland", "otel-receiver")

	h.run(f.addr)

	if got := h.terminatedPIDs(); len(got) != 1 || got[0] != 777 {
		t.Errorf("terminated = %v, want [777]", got)
	}
}

func TestStart_NonDreamlandListenerIsLeftAlone(t *testing.T) {
	cases := []struct {
		name string
		exe  string
		args []string
	}{
		{"node", "/usr/bin/node", []string{"dreamland", "otel-receiver"}},
		{"dreamland-helper", "/usr/local/bin/dreamland-helper", []string{"otel-receiver"}},
		{"python otel-receiver.py", "/usr/bin/python3", []string{"otel-receiver.py"}},
		{"dreamland without otel-receiver", "/usr/local/bin/dreamland", []string{"status"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newRecvHarness(t)
			f := startFake(t, http.NotFoundHandler())
			h.listenerIs(555)
			h.cmdlineIs(555, tc.exe, tc.args...)

			h.run(f.addr)

			h.assertNothingDone(tc.name)
			h.assertOneStderrLine(tc.name, portOf(f.addr), "not a dreamland receiver or could not be identified")
		})
	}
}

func TestStart_IdentificationFailureIsNonFatalAndSignalsNothing(t *testing.T) {
	cases := map[string]func(h *recvHarness){
		"lookup error": func(h *recvHarness) {
			lookupListenerPIDFn = func(int) ([]int, error) { return nil, errors.New("lsof: command not found") }
		},
		"lookup finds no pid": func(h *recvHarness) { h.listenerIs() },
		"two distinct pids": func(h *recvHarness) {
			h.listenerIs(100, 200)
			h.cmdlineIs(100, "/usr/local/bin/dreamland", "otel-receiver")
		},
		"command line unreadable": func(h *recvHarness) {
			h.listenerIs(100)
			processCommandLineFn = func(int) (string, []string, error) { return "", nil, errors.New("permission denied") }
		},
		"command line empty": func(h *recvHarness) {
			h.listenerIs(100)
			processCommandLineFn = func(int) (string, []string, error) { return "", nil, nil }
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			h := newRecvHarness(t)
			f := startFake(t, http.NotFoundHandler())
			setup(h)

			h.run(f.addr)

			h.assertNothingDone(name)
			h.assertOneStderrLine(name, portOf(f.addr), "not a dreamland receiver or could not be identified")
		})
	}
}

// A listener that accepts and immediately drops the connection is "something answers", not
// "connection refused": it must go through the identification path, never a blind spawn.
func TestStart_ListenerThatDropsConnectionsIsUnidentified(t *testing.T) {
	h := newRecvHarness(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("loopback unavailable: %v", err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()

	h.run(ln.Addr().String())

	h.assertNothingDone("connection-dropping listener")
	h.assertOneStderrLine("connection-dropping listener", "not a dreamland receiver or could not be identified")
}

func TestStart_HangingListenerIsTreatedAsUnidentifiedWithinTheProbeTimeout(t *testing.T) {
	h := newRecvHarness(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("loopback unavailable: %v", err)
	}
	var conns []net.Conn
	var cmu sync.Mutex
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			cmu.Lock()
			conns = append(conns, c) // accept and never answer
			cmu.Unlock()
		}
	}()
	t.Cleanup(func() {
		ln.Close()
		cmu.Lock()
		for _, c := range conns {
			c.Close()
		}
		cmu.Unlock()
	})

	start := time.Now()
	h.run(ln.Addr().String())

	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Errorf("start took %v; the health probe must time out at about 500 ms", elapsed)
	}
	h.assertNothingDone("hanging listener")
	h.assertOneStderrLine("hanging listener", "not a dreamland receiver or could not be identified")
}

// --- --replace ---------------------------------------------------------------------------

func TestReplace_EvictsEqualRevisionDreamlandReceiver(t *testing.T) {
	h := newRecvHarness(t)
	otelReceiverReplace = true
	f := startFake(t, healthHandler(otelreceiver.ReceiverRevision, 555, "same"))
	h.onTerminate = func(int) { f.Close() }
	h.cmdlineIs(555, "/usr/local/bin/dreamland", "otel-receiver", "--foreground")

	h.run(f.addr)

	if got := h.terminatedPIDs(); len(got) != 1 || got[0] != 555 {
		t.Fatalf("terminated = %v, want [555]", got)
	}
	if h.spawnCount() != 1 {
		t.Errorf("spawn count = %d, want 1", h.spawnCount())
	}
}

func TestReplace_EvictsNewerRevisionDreamlandReceiver(t *testing.T) {
	h := newRecvHarness(t)
	otelReceiverReplace = true
	f := startFake(t, healthHandler(otelreceiver.ReceiverRevision+5, 556, "newer"))
	h.onTerminate = func(int) { f.Close() }
	h.cmdlineIs(556, "/usr/local/bin/dreamland", "otel-receiver")

	h.run(f.addr)

	if got := h.terminatedPIDs(); len(got) != 1 || got[0] != 556 {
		t.Fatalf("terminated = %v, want [556]", got)
	}
	if h.spawnCount() != 1 {
		t.Errorf("spawn count = %d, want 1", h.spawnCount())
	}
}

func TestReplace_ResolvesPidByListenerLookupWhenHealthHasNone(t *testing.T) {
	h := newRecvHarness(t)
	otelReceiverReplace = true
	f := startFake(t, http.NotFoundHandler())
	h.onTerminate = func(int) { f.Close() }
	h.listenerIs(321)
	h.cmdlineIs(321, "/usr/local/bin/dreamland", "otel-receiver", "--foreground")

	h.run(f.addr)

	if got := h.terminatedPIDs(); len(got) != 1 || got[0] != 321 {
		t.Fatalf("terminated = %v, want [321]", got)
	}
	if h.spawnCount() != 1 {
		t.Errorf("spawn count = %d, want 1", h.spawnCount())
	}
}

func TestReplace_RefusesToSignalAnUnrelatedProcess(t *testing.T) {
	t.Run("health-reported pid", func(t *testing.T) {
		h := newRecvHarness(t)
		otelReceiverReplace = true
		f := startFake(t, healthHandler(otelreceiver.ReceiverRevision, 555, "same"))
		h.cmdlineIs(555, "/usr/bin/vim", "otel-receiver.md")

		h.run(f.addr)

		h.assertNothingDone("--replace with an unrelated health pid")
		if len(h.stderr.lines()) == 0 {
			t.Error("a message must be written to stderr")
		}
	})
	t.Run("listener lookup pid", func(t *testing.T) {
		h := newRecvHarness(t)
		otelReceiverReplace = true
		f := startFake(t, http.NotFoundHandler())
		h.listenerIs(321)
		h.cmdlineIs(321, "/usr/bin/python3", "otel-receiver.py")

		h.run(f.addr)

		h.assertNothingDone("--replace with an unrelated listener")
		if len(h.stderr.lines()) == 0 {
			t.Error("a message must be written to stderr")
		}
	})
}

func TestReplace_MissingLookupToolingPrintsManualInstructionsAndExitsZero(t *testing.T) {
	h := newRecvHarness(t)
	otelReceiverReplace = true
	f := startFake(t, http.NotFoundHandler())
	lookupListenerPIDFn = func(int) ([]int, error) { return nil, errors.New("lsof: not found") }

	h.run(f.addr)

	h.assertNothingDone("--replace without lookup tooling")
	if len(h.stderr.lines()) == 0 {
		t.Error("expected manual instructions on stderr")
	}
}

func TestReplace_WithNothingListeningJustStarts(t *testing.T) {
	h := newRecvHarness(t)
	otelReceiverReplace = true
	h.run(freeAddr(t))
	if h.spawnCount() != 1 || len(h.terminatedPIDs()) != 0 {
		t.Errorf("spawn=%d terminated=%v, want a plain start", h.spawnCount(), h.terminatedPIDs())
	}
}

// --- isDreamlandReceiver -----------------------------------------------------------------

func TestIsDreamlandReceiver(t *testing.T) {
	tests := []struct {
		name string
		exe  string
		args []string
		want bool
	}{
		{"unix path", "/usr/local/bin/dreamland", []string{"otel-receiver", "--foreground"}, true},
		{"bare name", "dreamland", []string{"otel-receiver"}, true},
		{"windows path", `C:\Users\x\dreamland.exe`, []string{"otel-receiver"}, true},
		{"windows path, upper case", `C:\Users\x\DREAMLAND.EXE`, []string{"otel-receiver"}, true},
		{"path containing spaces", "/Users/some one/My Tools/dreamland", []string{"otel-receiver", "--foreground", "--addr", "localhost:4318"}, true},
		{"windows path containing spaces", `C:\Program Files\dreamland\dreamland.exe`, []string{"otel-receiver"}, true},
		{"otel-receiver not first arg", "/usr/local/bin/dreamland", []string{"--verbose", "otel-receiver"}, true},

		{"dreamland-foo", "/usr/local/bin/dreamland-foo", []string{"otel-receiver"}, false},
		{"dreamland-helper", "/usr/local/bin/dreamland-helper", []string{"otel-receiver"}, false},
		{"node wrapper", "/usr/bin/node", []string{"dreamland", "otel-receiver"}, false},
		{"go run wrapper", "/usr/local/go/bin/go", []string{"run", ".", "otel-receiver"}, false},
		{"vim file", "/usr/bin/vim", []string{"otel-receiver.md"}, false},
		{"dreamland with otel-receiver.log", "/usr/local/bin/dreamland", []string{"otel-receiver.log"}, false},
		{"otel-receiver as a path segment", "/usr/local/bin/dreamland", []string{"/tmp/otel-receiver"}, false},
		{"dreamland other subcommand", "/usr/local/bin/dreamland", []string{"status"}, false},
		{"dreamland no args", "/usr/local/bin/dreamland", nil, false},
		{"empty exe", "", []string{"otel-receiver"}, false},
		{"empty everything", "", nil, false},
		{"exe with suffix", "/usr/local/bin/dreamland.sh", []string{"otel-receiver"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isDreamlandReceiver(tt.exe, tt.args); got != tt.want {
				t.Errorf("isDreamlandReceiver(%q, %q) = %v, want %v", tt.exe, tt.args, got, tt.want)
			}
		})
	}
}
