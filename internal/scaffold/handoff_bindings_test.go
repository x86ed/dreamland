package scaffold

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func commandsFor(t *testing.T, root, event, matcher string) []string {
	t.Helper()
	var cmds []string
	for _, e := range hookEntries(t, root, event) {
		if m, _ := e["matcher"].(string); m != matcher {
			continue
		}
		raw, _ := e["hooks"].([]any)
		for _, h := range raw {
			if c, _ := h.(map[string]any)["command"].(string); c != "" {
				cmds = append(cmds, c)
			}
		}
	}
	return cmds
}

func containsCmd(cmds []string, want string) bool {
	for _, c := range cmds {
		if c == want {
			return true
		}
	}
	return false
}

func TestInstall_ClaudeCode_HandoffBindings(t *testing.T) {
	root := fakeGitRepo(t)
	if _, err := Install(Config{RepoRoot: root, CodingTool: "Claude Code"}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	cases := []struct{ event, matcher, cmd string }{
		{"SubagentStop", "", "dreamland handoff record --hook"},
		{"PostToolUse", "Task|Agent", "dreamland handoff inject --hook"},
		{"PreToolUse", "Task|Agent", "dreamland handoff enforce --hook"},
		{"Stop", "", "dreamland handoff stop-check --hook"},
		{"UserPromptSubmit", "", "dreamland handoff release --hook"},
	}
	for _, c := range cases {
		if !containsCmd(commandsFor(t, root, c.event, c.matcher), c.cmd) {
			t.Errorf("%s[%q] missing %q", c.event, c.matcher, c.cmd)
		}
		if strings.ContainsAny(c.cmd, "&|;<>$`") {
			t.Errorf("%q contains a shell operator", c.cmd)
		}
	}
	if !containsCmd(commandsFor(t, root, "PreToolUse", "Task|Agent"), "dreamland coauthor --hook") {
		t.Error("existing coauthor --hook binding lost")
	}
}

func TestInstall_ClaudeCode_HandoffBindingsPreserveUserHooksAndAreIdempotent(t *testing.T) {
	root := fakeGitRepo(t)
	dir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	user := `{"hooks":{"Stop":[{"matcher":"","hooks":[{"type":"command","command":"my-own-stop-hook"}]}]}}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(user), 0o644); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := Install(Config{RepoRoot: root, CodingTool: "Claude Code", Force: true}); err != nil {
			t.Fatalf("Install %d: %v", i, err)
		}
	}
	stop := commandsFor(t, root, "Stop", "")
	if !containsCmd(stop, "my-own-stop-hook") || !containsCmd(stop, "dreamland handoff stop-check --hook") {
		t.Errorf("Stop commands = %v", stop)
	}
	n := 0
	for _, c := range stop {
		if c == "dreamland handoff stop-check --hook" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("stop-check bound %d times after re-install, want 1", n)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "settings.json"))
	if !json.Valid(data) {
		t.Error("settings.json is not valid JSON")
	}
}
