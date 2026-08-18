package workflowgraph

import (
	"os"
	"path/filepath"
	"sync"
	"time"
)

// liveWatchDirs are the platform directories a Watcher polls, relative to
// repo root — the same six agent directories the importer scans.
var liveWatchDirs = platformAgentDirs

// WatchPaths returns the absolute directories to poll for changes: whichever
// of the six live platform directories actually exist under repoRoot, plus
// activeChangeDir (the current OpenSpec change directory, for task/change
// status — see /hypnos-view's status overlay) if non-empty and present.
func WatchPaths(repoRoot, activeChangeDir string) []string {
	var paths []string
	for _, dir := range liveWatchDirs {
		full := filepath.Join(repoRoot, dir)
		if info, err := os.Stat(full); err == nil && info.IsDir() {
			paths = append(paths, full)
		}
	}
	if activeChangeDir != "" {
		if info, err := os.Stat(activeChangeDir); err == nil && info.IsDir() {
			paths = append(paths, activeChangeDir)
		}
	}
	return paths
}

// Watcher polls a set of directories for the most recent modification time
// across all files beneath them, on a fixed interval, calling OnChange
// whenever that latest mtime advances.
//
// Implemented as polling rather than an OS-native inotify/kqueue watcher
// (e.g. the fsnotify package) to avoid a new runtime dependency beyond the
// one already accepted for litegraph.js (see design.md's Goals: "no new
// runtime dependency beyond a vendored litegraph.js static asset"). For a
// single local dev server watching a handful of small directories, a short
// poll interval is more than adequate and keeps go.mod unchanged.
type Watcher struct {
	Paths    []string
	Interval time.Duration
	OnChange func()

	stop chan struct{}
	done chan struct{}
}

// NewWatcher constructs a Watcher. Call Start to begin polling.
func NewWatcher(paths []string, interval time.Duration, onChange func()) *Watcher {
	return &Watcher{
		Paths:    paths,
		Interval: interval,
		OnChange: onChange,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// Start begins polling in a background goroutine.
func (w *Watcher) Start() {
	go w.run()
}

// Stop ends polling and blocks until the background goroutine has exited.
func (w *Watcher) Stop() {
	close(w.stop)
	<-w.done
}

func (w *Watcher) run() {
	defer close(w.done)
	last := latestModTime(w.Paths)
	ticker := time.NewTicker(w.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-w.stop:
			return
		case <-ticker.C:
			cur := latestModTime(w.Paths)
			if cur.After(last) {
				last = cur
				if w.OnChange != nil {
					w.OnChange()
				}
			}
		}
	}
}

// latestModTime returns the most recent modification time of any file found
// beneath any of paths. Tolerates transient stat errors (e.g. a file deleted
// mid-walk) rather than failing the whole scan.
func latestModTime(paths []string) time.Time {
	var latest time.Time
	for _, root := range paths {
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.ModTime().After(latest) {
				latest = info.ModTime()
			}
			return nil
		})
	}
	return latest
}

// Broadcaster fans out refresh notifications to any number of subscribers
// (one per open SSE connection, once §7 wires this into the HTTP server).
// Safe for concurrent use.
type Broadcaster struct {
	mu   sync.Mutex
	subs map[chan struct{}]struct{}
}

// NewBroadcaster returns an empty Broadcaster.
func NewBroadcaster() *Broadcaster {
	return &Broadcaster{subs: map[chan struct{}]struct{}{}}
}

// Subscribe registers a new subscriber and returns its notification channel.
// The channel is buffered by 1 so a pending notification is never lost
// waiting for a slow reader, and further Publish calls coalesce into it
// rather than blocking.
func (b *Broadcaster) Subscribe() chan struct{} {
	ch := make(chan struct{}, 1)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes and closes ch. Safe to call once per channel returned
// by Subscribe.
func (b *Broadcaster) Unsubscribe(ch chan struct{}) {
	b.mu.Lock()
	if _, ok := b.subs[ch]; ok {
		delete(b.subs, ch)
		close(ch)
	}
	b.mu.Unlock()
}

// Publish notifies every current subscriber. Non-blocking: a subscriber with
// an already-pending notification is skipped rather than blocked on.
func (b *Broadcaster) Publish() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
