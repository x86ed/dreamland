## ADDED Requirements

### Requirement: Five agent definition files are installed per platform

After `dreamland init` completes, the scaffold installer SHALL write five agent definition files into the repository (or user-level plugin directory for Antigravity) using the platform's native path and format.

The five agents are:

- **orchestrator** — routes tasks and coordinates the other agents
- **spec-writer** — drafts and refines spec files
- **implementer** — writes production code from specs
- **tester** — runs validation and interprets test results
- **pr-closer** — finalizes the spec and opens/merges the pull request

#### Scenario: Claude Code agents installed

- **WHEN** the selected coding tool is "Claude Code" and `dreamland init` completes successfully
- **THEN** five `.md` files are written to `.claude/agents/` (`orchestrator.md`, `spec-writer.md`, `implementer.md`, `tester.md`, `pr-closer.md`), each containing valid Claude Code agent YAML frontmatter (`name`, `description`, `tools`) and a role-specific system prompt

#### Scenario: Codex CLI agents installed

- **WHEN** the selected coding tool is "Codex CLI" and `dreamland init` completes successfully
- **THEN** five `.toml` files are written to `.codex/agents/` (`orchestrator.toml`, `spec-writer.toml`, `implementer.toml`, `tester.toml`, `pr-closer.toml`), each containing the required fields `name`, `description`, and `developer_instructions`

#### Scenario: Cursor agents installed

- **WHEN** the selected coding tool is "Cursor" and `dreamland init` completes successfully
- **THEN** five `.mdc` files are written to `.cursor/rules/` (`orchestrator.mdc`, `spec-writer.mdc`, `implementer.mdc`, `tester.mdc`, `pr-closer.mdc`), each containing YAML frontmatter with `description` and `alwaysApply: false`, followed by role-specific instructions

#### Scenario: Kiro agents installed

- **WHEN** the selected coding tool is "Kiro" and `dreamland init` completes successfully
- **THEN** five `.md` files are written to `.kiro/steering/` (`orchestrator.md`, `spec-writer.md`, `implementer.md`, `tester.md`, `pr-closer.md`) as plain markdown steering documents

#### Scenario: Antigravity agents installed

- **WHEN** the selected coding tool is "Antigravity" and `dreamland init` completes successfully
- **THEN** five skill directories are created at `.agents/skills/<name>/` in the repo root (one per agent: `orchestrator`, `spec-writer`, `implementer`, `tester`, `pr-closer`), each containing a `SKILL.md` file with YAML frontmatter (`name`, `description`) and a markdown body of agent instructions
- **AND** the `SKILL.md` files are auto-discovered by Antigravity via the project-scoped `.agents/skills/` convention (no plugin install step required)

#### Scenario: GitHub Copilot agents installed

- **WHEN** the selected coding tool is "GitHub Copilot" and `dreamland init` completes successfully
- **THEN** five `.agent.md` files are written to `.github/agents/` (`orchestrator.agent.md`, `spec-writer.agent.md`, `implementer.agent.md`, `tester.agent.md`, `pr-closer.agent.md`), each with YAML frontmatter (`name`, `description`, `tools`)

### Requirement: Agent templates are embedded in the binary

The CLI binary SHALL contain all agent template content at compile time using Go embed directives; no template files SHALL be read from the host filesystem at install time.

#### Scenario: Binary installs agents without template directory present

- **WHEN** `dreamland init` is run in an environment where `internal/scaffold/templates/` does not exist on disk
- **THEN** agents are still installed correctly using templates compiled into the binary

### Requirement: Existing agent files are not overwritten by default

If any agent file already exists at its target path, the installer SHALL skip that file and report it as skipped to stdout.

#### Scenario: Agent file already exists, no force flag

- **WHEN** `.claude/agents/orchestrator.md` already exists and `dreamland init` is run without `--force`
- **THEN** the existing file is not modified and stdout includes `skipped (already exists): .claude/agents/orchestrator.md`

#### Scenario: Agent file overwritten with force flag

- **WHEN** `.claude/agents/orchestrator.md` already exists and `dreamland init` is run with `--force`
- **THEN** the file is overwritten with the current template content and stdout shows `installed (forced): .claude/agents/orchestrator.md`

### Requirement: Installed agent files are reported to the user

After scaffolding completes, the CLI SHALL print a summary listing each agent file that was written and each that was skipped.

#### Scenario: All agents newly written

- **WHEN** no agent files previously existed and `dreamland init` completes
- **THEN** stdout contains one `installed:` line per agent file written

#### Scenario: Mixed written and skipped

- **WHEN** some agent files exist and some do not
- **THEN** stdout contains `installed:` lines for new files and `skipped (already exists):` lines for existing ones
