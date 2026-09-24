package handoff

import (
	"encoding/json"
	"io/fs"
	"regexp"
	"sort"
	"strings"
	"testing"

	"dreamland/internal/scaffold"
)

// Same semantics as workflowgraph's handOffPattern, with the backticks optional
// because the Codex templates do not quote agent names.
var driftHandOff = regexp.MustCompile("hand(?:s)? off directly to `?([a-z0-9-]+)`?")

var driftPlatforms = map[string]func(agent string) string{
	"claude-code":    func(a string) string { return "templates/agents/claude-code/" + a + ".md" },
	"cursor":         func(a string) string { return "templates/agents/cursor/" + a + ".mdc" },
	"codex":          func(a string) string { return "templates/agents/codex/" + a + ".toml" },
	"kiro":           func(a string) string { return "templates/agents/kiro/" + a + ".md" },
	"antigravity":    func(a string) string { return "templates/agents/antigravity/" + a + "/SKILL.md" },
	"github-copilot": func(a string) string { return "templates/agents/github-copilot/" + a + ".agent.md" },
}

func readTemplate(t *testing.T, path string) string {
	t.Helper()
	data, err := fs.ReadFile(scaffold.TemplateFS, path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func sortedUnique(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

func TestDrift_TemplatesMatchEdgeTable(t *testing.T) {
	for platform, pathOf := range driftPlatforms {
		for agent, want := range Targets() {
			t.Run(platform+"/"+agent, func(t *testing.T) {
				body := readTemplate(t, pathOf(agent))
				var got []string
				for _, m := range driftHandOff.FindAllStringSubmatch(body, -1) {
					if m[1] == "that" { // "hand off directly to that agent" (iktomi's free-form routing)
						continue
					}
					got = append(got, m[1])
				}
				got = sortedUnique(got)
				want = sortedUnique(want)
				if strings.Join(got, ",") != strings.Join(want, ",") {
					t.Errorf("%s %s: template hand-off targets %v, edge table %v", platform, agent, got, want)
				}
			})
		}
	}
}

func TestDrift_TagInstructions(t *testing.T) {
	need := map[string][]string{
		"nyx":      {"[handoff: complete]", "[handoff: blocked]"},
		"morpheus": {"[handoff: complete]", "[handoff: blocked]"},
		"iktomi":   {"[handoff: complete]", "[handoff: blocked]"},
		"phobetor": {"[verdict: pass]", "[verdict: fail]", "[verdict: spec-defect]", "[verdict: unverified]", "[change: <slug>]"},
	}
	for platform, pathOf := range driftPlatforms {
		for agent, tags := range need {
			body := readTemplate(t, pathOf(agent))
			for _, tag := range tags {
				if !strings.Contains(body, tag) {
					t.Errorf("%s %s template missing %s", platform, agent, tag)
				}
			}
		}
	}
}

func TestDrift_IktomiHasNoFileChangedConditional(t *testing.T) {
	for platform, pathOf := range driftPlatforms {
		body := readTemplate(t, pathOf("iktomi"))
		for _, stale := range []string{"If your work included editing", "If your work made no file changes", "report completion or blockers to Janus"} {
			if strings.Contains(body, stale) {
				t.Errorf("%s iktomi template still contains %q", platform, stale)
			}
		}
	}
}

func TestDrift_NonHookPlatformsPointAtHandoffNext(t *testing.T) {
	for _, platform := range []string{"cursor", "codex", "kiro", "antigravity", "github-copilot"} {
		for _, agent := range []string{"nyx", "morpheus", "iktomi", "phobetor"} {
			body := readTemplate(t, driftPlatforms[platform](agent))
			if !strings.Contains(body, "dreamland handoff next --from "+agent) {
				t.Errorf("%s %s template missing the `dreamland handoff next` instruction", platform, agent)
			}
		}
	}
}

func TestDrift_SettingsPatchBindsAllModes(t *testing.T) {
	data := readTemplate(t, "templates/hooks/bindings/claude-code/settings-patch.json")
	var patch struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal([]byte(data), &patch); err != nil {
		t.Fatal(err)
	}
	want := []struct{ event, matcher, cmd string }{
		{"SubagentStop", "", "dreamland handoff record --hook"},
		{"PostToolUse", "Task|Agent", "dreamland handoff inject --hook"},
		{"PreToolUse", "Task|Agent", "dreamland handoff enforce --hook"},
		{"Stop", "", "dreamland handoff stop-check --hook"},
		{"UserPromptSubmit", "", "dreamland handoff prompt --hook"},
	}
	for _, w := range want {
		found := false
		for _, e := range patch.Hooks[w.event] {
			for _, h := range e.Hooks {
				if e.Matcher == w.matcher && h.Command == w.cmd {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("settings-patch.json does not bind %q on %s[%q]", w.cmd, w.event, w.matcher)
		}
	}
}
