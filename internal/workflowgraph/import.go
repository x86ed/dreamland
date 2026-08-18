package workflowgraph

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// platformAgentDir maps a platform key to its live agent directory, relative to
// repo root. A directory that doesn't exist is simply skipped — only platforms
// actually installed in a given repository contribute nodes (confirmed: this
// repository itself only has .claude/ installed, not the other five).
var platformAgentDirs = map[string]string{
	"claude-code":    ".claude/agents",
	"cursor":         ".cursor/rules",
	"codex":          ".codex/agents",
	"kiro":           ".kiro/steering",
	"antigravity":    ".agents/skills", // directory-per-agent, SKILL.md inside
	"github-copilot": ".github/agents",
}

// platformSkillDirs maps a platform key to its live skill directory (directory-
// per-skill layout, SKILL.md inside each).
var platformSkillDirs = map[string]string{
	"claude-code": ".claude/skills",
	"codex":       ".codex/skills",
	"antigravity": ".agents/skills", // shares its parent with agent personas; distinguished by not matching an agent id
}

// DreamlandManagedMarker marks a file as written by dreamland and safe to
// regenerate. Matches internal/scaffold's bareCommandMarker convention (same
// string), reused here for skill files so a future dreamland-authored skill
// (§4, the skill-authoring writer) can be told apart from an externally-owned
// one like this repo's openspec-* skills, none of which carry it today.
const DreamlandManagedMarker = "<!-- dreamland-managed: safe to overwrite on `dreamland init` -->"

// Import builds a Graph by scanning repoRoot's installed platform files. Never
// errors on a missing platform directory or file — the live-rebuild importer
// tolerates a repository with only some platforms installed.
func Import(repoRoot string) (*Graph, error) {
	g := New(repoRoot)

	if err := importAgents(repoRoot, g); err != nil {
		return nil, err
	}
	importRoutesTo(g)
	if err := importHooks(repoRoot, g); err != nil {
		return nil, err
	}
	if err := importSkills(repoRoot, g); err != nil {
		return nil, err
	}
	return g, nil
}

// --- Agents ---------------------------------------------------------------

var frontmatterPattern = regexp.MustCompile(`(?s)^---\r?\n(.*?)\r?\n---\r?\n?(.*)$`)

// splitFrontmatter separates a `---`-delimited frontmatter block from the body
// that follows it. ok is false if content has no frontmatter block (e.g. Codex's
// .toml agent files, which use a different format entirely).
func splitFrontmatter(content string) (frontmatter, body string, ok bool) {
	m := frontmatterPattern.FindStringSubmatch(content)
	if m == nil {
		return "", content, false
	}
	return m[1], m[2], true
}

// frontmatterField extracts a single-line `field: value` from a frontmatter
// block, trimming surrounding quotes. Returns "" if absent — deliberately
// simple (regex, not a YAML parser), matching internal/scaffold's existing
// commandNamePattern convention rather than adding a new YAML dependency.
func frontmatterField(frontmatter, field string) string {
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(field) + `:\s*(.+?)\s*$`)
	m := re.FindStringSubmatch(frontmatter)
	if m == nil {
		return ""
	}
	return strings.Trim(strings.TrimSpace(m[1]), `"`)
}

var (
	codexNamePattern = regexp.MustCompile(`(?m)^name\s*=\s*"([^"]*)"`)
	codexDescPattern = regexp.MustCompile(`(?m)^description\s*=\s*"([^"]*)"`)
	codexBodyPattern = regexp.MustCompile(`(?s)developer_instructions\s*=\s*"""\r?\n(.*?)\r?\n"""`)
)

// parseAgentFile extracts id/description/instructionBody/tools from one agent
// file's raw content. id falls back to the filename stem (not a frontmatter
// name field) on every platform for consistency — GitHub Copilot's `name:`
// value is capitalized ("Hypnos") in this repo's own templates, which would
// otherwise produce a different node id per platform for the same agent.
func parseAgentFile(platform, content string) (description, body, toolsRaw string) {
	if platform == "codex" {
		if m := codexDescPattern.FindStringSubmatch(content); m != nil {
			description = m[1]
		}
		if m := codexBodyPattern.FindStringSubmatch(content); m != nil {
			body = m[1]
		}
		return description, body, ""
	}

	fm, b, ok := splitFrontmatter(content)
	if !ok {
		return "", strings.TrimSpace(content), ""
	}
	description = frontmatterField(fm, "description")
	toolsRaw = frontmatterField(fm, "tools")
	return description, strings.TrimSpace(b), toolsRaw
}

// parseAgentRole extracts the frontmatter `role:` field (e.g. "router"),
// empty string on platforms/files with no frontmatter (Codex's .toml format,
// or any file with no `role:` line — the common case).
func parseAgentRole(platform, content string) string {
	if platform == "codex" {
		return ""
	}
	fm, _, ok := splitFrontmatter(content)
	if !ok {
		return ""
	}
	return frontmatterField(fm, "role")
}

// deriveTier reverse-maps a raw tools field (comma list, optionally bracketed)
// into agent-scaffolding's three tiers. Returns "" for an empty/unparseable
// value — the caller leaves AgentNode.Tier unset on platforms with no tools
// field at all (Cursor, Kiro, Antigravity), which is a real, pre-existing
// characteristic of those platforms, not an import failure.
func deriveTier(toolsRaw string) Tier {
	toolsRaw = strings.Trim(toolsRaw, "[]")
	if toolsRaw == "" {
		return ""
	}
	hasEdit, hasWrite := false, false
	for t := range strings.SplitSeq(toolsRaw, ",") {
		switch strings.TrimSpace(t) {
		case "Edit":
			hasEdit = true
		case "Write":
			hasWrite = true
		}
	}
	switch {
	case hasEdit && hasWrite:
		return TierFullEdit
	case hasWrite && !hasEdit:
		return TierWriteOnlyNoEdit
	default:
		return TierRouterExcluded
	}
}

func importAgents(repoRoot string, g *Graph) error {
	// Deterministic platform iteration order so the first platform to set an
	// agent's description/body/tier is always the same across runs.
	platforms := make([]string, 0, len(platformAgentDirs))
	for p := range platformAgentDirs {
		platforms = append(platforms, p)
	}
	sort.Strings(platforms)

	for _, platform := range platforms {
		dir := filepath.Join(repoRoot, platformAgentDirs[platform])
		entries, err := os.ReadDir(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}

		for _, entry := range entries {
			var id, relPath string
			var content []byte

			if platform == "antigravity" {
				if !entry.IsDir() {
					continue
				}
				id = entry.Name()
				relPath = filepath.Join(platformAgentDirs[platform], id, "SKILL.md")
				content, err = os.ReadFile(filepath.Join(dir, id, "SKILL.md"))
			} else {
				if entry.IsDir() {
					continue
				}
				id = strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
				relPath = filepath.Join(platformAgentDirs[platform], entry.Name())
				content, err = os.ReadFile(filepath.Join(dir, entry.Name()))
			}
			if err != nil {
				continue // unreadable file — skip rather than fail the whole import
			}

			description, body, toolsRaw := parseAgentFile(platform, string(content))
			role := parseAgentRole(platform, string(content))

			node, exists := g.Agents[id]
			if !exists {
				node = &AgentNode{ID: id, PlatformFiles: map[string]string{}}
				g.Agents[id] = node
			}
			node.PlatformFiles[platform] = relPath
			if node.Description == "" {
				node.Description = description
			}
			if node.InstructionBody == "" {
				node.InstructionBody = body
			}
			if node.Tier == "" {
				if tier := deriveTier(toolsRaw); tier != "" {
					node.Tier = tier
				}
			}
			if node.Role == "" {
				node.Role = role
			}
		}
	}
	return nil
}

// --- Routing edges ----------------------------------------------------------

// handOffPattern matches the canonical hand-off sentence pattern this graph's
// own writer generates and this importer re-parses ("hand off directly to `X`"
// and its "hands off directly to `X`" agent-subject variant already used in
// this repo's existing prose).
var handOffPattern = regexp.MustCompile("hand(?:s)? off directly to `([a-z0-9-]+)`")

// nearMissHandOffPattern catches prose that talks about handing off/delegating
// without matching the canonical backtick-quoted pattern — a real signal a
// human wrote a hand-off in different words, not proof of anything. Used only
// to set UnresolvedRouting on an agent with no canonical match at all, per the
// "flagged, not guessed" design decision: no edge is fabricated from it.
var nearMissHandOffPattern = regexp.MustCompile(`(?i)\b(hand off|hands off|delegate|delegates|report to|reports to)\b`)

// routingTableHeadingPattern finds a "Routing table:" heading line — the
// structural entry-point pattern (currently only janus, marked `role:
// router`) where dispatch targets are a lookup table, not sequential
// hand-off prose, so handOffPattern never matches them.
var routingTableHeadingPattern = regexp.MustCompile(`(?im)^routing table:\s*$`)

// routingTableBulletPattern matches one routing-table bullet line, capturing
// the backtick-quoted agent id(s) before the em/en-dash description (e.g.
// "- `nyx`/`morpheus` — work a code task...").
var routingTableBulletPattern = regexp.MustCompile("^-\\s+((?:`[a-z0-9-]+`\\s*/?\\s*)+)\\s*[—–-]\\s")

var routingTableTargetPattern = regexp.MustCompile("`([a-z0-9-]+)`")

// broadRoutingPattern flags an agent that dispatches dynamically to any other
// agent rather than a fixed hand-off target — real prose already used by
// iktomi ("You have the same broad routing capability as Janus itself:
// dispatch directly to any other agent..."), not a fabricated signal.
var broadRoutingPattern = regexp.MustCompile(`(?i)broad routing capability`)

// extractRoutingTableTargets reads the contiguous bullet block immediately
// following a "Routing table:" heading and returns every distinct agent id
// named in it, in first-seen order. Read-only: unlike extractRoutesTo, the
// table is authored structural content (janus's actual dispatch logic), not
// writer-regenerated boilerplate, so nothing is stripped from body.
func extractRoutingTableTargets(body string) []string {
	loc := routingTableHeadingPattern.FindStringIndex(body)
	if loc == nil {
		return nil
	}
	var targets []string
	seen := map[string]bool{}
	started := false
	for line := range strings.SplitSeq(body[loc[1]:], "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if started {
				break
			}
			continue
		}
		m := routingTableBulletPattern.FindStringSubmatch(line)
		if m == nil {
			break
		}
		started = true
		for _, t := range routingTableTargetPattern.FindAllStringSubmatch(m[1], -1) {
			if id := t[1]; !seen[id] {
				seen[id] = true
				targets = append(targets, id)
			}
		}
	}
	return targets
}

// importRoutesTo extracts routes_to edges from each agent's instruction body,
// strips the matched sentence(s) from what's stored (they're regenerated from
// the edge on every sync — see the "structured routes_to" decision), and flags
// UnresolvedRouting when the body mentions handing off in some other phrasing
// the canonical pattern doesn't match. Entry-point agents (role: router, e.g.
// janus) additionally get routing edges read straight from their routing
// table, and any agent whose body claims broad/dynamic routing capability
// (e.g. iktomi) is flagged via BroadRouting rather than given a fixed edge
// set it doesn't actually have.
func importRoutesTo(g *Graph) {
	ids := make([]string, 0, len(g.Agents))
	for id := range g.Agents {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		agent := g.Agents[id]
		cleaned, targets, unresolved := extractRoutesTo(agent.InstructionBody, g.Agents)
		agent.InstructionBody = cleaned
		agent.UnresolvedRouting = unresolved
		for _, target := range targets {
			g.Edges = append(g.Edges, Edge{Kind: EdgeRouting, From: id, To: target})
		}

		if agent.Role == "router" {
			for _, target := range extractRoutingTableTargets(agent.InstructionBody) {
				if _, ok := g.Agents[target]; !ok {
					continue // quoted name isn't a known agent id
				}
				g.Edges = append(g.Edges, Edge{Kind: EdgeRouting, From: id, To: target})
				agent.UnresolvedRouting = false // resolved via the table, not actually unresolved
			}
		}

		if broadRoutingPattern.MatchString(agent.InstructionBody) {
			agent.BroadRouting = true
		}
	}
}

type textSpan struct{ start, end int }

// extractRoutesTo finds every canonical hand-off match in body, resolves each
// against known agent ids, removes the containing sentence for each resolved
// match, and reports whether the (post-strip) body still shows an unresolved
// hand-off mention.
func extractRoutesTo(body string, agents map[string]*AgentNode) (cleaned string, targets []string, unresolved bool) {
	matches := handOffPattern.FindAllStringSubmatchIndex(body, -1)

	var spans []textSpan
	seen := map[string]bool{}
	for _, m := range matches {
		target := body[m[2]:m[3]]
		if _, ok := agents[target]; !ok {
			continue // quoted name isn't a known agent id — leave the sentence as free text
		}
		if !seen[target] {
			seen[target] = true
			targets = append(targets, target)
		}
		s, e := sentenceBounds(body, m[0], m[1])
		spans = append(spans, textSpan{s, e})
	}

	sort.Slice(spans, func(i, j int) bool { return spans[i].start > spans[j].start })
	for _, sp := range spans {
		body = body[:sp.start] + body[sp.end:]
	}
	cleaned = collapseWhitespace(body)

	if len(targets) == 0 && nearMissHandOffPattern.MatchString(cleaned) {
		unresolved = true
	}
	return cleaned, targets, unresolved
}

// sentenceBounds expands a match's [start,end) span out to the nearest
// preceding '.'/newline and the nearest following '.', so the whole sentence
// (not just the quoted target) is what gets removed.
func sentenceBounds(s string, matchStart, matchEnd int) (int, int) {
	start := 0
	for i := matchStart - 1; i >= 0; i-- {
		if s[i] == '.' || s[i] == '\n' {
			start = i + 1
			break
		}
	}
	end := len(s)
	for i := matchEnd; i < len(s); i++ {
		if s[i] == '.' {
			end = i + 1
			break
		}
	}
	return start, end
}

var (
	trailingSpaceBeforeNewline = regexp.MustCompile(`[ \t]+\n`)
	excessBlankLines           = regexp.MustCompile(`\n{3,}`)
	excessSpaces               = regexp.MustCompile(`  +`)
)

func collapseWhitespace(s string) string {
	s = trailingSpaceBeforeNewline.ReplaceAllString(s, "\n")
	s = excessBlankLines.ReplaceAllString(s, "\n\n")
	s = excessSpaces.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// --- Hooks -------------------------------------------------------------------

// claudeEventKeys maps Claude Code's real settings.json/frontmatter hook keys
// to the HookEvent enum.
var claudeEventKeys = map[string]HookEvent{
	"SessionStart": EventSessionStart,
	"PreToolUse":   EventPreToolUse,
	"PostToolUse":  EventPostToolUse,
	"Stop":         EventStop,
	"SubagentStop": EventSubagentStop,
}

func slugCommand(command string) string {
	s := strings.ToLower(command)
	s = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// importHooks parses .claude/settings.json's workspace-level hooks.<Event>[]
// arrays into project-scoped HookNodes, and every installed agent's per-agent
// frontmatter hooks block into agent-scoped HookNodes. A command bound both
// ways (e.g. this repo's `telemetry write`, both in the workspace SubagentStop
// array and every agent's Stop block) imports as two separate nodes — see the
// "genuinely two independent bindings" decision.
func importHooks(repoRoot string, g *Graph) error {
	if err := importWorkspaceHooks(repoRoot, g); err != nil {
		return err
	}
	return importAgentHooks(repoRoot, g)
}

func importWorkspaceHooks(repoRoot string, g *Graph) error {
	data, err := os.ReadFile(filepath.Join(repoRoot, ".claude", "settings.json"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	var settings struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Type    string `json:"type"`
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return err
	}

	eventKeys := make([]string, 0, len(settings.Hooks))
	for k := range settings.Hooks {
		eventKeys = append(eventKeys, k)
	}
	sort.Strings(eventKeys)

	for _, key := range eventKeys {
		event, ok := claudeEventKeys[key]
		if !ok {
			continue // unrecognized event key — skip rather than fail the whole import
		}
		for _, binding := range settings.Hooks[key] {
			for _, h := range binding.Hooks {
				if h.Command == "" {
					continue
				}
				id := "project:" + string(event) + ":" + slugCommand(h.Command)
				g.Hooks[id] = &HookNode{ID: id, Command: h.Command, Event: event, Scope: ScopeProject}
				g.Edges = append(g.Edges, Edge{Kind: EdgeHookBinding, From: id, To: "project"})
			}
		}
	}
	return nil
}

// hookEventLine matches a top-level event key line inside a frontmatter
// `hooks:` block, e.g. "  Stop:" or "  SubagentStop:".
var hookEventLine = regexp.MustCompile(`^\s{2}(\w+):\s*$`)

// hookCommandLine matches a `command: <value>` line at any deeper indentation.
var hookCommandLine = regexp.MustCompile(`^\s+command:\s*(.+?)\s*$`)

// hooksBlockField extracts the `hooks:` block from a frontmatter string: every
// line from the `hooks:` key up to (but not including) the next top-level
// (column-0) key.
func hooksBlockField(frontmatter string) string {
	lines := strings.Split(frontmatter, "\n")
	start := -1
	for i, l := range lines {
		if l == "hooks:" {
			start = i + 1
			break
		}
	}
	if start == -1 {
		return ""
	}
	end := len(lines)
	for i := start; i < len(lines); i++ {
		if lines[i] != "" && !strings.HasPrefix(lines[i], " ") {
			end = i
			break
		}
	}
	return strings.Join(lines[start:end], "\n")
}

// parseAgentHookBlock scans a frontmatter hooks: block for (event, command)
// pairs, using each hookEventLine as the current event for hookCommandLines
// that follow it until the next hookEventLine.
func parseAgentHookBlock(block string) []struct {
	Event   HookEvent
	Command string
} {
	var out []struct {
		Event   HookEvent
		Command string
	}
	var currentEvent HookEvent
	for line := range strings.SplitSeq(block, "\n") {
		if m := hookEventLine.FindStringSubmatch(line); m != nil {
			currentEvent = claudeEventKeys[m[1]]
			continue
		}
		if m := hookCommandLine.FindStringSubmatch(line); m != nil && currentEvent != "" {
			out = append(out, struct {
				Event   HookEvent
				Command string
			}{currentEvent, m[1]})
		}
	}
	return out
}

// importAgentHooks parses per-agent frontmatter hooks blocks (Claude Code's
// Stop block, GitHub Copilot's SubagentStart/SubagentStop) into agent-scoped
// HookNodes. Cursor/Codex/Kiro/Antigravity have no per-agent hook mechanism —
// confirmed against their real template files — so they contribute nothing here.
func importAgentHooks(repoRoot string, g *Graph) error {
	for _, platform := range []string{"claude-code", "github-copilot"} {
		dir := filepath.Join(repoRoot, platformAgentDirs[platform])
		entries, err := os.ReadDir(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			agentID := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
			if _, ok := g.Agents[agentID]; !ok {
				continue
			}
			content, err := os.ReadFile(filepath.Join(dir, entry.Name()))
			if err != nil {
				continue
			}
			fm, _, ok := splitFrontmatter(string(content))
			if !ok {
				continue
			}
			for _, binding := range parseAgentHookBlock(hooksBlockField(fm)) {
				id := agentID + ":" + string(binding.Event) + ":" + slugCommand(binding.Command)
				g.Hooks[id] = &HookNode{ID: id, Command: binding.Command, Event: binding.Event, Scope: ScopeAgent}
				g.Edges = append(g.Edges, Edge{Kind: EdgeHookBinding, From: id, To: agentID})
			}
		}
	}
	return nil
}

// --- Skills -------------------------------------------------------------------

// importSkills scans the installed skill directories and builds a SkillNode
// per skill, owner: dreamland only if the file carries DreamlandManagedMarker
// (none of this repo's current openspec-* skills do — they're owner: external).
func importSkills(repoRoot string, g *Graph) error {
	platforms := make([]string, 0, len(platformSkillDirs))
	for p := range platformSkillDirs {
		platforms = append(platforms, p)
	}
	sort.Strings(platforms)

	for _, platform := range platforms {
		dir := filepath.Join(repoRoot, platformSkillDirs[platform])
		entries, err := os.ReadDir(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			id := entry.Name()
			if platform == "antigravity" {
				if _, isAgent := g.Agents[id]; isAgent {
					continue // .agents/skills is shared with agent personas
				}
			}
			if _, exists := g.Skills[id]; exists {
				continue
			}
			content, err := os.ReadFile(filepath.Join(dir, id, "SKILL.md"))
			if err != nil {
				continue
			}
			fm, _, ok := splitFrontmatter(string(content))
			description := ""
			if ok {
				description = frontmatterField(fm, "description")
			}
			owner := OwnerExternal
			if strings.Contains(string(content), DreamlandManagedMarker) {
				owner = OwnerDreamland
			}
			g.Skills[id] = &SkillNode{ID: id, Description: description, Owner: owner}
		}
	}
	return nil
}
