// Package zhougongdash serves the embedded zhougong metrics dashboard on loopback only.
package zhougongdash

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"dreamland/internal/zhougongdata"
)

//go:embed static
var staticFS embed.FS

// AttributionNote is the banner shown on every dashboard view.
const AttributionNote = "Attribution is commit-author based and may miss internal turns."

// Store holds live-collected datasets in memory and merges in archived run records.
type Store struct {
	repoRoot string
	mu       sync.Mutex
	live     []zhougongdata.Dataset
	collect  sync.Mutex
}

// NewStore returns a Store that reads archived records from repoRoot.
func NewStore(repoRoot string) *Store {
	return &Store{repoRoot: repoRoot, inflight: map[string]bool{}, failed: map[string]failure{}}
}

// localBranches lists refs/heads, current branch first.
func (s *Store) localBranches() []string {
	out, err := exec.Command("git", "-C", s.repoRoot, "for-each-ref", "--format=%(refname:short)", "refs/heads").Output()
	if err != nil {
		return nil
	}
	cur := s.CurrentBranch()
	var branches []string
	if cur != "" {
		branches = append(branches, cur)
	}
	for _, b := range strings.Fields(string(out)) {
		if b != cur {
			branches = append(branches, b)
		}
	}
	return branches
}

// Kick starts background collection for every local branch lacking a fresh cache entry (once
// per HEAD sha, never twice concurrently) and returns the branches still collecting plus any
// collect errors. The disk cache is the only shared state.
func (s *Store) Kick() (collecting []string, errs string) {
	s.cmu.Lock()
	var batch []string
	var msgs []string
	for _, b := range s.localBranches() {
		if s.inflight[b] {
			continue
		}
		sha, err := zhougongdata.HeadSha(s.repoRoot, b)
		if err != nil {
			msgs = append(msgs, b+": "+err.Error())
			continue
		}
		if e, ok, err := zhougongdata.Read(s.repoRoot, b); err == nil && ok && !zhougongdata.IsStale(e, sha) {
			delete(s.failed, b)
			continue
		}
		if f, ok := s.failed[b]; ok && f.sha == sha {
			msgs = append(msgs, b+": "+f.msg)
			continue
		}
		s.inflight[b] = true
		s.order = append(s.order, b)
		batch = append(batch, b)
	}
	collecting = append(collecting, s.order...)
	s.cmu.Unlock()
	if len(batch) > 0 {
		go s.run(batch)
	}
	return collecting, strings.Join(msgs, "; ")
}

func (s *Store) run(batch []string) {
	s.worker.Lock()
	defer s.worker.Unlock()
	for _, b := range batch {
		var fail *failure
		ds, e, err := collectFn(s.repoRoot, b, false)
		_ = ds
		if err == nil && e != nil {
			err = zhougongdata.Write(s.repoRoot, *e)
		}
		if err != nil {
			sha, _ := zhougongdata.HeadSha(s.repoRoot, b)
			fail = &failure{sha: sha, msg: err.Error()}
		}
		s.cmu.Lock()
		delete(s.inflight, b)
		for i, o := range s.order {
			if o == b {
				s.order = append(s.order[:i], s.order[i+1:]...)
				break
			}
		}
		if fail != nil {
			s.failed[b] = *fail
		} else {
			delete(s.failed, b)
		}
		s.cmu.Unlock()
	}
}

// Put adds or replaces a live dataset by name.
func (s *Store) Put(ds zhougongdata.Dataset) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.live {
		if s.live[i].Name == ds.Name {
			s.live[i] = ds
			return
		}
	}
	s.live = append(s.live, ds)
}

// Has reports whether a live dataset with the given name is already collected.
func (s *Store) Has(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, d := range s.live {
		if d.Name == name {
			return true
		}
	}
	return false
}

// All returns live datasets (disk cache entries, overridden by in-memory ones) followed by archived records. An archived record whose
// name collides with a live dataset is renamed "<name> (archived)".
func (s *Store) All() []zhougongdata.Dataset {
	s.mu.Lock()
	live := append([]zhougongdata.Dataset{}, s.live...)
	s.mu.Unlock()
	out := []zhougongdata.Dataset{}
	inMem := map[string]bool{}
	for _, d := range live {
		inMem[d.Name] = true
	}
	for _, e := range zhougongdata.ReadAll(s.repoRoot) {
		if !inMem[e.Branch] {
			ds := e.Dataset
			ds.Name, ds.Source = e.Branch, "live"
			out = append(out, ds)
		}
	}
	out = append(out, live...)
	taken := map[string]bool{}
	for _, d := range out {
		taken[d.Name] = true
	}
	archived, err := zhougongdata.LoadArchived(s.repoRoot)
	if err != nil {
		return out
	}
	for _, a := range archived {
		if taken[a.Name] {
			a.Name += " (archived)"
		}
		out = append(out, a)
	}
	return out
}

// Resolve maps a selection to a dataset: exact name, then a name containing sel (so a
// change slug finds its branch). Unresolved selections become a no-data placeholder.
func (s *Store) Resolve(sel string) zhougongdata.Dataset {
	all := s.All()
	for _, d := range all {
		if d.Name == sel {
			return d
		}
	}
	for _, d := range all {
		if strings.Contains(d.Branch, sel) || strings.Contains(d.Name, sel) {
			return d
		}
	}
	return zhougongdata.NoData(sel)
}

// CurrentBranch returns the branch checked out at the store's repo root, or "" when
// unavailable or detached.
func (s *Store) CurrentBranch() string {
	out, err := exec.Command("git", "-C", s.repoRoot, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return ""
	}
	b := strings.TrimSpace(string(out))
	if b == "HEAD" {
		return ""
	}
	return b
}

// Dashboard is a start/stop-able localhost HTTP server.
type Dashboard struct {
	store *Store
	mu    sync.Mutex
	srv   *http.Server
	url   string
	addr  string
}

// New returns a stopped Dashboard backed by store.
func New(store *Store) *Dashboard { return &Dashboard{store: store} }

// Start binds 127.0.0.1:port (0 picks a free port) and returns the URL. If already
// running it returns the existing URL.
func (d *Dashboard) Start(port int) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.srv != nil {
		return d.url, nil
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return "", err
	}
	d.srv = &http.Server{Handler: d.Handler(), ReadHeaderTimeout: 10 * time.Second}
	d.addr = ln.Addr().String()
	d.url = "http://" + d.addr
	go func(s *http.Server) { _ = s.Serve(ln) }(d.srv)
	return d.url, nil
}

// Addr returns the bound listener address, or "" when stopped.
func (d *Dashboard) Addr() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.addr
}

// Stop shuts the server down and releases the port. Stopping a stopped dashboard is a no-op.
func (d *Dashboard) Stop() error {
	d.mu.Lock()
	srv := d.srv
	d.srv, d.url, d.addr = nil, "", ""
	d.mu.Unlock()
	if srv == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		return srv.Close()
	}
	return nil
}

// Entry is the per-dataset payload of /api/summary.
type Entry struct {
	Summary     zhougongdata.Summary      `json:"summary"`
	Agents      []zhougongdata.AgentStat  `json:"agents"`
	Runs        []zhougongdata.Run        `json:"runs"`
	Transitions []zhougongdata.Transition `json:"transitions"`
}

// Handler returns the dashboard's HTTP handler.
func (d *Dashboard) Handler() http.Handler {
	mux := http.NewServeMux()
	sub, _ := fs.Sub(staticFS, "static")
	mux.Handle("/", http.FileServer(http.FS(sub)))
	mux.HandleFunc("/api/summary", d.summary)
	mux.HandleFunc("/api/compare", d.compare)
	return mux
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func (d *Dashboard) summary(w http.ResponseWriter, _ *http.Request) {
	collecting, collectErr := d.store.Kick()
	all := d.store.All()
	entries := []Entry{}
	for _, ds := range all {
		entries = append(entries, Entry{
			Summary: zhougongdata.Summarize(ds), Agents: zhougongdata.AgentStats(ds.Runs),
			Runs: ds.Runs, Transitions: zhougongdata.Transitions(ds.Runs),
		})
	}
	current, currentDataset := d.store.CurrentBranch(), ""
	if current != "" {
		if r := d.store.Resolve(current); r.Source != "nodata" {
			currentDataset = r.Name
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"currentBranch":  current,
		"currentDataset": currentDataset,
		"collectError":   collectErr,
		"collecting":     collecting,
		"datasets":       entries,
		"typicalFlow":    zhougongdata.TypicalFlow(all),
		"exclusions":     zhougongdata.ExcludedCodeGlobs,
		"attribution":    AttributionNote,
		"maxCompare":     zhougongdata.MaxBranches,
		"minCompare":     2,
	})
}

// ParseSelection splits a comma-separated branch list and enforces the 2..8 range.
func ParseSelection(raw string) ([]string, error) {
	var names []string
	for _, n := range strings.Split(raw, ",") {
		if n = strings.TrimSpace(n); n != "" {
			names = append(names, n)
		}
	}
	if len(names) < 2 {
		return nil, fmt.Errorf("select between 2 and %d branches, got %d", zhougongdata.MaxBranches, len(names))
	}
	if err := zhougongdata.CheckMaxBranches(len(names)); err != nil {
		return nil, fmt.Errorf("select between 2 and %d branches: %w", zhougongdata.MaxBranches, err)
	}
	return names, nil
}

func (d *Dashboard) compare(w http.ResponseWriter, r *http.Request) {
	names, err := ParseSelection(r.URL.Query().Get("branches"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	sets := make([]zhougongdata.Dataset, 0, len(names))
	for _, n := range names {
		sets = append(sets, d.store.Resolve(n))
	}
	baseline := r.URL.Query().Get("baseline")
	if baseline != "" {
		baseline = d.store.Resolve(baseline).Name
	}
	writeJSON(w, http.StatusOK, zhougongdata.Compare(sets, baseline))
}
