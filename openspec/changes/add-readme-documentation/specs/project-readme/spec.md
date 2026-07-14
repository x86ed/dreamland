## ADDED Requirements

### Requirement: README states dreamland's purpose and distillation philosophy
`README.md` SHALL open with a purpose statement describing dreamland as a spec-driven AI SDLC harness that is designed to be self-improving: it starts by leaning heavily on general-purpose agents and hooks, and is intended to be deliberately distilled over a project's life toward deterministic, code-generated scripts and specialized agents as patterns stabilize.

#### Scenario: Purpose statement present
- **WHEN** a reader opens `README.md`
- **THEN** the introduction explains dreamland as a self-improving agentic SDLC tool and states the intended progression from primarily agentic workflows toward primarily deterministic, codegen-based ones as the project matures

### Requirement: README provides multi-platform installation instructions
`README.md` SHALL contain an installation section covering building the `dreamland` binary from source on macOS, Linux, and Windows, using the Go toolchain version declared in `go.mod`. It SHALL NOT instruct a `go install <remote-path>@latest` command unless the module path in `go.mod` matches the repository's import path.

#### Scenario: Each platform has a build path
- **WHEN** a reader reads the installation section
- **THEN** they find distinct, correct instructions (or a shared instruction explicitly noted as cross-platform) for macOS, Linux, and Windows that result in a runnable `dreamland` binary

#### Scenario: No broken install command
- **WHEN** a reader copies any command from the installation section
- **THEN** the command succeeds given only a cloned copy of the repository and a working Go toolchain (no dependency on unpublished releases or mismatched module paths)

### Requirement: README documents the feature-building workflow via agents and OpenSpec
`README.md` SHALL explain how to build a feature using the five agents installed by `dreamland init` (orchestrator, spec-writer, implementer, tester, pr-closer) together with the OpenSpec change lifecycle (propose, apply, archive).

#### Scenario: Reader can start a change
- **WHEN** a reader follows the workflow section
- **THEN** they can identify the command or agent invocation to start a new OpenSpec change and understand which of the five agents is responsible for each stage from proposal through merge

### Requirement: README documents improving the harness from telemetry
`README.md` SHALL contain a section explaining how the telemetry dreamland already collects (`dreamland telemetry snapshot`, OTel traces from `dreamland serve`, commit trailers) can be used by maintainers to identify repeated agent behavior and graduate it into project-specific agents, hooks, or deterministic scripts. This section SHALL be framed as guidance for maintainers, not as a claim that automatic specialization is already implemented.

#### Scenario: Reader understands the feedback loop
- **WHEN** a reader reads the harness-improvement section
- **THEN** they can describe, in their own words, how to go from "telemetry shows agents repeatedly doing X" to "a specialized agent or script now handles X deterministically"

#### Scenario: No overclaiming of automation
- **WHEN** a reader reads the harness-improvement section
- **THEN** the text does not state or imply that dreamland automatically generates specialized agents without maintainer action

### Requirement: README uses a classified-document presentational style
`README.md` SHALL use a consistent, lightly humorous classified/redacted-document visual voice (e.g., section framing, a mock classification banner) in its prose and headers, while keeping all commands, code blocks, and required steps in plain, unstyled, copy-pasteable form.

#### Scenario: Styling does not obscure commands
- **WHEN** a reader looks at any shell command or code block in the README
- **THEN** the command is rendered as a standard markdown code block with no styling that would prevent copy-paste or break rendering on GitHub

#### Scenario: Tone is present but not disruptive
- **WHEN** a reader scans the document headers and section intros
- **THEN** they encounter a consistent lightly-classified/Area-51-style framing (e.g., redacted banner, "clearance" language) that is clearly tongue-in-cheek and does not replace or hide technical content
