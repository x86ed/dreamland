// Package workflowgraph defines the node/edge model rendered by `dreamland hypnos-serve`
// and mutated by /hypnos-interactive and `--mode=apply-plan`. The graph is a live-reloaded
// cache derived from a repository's installed platform files (see design.md's "The six live
// platform files are canonical" decision) — never a persisted source of truth.
package workflowgraph

import (
	"encoding/json"
	"fmt"
	"os"
)

// Tier mirrors agent-scaffolding's three tool-binding tiers. Empty on platforms
// that don't represent tier at all (Cursor, Kiro, Antigravity).
type Tier string

const (
	TierRouterExcluded  Tier = "router-excluded"
	TierFullEdit        Tier = "full-edit"
	TierWriteOnlyNoEdit Tier = "write-only-no-edit"
)

// HookEvent is a platform-agnostic hook trigger point, mapped to each platform's
// real event name by the per-platform writer (Claude Code's Stop/SubagentStop/
// PreToolUse/SessionStart, GitHub Copilot's SubagentStart/SubagentStop).
type HookEvent string

const (
	EventTurnEnd      HookEvent = "turn_end"
	EventTurnStart    HookEvent = "turn_start"
	EventPreToolUse   HookEvent = "pre_tool_use"
	EventSessionStart HookEvent = "session_start"
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
	BroadRouting      bool              `json:"broadRouting"`
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

// Load reads the graph cache at path. Absence is not an error — callers should treat
// a missing cache as "rebuild from disk," per the live-reload design (the cache is a
// fast-reload convenience, never a required source of truth).
func Load(path string) (*Graph, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read graph cache: %w", err)
	}
	var g Graph
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, fmt.Errorf("parse graph cache: %w", err)
	}
	return &g, nil
}

// Save writes the graph cache to path, creating it if absent.
func Save(path string, g *Graph) error {
	data, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal graph cache: %w", err)
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
