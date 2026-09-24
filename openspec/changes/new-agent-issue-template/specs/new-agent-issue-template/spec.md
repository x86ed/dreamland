## ADDED Requirements

### Requirement: Repository provides a new-agent issue form

`.github/ISSUE_TEMPLATE/new-agent.yml` SHALL exist as a GitHub issue form with required fields `name`, `role`, `rationale`, `tool_tier` (dropdown of `router`, `read-dispatch-only`, `full-edit`, `write-only-no-edit`), `routing`, `acceptance_criteria`, and the label `new-agent`.

#### Scenario: Init writes the template

- **WHEN** `dreamland init` runs in a repo lacking the file
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

### Requirement: Same operation is available through MCP

`zhougong_new_agent_issue` on the `dreamland-zhougong` MCP server SHALL accept the same fields and delegate to the same core function as the command, returning the issue URL, or `IsError: true` with the failure message.

#### Scenario: MCP parity

- **WHEN** the tool is called with valid fields and a stubbed `gh`
- **THEN** the stub receives the same title, body and label the command would send

### Requirement: Phantasos drafts changes from new-agent issues

`phantasos`'s instructions SHALL state that, given a new-agent issue number, it reads the issue with `gh issue view <n> --json title,body,labels` and drafts `proposal.md`/`design.md`/`tasks.md` from its fields, scoping implementation to `hypnos`.

#### Scenario: Instruction present

- **WHEN** `.claude/agents/phantasos.md` is scaffolded
- **THEN** its body contains the issue-intake instruction
