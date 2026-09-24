package handoff

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNext_Table(t *testing.T) {
	cases := []struct {
		name    string
		from    string
		tags    Tags
		in      Counter
		kind    string
		target  string
		out     Counter
		clear   bool
		missing bool
	}{
		{"morpheus complete", "morpheus", Tags{Handoff: "complete"}, Counter{}, KindDispatch, "phobetor", Counter{}, false, false},
		{"iktomi complete", "iktomi", Tags{Handoff: "complete"}, Counter{}, KindDispatch, "phobetor", Counter{}, false, false},
		{"nyx complete", "nyx", Tags{Handoff: "complete"}, Counter{}, KindDispatch, "morpheus", Counter{}, false, false},
		{"missing handoff tag means complete", "morpheus", Tags{}, Counter{}, KindDispatch, "phobetor", Counter{}, false, true},
		{"iktomi blocked", "iktomi", Tags{Handoff: "blocked"}, Counter{}, KindReport, "", Counter{}, false, false},
		{"morpheus blocked leaves counter", "morpheus", Tags{Handoff: "blocked"}, Counter{PhobetorFailures: 1}, KindReport, "", Counter{PhobetorFailures: 1}, false, false},
		{"pass", "phobetor", Tags{Verdict: "pass"}, Counter{PhobetorFailures: 1}, KindDispatch, "baku", Counter{}, true, false},
		{"first fail", "phobetor", Tags{Verdict: "fail"}, Counter{}, KindDispatch, "morpheus", Counter{PhobetorFailures: 1}, false, false},
		{"fail after retry", "phobetor", Tags{Verdict: "fail"}, Counter{PhobetorFailures: 1}, KindDispatch, "phantasos", Counter{PhobetorFailures: 2}, false, false},
		{"spec-defect", "phobetor", Tags{Verdict: "spec-defect"}, Counter{PhobetorFailures: 1}, KindDispatch, "phantasos", Counter{PhobetorFailures: 1}, false, false},
		{"unverified", "phobetor", Tags{Verdict: "unverified"}, Counter{PhobetorFailures: 1}, KindReport, "", Counter{PhobetorFailures: 1}, false, false},
		{"missing verdict first", "phobetor", Tags{}, Counter{}, KindDispatch, "phobetor", Counter{VerdictRetries: 1}, false, true},
		{"missing verdict second", "phobetor", Tags{}, Counter{VerdictRetries: 1}, KindReport, "", Counter{VerdictRetries: 1}, false, true},
		{"valid verdict resets retries", "phobetor", Tags{Verdict: "fail"}, Counter{VerdictRetries: 1}, KindDispatch, "morpheus", Counter{PhobetorFailures: 1}, false, false},
		{"phantasos resets", "phantasos", Tags{}, Counter{PhobetorFailures: 2}, "", "", Counter{}, true, false},
		{"baku clears", "baku", Tags{}, Counter{PhobetorFailures: 1}, "", "", Counter{}, true, false},
		{"unlisted agent", "hypnos", Tags{Handoff: "complete"}, Counter{PhobetorFailures: 1}, "", "", Counter{PhobetorFailures: 1}, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d, out := Next(c.from, c.tags, c.in, -1)
			if d.Kind != c.kind || d.Target != c.target {
				t.Errorf("got kind=%q target=%q, want %q %q", d.Kind, d.Target, c.kind, c.target)
			}
			if out != c.out {
				t.Errorf("counter = %+v, want %+v", out, c.out)
			}
			if d.ClearCounter != c.clear {
				t.Errorf("ClearCounter = %v, want %v", d.ClearCounter, c.clear)
			}
			if d.TagMissing != c.missing {
				t.Errorf("TagMissing = %v, want %v", d.TagMissing, c.missing)
			}
		})
	}
}

func TestNext_BlockedReportNamesJanus(t *testing.T) {
	d, _ := Next("iktomi", Tags{Handoff: "blocked"}, Counter{}, -1)
	if d.Reason == "" || !contains(d.Reason, "Janus") {
		t.Errorf("reason %q does not name Janus", d.Reason)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestParseTags(t *testing.T) {
	cases := []struct {
		name, in string
		want     Tags
	}{
		{"own line", "done\n[handoff: complete]\n", Tags{Handoff: "complete"}},
		{"crlf", "done\r\n[verdict: pass]\r\n[change: c1]\r\n", Tags{Verdict: "pass", Change: "c1"}},
		{"quoted inline ignored, last wins", "I saw [verdict: fail] earlier\n[verdict: pass]", Tags{Verdict: "pass"}},
		{"last wins", "[verdict: fail]\n[verdict: pass]", Tags{Verdict: "pass"}},
		{"out of set is absent", "[verdict: maybe]", Tags{}},
		{"later invalid replaces earlier valid", "[verdict: pass]\n[verdict: maybe]", Tags{}},
		{"bad change slug", "[change: Bad_Slug]", Tags{}},
		{"indented", "   [handoff: blocked]  ", Tags{Handoff: "blocked"}},
		{"none", "nothing here", Tags{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ParseTags(c.in); got != c.want {
				t.Errorf("got %+v want %+v", got, c.want)
			}
		})
	}
}

func TestNext_PartialPass(t *testing.T) {
	for _, c := range []struct {
		remaining int
		kind      string
		target    string
	}{{0, KindDispatch, "baku"}, {-1, KindDispatch, "baku"}, {3, KindReport, ""}} {
		d, out := Next("phobetor", Tags{Verdict: "pass", Change: "c1"}, Counter{PhobetorFailures: 1}, c.remaining)
		if d.Kind != c.kind || d.Target != c.target || !d.ClearCounter || out != (Counter{}) {
			t.Errorf("remaining=%d: %+v %+v", c.remaining, d, out)
		}
		if c.remaining > 0 && (!contains(d.Reason, "partial pass: 3 tasks of c1 unticked") || !contains(d.Reason, "baku only after")) {
			t.Errorf("reason = %q", d.Reason)
		}
	}
}

func TestTasksRemaining(t *testing.T) {
	repo := t.TempDir()
	write := func(slug, body string) {
		dir := filepath.Join(repo, "openspec", "changes", slug)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "tasks.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a", "- [x] 1.1 x\r\n- [ ] 1.2 y\r\n  - [ ] 1.3 z\r\n")
	write("done", "- [x] 1\n- [X] 2\n")
	write("none", "just prose\n")
	for slug, want := range map[string]int{"a": 2, "done": 0, "none": -1, "missing": -1, "": -1, "../x": -1} {
		if got := TasksRemaining(repo, slug); got != want {
			t.Errorf("TasksRemaining(%q) = %d, want %d", slug, got, want)
		}
	}
}
