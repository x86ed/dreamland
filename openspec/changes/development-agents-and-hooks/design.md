## Context

Dreamland is a Go CLI (`cobra` + `huh`) that scaffolds and drives a spec-driven AI development workflow. The `init` wizard today collects tool/language/command settings and writes `.dreamland.json`. Once initialized, there is no automation: no agents to execute workflow steps, no hooks to enforce lifecycle events. Developers who want agents or hooks must configure them manually and differently per platform.

All six supported coding tools have native agent and hook conventions. A key finding from documentation research: all platforms share an equivalent "end of agent turn" event (referred to as `Stop`, `stop`, or `Agent Stop` depending on platform). This is the event that fires once the AI finishes generating its response and all tool calls in that turn complete — analogous to a function returning. **All four dreamland hooks bind exclusively to this event**, with conditional logic inside each script determining whether to act. This keeps platform bindings trivially consistent: one event per platform, no per-hook event variation.

| Platform | Agent path & format | Hook config path | End-of-turn event name |
| --- | --- | --- | --- |
| Claude Code | `.claude/agents/*.md` (YAML frontmatter + prompt) | `.claude/settings.json` → `hooks.Stop` | `Stop` |
| Codex CLI | `.codex/agents/*.toml` (`name`, `description`, `developer_instructions`) | `.codex/hooks.json` → `stop` | `Stop` |
| Cursor | `.cursor/rules/*.mdc` (YAML frontmatter: `alwaysApply`, `description`, `globs`) | `.cursor/hooks.json` → `stop` | `stop` |
| Kiro | `.kiro/steering/*.md` (markdown with optional `inclusion` frontmatter) | `.kiro/agent.json` → `hooks.stop` (CLI agent config) | `stop` |
| Antigravity | `agents/*.md` inside plugin bundle `~/.gemini/antigravity-cli/plugins/dreamland/` | `hooks.json` inside plugin bundle → `Stop` | `Stop` |
| GitHub Copilot | `.github/copilot-agents/*.instructions.md` | `.github/copilot-hooks/hooks-manifest.json` (stub — schema not public) | unknown |

**Kiro note**: Kiro has two hook systems — the IDE visual hooks (`.kiro/hooks/` YAML with `trigger`/`instructions` fields, designed for agent-prompt actions) and the CLI agent config hooks (JSON with `agentSpawn`/`userPromptSubmit`/`preToolUse`/`postToolUse`/`stop` events, designed for shell commands). Dreamland uses the CLI config approach since our hooks are shell scripts, not natural-language agent prompts. The CLI hook schema matches Claude Code's event model closely; the tool matcher for bash is `execute_bash` rather than `Bash`.

## Goals / Non-Goals

**Goals:**

- Embed all agent and hook templates in the binary (zero disk reads at install time)
- Install five agent definition files per coding tool after `dreamland init`, in the platform's native format
- Install four hook scripts that all bind to the platform's end-of-turn event; conditional logic lives inside the scripts
- Add a repo-root step to the `init` wizard (new step 1, defaults to detected git root)
- Expand coding tool list to include Codex CLI and Cursor
- Keep hook business logic and platform bindings structurally identical; only event name strings differ

**Non-Goals:**

- Runtime execution of agents or hooks (dreamland only scaffolds them)
- Updating existing agent/hook files when `dreamland init` is re-run without `--force`
- Generating agent prompts dynamically from project state
- Full Antigravity plugin lifecycle management (dreamland writes the files; the user registers the plugin separately)
- Supporting Kiro IDE visual hooks (`.kiro/hooks/` YAML) — those are agent-prompt hooks, not shell-command hooks

## Decisions

### 1. Go `embed` FS for all templates

All templates live under `internal/scaffold/templates/` and are embedded with `//go:embed all:templates`. The scaffold package exposes a single `fs.FS` reference; nothing is read from disk at install time.

**Note**: `//go:embed all:templates` is used rather than `templates/**` because Go's `**` glob does not recurse into subdirectories by default in all cases; `all:` handles nested directories and dotfiles correctly.

**Alternative considered**: Download templates from a remote URL at install time. Rejected — requires network access, complicates air-gapped use, and makes the binary non-self-contained.

### 2. Template layout mirrors output layout, one directory per platform

```text
internal/scaffold/templates/
  agents/
    claude-code/
      orchestrator.md
      spec-writer.md
      implementer.md
      tester.md
      pr-closer.md
    codex/
      orchestrator.toml
      spec-writer.toml
      implementer.toml
      tester.toml
      pr-closer.toml
    cursor/
      orchestrator.mdc
      spec-writer.mdc
      implementer.mdc
      tester.mdc
      pr-closer.mdc
    kiro/
      orchestrator.md       ← goes to .kiro/steering/; uses inclusion frontmatter
      spec-writer.md
      implementer.md
      tester.md
      pr-closer.md
    antigravity/
      orchestrator.md       ← goes to plugin bundle agents/
      spec-writer.md
      implementer.md
      tester.md
      pr-closer.md
      plugin.json           ← plugin manifest template
    github-copilot/
      orchestrator.instructions.md
      spec-writer.instructions.md
      implementer.instructions.md
      tester.instructions.md
      pr-closer.instructions.md
  hooks/
    scripts/
      version-bump.sh
      transition-log.sh
      run-tests.sh
      coauthor.sh
    bindings/
      claude-code/settings-patch.json   ← merged into .claude/settings.json
      codex/hooks.json                  ← written to .codex/hooks.json
      cursor/hooks.json                 ← written to .cursor/hooks.json
      kiro/agent-patch.json             ← merged into .kiro/agent.json
      antigravity/hooks.json            ← written into plugin bundle
      github-copilot/hooks-manifest.json  ← stub
```

**Alternative considered**: A single template tree with `{{.Platform}}` substitution. Rejected — agent formats differ substantially between platforms (TOML vs frontmatter MD vs plain MD); separate files are easier to read, test, and maintain.

### 3. All four hooks bind to the end-of-turn event; scripts contain conditional logic

The end-of-turn event (variously named `Stop`, `stop`, or `Agent Stop`) fires once per agent turn after all tool calls complete. Binding all four hooks to this single event per platform gives consistent, deterministic behavior: hooks run predictably at turn boundaries, not mid-work.

Each script decides at runtime whether to act:

| Hook script | Acts when… |
| --- | --- |
| `version-bump.sh` | `git diff HEAD` shows changes since the last commit |
| `transition-log.sh` | Always — appends a timestamped turn summary to `.dreamland/transition.log` |
| `run-tests.sh` | Executable code files changed during this turn (checks `git diff --name-only HEAD` against source file extensions) |
| `coauthor.sh` | A commit was made during this turn (checks `git log` since session start timestamp); otherwise updates `git config user.name/user.email` locally |

End-of-turn event names by platform:

| Platform | Event name in binding config |
| --- | --- |
| Claude Code | `Stop` |
| Codex CLI | `Stop` |
| Cursor | `stop` |
| Kiro CLI | `stop` |
| Antigravity | `Stop` |
| GitHub Copilot | stub |

**Alternative considered**: Map each hook to the most semantically precise event (e.g., `PreToolUse(Bash)` for run-tests, `PostToolUse` for transition-log). Rejected — different events per hook creates inconsistent bindings across platforms and makes the binding files diverge. Conditional logic in scripts is more portable and easier to reason about.

### 4. `scaffold.Install` is the public API

```go
package scaffold

type Config struct {
    RepoRoot   string // absolute path
    CodingTool string // matches coding_tool values from init wizard
    Force      bool
}

// Install writes agent and hook files for the given config.
// Existing files are not overwritten unless Force is true.
// Returns one Result per file attempted.
func Install(cfg Config) ([]Result, error)
```

JSON config files (settings.json, hooks.json) that already exist are merged atomically: read → merge in memory → write. If the write fails, the original is untouched. No backup files are created.

### 5. JSON config merge is atomic, no backups needed

For platforms that write to an existing JSON config (Claude Code's `settings.json`, Codex's `hooks.json`, Cursor's `hooks.json`, Kiro's `agent.json`): the installer reads the file, merges the `hooks` key in memory, and writes the result. Because the merge happens before any write, a failed write leaves the original intact. Backup files add complexity without meaningful safety benefit given this approach.

### 6. Platform-specific hook registration strategies

- **Claude Code**: Merge `hooks.Stop` array into `.claude/settings.json` (create if absent)
- **Codex CLI**: Merge `hooks.Stop` array into `.codex/hooks.json` (create if absent)
- **Cursor**: Merge `hooks.stop` array into `.cursor/hooks.json` with `version: 1` envelope (create if absent)
- **Kiro**: Merge `hooks.stop` array into `.kiro/agent.json` using `execute_bash` tool matcher convention (create if absent)
- **Antigravity**: Write `hooks.json` into plugin bundle `~/.gemini/antigravity-cli/plugins/dreamland/`; write `plugin.json` manifest; also write agent files under `agents/`
- **GitHub Copilot**: Write `.github/copilot-hooks/hooks-manifest.json` stub with explanatory note

### 7. Kiro agent files use `inclusion: always` frontmatter

Kiro steering documents support optional YAML frontmatter controlling when the file is loaded. For dreamland's five role agents, all should be `inclusion: always` so the agent context is present in every session. Without frontmatter, Kiro defaults to `auto` (model-decided inclusion), which is non-deterministic for role-defining instructions.

### 8. Antigravity uses a plugin bundle, not a repo-local directory

Antigravity's config lives in a user-level plugin directory. The installer creates `~/.gemini/antigravity-cli/plugins/dreamland/` with `plugin.json`, five agent files under `agents/`, and `hooks.json`. Repo-local files are not discovered by the Antigravity CLI.

### 9. Repo root step is step 1 in the wizard

The `init` wizard gains a new step (displayed as "Step 1 of 6") that presents an editable text input pre-filled with the **detected git root** (not the current working directory). All subsequent step numbers increment by one. `wizardResult` gains a `repoRoot string` field; `config.Config` gains `"repo_root"` JSON field.

## Risks / Trade-offs

- **Antigravity API in preview** → Hook event schema may change before GA. Mitigation: binding file is a separate embedded asset; update in a patch release without touching hook scripts or agent templates.
- **Kiro CLI agent config filename** → The exact filename of the Kiro CLI agent config is not confirmed in public docs (referenced as "agent configuration file"). Implementation should verify against the official Kiro CLI reference before writing. Placeholder: `.kiro/agent.json`.
- **GitHub Copilot hook binding unknown** → Stub file with note; no shell command hooks are wired for Copilot until the schema is published.
- **Agent prompt quality** → Embedded prompts are starting points; users own the files after install and dreamland does not overwrite them by default.
- **Antigravity plugin registration** → Dreamland writes the files but cannot run `antigravity plugin install` on the user's behalf. Mitigation: print a post-install instruction to stdout.
- **`coauthor.sh` detecting commits made during a turn** → The script uses a session-start timestamp written to `.dreamland/session-start` at `agentSpawn`/`SessionStart` time to bound the `git log --since` query. If the session-start file is absent, the script falls back to updating git config only.

## Migration Plan

1. `internal/config.Config` gains `RepoRoot` field — backward compatible (old `.dreamland.json` files omit the field; callers fall back to git root detection).
2. `dreamland init` re-run on an existing project: wizard shows new step 1 and expanded tool list; `scaffold.Install` skips files that already exist and reports which were skipped.
3. JSON config merges are additive — existing hook arrays in settings files are preserved and dreamland entries are appended.

## Open Questions

- Confirm the exact filename for Kiro CLI agent config (currently assumed `.kiro/agent.json`).
- Antigravity `hooks.json` event schema: track against GA release notes; binding file may need update.
- Should `version-bump.sh` bump `patch`, `minor`, or make it configurable? Current assumption: `patch`.
