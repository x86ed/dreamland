## Context

`dreamland` is a Go CLI built on [cobra](https://github.com/spf13/cobra). It currently has a single `serve` subcommand that starts an MCP server. Subsequent commands need to know the project's AI coding tool, primary language, and test runner to tailor their behavior—but there is no mechanism to capture or persist that information.

The wizard must run in an interactive terminal, store choices durably per-repository, and make them accessible to all future invocations without user re-prompting.

## Goals / Non-Goals

**Goals:**
- Add `dreamland init` as a first-class cobra subcommand.
- Present an ordered 5-step wizard: coding tool → language → test command → doc command → version command.
- Persist answers to `.dreamland.json` at the repository root.
- Auto-load `.dreamland.json` on every CLI start and expose config to subcommands via cobra's context or a package-level accessor.
- Achieve ≥ 95 % unit test coverage for new code.

**Non-Goals:**
- Editing individual config values after init (no `config set` command in this change).
- Remote/shared config (`.dreamland.json` is user-local).
- Validation of the test command (arbitrary string; user's responsibility).
- Non-interactive / `--yes` batch mode.

## Decisions

### 1 — Interactive prompt library: `charmbracelet/huh`

**Decision:** Use [`github.com/charmbracelet/huh`](https://github.com/charmbracelet/huh) for interactive prompts.

**Rationale:** `huh` is actively maintained, provides `Select` and `Input` field types with built-in validation, TTY detection, and accessible styling out of the box. It is part of the well-supported Charm ecosystem.

**Alternative considered:** `github.com/AlecAivazis/survey/v2` — archived in 2023; its community fork (`go-survey/survey`) is a drop-in replacement but still lightly maintained compared to `huh`.

### 2 — Config format: JSON via `encoding/json`

**Decision:** Store config as JSON in `.dreamland.json`.

**Rationale:** JSON is universally parseable via the standard library in every language the project may target (Go, Node, Python, Rust). It introduces no new dependency, has no parsing ambiguities (no Norway problem, no boolean coercion), and is trivially machine-readable by shell scripts with `jq`.

**Alternative considered:** YAML (`gopkg.in/yaml.v3`) — more human-readable but requires a library in every language and has well-known parsing foot-guns (implicit type coercion, 1.1 vs 1.2 differences). Rejected for multi-language portability.

### 3 — Config file location: walk up to `.git` root

**Decision:** Locate `.dreamland.json` by walking parent directories until a `.git` directory or filesystem root is found; write/read from that directory.

**Rationale:** Ties config to the repository, not the working directory, so it works from any subdirectory. Matches the behavior of tools like `go`, `git`, and `golangci-lint`.

**Alternative considered:** Current working directory only — simpler but fails when invoked from a subdirectory.

### 4 — Config exposure: package-level singleton loaded at cobra `PersistentPreRunE`

**Decision:** Load config once in `rootCmd.PersistentPreRunE`, store in a package-level `*config.Config` in the `cmd` package, and expose a `GetConfig()` accessor.

**Rationale:** Avoids threading config through every cobra `RunE` signature. PersistentPreRunE runs before any subcommand, so all commands see the loaded config.

**Alternative considered:** cobra context — cleaner but requires context threading and is unfamiliar to most contributors.

## Risks / Trade-offs

- **Non-TTY environments (CI):** `huh` exits gracefully when stdin is not a terminal. → Mitigation: document that `init` requires an interactive terminal; future batch mode can be added with `--tool`, `--lang`, `--test-cmd` flags.
- **Config committed:** `.dreamland.json` is committed to the repository as shared team config. Teams must re-run `init` deliberately to change the stored values.
- **Overwriting existing config:** Running `init` a second time overwrites silently. → Mitigation: detect existing config and prompt "Re-initialize? [y/N]" before overwriting.
