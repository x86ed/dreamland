package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// toolTierGrant describes one tool-tier row of the agent-scaffolding capability's
// tool-binding matrix: Read and Bash are granted on every tier, Edit/Write vary.
type toolTierGrant struct {
	Edit  bool
	Write bool
}

// toolTierGrants maps the four accepted --tool-tier values (oneiroi-seed-naming
// capability) to their Edit/Write grant, mirroring the fixed ten's per-role matrix.
var toolTierGrants = map[string]toolTierGrant{
	"router":              {Edit: false, Write: false},
	"read-dispatch-only":  {Edit: false, Write: false},
	"full-edit":           {Edit: true, Write: true},
	"write-only-no-edit":  {Edit: false, Write: true},
}

// claudeCodeTools renders a tier's grant as a Claude Code frontmatter `tools:` value,
// in the same Read, Edit, Write, Bash ordering the fixed ten's templates use.
func claudeCodeTools(g toolTierGrant) string {
	tools := []string{"Read"}
	if g.Edit {
		tools = append(tools, "Edit")
	}
	if g.Write {
		tools = append(tools, "Write")
	}
	tools = append(tools, "Bash")
	return strings.Join(tools, ", ")
}

// githubCopilotTools renders a tier's grant as a GitHub Copilot frontmatter `tools:`
// list, including the `agent` grant every dispatching agent carries.
func githubCopilotTools(g toolTierGrant) string {
	return "[" + claudeCodeTools(g) + ", agent]"
}

// stubPlaceholderBody is the instruction-body placeholder every stub carries in place of
// persona prose (design.md decision 5), naming hypnos as the next editor.
func stubPlaceholderBody(name string) string {
	return fmt.Sprintf("<!-- TODO(hypnos): persona-specific instructions for %s -->\n", name)
}

// InstallAgentStub renders and writes a stub agent template file for name at the
// standard per-platform agent-file path, for all six supported platforms, per the
// agent-scaffolding capability's "Per-platform installers accept an externally supplied
// agent name and tool tier" requirement. Unlike the fixed ten (installFlatAgents/
// installSkills, which render from hand-authored embedded templates), the stub's
// content — frontmatter/hook block substituted with name, instruction body a
// placeholder — is generated here; the fixed ten's install path is unaffected.
func InstallAgentStub(cfg Config, name, role, toolTier string) ([]Result, error) {
	grant, ok := toolTierGrants[toolTier]
	if !ok {
		return nil, fmt.Errorf("unknown tool tier %q", toolTier)
	}

	type target struct {
		platform string
		path     string
		content  []byte
	}

	targets := []target{
		{
			platform: "Claude Code",
			path:     filepath.Join(cfg.RepoRoot, ".claude", "agents", name+".md"),
			content:  renderClaudeCodeStub(name, role, grant),
		},
		{
			platform: "Codex CLI",
			path:     filepath.Join(cfg.RepoRoot, ".codex", "agents", name+".toml"),
			content:  renderCodexStub(name, role),
		},
		{
			platform: "Cursor",
			path:     filepath.Join(cfg.RepoRoot, ".cursor", "rules", name+".mdc"),
			content:  renderCursorStub(name, role),
		},
		{
			platform: "Kiro",
			path:     filepath.Join(cfg.RepoRoot, ".kiro", "steering", name+".md"),
			content:  renderKiroStub(name, role),
		},
		{
			platform: "Antigravity",
			path:     filepath.Join(cfg.RepoRoot, ".agents", "skills", name, "SKILL.md"),
			content:  renderAntigravityStub(name, role),
		},
		{
			platform: "GitHub Copilot",
			path:     filepath.Join(cfg.RepoRoot, ".github", "agents", name+".agent.md"),
			content:  renderGitHubCopilotStub(name, role, grant),
		},
	}

	var results []Result
	for _, t := range targets {
		if _, err := os.Stat(t.path); err == nil && !cfg.Force {
			results = append(results, Result{Path: t.path, Action: "skipped (already exists)"})
			continue
		}
		if err := os.MkdirAll(filepath.Dir(t.path), 0o755); err != nil {
			return results, fmt.Errorf("create %s dir: %w", t.platform, err)
		}
		if err := os.WriteFile(t.path, t.content, 0o644); err != nil {
			return results, err
		}
		results = append(results, Result{Path: t.path, Action: "installed"})
	}
	return results, nil
}

func renderClaudeCodeStub(name, role string, grant toolTierGrant) []byte {
	return []byte(fmt.Sprintf(`---
name: %s
description: %s
tools: %s
hooks:
  Stop:
    - hooks:
        - type: command
          command: dreamland coauthor --hook --agent-name %s
        - type: command
          command: dreamland telemetry write --tool claude-code --agent-name %s
        - type: command
          command: dreamland version-bump --patch
        - type: command
          command: dreamland version-bump --minor --if-agent janus
        - type: command
          command: dreamland commit --reason handoff --agent-name %s
---

You are the %s agent for this repository's spec-driven AI development workflow.

%s`, name, role, claudeCodeTools(grant), name, name, name, name, stubPlaceholderBody(name)))
}

func renderCodexStub(name, role string) []byte {
	return []byte(fmt.Sprintf(`[agent]
name = "%s"
description = "%s"
developer_instructions = """
You are the %s agent for this repository's spec-driven AI development workflow.

%s"""
`, name, role, name, stubPlaceholderBody(name)))
}

func renderCursorStub(name, role string) []byte {
	return []byte(fmt.Sprintf(`---
description: %s
alwaysApply: false
---

You are the %s agent for this repository's spec-driven AI development workflow.

%s`, role, name, stubPlaceholderBody(name)))
}

func renderKiroStub(name, role string) []byte {
	return []byte(fmt.Sprintf(`---
name: %s
description: %s
inclusion: always
---

# %s

You are the %s agent for this repository's spec-driven AI development workflow.

%s`, name, role, name, name, stubPlaceholderBody(name)))
}

func renderAntigravityStub(name, role string) []byte {
	return []byte(fmt.Sprintf(`---
name: %s
description: %s
---

You are the %s agent for this repository's spec-driven AI development workflow.

%s`, name, role, name, stubPlaceholderBody(name)))
}

func renderGitHubCopilotStub(name, role string, grant toolTierGrant) []byte {
	return []byte(fmt.Sprintf(`---
name: %s
description: %s
tools: %s
agents: [janus]
hooks:
  SubagentStart:
    - type: command
      command: dreamland coauthor --agent-name %s
  SubagentStop:
    - type: command
      command: dreamland coauthor --agent-name %s
    - type: command
      command: dreamland telemetry write --tool github-copilot
    - type: command
      command: dreamland version-bump --patch
    - type: command
      command: dreamland commit --reason handoff --agent-name %s
---

You are the %s agent for this repository's spec-driven AI development workflow.

%s`, name, role, githubCopilotTools(grant), name, name, name, name, stubPlaceholderBody(name)))
}

// InstallAgentCommand renders and writes a per-agent direct-invoke slash command file
// for name, following the same per-platform target-path/frontmatter conventions the
// fixed nine agents use (see the router-slash-commands capability), for all six
// supported platforms.
func InstallAgentCommand(cfg Config, name string) ([]Result, error) {
	display := strings.ToUpper(name[:1]) + name[1:]
	description := fmt.Sprintf("Route directly to the %s agent via Janus", display)
	body := fmt.Sprintf("Delegate this request to the `janus` agent with an explicit instruction: route directly to `%s`, overriding Janus's own judgment about which agent fits. Janus still performs the hand-off (including its normal identity/telemetry steps) — this command just fixes the destination.\n", name)

	type target struct {
		platform string
		path     string
		content  []byte
	}

	targets := []target{
		{
			platform: "Claude Code",
			path:     filepath.Join(cfg.RepoRoot, ".claude", "commands", "drmlnd", name+".md"),
			content: []byte(fmt.Sprintf(`---
name: "%s"
description: %s
category: Workflow
tags: [workflow, routing]
---

%s`, display, description, body)),
		},
		{
			platform: "Cursor",
			path:     filepath.Join(cfg.RepoRoot, ".cursor", "commands", name+".md"),
			content: []byte(fmt.Sprintf(`---
name: drmlnd-%s
description: %s
---

# %s

%s`, name, description, display, body)),
		},
		{
			platform: "GitHub Copilot",
			path:     filepath.Join(cfg.RepoRoot, ".github", "prompts", name+".prompt.md"),
			content: []byte(fmt.Sprintf(`---
description: %s
name: drmlnd-%s
agent: janus
---

%s`, description, name, body)),
		},
		{
			platform: "Kiro",
			path:     filepath.Join(cfg.RepoRoot, ".kiro", "steering", "drmlnd-"+name+".md"),
			content: []byte(fmt.Sprintf(`---
name: drmlnd-%s
description: %s
inclusion: manual
---

# %s

%s`, name, description, display, body)),
		},
		{
			platform: "Antigravity",
			path:     filepath.Join(cfg.RepoRoot, ".agents", "skills", "drmlnd-"+name+".md"),
			content: []byte(fmt.Sprintf(`---
name: drmlnd-%s
description: %s
---

%s`, name, description, body)),
		},
		{
			platform: "Codex CLI",
			path:     filepath.Join(cfg.RepoRoot, ".codex", "skills", "drmlnd-"+name, "SKILL.md"),
			content: []byte(fmt.Sprintf(`---
name: drmlnd-%s
description: %s
---

%s`, name, description, body)),
		},
	}

	var results []Result
	for _, t := range targets {
		if err := os.MkdirAll(filepath.Dir(t.path), 0o755); err != nil {
			return results, fmt.Errorf("create %s commands dir: %w", t.platform, err)
		}
		if err := os.WriteFile(t.path, t.content, 0o644); err != nil {
			return results, err
		}
		results = append(results, Result{Path: t.path, Action: "installed"})
	}
	return results, nil
}
