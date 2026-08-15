package cmd

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"dreamland/internal/config"
	"dreamland/internal/workflowgraph"
)

var (
	hypnosServeMode string
	hypnosServePlan string
)

var hypnosServeCmd = &cobra.Command{
	Use:   "hypnos-serve",
	Short: "Serve (or headlessly apply a plan to) the workflow graph",
	Long: `dreamland hypnos-serve renders every agent, skill, and hook in the current
repository as a litegraph.js node graph.

  --mode=view         read-only server, no write routes mounted
  --mode=interactive   editable server, write routes mounted
  --mode=apply-plan    headless: apply --plan <file>'s operations and exit, no server`,
	RunE: runHypnosServe,
}

func init() {
	hypnosServeCmd.Flags().StringVar(&hypnosServeMode, "mode", "", "view | interactive | apply-plan")
	hypnosServeCmd.Flags().StringVar(&hypnosServePlan, "plan", "", "plan file (--mode=apply-plan only)")
	rootCmd.AddCommand(hypnosServeCmd)
}

func runHypnosServe(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	repoRoot, err := config.FindRepoRoot(cwd)
	if err != nil {
		return fmt.Errorf("hypnos-serve: %w", err)
	}

	switch hypnosServeMode {
	case "view":
		return serveGraph(cmd, repoRoot, false)
	case "interactive":
		return serveGraph(cmd, repoRoot, true)
	case "apply-plan":
		return runApplyPlan(cmd, repoRoot, hypnosServePlan)
	case "":
		return fmt.Errorf("hypnos-serve: --mode is required (view | interactive | apply-plan)")
	default:
		return fmt.Errorf("hypnos-serve: unknown --mode %q (want view | interactive | apply-plan)", hypnosServeMode)
	}
}

// guardedGraph holds the current in-memory graph behind a lock. Readers get
// a snapshot pointer; writers always Set a freshly-rebuilt-then-mutated
// graph rather than mutating the held one in place, so a concurrent reader
// never observes a partially-applied mutation.
type guardedGraph struct {
	mu sync.RWMutex
	g  *workflowgraph.Graph
}

func (gg *guardedGraph) Get() *workflowgraph.Graph {
	gg.mu.RLock()
	defer gg.mu.RUnlock()
	return gg.g
}

func (gg *guardedGraph) Set(g *workflowgraph.Graph) {
	gg.mu.Lock()
	gg.g = g
	gg.mu.Unlock()
}

func cachePathFor(repoRoot string) string {
	return filepath.Join(repoRoot, ".dreamland", "workflow-graph.json")
}

func lockPathFor(repoRoot string) string {
	return filepath.Join(repoRoot, ".dreamland", "hypnos.lock")
}

// serveGraph starts the local server: read-only in view mode, with write
// routes additionally mounted in interactive mode.
func serveGraph(cmd *cobra.Command, repoRoot string, interactive bool) error {
	initial, err := workflowgraph.Import(repoRoot)
	if err != nil {
		return fmt.Errorf("initial graph import: %w", err)
	}
	_ = workflowgraph.Save(cachePathFor(repoRoot), initial) // best-effort fast-reload convenience, not required

	guarded := &guardedGraph{g: initial}
	broadcaster := workflowgraph.NewBroadcaster()

	watcher := workflowgraph.NewWatcher(workflowgraph.WatchPaths(repoRoot, ""), 2*time.Second, func() {
		g, err := workflowgraph.Import(repoRoot)
		if err != nil {
			return // transient — next poll tries again
		}
		_ = workflowgraph.Save(cachePathFor(repoRoot), g)
		guarded.Set(g)
		broadcaster.Publish()
	})
	watcher.Start()
	defer watcher.Stop()

	mux, err := newHypnosMux(repoRoot, interactive, guarded, broadcaster)
	if err != nil {
		return err
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	url := fmt.Sprintf("http://%s/", ln.Addr().String())
	fmt.Fprintf(cmd.OutOrStdout(), "hypnos-serve (%s mode) listening on %s\n", modeLabel(interactive), url)
	openBrowser(url)

	server := &http.Server{Handler: mux}
	return server.Serve(ln)
}

// newHypnosMux builds the route table serveGraph listens on: read-only graph
// routes in every mode, write routes (/api/mutate) only when interactive is
// true. Split out from serveGraph so it can be exercised directly against
// httptest.NewServer without opening a real ephemeral port or starting the
// background watcher.
func newHypnosMux(repoRoot string, interactive bool, guarded *guardedGraph, broadcaster *workflowgraph.Broadcaster) (http.Handler, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/graph", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(guarded.Get())
	})
	mux.HandleFunc("GET /api/events", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		ch := broadcaster.Subscribe()
		defer broadcaster.Unsubscribe(ch)
		w.WriteHeader(http.StatusOK)
		flusher.Flush()
		for {
			select {
			case <-r.Context().Done():
				return
			case _, ok := <-ch:
				if !ok {
					return
				}
				fmt.Fprint(w, "data: refresh\n\n")
				flusher.Flush()
			}
		}
	})

	if interactive {
		mux.HandleFunc("POST /api/mutate", func(w http.ResponseWriter, r *http.Request) {
			var ops []workflowgraph.Operation
			if err := json.NewDecoder(r.Body).Decode(&ops); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			var applied int
			lockErr := workflowgraph.WithLock(lockPathFor(repoRoot), func() error {
				g, err := workflowgraph.Import(repoRoot) // always mutate from freshly re-imported disk state
				if err != nil {
					return err
				}
				applied, err = workflowgraph.ApplyOperations(repoRoot, g, ops)
				if err != nil {
					return err
				}
				if err := workflowgraph.Save(cachePathFor(repoRoot), g); err != nil {
					return err
				}
				guarded.Set(g)
				return nil
			})
			if lockErr != nil {
				http.Error(w, lockErr.Error(), http.StatusBadRequest)
				return
			}
			broadcaster.Publish()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]int{"applied": applied})
		})
	}

	staticFS, err := fs.Sub(workflowgraph.StaticAssets, "static")
	if err != nil {
		return nil, err
	}
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	return mux, nil
}

func modeLabel(interactive bool) string {
	if interactive {
		return "interactive"
	}
	return "view"
}

// runApplyPlan applies a plan file's operations headlessly: no HTTP listener
// is ever opened. Acquires the same advisory lock interactive saves use,
// rebuilds the graph from current disk state, applies the plan, releases the
// lock, and reports per-run results.
func runApplyPlan(cmd *cobra.Command, repoRoot, planPath string) error {
	if planPath == "" {
		return fmt.Errorf("hypnos-serve --mode=apply-plan: --plan <file> is required")
	}
	data, err := os.ReadFile(planPath)
	if err != nil {
		return fmt.Errorf("read plan file: %w", err)
	}
	var ops []workflowgraph.Operation
	if err := json.Unmarshal(data, &ops); err != nil {
		return fmt.Errorf("parse plan file: %w", err)
	}

	var applied int
	var applyErr error
	lockErr := workflowgraph.WithLock(lockPathFor(repoRoot), func() error {
		g, err := workflowgraph.Import(repoRoot)
		if err != nil {
			return err
		}
		applied, applyErr = workflowgraph.ApplyOperations(repoRoot, g, ops)
		if saveErr := workflowgraph.Save(cachePathFor(repoRoot), g); saveErr != nil && applyErr == nil {
			applyErr = saveErr
		}
		return nil // report applyErr below, not via the lock's own error path
	})
	if lockErr != nil {
		return fmt.Errorf("acquire lock: %w", lockErr)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "applied %d/%d operations\n", applied, len(ops))
	if applyErr != nil {
		return fmt.Errorf("plan apply failed at operation %d: %w", applied, applyErr)
	}
	return nil
}

func openBrowser(url string) {
	var c *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		c = exec.Command("open", url)
	case "windows":
		c = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		c = exec.Command("xdg-open", url)
	}
	_ = c.Start() // best-effort — printing the URL above is the fallback
}
