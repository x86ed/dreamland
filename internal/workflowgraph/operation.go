package workflowgraph

import "fmt"

// OpType is a graph mutation operation kind — the same set the interactive
// editor's write routes accept and a `hypnos`-authored plan file (§8) is an
// ordered array of.
type OpType string

const (
	OpCreateNode OpType = "create_node"
	OpUpdateNode OpType = "update_node"
	OpDeleteNode OpType = "delete_node"
	OpCreateEdge OpType = "create_edge"
	OpDeleteEdge OpType = "delete_edge"
)

// NodeKind selects which node type a node operation targets.
type NodeKind string

const (
	NodeKindAgent NodeKind = "agent"
	NodeKindHook  NodeKind = "hook"
	NodeKindSkill NodeKind = "skill"
)

// Operation is one mutation, as accepted by the interactive editor's write
// routes and by a plan file's operation array. Fields are interpreted per
// Type/Kind/EdgeKind — see Apply.
type Operation struct {
	Type OpType `json:"type"`

	// Node operations (create_node/update_node/delete_node).
	Kind        NodeKind `json:"kind,omitempty"`
	ID          string   `json:"id,omitempty"`
	Description string   `json:"description,omitempty"`
	Tier        Tier     `json:"tier,omitempty"`
	PosX        *float64 `json:"posX,omitempty"`
	PosY        *float64 `json:"posY,omitempty"`

	// Edge operations (create_edge/delete_edge).
	EdgeKind EdgeKind `json:"edgeKind,omitempty"`
	From     string   `json:"from,omitempty"` // routing: source agent id; attachment: skill id
	To       string   `json:"to,omitempty"`   // routing: target agent id; attachment: agent id; hookbinding: "project" or an agent id

	// Hook-binding edge operations only — a hook node is derived from
	// (Event, Command), not referenced by a pre-existing id, since hook
	// nodes come into existence via the binding itself.
	Event   HookEvent `json:"event,omitempty"`
	Command string    `json:"command,omitempty"`
}

// ValidateOperations checks every operation's node/edge references resolve —
// against the graph's current state plus any node an earlier operation in
// the same list creates — before any operation is applied. A hook-binding
// edge's "From" is not checked against existing ids, since hook nodes are
// derived from (Event, Command) at apply time, not pre-declared.
func ValidateOperations(g *Graph, ops []Operation) error {
	exists := map[string]bool{"project": true}
	for id := range g.Agents {
		exists[id] = true
	}
	for id := range g.Hooks {
		exists[id] = true
	}
	for id := range g.Skills {
		exists[id] = true
	}

	for i, op := range ops {
		switch op.Type {
		case OpCreateNode:
			if op.ID == "" {
				return fmt.Errorf("operation %d: %s missing id", i, op.Type)
			}
			if exists[op.ID] {
				return fmt.Errorf("operation %d: %s id %q already exists", i, op.Type, op.ID)
			}
			exists[op.ID] = true
		case OpUpdateNode, OpDeleteNode:
			if !exists[op.ID] {
				return fmt.Errorf("operation %d: %s references unknown node id %q", i, op.Type, op.ID)
			}
			if op.Type == OpDeleteNode {
				delete(exists, op.ID)
			}
		case OpCreateEdge, OpDeleteEdge:
			if op.EdgeKind == EdgeHookBinding {
				if !exists[op.To] {
					return fmt.Errorf("operation %d: %s references unknown target id %q", i, op.Type, op.To)
				}
				continue
			}
			if !exists[op.From] {
				return fmt.Errorf("operation %d: %s references unknown source id %q", i, op.Type, op.From)
			}
			if !exists[op.To] {
				return fmt.Errorf("operation %d: %s references unknown target id %q", i, op.Type, op.To)
			}
		default:
			return fmt.Errorf("operation %d: unknown operation type %q", i, op.Type)
		}
	}
	return nil
}

// ApplyOperations validates the whole list first (see ValidateOperations),
// then applies each operation in order against the already-loaded graph,
// stopping at the first runtime error. Returns the number of operations
// successfully applied.
func ApplyOperations(repoRoot string, g *Graph, ops []Operation) (applied int, err error) {
	if err := ValidateOperations(g, ops); err != nil {
		return 0, err
	}
	for i, op := range ops {
		if err := applyOne(repoRoot, g, op); err != nil {
			return i, fmt.Errorf("operation %d (%s): %w", i, op.Type, err)
		}
	}
	return len(ops), nil
}

func applyOne(repoRoot string, g *Graph, op Operation) error {
	switch op.Type {
	case OpCreateNode:
		switch op.Kind {
		case NodeKindAgent:
			return CreateAgent(repoRoot, g, op.ID, op.Description, op.Tier)
		case NodeKindSkill:
			return CreateSkill(repoRoot, g, op.ID, op.Description)
		default:
			return fmt.Errorf("create_node: unsupported kind %q (hook nodes come into existence via create_edge, not directly)", op.Kind)
		}

	case OpUpdateNode:
		if op.Kind != NodeKindAgent {
			return fmt.Errorf("update_node: unsupported kind %q", op.Kind)
		}
		agent, ok := g.Agents[op.ID]
		if !ok {
			return fmt.Errorf("update_node: unknown agent %q", op.ID)
		}
		if op.PosX != nil {
			agent.PosX = *op.PosX
		}
		if op.PosY != nil {
			agent.PosY = *op.PosY
		}
		if op.Description != "" {
			agent.Description = op.Description
			return syncAgent(repoRoot, g, op.ID)
		}
		return nil // position-only update never triggers a platform sync

	case OpDeleteNode:
		switch op.Kind {
		case NodeKindAgent:
			return DeleteAgent(repoRoot, g, op.ID)
		case NodeKindSkill:
			return DeleteSkill(repoRoot, g, op.ID)
		default:
			return fmt.Errorf("delete_node: unsupported kind %q", op.Kind)
		}

	case OpCreateEdge:
		switch op.EdgeKind {
		case EdgeRouting:
			return AddRoutingEdge(repoRoot, g, op.From, op.To)
		case EdgeAttachment:
			return AttachSkill(g, op.To, op.From)
		case EdgeHookBinding:
			if op.To == "project" {
				return AttachHookToProject(repoRoot, g, op.Event, op.Command)
			}
			return AttachHookToAgent(repoRoot, g, op.To, op.Event, op.Command)
		default:
			return fmt.Errorf("create_edge: unknown edge kind %q", op.EdgeKind)
		}

	case OpDeleteEdge:
		switch op.EdgeKind {
		case EdgeRouting:
			return RemoveRoutingEdge(repoRoot, g, op.From, op.To)
		case EdgeAttachment:
			return DetachSkill(g, op.To, op.From)
		case EdgeHookBinding:
			if op.To == "project" {
				return DetachHookFromProject(repoRoot, g, op.Event, op.Command)
			}
			return DetachHookFromAgent(repoRoot, g, op.To, op.Event, op.Command)
		default:
			return fmt.Errorf("delete_edge: unknown edge kind %q", op.EdgeKind)
		}

	default:
		return fmt.Errorf("unknown operation type %q", op.Type)
	}
}
