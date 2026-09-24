package agentissue

import (
	"errors"
	"strings"
	"testing"
	"time"
)

type call struct{ args []string }

func fake(t *testing.T, list string) (*[]call, func()) {
	var calls []call
	restore := SetRunGh(func(args ...string) (string, error) {
		calls = append(calls, call{args})
		if args[1] == "list" {
			return list, nil
		}
		return "https://github.com/o/r/issues/7\n", nil
	})
	return &calls, restore
}

func valid() Fields {
	return Fields{Name: "sandman", Role: "r", Rationale: "why", Tier: "full-edit", Routing: "a -> b", Criteria: "works"}
}

func TestCreateMissingFieldNoGh(t *testing.T) {
	calls, restore := fake(t, "")
	defer restore()
	f := valid()
	f.Name = ""
	if _, err := Create(f); err == nil || !strings.Contains(err.Error(), "name") {
		t.Fatalf("err = %v", err)
	}
	f = valid()
	f.Tier = "bogus"
	if _, err := Create(f); err == nil {
		t.Fatal("bad tier accepted")
	}
	if len(*calls) != 0 {
		t.Errorf("gh called: %v", *calls)
	}
}

func TestCreateGhMissing(t *testing.T) {
	defer SetRunGh(func(...string) (string, error) { return "", ErrGhMissing })()
	if _, err := Create(valid()); !errors.Is(err, ErrGhMissing) || !strings.Contains(err.Error(), "gh is required") {
		t.Fatalf("err = %v", err)
	}
}

func TestCreateDuplicate(t *testing.T) {
	calls, restore := fake(t, "New agent: other\nNew agent: sandman\n")
	defer restore()
	if _, err := Create(valid()); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("err = %v", err)
	}
	if len(*calls) != 1 {
		t.Errorf("create called despite duplicate")
	}
}

func TestCreateSuccessBodyHasEverySection(t *testing.T) {
	calls, restore := fake(t, "New agent: sandmanx\n")
	defer restore()
	url, err := Create(valid())
	if err != nil || url != "https://github.com/o/r/issues/7" {
		t.Fatalf("%q %v", url, err)
	}
	c := (*calls)[1].args
	joined := strings.Join(c, "\x00")
	if c[0] != "issue" || c[1] != "create" || !strings.Contains(joined, "--label\x00new-agent") || !strings.Contains(joined, "New agent: sandman") {
		t.Errorf("args = %q", c)
	}
	var body string
	for i, a := range c {
		if a == "--body" {
			body = c[i+1]
		}
	}
	for _, h := range []string{"### Name", "### Role", "### Rationale", "### Tool tier", "### Routing", "### Acceptance criteria"} {
		if !strings.Contains(body, h) {
			t.Errorf("body missing %s", h)
		}
	}
}

func TestPreviewStore(t *testing.T) {
	s := NewPreviewStore()
	now := time.Now()
	s.SetClock(func() time.Time { return now })
	id, err := s.Put(valid())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Take("nope", valid()); err == nil {
		t.Error("unknown id accepted")
	}
	other := valid()
	other.Role = "changed"
	if err := s.Take(id, other); err == nil {
		t.Error("changed fields accepted")
	}
	if err := s.Take(id, valid()); err != nil {
		t.Errorf("take: %v", err)
	}
	if err := s.Take(id, valid()); err == nil {
		t.Error("id reusable")
	}
	id, _ = s.Put(valid())
	now = now.Add(16 * time.Minute)
	if err := s.Take(id, valid()); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Errorf("expiry: %v", err)
	}
	if _, err := s.Put(Fields{}); err == nil {
		t.Error("invalid fields stored")
	}
}
