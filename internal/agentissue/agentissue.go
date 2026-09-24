// Package agentissue creates the structured GitHub issue that proposes a new agent.
package agentissue

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const Label = "new-agent"

var Tiers = []string{"router", "read-dispatch-only", "full-edit", "write-only-no-edit"}

// Fields are the values of the new-agent issue form.
type Fields struct {
	Name, Role, Rationale, Tier, Routing, Criteria string
}

// ErrGhMissing is returned when the gh CLI is not on PATH.
var ErrGhMissing = errors.New("gh is required to create issues but was not found on PATH")

// runGh invokes the gh CLI and returns its stdout. Replaced in tests.
var runGh = func(args ...string) (string, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return "", ErrGhMissing
	}
	out, err := exec.Command("gh", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gh %s: %w: %s", args[0], err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// SetRunGh swaps the gh runner and returns a restore func; for tests in other packages.
func SetRunGh(f func(args ...string) (string, error)) func() {
	old := runGh
	runGh = f
	return func() { runGh = old }
}

// Validate reports the first missing or invalid field, without side effects.
func (f Fields) Validate() error {
	for _, c := range []struct{ flag, v string }{
		{"name", f.Name}, {"role", f.Role}, {"rationale", f.Rationale},
		{"tier", f.Tier}, {"routing", f.Routing}, {"criteria", f.Criteria},
	} {
		if strings.TrimSpace(c.v) == "" {
			return fmt.Errorf("missing required field: %s", c.flag)
		}
	}
	for _, t := range Tiers {
		if f.Tier == t {
			return nil
		}
	}
	return fmt.Errorf("invalid tier %q: must be one of %s", f.Tier, strings.Join(Tiers, ", "))
}

// Title is the issue title.
func (f Fields) Title() string { return "New agent: " + strings.TrimSpace(f.Name) }

// Body renders one section per issue-form field, using the form's labels.
func (f Fields) Body() string {
	sec := func(h, v string) string { return "### " + h + "\n\n" + strings.TrimSpace(v) + "\n\n" }
	return strings.TrimRight(
		sec("Name", f.Name)+sec("Role", f.Role)+sec("Rationale", f.Rationale)+
			sec("Tool tier", f.Tier)+sec("Routing", f.Routing)+sec("Acceptance criteria", f.Criteria), "\n") + "\n"
}

// Preview renders the title and body for human review.
func (f Fields) Preview() string {
	return "Title: " + f.Title() + "\nLabel: " + Label + "\n\n" + f.Body()
}

// Create validates f, refuses an exact-title duplicate, then creates the issue and returns its URL.
func Create(f Fields) (string, error) {
	if err := f.Validate(); err != nil {
		return "", err
	}
	title := f.Title()
	out, err := runGh("issue", "list", "--label", Label, "--state", "all", "--search", strings.TrimSpace(f.Name)+" in:title", "--json", "title", "--jq", ".[].title")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == title {
			return "", fmt.Errorf("an issue titled %q already exists", title)
		}
	}
	out, err = runGh("issue", "create", "--title", title, "--body", f.Body(), "--label", Label)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

const previewTTL = 15 * time.Minute

// PreviewStore holds pending previews in memory (single process).
type PreviewStore struct {
	mu  sync.Mutex
	now func() time.Time
	m   map[string]pending
	n   int
}

type pending struct {
	f       Fields
	expires time.Time
}

func NewPreviewStore() *PreviewStore {
	return &PreviewStore{now: time.Now, m: map[string]pending{}}
}

// SetClock replaces the store's clock; for tests.
func (s *PreviewStore) SetClock(now func() time.Time) { s.now = now }

// Put validates f, stores it and returns a previewId.
func (s *PreviewStore) Put(f Fields) (string, error) {
	if err := f.Validate(); err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	t := s.now()
	for k, p := range s.m {
		if t.After(p.expires) {
			delete(s.m, k)
		}
	}
	s.n++
	id := fmt.Sprintf("preview-%d-%d", t.UnixNano(), s.n)
	s.m[id] = pending{f: f, expires: t.Add(previewTTL)}
	return id, nil
}

// Take returns and removes the fields stored under id, if present, unexpired and equal to f.
func (s *PreviewStore) Take(id string, f Fields) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.m[id]
	if !ok {
		return errors.New("unknown previewId: call with confirm=false first")
	}
	if s.now().After(p.expires) {
		delete(s.m, id)
		return errors.New("previewId expired: call with confirm=false again")
	}
	if p.f != f {
		return errors.New("fields differ from the previewed issue: call with confirm=false again")
	}
	delete(s.m, id)
	return nil
}
