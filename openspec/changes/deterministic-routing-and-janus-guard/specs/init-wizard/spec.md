## ADDED Requirements

### Requirement: Build and install step

After the version-command step and before the OpenTelemetry step, the wizard SHALL present a build step (the wizard's step titles become "Step n/8"; the OpenTelemetry step becomes 8 of 8), all of it optional:

1. `Build command (optional, press Enter to skip)?`, pre-filled with `go build -o {output} .` when the language is Go and empty for every other language, validated by `ValidateBuildCommand` (see the `project-config` capability). The prompt text states that the command is run without a shell, that only the placeholders `{output}` and `{commit}` exist, and that a linker flag that embeds the commit is written without spaces, for example `-ldflags=-X=<module>/cmd.buildCommit={commit}`.
2. Only if a build command was entered: `Build output path (relative to the repository root)?`, pre-filled with the repository directory's base name for Go and empty otherwise, validated by `ValidateBuildOutput`; required when the command contains `{output}`.
3. Only if a build output was entered: `Install the built binary after building?` with the choices "No", "Yes, to ~/.local/bin (user-bin)", and "Yes, to a custom absolute path" (which then asks for the path, validated by `ResolveBuildInstall`). The default choice is "No".

Skipping the command clears the other two answers. A validation failure re-prompts the same field with the reason. On success the values are written to `build_command`, `build_output`, and `build_install`. After the config is saved and the scaffold installer has run, `dreamland init` SHALL ensure the build output is gitignored (`EnsureGitignoreEntry` with `/<build_output>` unless already present), and, on Claude Code, the scaffold merge SHALL leave `.claude/settings.json` `permissions.allow` containing `Bash(dreamland build)` (see the `router-dispatch-guard` capability). It SHALL print the `PATH` advisory when the install directory is not on `PATH`. The prompt text of the build command field SHALL also state that, once the configuration is saved, `dreamland init` runs the command once to perform the first build and install (see the "Init performs the first build" requirement below), so the user knows what confirming the value does.

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

#### Scenario: Confirming a build command announces the first build

- **WHEN** the wizard shows the build command prompt
- **THEN** its description says the command runs once after the configuration is saved, without a shell

### Requirement: Init performs the first build

After the values pass validation and the user has just entered or confirmed them (a pre-filled value accepted with Enter counts as confirmed), and only after everything else init does has finished (`.dreamland.json` saved; scaffold installed; commit-msg hook and OpenTelemetry files written; every `.gitignore` entry ensured, including `/<build_output>`, so the artifact is ignored before it exists), `dreamland init` SHALL run the first build and install by calling `performBuild`, the same function `dreamland build` calls (see the `project-build` capability): same validators, same argument-vector execution with no shell, same atomic install. There is no init-specific build implementation, and no init-specific build settings or flags. Build output is streamed to init's stdout and stderr, and init prints the same `built ...`/`installed ...` lines as `dreamland build`. Init SHALL run no other command and write nothing outside the repository except through the configured install step.

Whether init builds:

1. If `build_command` is empty (skipped, or absent on re-init), init SHALL NOT build and SHALL print nothing about building.
2. On a first init (no `.dreamland.json` before this run) with a `build_command`, init SHALL build.
3. On re-initialization, init SHALL build only when at least one of these holds: (a) `build_command`, `build_output`, or `build_install` differs from the value in the existing `.dreamland.json` (a key that was absent counts as empty); (b) `build_output` is set and the artifact does not exist at its resolved path under the repository root (with `.exe` appended on Windows when it has no extension); (c) `build_install` is set and the installed file does not exist at the resolved destination. Otherwise init SHALL skip the build and print one line saying the build settings are unchanged, the artifact is present, and `dreamland build` rebuilds. A command that declares no `build_output` (for example `make`) cannot be checked for a missing artifact, so on re-init it builds only under (a).
4. `dreamland init` has no non-interactive or scripted mode: the wizard needs a terminal, and if it cannot run or is cancelled init exits with the existing error before saving anything, so nothing is built. The test seam that substitutes the wizard is not a mode; tests stub the build runner as well. `--force` overwrites scaffold files only and has no effect on whether init builds. Any future non-interactive mode SHALL apply rules 1 through 3 unchanged and SHALL NOT build values that have not passed the validators.

A build or install failure SHALL NOT fail init and SHALL NOT roll back anything: `.dreamland.json`, the scaffold, and the gitignore entries stay as written, and init exits 0 (a failure of any earlier init step keeps its existing handling and never reaches the build). Init SHALL write to stderr `warning: first build failed: <error>` (or `warning: built <artifact path> but install failed: <error>` when only the install step failed) followed by `the recorded build settings were kept; retry with: dreamland build`. The build phase and install phase are distinguishable in the message, and the artifact-missing check is a build failure. On Windows the behavior is identical, including `.exe` naming and the rename-aside fallback of an installed `dreamland.exe` that is running; if that fallback also fails it is an install failure reported as a warning.

Init SHALL NOT stop, signal, or restart any process (including a running OpenTelemetry receiver, or the `dreamland` process performing the init itself), SHALL NOT write into the destination in place, and SHALL NOT fall back to a non-atomic copy; it installs only through the atomic replace (or Windows rename-aside) of `performBuild`.

#### Scenario: Fresh init builds and installs once

- **WHEN** the user runs `dreamland init` in a repository with no `.dreamland.json`, enters `go build -o {output} .`, output `dreamland`, and install "Yes, to ~/.local/bin (user-bin)"
- **THEN** after the config, scaffold, and gitignore steps, `performBuild` runs once, `./dreamland` and `~/.local/bin/dreamland` exist, `.gitignore` already contained `/dreamland` before the build ran, and init exits 0

#### Scenario: Skipped build command does not build

- **WHEN** the user leaves the build command empty
- **THEN** no build runs, nothing about building is printed, and init exits 0

#### Scenario: A build failure is a warning and keeps the configuration

- **WHEN** the first build exits nonzero
- **THEN** init exits 0, `.dreamland.json` still has the three build keys, stderr has `warning: first build failed:` and `retry with: dreamland build`, and nothing was installed

#### Scenario: An install failure after a good build is reported as such

- **WHEN** the build succeeds but the install step returns an error
- **THEN** init exits 0 with `warning: built <artifact path> but install failed:` and the retry instruction, and the previously installed binary is untouched

#### Scenario: Re-init with unchanged settings and a present artifact skips the build

- **WHEN** `.dreamland.json` already holds the build keys, the user re-runs `dreamland init` and accepts the pre-filled values unchanged, and the artifact (and installed file, if configured) exist
- **THEN** no build runs and init prints one line that the settings are unchanged and `dreamland build` rebuilds

#### Scenario: Re-init builds when the settings changed

- **WHEN** an existing repository re-runs init and changes `build_command` (or adds the three keys for the first time)
- **THEN** the first build runs after the config is saved

#### Scenario: Re-init builds when the artifact is missing

- **WHEN** re-init leaves the settings unchanged but `build_output` does not exist under the repository root
- **THEN** the build runs

#### Scenario: Re-init builds when the installed file is missing

- **WHEN** re-init leaves the settings unchanged, the artifact exists, `build_install` is `user-bin`, and the installed file does not exist
- **THEN** the build and install run

#### Scenario: A command without an output is built on re-init only when it changed

- **WHEN** `build_command` is `make` with no `build_output`, and re-init leaves it unchanged
- **THEN** no build runs

#### Scenario: Init never installs non-atomically

- **WHEN** the install step's rename fails for a reason other than the Windows running-executable case
- **THEN** init reports the install warning, exits 0, and the destination is unchanged

#### Scenario: Init does not signal other processes

- **WHEN** init replaces an installed `dreamland` that an OpenTelemetry receiver is running
- **THEN** the receiver process is not signalled or restarted, and only the atomic replace (or Windows rename-aside) touches the destination

### Requirement: Re-initialization preserves the build prompts' prior answers and settings the wizard does not ask about

When `.dreamland.json` already exists and the user confirms re-initialization, the build prompts SHALL be pre-filled from the existing `build_command`, `build_output`, and `build_install` (an existing value wins over the language default), and the written config SHALL carry over the existing `main_thread_guard` value (see the `router-dispatch-guard` capability) and any other key the wizard does not ask about, rather than dropping it. This is how a repository initialised before this change acquires build settings: one full `dreamland init` re-run adds the three keys, performs the first build (the settings are new, so rule 3(a) of the "Init performs the first build" requirement applies), and, through the scaffold merge, adds the `Bash(dreamland build)` permission. There is no `dreamland init --build-only` or other partial mode: the full wizard re-run is the upgrade path, by user decision. Until it is re-run, `dreamland build` fails with the "no build command configured" message and the guard's block message says so; nothing is inferred.

#### Scenario: A pre-existing repository is upgraded by re-running init

- **WHEN** a repository whose `.dreamland.json` has no build keys re-runs `dreamland init`, confirms, and enters a build command
- **THEN** the three keys are written, `main_thread_guard` and other unasked keys keep their previous values, and `.claude/settings.json` `permissions.allow` gains `Bash(dreamland build)` exactly once, and the first build runs

#### Scenario: No partial init mode exists

- **WHEN** `dreamland init --build-only` (or any other flag not listed in the `init` command's flags) is passed
- **THEN** cobra rejects it as an unknown flag and nothing is written

#### Scenario: Re-init does not drop the guard opt-out

- **WHEN** `.dreamland.json` has `"main_thread_guard": "allow"` and the user re-runs `dreamland init`
- **THEN** the written file still has `"main_thread_guard": "allow"`
