## ADDED Requirements

### Requirement: Repository provides a new-agent issue form

`.github/ISSUE_TEMPLATE/new-agent.yml` SHALL exist as a GitHub issue form with required fields `name`, `role`, `rationale`, `tool_tier` (dropdown of `router`, `read-dispatch-only`, `full-edit`, `write-only-no-edit`), `routing`, `acceptance_criteria`, and the label `new-agent`.

#### Scenario: Init writes the template

- **WHEN** `dreamland init` runs (template writing is part of standard setup, not opt-in) in a repo lacking the file
- **THEN** `.github/ISSUE_TEMPLATE/new-agent.yml` is created and parses as valid YAML with the required fields

#### Scenario: Idempotent

- **WHEN** `dreamland agent-issue --template` runs twice
- **THEN** the file is byte-identical after both runs and no error is returned

### Requirement: Command creates a filled new-agent issue

`dreamland agent-issue --create` with `--name`, `--role`, `--rationale`, `--tier`, `--routing`, `--criteria` SHALL create a GitHub issue through `gh issue create` labelled `new-agent` whose body has one section per template field, and print the issue URL.

#### Scenario: Missing required flag

- **WHEN** `--create` is run without `--name`
- **THEN** it exits non-zero naming the missing flag and does not call `gh`

#### Scenario: gh unavailable

- **WHEN** `gh` is not on PATH
- **THEN** it exits non-zero with a message stating `gh` is required

### Requirement: Issue creation requires human confirmation

`dreamland agent-issue --create` SHALL display the rendered title and body and require an affirmative `y` before calling `gh`; any other input SHALL abort with exit code 1 and no `gh` call. `--yes` skips the prompt for human-driven scripting.

#### Scenario: Declined

- **WHEN** the user answers `n`
- **THEN** no issue is created and the command exits 1

### Requirement: MCP creation is two-phase

`zhougong_new_agent_issue` with `confirm=false` SHALL return the rendered preview and a `previewId` without calling `gh`. With `confirm=true` and a matching `previewId` it SHALL create the issue. A `confirm=true` call with a missing or non-matching `previewId` SHALL fail without calling `gh`.

#### Scenario: Preview does not create

- **WHEN** the tool is called with `confirm=false`
- **THEN** the stubbed `gh` receives no call

#### Scenario: Confirm without preview

- **WHEN** `confirm=true` is sent with an unknown `previewId`
- **THEN** it returns `IsError: true` and `gh` is not called

### Requirement: Same operation is available through MCP

`zhougong_new_agent_issue` on the `dreamland-zhougong` MCP server SHALL accept the same fields and delegate to the same core function as the command, returning the issue URL, or `IsError: true` with the failure message.

#### Scenario: MCP parity

- **WHEN** the tool is called with valid fields, confirmed via preview, and a stubbed `gh`
- **THEN** the stub receives the same title, body and label the command would send

#### Scenario: Tool is listed and granted to zhougong

- **WHEN** an in-process MCP client lists tools on the `dreamland-zhougong` server and `.claude/agents/zhougong.md` is scaffolded
- **THEN** `zhougong_new_agent_issue` is listed and `mcp__dreamland-zhougong__zhougong_new_agent_issue` is in zhougong's `tools`

### Requirement: Phantasos drafts changes from new-agent issues

`phantasos`'s instructions SHALL state that, given a new-agent issue number, it reads the issue with `gh issue view <n> --json title,body,labels` and drafts `proposal.md`/`design.md`/`tasks.md` from its fields, scoping implementation to `hypnos`.

#### Scenario: Instruction present

- **WHEN** `.claude/agents/phantasos.md` is scaffolded
- **THEN** its body contains the issue-intake instruction
