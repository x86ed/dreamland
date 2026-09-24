package handoff

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// Lock parameters; variables so tests can shorten them.
var (
	LockTimeout = 5 * time.Second
	LockStale   = 30 * time.Second
	PruneAge    = 14 * 24 * time.Hour
)

// ErrLockTimeout means the lock could not be acquired; callers fail open.
var ErrLockTimeout = errors.New("handoff: lock timeout")

var (
	changeRe  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
	sessionRe = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)
)

// ValidChange reports whether s is an acceptable change slug.
func ValidChange(s string) bool { return changeRe.MatchString(s) }

// ValidSessionID reports whether s is safe to use as a file name component.
func ValidSessionID(s string) bool { return sessionRe.MatchString(s) && s != "." && s != ".." }

func validKey(s string) bool {
	if strings.HasPrefix(s, "_session-") {
		return ValidSessionID(strings.TrimPrefix(s, "_session-"))
	}
	return ValidChange(s)
}

// StateRoot is the per-user state root: $DREAMLAND_STATE_DIR, else
// <UserCacheDir>/dreamland, else <TempDir>/dreamland.
func StateRoot() string {
	if d := os.Getenv("DREAMLAND_STATE_DIR"); d != "" {
		return d
	}
	if d, err := os.UserCacheDir(); err == nil && d != "" {
		return filepath.Join(d, "dreamland")
	}
	return filepath.Join(os.TempDir(), "dreamland")
}

// RepoID is the first 16 hex chars of the SHA-256 of the cleaned absolute repo
// root, lower-cased on Windows.
func RepoID(repoRoot string) string {
	p := filepath.Clean(repoRoot)
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	if runtime.GOOS == "windows" {
		p = strings.ToLower(p)
	}
	sum := sha256.Sum256([]byte(p))
	return hex.EncodeToString(sum[:])[:16]
}

// ResolveChange picks the counter key: a valid change tag, else the sole active
// change reported by active, else _session-<sessionID>.
func ResolveChange(tag, sessionID string, active func() ([]string, error)) string {
	if ValidChange(tag) {
		return tag
	}
	if active != nil {
		if names, err := active(); err == nil && len(names) == 1 && ValidChange(names[0]) {
			return names[0]
		}
	}
	return "_session-" + sessionID
}

// Store is the on-disk state for one repository.
type Store struct {
	Dir string // <root>/handoff/<repo-id>
}

// NewStore returns the store for repoRoot under the default state root.
func NewStore(repoRoot string) *Store {
	return NewStoreAt(StateRoot(), repoRoot)
}

// NewStoreAt returns the store for repoRoot under an explicit state root.
func NewStoreAt(root, repoRoot string) *Store {
	return &Store{Dir: filepath.Join(root, "handoff", RepoID(repoRoot))}
}

func (s *Store) counterPath(key string) string { return filepath.Join(s.Dir, key+".json") }
func (s *Store) pendingDir() string            { return filepath.Join(s.Dir, "pending") }
func (s *Store) pendingPath(session string) string {
	return filepath.Join(s.pendingDir(), session+".json")
}

// withLock runs fn while holding path+".lock", created with O_EXCL.
func withLock(path string, fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	lock := path + ".lock"
	deadline := time.Now().Add(LockTimeout)
	for {
		f, err := os.OpenFile(lock, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			f.Close()
			break
		}
		if !errors.Is(err, os.ErrExist) {
			// On Windows a racing create can surface as a permission error.
			if !os.IsPermission(err) {
				return err
			}
		}
		if fi, statErr := os.Stat(lock); statErr == nil && time.Since(fi.ModTime()) > LockStale {
			_ = os.Remove(lock)
			continue
		}
		if time.Now().After(deadline) {
			return ErrLockTimeout
		}
		time.Sleep(5 * time.Millisecond)
	}
	defer os.Remove(lock)
	return fn()
}

func writeAtomic(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

type counterFile struct {
	PhobetorFailures int    `json:"phobetor_failures"`
	VerdictRetries   int    `json:"verdict_retries"`
	UpdatedAt        string `json:"updated_at"`
}

// ReadCounter returns the counter for key (zero when absent).
func (s *Store) ReadCounter(key string) (Counter, error) {
	if !validKey(key) {
		return Counter{}, fmt.Errorf("handoff: invalid change key %q", key)
	}
	data, err := os.ReadFile(s.counterPath(key))
	if errors.Is(err, os.ErrNotExist) {
		return Counter{}, nil
	}
	if err != nil {
		return Counter{}, err
	}
	var cf counterFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return Counter{}, err
	}
	return Counter{PhobetorFailures: cf.PhobetorFailures, VerdictRetries: cf.VerdictRetries}, nil
}

// UpdateCounter runs a locked read-modify-write on key. fn returns the new
// counter and whether the file should be deleted instead of written.
func (s *Store) UpdateCounter(key string, fn func(Counter) (Counter, bool)) error {
	if !validKey(key) {
		return fmt.Errorf("handoff: invalid change key %q", key)
	}
	path := s.counterPath(key)
	return withLock(path, func() error {
		cur, err := s.ReadCounter(key)
		if err != nil {
			return err
		}
		next, del := fn(cur)
		if del {
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			return nil
		}
		if err := writeAtomic(path, counterFile{
			PhobetorFailures: next.PhobetorFailures,
			VerdictRetries:   next.VerdictRetries,
			UpdatedAt:        time.Now().UTC().Format(time.RFC3339),
		}); err != nil {
			return err
		}
		s.prune()
		return nil
	})
}

// prune removes state files not modified within PruneAge.
func (s *Store) prune() {
	for _, dir := range []string{s.Dir, s.pendingDir()} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if info, err := e.Info(); err == nil && time.Since(info.ModTime()) > PruneAge {
				_ = os.Remove(filepath.Join(dir, e.Name()))
			}
		}
	}
}

// Entry states.
const (
	StatePending   = "pending"
	StateAbandoned = "abandoned"
	StateReleased  = "released-by-user"
	StateSatisfied = "satisfied"
	MaxBlocks      = 3
)

// Entry is one pending directive for a dispatcher session.
type Entry struct {
	ID         string    `json:"id"`
	Agent      string    `json:"agent"`
	Change     string    `json:"change,omitempty"`
	Directive  Directive `json:"directive"`
	TagMissing bool      `json:"tag_missing,omitempty"`
	Blocks     int       `json:"blocks"`
	State      string    `json:"state"`
	CreatedAt  string    `json:"created_at"`
}

// Blocking reports whether the entry currently requires a dispatch.
func (e Entry) Blocking() bool {
	return e.State == StatePending && e.Directive.Kind == KindDispatch
}

type pendingFile struct {
	Entries []Entry `json:"entries"`
}

// DirectiveID is the idempotency id: SHA-256 of session id, agent, and report.
func DirectiveID(session, agent, report string) string {
	sum := sha256.Sum256([]byte(session + "\x00" + agent + "\x00" + report))
	return hex.EncodeToString(sum[:])
}

// ReadPending returns the entries for a session (nil when none). A corrupt file
// returns an error so callers can fail open.
func (s *Store) ReadPending(session string) ([]Entry, error) {
	if !ValidSessionID(session) {
		return nil, fmt.Errorf("handoff: invalid session id %q", session)
	}
	data, err := os.ReadFile(s.pendingPath(session))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var pf pendingFile
	if err := json.Unmarshal(data, &pf); err != nil {
		return nil, err
	}
	return pf.Entries, nil
}

// UpdatePending runs a locked read-modify-write on a session's entries.
func (s *Store) UpdatePending(session string, fn func([]Entry) []Entry) error {
	if !ValidSessionID(session) {
		return fmt.Errorf("handoff: invalid session id %q", session)
	}
	path := s.pendingPath(session)
	return withLock(path, func() error {
		cur, err := s.ReadPending(session)
		if err != nil {
			return err
		}
		next := fn(cur)
		if len(next) == 0 {
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			return nil
		}
		return writeAtomic(path, pendingFile{Entries: next})
	})
}

// Sessions lists sessions that have a pending file.
func (s *Store) Sessions() []string {
	entries, err := os.ReadDir(s.pendingDir())
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if n := e.Name(); !e.IsDir() && strings.HasSuffix(n, ".json") {
			out = append(out, strings.TrimSuffix(n, ".json"))
		}
	}
	return out
}
