package workflowgraph

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// platformAgentFilename returns the canonical target filename (or, for
// Antigravity, directory-relative SKILL.md path) for a newly rendered agent
// on platform, given its id. Only used by the writer — the importer keys off
// whatever filename actually exists on disk instead.
func platformAgentFilename(platform, id string) (string, error) {
	switch platform {
	case "claude-code", "kiro":
		return id + ".md", nil
	case "cursor":
		return id + ".mdc", nil
	case "codex":
		return id + ".toml", nil
	case "antigravity":
		return filepath.Join(id, "SKILL.md"), nil
	case "github-copilot":
		return id + ".agent.md", nil
	default:
		return "", fmt.Errorf("unknown platform %q", platform)
	}
}

// installedPlatforms returns the subset of platformAgentDirs whose directory
// actually exists under repoRoot — mutations only touch platforms a repo has
// installed, the same way the importer only imports from them.
func installedPlatforms(repoRoot string) []string {
	var out []string
	for platform, dir := range platformAgentDirs {
		if info, err := os.Stat(filepath.Join(repoRoot, dir)); err == nil && info.IsDir() {
			out = append(out, platform)
		}
	}
	sort.Strings(out)
	return out
}

// routesToFor returns the sorted list of routing targets for agent id, from
// the graph's edges — the source of truth, never read from AgentNode itself.
func routesToFor(g *Graph, id string) []string {
	var targets []string
	for _, e := range g.Edges {
		if e.Kind == EdgeRouting && e.From == id {
			targets = append(targets, e.To)
		}
	}
	sort.Strings(targets)
	return targets
}

// agentHooksFor returns the sorted agent-scoped hook bindings for agent id.
func agentHooksFor(g *Graph, id string) []hookLine {
	var lines []hookLine
	for _, e := range g.Edges {
		if e.Kind != EdgeHookBinding || e.To != id {
			continue
		}
		hook, ok := g.Hooks[e.From]
		if !ok || hook.Scope != ScopeAgent {
			continue
		}
		lines = append(lines, hookLine{Event: hook.Event, Command: hook.Command})
	}
	sort.Slice(lines, func(i, j int) bool {
		if lines[i].Event != lines[j].Event {
			return lines[i].Event < lines[j].Event
		}
		return lines[i].Command < lines[j].Command
	})
	return lines
}

// syncAgent regenerates and writes agent id's file on every platform actually
// installed in repoRoot, deriving RoutesTo/Hooks from the graph's edges (never
// from stale fields on the node itself — the edges are the source of truth).
func syncAgent(repoRoot string, g *Graph, id string) error {
	agent, ok := g.Agents[id]
	if !ok {
		return fmt.Errorf("syncAgent: unknown agent %q", id)
	}
	in := renderInputs{
		Agent:    agent,
		RoutesTo: routesToFor(g, id),
		Hooks:    agentHooksFor(g, id),
	}

	for _, platform := range installedPlatforms(repoRoot) {
		render, ok := renderers[platform]
		if !ok {
			continue
		}
		filename, err := platformAgentFilename(platform, id)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(repoRoot, platformAgentDirs[platform], filename)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(targetPath, []byte(render(in)), 0o644); err != nil {
			return err
		}
		if agent.PlatformFiles == nil {
			agent.PlatformFiles = map[string]string{}
		}
		agent.PlatformFiles[platform] = filepath.Join(platformAgentDirs[platform], filename)
	}
	return nil
}

// CreateAgent adds a new agent node (with the platform's standard
// telemetry/coauthor/commit hook baseline where the platform has one — Claude
// Code and GitHub Copilot only, confirmed against real template files) and
// writes its file on every installed platform.
func CreateAgent(repoRoot string, g *Graph, id, description string, tier Tier) error {
	if _, exists := g.Agents[id]; exists {
		return fmt.Errorf("CreateAgent: agent %q already exists", id)
	}
	g.Agents[id] = &AgentNode{ID: id, Description: description, Tier: tier, PlatformFiles: map[string]string{}}

	installed := installedPlatforms(repoRoot)
	hasClaude, hasCopilot := false, false
	for _, p := range installed {
		if p == "claude-code" {
			hasClaude = true
		}
		if p == "github-copilot" {
			hasCopilot = true
		}
	}
	if hasClaude {
		addHookBaseline(g, id, "claude-code")
	}
	if hasCopilot {
		addHookBaseline(g, id, "github-copilot")
	}

	return syncAgent(repoRoot, g, id)
}

// claudeStopBaseline and copilotBaseline are the standard per-agent hook
// commands every existing agent on that platform carries (checked against
// this repo's shipped templates, e.g. internal/scaffold/templates/agents/
// claude-code/hypnos.md and .../github-copilot/hypnos.agent.md).
var claudeStopBaseline = []string{
	"dreamland coauthor --hook --agent-name %s",
	"dreamland telemetry write --tool claude-code",
	"dreamland version-bump --patch",
	"dreamland version-bump --minor --if-agent janus",
	"dreamland commit --reason handoff --agent-name %s",
}

var copilotSubagentStartBaseline = "dreamland coauthor --agent-name %s"
var copilotSubagentStopBaseline = []string{
	"dreamland coauthor --agent-name %s",
	"dreamland telemetry write --tool github-copilot",
	"dreamland version-bump --patch",
	"dreamland commit --reason handoff --agent-name %s",
}

func addHookBaseline(g *Graph, agentID, platform string) {
	add := func(event HookEvent, commandTemplate string) {
		// Not every baseline command takes --agent-name (e.g. telemetry write,
		// version-bump) — fmt.Sprintf on a template with no %s verb still
		// appends the unused argument as "%!(EXTRA string=...)" to the output,
		// a real bug this repo's own live .claude/agents/hypnos.md hit during
		// manual browser testing (real hand-off sentence corrupted). Only
		// substitute when the template actually has a verb to fill.
		command := commandTemplate
		if strings.Contains(commandTemplate, "%s") {
			command = fmt.Sprintf(commandTemplate, agentID)
		}
		id := agentID + ":" + string(event) + ":" + slugCommand(command)
		g.Hooks[id] = &HookNode{ID: id, Command: command, Event: event, Scope: ScopeAgent}
		g.Edges = append(g.Edges, Edge{Kind: EdgeHookBinding, From: id, To: agentID})
	}
	switch platform {
	case "claude-code":
		for _, tmpl := range claudeStopBaseline {
			add(EventStop, tmpl)
		}
	case "github-copilot":
		add(EventSubagentStart, copilotSubagentStartBaseline)
		for _, tmpl := range copilotSubagentStopBaseline {
			add(EventSubagentStop, tmpl)
		}
	}
}

// DeleteAgent removes agent id's file from every installed platform, removes
// its own agent-scoped hook nodes, and cleans up any routing edges pointing
// at it, re-syncing every source agent whose hand-off sentence changes as a
// result. This implements the verified minimum — outright removal — since no
// existing code (agent-side or mengpo's own process) defines an "archive to a
// separate directory" convention to match; that's a gap to close if/when one
// is established, not assumed here.
func DeleteAgent(repoRoot string, g *Graph, id string) error {
	agent, ok := g.Agents[id]
	if !ok {
		return fmt.Errorf("DeleteAgent: unknown agent %q", id)
	}

	for platform, relPath := range agent.PlatformFiles {
		targetPath := filepath.Join(repoRoot, relPath)
		if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s agent file: %w", platform, err)
		}
		if platform == "antigravity" {
			_ = os.Remove(filepath.Dir(targetPath)) // best-effort: drop the now-empty skill-style dir
		}
	}

	for hookID, hook := range g.Hooks {
		if hook.Scope == ScopeAgent {
			for _, e := range g.Edges {
				if e.Kind == EdgeHookBinding && e.From == hookID && e.To == id {
					delete(g.Hooks, hookID)
					break
				}
			}
		}
	}

	var remaining []Edge
	sourcesToResync := map[string]bool{}
	for _, e := range g.Edges {
		if e.Kind == EdgeHookBinding && e.To == id {
			continue // dropped above with the hook node
		}
		if e.Kind == EdgeRouting && e.To == id {
			sourcesToResync[e.From] = true
			continue
		}
		if e.Kind == EdgeAttachment && e.To == id {
			continue // dangling attachment edge targeting the deleted agent
		}
		if e.From == id {
			continue // any remaining edge sourced from the deleted agent
		}
		remaining = append(remaining, e)
	}
	g.Edges = remaining
	delete(g.Agents, id)

	sources := make([]string, 0, len(sourcesToResync))
	for s := range sourcesToResync {
		sources = append(sources, s)
	}
	sort.Strings(sources)
	for _, s := range sources {
		if err := syncAgent(repoRoot, g, s); err != nil {
			return err
		}
	}
	return nil
}

// AddRoutingEdge connects from -> to and re-syncs from's file(s) so the
// regenerated hand-off sentence takes effect.
func AddRoutingEdge(repoRoot string, g *Graph, from, to string) error {
	if _, ok := g.Agents[from]; !ok {
		return fmt.Errorf("AddRoutingEdge: unknown source agent %q", from)
	}
	if _, ok := g.Agents[to]; !ok {
		return fmt.Errorf("AddRoutingEdge: unknown target agent %q", to)
	}
	for _, e := range g.Edges {
		if e.Kind == EdgeRouting && e.From == from && e.To == to {
			return nil // already present
		}
	}
	g.Edges = append(g.Edges, Edge{Kind: EdgeRouting, From: from, To: to})
	return syncAgent(repoRoot, g, from)
}

// RemoveRoutingEdge disconnects from -> to and re-syncs from's file(s).
func RemoveRoutingEdge(repoRoot string, g *Graph, from, to string) error {
	var remaining []Edge
	found := false
	for _, e := range g.Edges {
		if e.Kind == EdgeRouting && e.From == from && e.To == to {
			found = true
			continue
		}
		remaining = append(remaining, e)
	}
	if !found {
		return nil
	}
	g.Edges = remaining
	return syncAgent(repoRoot, g, from)
}

// --- Skill attach/detach ------------------------------------------------------

// AttachSkill records that agent may invoke skill as an EdgeAttachment. This
// is currently graph-only: no platform's agent file format has a field for
// "skills this agent may invoke" today (checked all six templates), so there
// is nothing to write on the agent's side yet — the edge is the only record.
// Closing that gap (adding such a field, or another representation) is future
// work, not assumed here.
func AttachSkill(g *Graph, agentID, skillID string) error {
	if _, ok := g.Agents[agentID]; !ok {
		return fmt.Errorf("AttachSkill: unknown agent %q", agentID)
	}
	if _, ok := g.Skills[skillID]; !ok {
		return fmt.Errorf("AttachSkill: unknown skill %q", skillID)
	}
	for _, e := range g.Edges {
		if e.Kind == EdgeAttachment && e.From == skillID && e.To == agentID {
			return nil
		}
	}
	g.Edges = append(g.Edges, Edge{Kind: EdgeAttachment, From: skillID, To: agentID})
	return nil
}

// DetachSkill removes the attachment edge. Never touches the skill's own file.
func DetachSkill(g *Graph, agentID, skillID string) error {
	var remaining []Edge
	for _, e := range g.Edges {
		if e.Kind == EdgeAttachment && e.From == skillID && e.To == agentID {
			continue
		}
		remaining = append(remaining, e)
	}
	g.Edges = remaining
	return nil
}

// --- Hook attach/detach --------------------------------------------------------

// claudeEventKeyNames is the reverse of claudeEventKeys (import.go), used to
// render the real settings.json/frontmatter key for an abstracted HookEvent.
var claudeEventKeyNames = map[HookEvent]string{
	EventSessionStart: "SessionStart",
	EventPreToolUse:   "PreToolUse",
	EventPostToolUse:  "PostToolUse",
	EventStop:         "Stop",
	EventSubagentStop: "SubagentStop",
}

// sortedAgentIDs returns g.Agents' keys in deterministic order.
func sortedAgentIDs(g *Graph) []string {
	ids := make([]string, 0, len(g.Agents))
	for id := range g.Agents {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// AttachHookToProject creates (or reuses) a project-scoped HookNode for
// (event, command) and writes it wherever a "project scope" binding actually
// lands per platform: Claude Code's workspace hooks.<Event>[] array (true
// workspace-level mechanism); GitHub Copilot has none, so — per the modified
// litegraph-workflow-editor requirement — it falls back to writing the
// identical binding onto every agent's frontmatter individually, the closest
// available equivalent.
func AttachHookToProject(repoRoot string, g *Graph, event HookEvent, command string) error {
	id := "project:" + string(event) + ":" + slugCommand(command)
	if _, exists := g.Hooks[id]; !exists {
		g.Hooks[id] = &HookNode{ID: id, Command: command, Event: event, Scope: ScopeProject}
		g.Edges = append(g.Edges, Edge{Kind: EdgeHookBinding, From: id, To: "project"})
	}

	installed := installedPlatformSet(repoRoot)
	if installed["claude-code"] {
		eventKey, ok := claudeEventKeyNames[event]
		if !ok {
			return fmt.Errorf("AttachHookToProject: event %q has no Claude Code key", event)
		}
		if err := mutateClaudeSettingsHooks(repoRoot, func(hooks map[string][]any) {
			hooks[eventKey] = upsertUnscopedCommand(hooks[eventKey], command)
		}); err != nil {
			return err
		}
	}
	if installed["github-copilot"] {
		for _, agentID := range sortedAgentIDs(g) {
			if err := AttachHookToAgent(repoRoot, g, agentID, event, command); err != nil {
				return err
			}
		}
	}
	return nil
}

// DetachHookFromProject removes a project-scoped hook node/edge and its entry
// wherever AttachHookToProject would have written it — including the
// per-agent GitHub Copilot fallback.
func DetachHookFromProject(repoRoot string, g *Graph, event HookEvent, command string) error {
	id := "project:" + string(event) + ":" + slugCommand(command)
	delete(g.Hooks, id)
	var remaining []Edge
	for _, e := range g.Edges {
		if e.Kind == EdgeHookBinding && e.From == id {
			continue
		}
		remaining = append(remaining, e)
	}
	g.Edges = remaining

	installed := installedPlatformSet(repoRoot)
	if installed["claude-code"] {
		if eventKey, ok := claudeEventKeyNames[event]; ok {
			if err := mutateClaudeSettingsHooks(repoRoot, func(hooks map[string][]any) {
				hooks[eventKey] = removeUnscopedCommand(hooks[eventKey], command)
			}); err != nil {
				return err
			}
		}
	}
	if installed["github-copilot"] {
		for _, agentID := range sortedAgentIDs(g) {
			if err := DetachHookFromAgent(repoRoot, g, agentID, event, command); err != nil {
				return err
			}
		}
	}
	return nil
}

// AttachHookToAgent creates (or reuses) an agent-scoped HookNode and re-syncs
// the agent's file(s) so the regenerated hooks block includes it. Only valid
// on Claude Code/GitHub Copilot — the platforms with a per-agent hook
// mechanism at all.
func AttachHookToAgent(repoRoot string, g *Graph, agentID string, event HookEvent, command string) error {
	if _, ok := g.Agents[agentID]; !ok {
		return fmt.Errorf("AttachHookToAgent: unknown agent %q", agentID)
	}
	id := agentID + ":" + string(event) + ":" + slugCommand(command)
	if _, exists := g.Hooks[id]; !exists {
		g.Hooks[id] = &HookNode{ID: id, Command: command, Event: event, Scope: ScopeAgent}
		g.Edges = append(g.Edges, Edge{Kind: EdgeHookBinding, From: id, To: agentID})
	}
	return syncAgent(repoRoot, g, agentID)
}

// DetachHookFromAgent removes an agent-scoped hook node/edge and re-syncs.
func DetachHookFromAgent(repoRoot string, g *Graph, agentID string, event HookEvent, command string) error {
	id := agentID + ":" + string(event) + ":" + slugCommand(command)
	delete(g.Hooks, id)
	var remaining []Edge
	for _, e := range g.Edges {
		if e.Kind == EdgeHookBinding && e.From == id {
			continue
		}
		remaining = append(remaining, e)
	}
	g.Edges = remaining
	return syncAgent(repoRoot, g, agentID)
}

func installedPlatformSet(repoRoot string) map[string]bool {
	set := map[string]bool{}
	for _, p := range installedPlatforms(repoRoot) {
		set[p] = true
	}
	return set
}

// mutateClaudeSettingsHooks reads .claude/settings.json, applies fn to its
// "hooks" object (creating it if absent), and writes the result back
// atomically (temp file + rename, matching internal/scaffold's
// atomicJSONMerge safety). This is a direct read-modify-write, not a reuse of
// atomicJSONMerge — that function only additively merges a static patch, with
// no removal path, which detach genuinely needs.
func mutateClaudeSettingsHooks(repoRoot string, fn func(hooks map[string][]any)) error {
	target := filepath.Join(repoRoot, ".claude", "settings.json")

	var doc map[string]any
	raw, err := os.ReadFile(target)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		doc = map[string]any{}
	} else if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("parse %s: %w", target, err)
	}

	hooksRaw, _ := doc["hooks"].(map[string]any)
	if hooksRaw == nil {
		hooksRaw = map[string]any{}
	}
	hooks := map[string][]any{}
	for k, v := range hooksRaw {
		if arr, ok := v.([]any); ok {
			hooks[k] = arr
		}
	}

	fn(hooks)

	newHooksRaw := make(map[string]any, len(hooks))
	for k, v := range hooks {
		if len(v) == 0 {
			continue // an event key left with no bindings is dropped, not written as null/[]
		}
		newHooksRaw[k] = v
	}
	doc["hooks"] = newHooksRaw

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), ".dreamland-tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(append(out, '\n')); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, target)
}

// upsertUnscopedCommand ensures binding.hooks[] under the unscoped
// (matcher == "") entry of arr includes {type:"command", command}, adding an
// unscoped entry if none exists yet.
func upsertUnscopedCommand(arr []any, command string) []any {
	for _, item := range arr {
		binding, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if matcher, _ := binding["matcher"].(string); matcher != "" {
			continue
		}
		hooks, _ := binding["hooks"].([]any)
		for _, h := range hooks {
			if hm, ok := h.(map[string]any); ok {
				if c, _ := hm["command"].(string); c == command {
					return arr // already present
				}
			}
		}
		binding["hooks"] = append(hooks, map[string]any{"type": "command", "command": command})
		return arr
	}
	return append(arr, map[string]any{
		"matcher": "",
		"hooks":   []any{map[string]any{"type": "command", "command": command}},
	})
}

// removeUnscopedCommand removes command from every unscoped binding's hooks[]
// in arr, dropping any binding left with an empty hooks[] as a result.
func removeUnscopedCommand(arr []any, command string) []any {
	var out []any
	for _, item := range arr {
		binding, ok := item.(map[string]any)
		if !ok {
			out = append(out, item)
			continue
		}
		if matcher, _ := binding["matcher"].(string); matcher != "" {
			out = append(out, item)
			continue
		}
		hooks, _ := binding["hooks"].([]any)
		var remaining []any
		for _, h := range hooks {
			if hm, ok := h.(map[string]any); ok {
				if c, _ := hm["command"].(string); c == command {
					continue
				}
			}
			remaining = append(remaining, h)
		}
		if len(remaining) == 0 {
			continue // drop the now-empty binding entirely
		}
		binding["hooks"] = remaining
		out = append(out, binding)
	}
	return out
}
