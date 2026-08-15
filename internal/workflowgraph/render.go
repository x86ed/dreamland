package workflowgraph

import (
	"fmt"
	"sort"
	"strings"
)

// hookLine is one (event, command) binding rendered into an agent's frontmatter.
type hookLine struct {
	Event   HookEvent
	Command string
}

// renderInputs is everything a per-platform renderer needs to produce one
// agent's full file content — the AgentNode itself plus the graph-derived
// facts (routing targets, agent-scoped hook bindings) that aren't stored on
// the node so they can't drift from the edges that are the source of truth.
type renderInputs struct {
	Agent    *AgentNode
	RoutesTo []string   // resolved target agent ids, sorted
	Hooks    []hookLine // this agent's own agent-scoped hook bindings, sorted by event then command
}

// handOffSentences renders one canonical, re-parseable hand-off sentence per
// routing target — the same pattern importRoutesTo's handOffPattern parses
// back on the next live rebuild.
func handOffSentences(targets []string) string {
	if len(targets) == 0 {
		return ""
	}
	sorted := append([]string(nil), targets...)
	sort.Strings(sorted)
	var sentences []string
	for _, t := range sorted {
		sentences = append(sentences, fmt.Sprintf("Once your work is complete, hand off directly to `%s`.", t))
	}
	return strings.Join(sentences, " ")
}

// bodyWithHandOffs appends the rendered hand-off sentences to an agent's
// stored instruction body (which never itself contains them — see the
// "structured routes_to" design decision).
func bodyWithHandOffs(in renderInputs) string {
	body := strings.TrimSpace(in.Agent.InstructionBody)
	sentences := handOffSentences(in.RoutesTo)
	if sentences == "" {
		return body
	}
	if body == "" {
		return sentences
	}
	return body + "\n\n" + sentences
}

// claudeToolsField renders a Tier into Claude Code's comma-separated tools:
// value, per agent-scaffolding's tier matrix.
func claudeToolsField(t Tier) string {
	switch t {
	case TierFullEdit:
		return "Read, Edit, Write, Bash"
	case TierWriteOnlyNoEdit:
		return "Write, Read, Bash"
	case TierRouterExcluded:
		return "Read, Bash"
	default:
		return "Read, Edit, Write, Bash" // no tier info (e.g. newly created without one) defaults to full-edit
	}
}

// copilotToolsField renders a Tier into GitHub Copilot's bracketed tools: value.
func copilotToolsField(t Tier) string {
	return "[" + claudeToolsField(t) + "]"
}

// eventGroup is one event's ordered list of bound commands, derived from
// hookLine entries that share an event (agentHooksFor already sorts by event
// then command, so grouping by first-seen order is stable).
type eventGroup struct {
	event    HookEvent
	commands []string
}

func groupHooksByEvent(hooks []hookLine) []eventGroup {
	var groups []eventGroup
	index := map[HookEvent]int{}
	for _, h := range hooks {
		if i, ok := index[h.Event]; ok {
			groups[i].commands = append(groups[i].commands, h.Command)
			continue
		}
		index[h.Event] = len(groups)
		groups = append(groups, eventGroup{event: h.Event, commands: []string{h.Command}})
	}
	return groups
}

// claudeHooksBlock renders Claude Code's frontmatter hooks: block from the
// agent's actual bound hooks — never a hardcoded baseline, so an attached
// custom hook (or a baseline command someone detaches) is reflected exactly.
func claudeHooksBlock(hooks []hookLine) string {
	if len(hooks) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("hooks:\n")
	for _, g := range groupHooksByEvent(hooks) {
		key, ok := claudeEventKeyNames[g.event]
		if !ok {
			continue // event has no Claude Code frontmatter key (e.g. session_start isn't per-agent there)
		}
		fmt.Fprintf(&b, "  %s:\n    - hooks:\n", key)
		for _, cmd := range g.commands {
			fmt.Fprintf(&b, "        - type: command\n          command: %s\n", cmd)
		}
	}
	return b.String()
}

// copilotEventKeyNames maps a HookEvent to GitHub Copilot's frontmatter key —
// includes SubagentStart, which Claude Code has no per-agent equivalent for.
var copilotEventKeyNames = map[HookEvent]string{
	EventSubagentStart: "SubagentStart",
	EventSubagentStop:  "SubagentStop",
}

// copilotHooksBlock renders GitHub Copilot's frontmatter hooks: block from
// the agent's actual bound hooks. Copilot's per-event arrays are flat
// {type,command} lists (no nested "- hooks:" wrapper, unlike Claude Code) —
// matches internal/scaffold/templates/agents/github-copilot/hypnos.agent.md.
func copilotHooksBlock(hooks []hookLine) string {
	if len(hooks) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("hooks:\n")
	for _, g := range groupHooksByEvent(hooks) {
		key, ok := copilotEventKeyNames[g.event]
		if !ok {
			continue
		}
		fmt.Fprintf(&b, "  %s:\n", key)
		for _, cmd := range g.commands {
			fmt.Fprintf(&b, "    - type: command\n      command: %s\n", cmd)
		}
	}
	return b.String()
}

// renderClaudeCode returns the full .claude/agents/<id>.md content.
func renderClaudeCode(in renderInputs) string {
	a := in.Agent
	var fm strings.Builder
	fmt.Fprintf(&fm, "---\nname: %s\ndescription: %s\ntools: %s\n", a.ID, a.Description, claudeToolsField(a.Tier))
	fm.WriteString(claudeHooksBlock(in.Hooks))
	fm.WriteString("---\n\n")
	fm.WriteString(bodyWithHandOffs(in))
	fm.WriteString("\n")
	return fm.String()
}

// renderCursor returns the full .cursor/rules/<id>.mdc content. Cursor has no
// tools/hooks frontmatter — confirmed against the real template files.
func renderCursor(in renderInputs) string {
	a := in.Agent
	var b strings.Builder
	fmt.Fprintf(&b, "---\ndescription: %s\nalwaysApply: false\n---\n\n", a.Description)
	b.WriteString(bodyWithHandOffs(in))
	b.WriteString("\n")
	return b.String()
}

// renderCodex returns the full .codex/agents/<id>.toml content.
func renderCodex(in renderInputs) string {
	a := in.Agent
	var b strings.Builder
	fmt.Fprintf(&b, "[agent]\nname = %q\ndescription = %q\ndeveloper_instructions = \"\"\"\n%s\n\"\"\"\n",
		a.ID, a.Description, bodyWithHandOffs(in))
	return b.String()
}

// renderKiro returns the full .kiro/steering/<id>.md content.
func renderKiro(in renderInputs) string {
	a := in.Agent
	var b strings.Builder
	fmt.Fprintf(&b, "---\nname: %s\ndescription: %s\ninclusion: always\n---\n\n# %s\n\n", a.ID, a.Description, titleCase(a.ID))
	b.WriteString(bodyWithHandOffs(in))
	b.WriteString("\n")
	return b.String()
}

// renderAntigravity returns the full .agents/skills/<id>/SKILL.md content.
func renderAntigravity(in renderInputs) string {
	a := in.Agent
	var b strings.Builder
	fmt.Fprintf(&b, "---\nname: %s\ndescription: %s\n---\n\n", a.ID, a.Description)
	b.WriteString(bodyWithHandOffs(in))
	b.WriteString("\n")
	return b.String()
}

// renderGitHubCopilot returns the full .github/agents/<id>.agent.md content.
func renderGitHubCopilot(in renderInputs) string {
	a := in.Agent
	var fm strings.Builder
	fmt.Fprintf(&fm, "---\nname: %s\ndescription: %s\ntools: %s\n", a.ID, a.Description, copilotToolsField(a.Tier))
	fm.WriteString(copilotHooksBlock(in.Hooks))
	fm.WriteString("---\n\n")
	fm.WriteString(bodyWithHandOffs(in))
	fm.WriteString("\n")
	return fm.String()
}

func titleCase(id string) string {
	parts := strings.Split(id, "-")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}

// renderers maps a platform key to its file-content renderer.
var renderers = map[string]func(renderInputs) string{
	"claude-code":    renderClaudeCode,
	"cursor":         renderCursor,
	"codex":          renderCodex,
	"kiro":           renderKiro,
	"antigravity":    renderAntigravity,
	"github-copilot": renderGitHubCopilot,
}
