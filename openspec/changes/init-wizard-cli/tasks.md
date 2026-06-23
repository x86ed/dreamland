## 1. Dependencies

- [ ] 1.1 Add `github.com/charmbracelet/huh` to `go.mod` / `go.sum` via `go get`
- [ ] 1.2 Verify `encoding/json` is available (stdlib — no `go get` needed)

## 2. Project Config Package

- [ ] 2.1 Create `internal/config/config.go` with `Config` struct (`CodingTool`, `Language`, `TestCommand`, `DocCommand`, `VersionCommand` string fields and `json` tags)
- [ ] 2.2 Implement `FindRepoRoot(dir string) (string, error)` — walks parent dirs looking for `.git`
- [ ] 2.3 Implement `Load(dir string) (*Config, error)` — finds repo root, reads `.dreamland.json`; returns nil config (not error) when file is absent
- [ ] 2.4 Implement `Save(dir string, cfg *Config) error` — atomic write (temp file + rename) to repo root, serialising with `encoding/json`
- [ ] 2.5 Write `internal/config/config_test.go` covering `FindRepoRoot`, `Load` (present/absent/invalid), and `Save` (success + mid-flight failure) to reach ≥ 95 % coverage

## 3. Init Wizard Command

- [ ] 3.1 Create `cmd/init.go` — register `initCmd` on `rootCmd` in `init()`
- [ ] 3.2 Implement step 1: `huh.Select` prompt for coding tool (Claude Code, GitHub Copilot, Antigravity, Kiro)
- [ ] 3.3 Implement step 2: `huh.Select` prompt for language (Go, Node/TypeScript, Rust, Python)
- [ ] 3.4 Implement step 3: `huh.Input` prompt for test command with a non-empty validator
- [ ] 3.5 Implement step 4: `huh.Input` prompt for doc command with no validator (empty = skip, stored as `""`)
- [ ] 3.6 Implement step 5: `huh.Input` prompt for version command pre-filled with a language-derived default (`go version`, `node --version`, `rustc --version`, `python3 --version`); allow override
- [ ] 3.7 Implement re-init guard: detect existing `.dreamland.json`, prompt confirm before overwriting; exit 0 on decline
- [ ] 3.8 After writing config, print the config file path and a success message to stdout
- [ ] 3.9 Write `cmd/init_test.go` covering: help text, re-init guard (confirm/decline), successful wizard run (stub huh), empty test command re-prompt, doc command skipped, version command default and override, and Ctrl-C / stdin-close abort — targeting ≥ 95 % coverage of `cmd/init.go`

## 4. Root Command Integration

- [ ] 4.1 Add `PersistentPreRunE` to `rootCmd` in `cmd/root.go` that calls `config.Load(cwd)` and stores the result in a package-level `currentConfig *config.Config`
- [ ] 4.2 Export `GetConfig() *config.Config` accessor from the `cmd` package
- [ ] 4.3 Add unit test in `cmd/root_test.go` verifying that `PersistentPreRunE` sets `currentConfig` when a valid config file exists and leaves it nil when absent

## 5. Integration & Coverage Gate

- [ ] 5.1 Run `go test ./... -coverprofile=coverage.out` and confirm overall new-file coverage ≥ 95 %
- [ ] 5.2 Run `go vet ./...` and resolve any issues
- [ ] 5.3 Manually run `go run . init` in a terminal and complete the wizard end-to-end; verify `.dreamland.json` is created with correct content and can be committed to git
