## ADDED Requirements

### Requirement: Build and install step

After the version-command step and before the OpenTelemetry step, the wizard SHALL present a build step (the wizard's step titles become "Step n/8"; the OpenTelemetry step becomes 8 of 8), all of it optional:

1. `Build command (optional, press Enter to skip)?`, pre-filled with `go build -o {output} .` when the language is Go and empty for every other language, validated by `ValidateBuildCommand` (see the `project-config` capability). The prompt text states that the command is run without a shell, that only the placeholders `{output}` and `{commit}` exist, and that a linker flag that embeds the commit is written without spaces, for example `-ldflags=-X=<module>/cmd.buildCommit={commit}`.
2. Only if a build command was entered: `Build output path (relative to the repository root)?`, pre-filled with the repository directory's base name for Go and empty otherwise, validated by `ValidateBuildOutput`; required when the command contains `{output}`.
3. Only if a build output was entered: `Install the built binary after building?` with the choices "No", "Yes, to ~/.local/bin (user-bin)", and "Yes, to a custom absolute path" (which then asks for the path, validated by `ResolveBuildInstall`). The default choice is "No".

Skipping the command clears the other two answers. A validation failure re-prompts the same field with the reason. On success the values are written to `build_command`, `build_output`, and `build_install`. After the config is saved and the scaffold installer has run, `dreamland init` SHALL ensure the build output is gitignored (`EnsureGitignoreEntry` with `/<build_output>` unless already present), and, on Claude Code, the scaffold merge SHALL leave `.claude/settings.json` `permissions.allow` containing `Bash(dreamland build)` (see the `router-dispatch-guard` capability). It SHALL print the `PATH` advisory when the install directory is not on `PATH`. Init SHALL NOT run the build or install anything itself.

#### Scenario: Go repository accepts the defaults

- **WHEN** the user selects Go, accepts the pre-filled build command and output, and answers "No" to install
- **THEN** `.dreamland.json` has `build_command` `go build -o {output} .`, `build_output` the directory base name, and no `build_install`, and the output path is added to `.gitignore`

#### Scenario: Install to the user bin directory

- **WHEN** the user answers "Yes, to ~/.local/bin (user-bin)"
- **THEN** `.dreamland.json` has `"build_install": "user-bin"`

#### Scenario: Skipping the build command

- **WHEN** the user leaves the build command empty
- **THEN** no output or install prompt is shown, and none of the three keys is written

#### Scenario: An invalid command is re-prompted

- **WHEN** the user enters `go build ; rm -rf x`
- **THEN** the wizard shows the validation error and asks for the build command again

#### Scenario: Init does not build

- **WHEN** the wizard completes with a build command configured
- **THEN** no build command is executed and nothing is written outside the repository

### Requirement: Re-initialization preserves the build prompts' prior answers and settings the wizard does not ask about

When `.dreamland.json` already exists and the user confirms re-initialization, the build prompts SHALL be pre-filled from the existing `build_command`, `build_output`, and `build_install` (an existing value wins over the language default), and the written config SHALL carry over the existing `main_thread_guard` value (see the `router-dispatch-guard` capability) and any other key the wizard does not ask about, rather than dropping it. This is how a repository initialised before this change acquires build settings: one `dreamland init` re-run adds the three keys and, through the scaffold merge, the `Bash(dreamland build)` permission. Until it is re-run, `dreamland build` fails with the "no build command configured" message and the guard's block message says so; nothing is inferred.

#### Scenario: A pre-existing repository is upgraded by re-running init

- **WHEN** a repository whose `.dreamland.json` has no build keys re-runs `dreamland init`, confirms, and enters a build command
- **THEN** the three keys are written, `main_thread_guard` and other unasked keys keep their previous values, and `.claude/settings.json` `permissions.allow` gains `Bash(dreamland build)` exactly once

#### Scenario: Re-init does not drop the guard opt-out

- **WHEN** `.dreamland.json` has `"main_thread_guard": "allow"` and the user re-runs `dreamland init`
- **THEN** the written file still has `"main_thread_guard": "allow"`
