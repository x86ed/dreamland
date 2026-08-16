// Package workflowgraph defines the node/edge model rendered by `dreamland hypnos-serve`
// and mutated by /hypnos-interactive and `--mode=apply-plan`. The graph is a live-reloaded
// cache derived from a repository's installed platform files (see design.md's "The six live
// platform files are canonical" decision) — never a persisted source of truth.
package workflowgraph

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Tier mirrors agent-scaffolding's three tool-binding tiers. Empty on platforms
// that don't represent tier at all (Cursor, Kiro, Antigravity).
type Tier string

const (
	TierRouterExcluded  Tier = "router-excluded"
	TierFullEdit        Tier = "full-edit"
	TierWriteOnlyNoEdit Tier = "write-only-no-edit"
)

// HookEvent is a hook trigger point. Named close to each platform's own event
// keys rather than collapsed into a smaller abstracted set — confirmed by reading
// this repo's own `.claude/settings.json`, which has five distinct workspace-level
// event keys (PostToolUse, PreToolUse, SessionStart, Stop, SubagentStop); Stop and
// SubagentStop are genuinely different events (whole-session end vs. one subagent's
// turn end) and must not be collapsed into a single bucket, or the writer can't
// round-trip which real key a binding belongs under. GitHub Copilot's frontmatter
// hooks use SubagentStart/SubagentStop; Claude Code's per-agent frontmatter uses
// only Stop.
type HookEvent string

const (
	EventSessionStart  HookEvent = "session_start"
	EventPreToolUse    HookEvent = "pre_tool_use"
	EventPostToolUse   HookEvent = "post_tool_use"
	EventStop          HookEvent = "stop"           // Claude Code: Stop
	EventSubagentStart HookEvent = "subagent_start" // GitHub Copilot: SubagentStart
	EventSubagentStop  HookEvent = "subagent_stop"  // Claude Code/Copilot: SubagentStop
)

// HookScope distinguishes a workspace-wide binding (wired to the ProjectNode) from
// one scoped to a single agent's own frontmatter.
type HookScope string

const (
	ScopeProject HookScope = "project"
	ScopeAgent   HookScope = "agent"
)

// SkillOwner distinguishes a skill file dreamland can regenerate from one owned by
// another tool (e.g. the openspec CLI's openspec-* skills), which the graph may only
// attach/detach, never create/edit/delete.
type SkillOwner string

const (
	OwnerDreamland SkillOwner = "dreamland"
	OwnerExternal  SkillOwner = "external"
)

// EdgeKind is the litegraph typed-slot kind an edge belongs to. Connections are only
// valid within one kind — a routing edge can't land on a hookbinding input, etc.
type EdgeKind string

const (
	EdgeRouting     EdgeKind = "routing"     // Agent.routes_to -> Agent.routed_from
	EdgeHookBinding EdgeKind = "hookbinding" // Hook.bound_to -> Project.hooks | Agent.hooks
	EdgeAttachment  EdgeKind = "attachment"  // Skill.available_to -> Agent.skills
)

// ProjectNode is the singleton representing the repository's workspace scope — the
// target for project-level (workspace-wide) hook bindings.
type ProjectNode struct {
	RepoRoot string `json:"repoRoot"`
}

// AgentNode represents one installed agent.
type AgentNode struct {
	ID                string            `json:"id"`
	Description       string            `json:"description"`
	Tier              Tier              `json:"tier,omitempty"`
	Role              string            `json:"role,omitempty"`         // frontmatter `role:` field, e.g. "router" (currently only janus)
	BroadRouting      bool              `json:"broadRouting"`           // dispatches dynamically to any agent rather than a fixed set (currently only iktomi)
	InstructionBody   string            `json:"instructionBody"`
	PlatformFiles     map[string]string `json:"platformFiles,omitempty"` // platform -> file path
	PosX              float64           `json:"posX"`
	PosY              float64           `json:"posY"`
	UnresolvedRouting bool              `json:"unresolvedRouting,omitempty"`
}

// HookNode represents one distinct (command, event) binding — not one per raw command
// string. The same command bound both at project scope and per-agent scope (e.g.
// `telemetry write` in this repo today) imports as two separate HookNodes.
type HookNode struct {
	ID      string    `json:"id"`
	Command string    `json:"command"`
	Event   HookEvent `json:"event"`
	Scope   HookScope `json:"scope"`
}

// SkillNode represents one installed skill directory.
type SkillNode struct {
	ID          string     `json:"id"`
	Description string     `json:"description"`
	Owner       SkillOwner `json:"owner"`
}

// Edge is a directed, typed connection between two node IDs.
type Edge struct {
	Kind EdgeKind `json:"kind"`
	From string   `json:"from"`
	To   string   `json:"to"`
}

// Graph is the full node/edge model for one repository, as rendered by the litegraph UI.
type Graph struct {
	Project *ProjectNode          `json:"project"`
	Agents  map[string]*AgentNode `json:"agents"`
	Hooks   map[string]*HookNode  `json:"hooks"`
	Skills  map[string]*SkillNode `json:"skills"`
	Edges   []Edge                `json:"edges"`
}

// New returns an empty graph with the singleton ProjectNode set.
func New(repoRoot string) *Graph {
	return &Graph{
		Project: &ProjectNode{RepoRoot: repoRoot},
		Agents:  map[string]*AgentNode{},
		Hooks:   map[string]*HookNode{},
		Skills:  map[string]*SkillNode{},
	}
}

// AgentPosition is one agent's saved canvas position — the only per-agent
// state with no representation in any platform file (nothing in a .md/.toml/
// SKILL.md carries "where this agent sits on a diagram"), and so the only
// thing worth persisting locally at all. Everything else the graph renders
// (agents, hooks, skills, edges) is always freshly re-derivable from the six
// live platform directories — see Import — so there's nothing else to cache.
type AgentPosition struct {
	PosX float64 `json:"posX"`
	PosY float64 `json:"posY"`
}

// SavePositions writes every agent's current position to path, creating the
// parent directory if absent.
func SavePositions(path string, g *Graph) error {
	positions := make(map[string]AgentPosition, len(g.Agents))
	for id, a := range g.Agents {
		positions[id] = AgentPosition{PosX: a.PosX, PosY: a.PosY}
	}
	data, err := json.MarshalIndent(positions, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal positions: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create positions dir: %w", err)
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// LoadPositions reads a previously-saved position map. Absence is not an
// error — a missing file just means no agent has been repositioned yet.
func LoadPositions(path string) (map[string]AgentPosition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read positions: %w", err)
	}
	var positions map[string]AgentPosition
	if err := json.Unmarshal(data, &positions); err != nil {
		return nil, fmt.Errorf("parse positions: %w", err)
	}
	return positions, nil
}
