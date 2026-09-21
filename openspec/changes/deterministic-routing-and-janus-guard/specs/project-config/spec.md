## ADDED Requirements

### Requirement: Build settings are optional, validated by one shared function set, and never guessed

`.dreamland.json` MAY contain three optional keys, all omitted when unset (`omitempty`): `build_command` (string), `build_output` (string), and `build_install` (string). They are read by `dreamland build` and written only by `dreamland init` (or by hand). The `internal/config` package SHALL expose the validators `ValidateBuildCommand(command string, hasOutput bool) ([]string, error)`, `ValidateBuildOutput(output string) (string, error)`, and `ResolveBuildInstall(spec, repoRoot string) (dir string, err error)`, used by both the init wizard's field validators and `dreamland build`, so init-time and run-time rules cannot drift. Validation is a reject list of shell-shaped input plus a positive charset, not an attempt to parse or escape shell text.

`build_command`:

1. Split with `strings.Fields`; at most 32 tokens, each at most 256 bytes; non-empty.
2. After removing the substrings `{commit}` and `{output}`, every token matches `^[A-Za-z0-9_.,:/=+@-]+$`. This excludes whitespace inside a token, quotes, backslash, and the characters `; & | < > $ ( ) { } * ? [ ] ~ ! # % ^` and backtick, so there is nothing for a shell to interpret even if one were ever used, and no quoting mechanism exists (a flag value that would need a space cannot be expressed; the Go linker flag is written `-ldflags=-X=<pkg>.<var>={commit}`, which contains none).
3. The first token is a bare executable name: no `/`, `\`, or `:`, not `.` or `..`, and not a shell or launcher (`sh`, `bash`, `zsh`, `dash`, `fish`, `csh`, `ksh`, `cmd`, `powershell`, `pwsh`, `env`, `xargs`, `eval`, `exec`, `sudo`, `doas`, compared case-insensitively with a trailing `.exe` ignored). It SHALL resolve with `exec.LookPath`; at init time a lookup failure is a validation error, and at run time an execution error. This list is defense in depth against accidental shell use, not a sandbox: any build tool (`make`, `npm`, `cargo`) runs arbitrary repository code.
4. `{output}` is permitted only when `build_output` is non-empty (`hasOutput`); `{commit}` needs no other key.

`build_output` (the path of the artifact, relative to the repository root):

- Non-empty when `build_install` is set; each `/`- or `\`-separated segment matches `^[A-Za-z0-9_.-]+$`, no segment is `.` or `..`, the first segment does not begin with `-` or equal `.git`, it is not absolute, and `filepath.VolumeName` is empty (no drive letters, no UNC). The value is stored with `/` separators.

`build_install`:

- One of the empty string (no install), `user-bin`, or an absolute path. An absolute path must be `filepath.IsAbs`, equal to its own `filepath.Clean`, contain no `..` segment and no control character, not be a filesystem root, and not lie inside the repository root or inside any `.git` directory. It must exist as a directory or have an existing parent directory. If the install directory is not on `PATH`, init prints an advisory note (not an error).

#### Scenario: Valid Go build settings

- **WHEN** `build_command` is `go build -o {output} .`, `build_output` is `dreamland`, and `build_install` is `user-bin`
- **THEN** all three validators succeed

#### Scenario: Shell metacharacters are rejected

- **WHEN** `build_command` is each of `go build ; rm -rf x`, `go build $(id)`, `go build > out`, `go build "a b"`, `go build | tee x`, and `go build -o {output} .\n`
- **THEN** `ValidateBuildCommand` returns an error for each

#### Scenario: A shell or path as the executable is rejected

- **WHEN** `build_command` begins with `bash -c x`, `/usr/bin/make`, `./build.sh`, or `C:/tools/make.exe`
- **THEN** `ValidateBuildCommand` returns an error

#### Scenario: Output and install paths that escape are rejected

- **WHEN** `build_output` is `../x`, `/abs/x`, `C:/x`, `-o`, `.git/x`, or `a b`, or `build_install` is `~/bin`, `relative/dir`, `/a/../b`, a path inside the repository root, or `/`
- **THEN** the corresponding validator returns an error

#### Scenario: Output placeholder needs an output

- **WHEN** `build_command` contains `{output}` and `build_output` is empty
- **THEN** validation fails

#### Scenario: Older config files stay loadable

- **WHEN** `.dreamland.json` written before this change is loaded
- **THEN** it loads with all three build fields empty and no error, and `config.Save` of that value does not add the keys
