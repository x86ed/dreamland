## 1. Seed word list

- [x] 1.1 Create `internal/oneiroi/seedwords/gen/main.go`: fetches https://en.wikipedia.org/wiki/List_of_U.S._Department_of_Defense_and_partner_code_names, parses the code-name table(s), splits each multi-word code name on whitespace, lowercases, dedupes, and writes `internal/oneiroi/seedwords/words.json` as `{"words": [...]}` (sorted for deterministic diffs). Document at the top of the file that it is a one-off generator, not part of `dreamland`'s command tree, and not run in CI.
- [x] 1.2 Run `internal/oneiroi/seedwords/gen/main.go` once and commit the resulting `internal/oneiroi/seedwords/words.json`.
- [x] 1.3 Add `internal/oneiroi/seedwords/words.go` with a `//go:embed words.json` directive and a `Load() ([]string, error)` function returning the parsed word list, so `dreamland oneiroi` reads the embedded copy (no filesystem read at install time), matching the `agent-scaffolding` capability's "Agent templates are embedded in the binary" precedent.
- [x] 1.4 Unit test: `words.json` parses, contains >100 entries, no duplicates, all lowercase.

## 2. Open agent registry

- [x] 2.1 Create `internal/oneiroi/registry.go`: `type Entry struct { Name string; Words []string; Role string; ToolTier string; Parent string; Created time.Time; Revisions []Revision }` and `type Revision struct { Word string; Reason string; At time.Time }`; `Load(repoRoot string) (*Registry, error)` (returns an empty registry, not an error, if `.dreamland/oneiroi/registry.json` does not exist) and `(*Registry) Save(repoRoot string) error` (atomic temp-file + rename, matching `atomicJSONMerge`'s pattern in `internal/scaffold/scaffold.go`).
- [x] 2.2 Add `(*Registry) Names() []string` and `(*Registry) HasFamily(word1, word2 string) bool` (case-insensitive) helpers.
- [x] 2.3 Refactor `internal/agentidentity/agentidentity.go`: rename the exported `Registered` map to an unexported `builtin` map; add `Registered(repoRoot string) map[string]bool` (function) returning `builtin` unioned with any registry entries found via `internal/oneiroi.Load(repoRoot)` (import cycle check: if `internal/oneiroi` cannot depend on `internal/agentidentity` cleanly, extract the registry-name-listing helper into a small shared/leaf package instead — see the `oneiroi-seed-naming` capability's registry requirement for the exact behavior needed); update `IsRegistered` to `IsRegistered(name, repoRoot string) bool`.
- [x] 2.4 Update every call site of `agentidentity.IsRegistered`/`Registered` (`cmd/coauthor.go`, `internal/telemetry/tools/claude.go`, any others found via `grep -rn "agentidentity\."`) to pass the resolved repo root (already available at each call site via `config.FindRepoRoot`).
- [x] 2.5 Update `internal/agentidentity/agentidentity_test.go`'s `TestIsRegistered` and any other test referencing the old `Registered` var/old `IsRegistered` signature.

## 3. Name generation

- [x] 3.1 Create `internal/oneiroi/seed.go`: `GenerateFamily(pool []string, existing *Registry) (word1, word2 string, err error)` — draws two distinct random words via `math/rand/v2`, retries on collision against `existing.HasFamily`, up to 50 attempts, returns a descriptive error on exhaustion.
- [x] 3.2 Add `GenerateThirdWord(pool []string, familyWords []string, excludeWords []string) (string, error)` — draws one word not in `familyWords` or `excludeWords`, same retry/exhaustion behavior.
- [x] 3.3 Table-driven unit tests: fresh family generation with no collisions; forced collision (small fake pool) triggers redraw; exhaustion after all combinations forced to collide returns an error, not a panic or silent fallback.

## 4. Generalized scaffold installer entry point

- [x] 4.1 In `internal/scaffold/scaffold.go`, extract the per-platform template-rendering logic (`installFlatAgents`, `installFlatCommands`, `installSkills`, `platformAgentSpec`, `platformCommandSpec`) into a shape that accepts an explicit `(name, role, toolTier string)` in addition to today's fixed ten-name iteration; add `InstallAgentStub(cfg Config, name, role, toolTier string) ([]Result, error)` as the new externally callable entry point, rendering a stub (frontmatter/hook block per tier, placeholder instruction body) at the same per-platform path convention the fixed ten use, for all six platforms.
- [x] 4.2 Add `InstallAgentCommand(cfg Config, name string) ([]Result, error)` mirroring the existing per-agent slash-command installation for an arbitrary name, reusing `platformCommandSpec`.
- [x] 4.3 Verify the fixed-ten install path (`Install`, `installAgents`, `installCommands`) is unchanged in behavior — no new tests should be needed to pass for the existing `internal/scaffold/scaffold_test.go` suite; run it to confirm.
- [x] 4.4 Unit tests for `InstallAgentStub`/`InstallAgentCommand`: given `name="amber-falcon"`, `toolTier="full-edit"`, files are written at the correct path for all six platforms with the correct `Edit`/`Write`/`Read`/`Bash` grant and a placeholder body; given an unknown `toolTier` value, return an error.

## 5. dreamland oneiroi CLI commands

- [x] 5.1 Create `cmd/oneiroi.go` with a `dreamland oneiroi` command group and three subcommands: `seed` (`--role`, `--tool-tier`, default `full-edit`), `revise` (`--agent`, `--reason`), `fork` (`--agent`, `--role`, `--tool-tier` inherited from parent unless overridden).
- [x] 5.2 `runOneiroiSeed`: load registry, generate family name (task 3.1), write registry entry (task 2.1), call `InstallAgentStub`/`InstallAgentCommand` (task 4.1-4.2), add a stub routing-table delegation line to all six `janus.*` files (new small helper, e.g. `internal/oneiroi/routing.go`'s `AddStubEdge(repoRoot, platform, agentName string) error` — insert a marked TODO line rather than attempting full prose generation), collect the list of files written, and hand off to the commit step (task 6).
- [x] 5.3 `runOneiroiRevise`: load registry, look up `--agent`, generate third word (task 3.2) excluding existing family words and any prior third word, update the registry entry's live `Words` and append the replaced word (if any) to `Revisions`, rewrite the agent's `--agent-name`/frontmatter `name:` references across its six stub/template files and its slash command file to the new 3-word name, commit.
- [x] 5.4 `runOneiroiFork`: load registry, look up `--agent` (parent), generate a third word excluding the parent's own current third word and any existing sibling fork's third word for that family, create a new registry entry with `Parent` set, call `InstallAgentStub`/`InstallAgentCommand` for the new 3-word name, add routing-table stub edges, commit. Parent's own registry entry and files are untouched — assert this in tests.
- [x] 5.5 Unit tests for each subcommand's happy path and its registry effects; a fake/temp git repo fixture (see existing pattern in `cmd/coauthor_test.go`/`cmd/commit_test.go`) for the commit-producing assertions in task 6.

## 6. Self-authored commit provenance

- [ ] 6.1 In `internal/oneiroi/commit.go`, add `CommitScaffold(repoRoot string, paths []string, subject string) error`: stages exactly `paths` (`git add -- <paths...>`, never `-A`), writes a commit message file containing `<subject>\n\nTokens: input=0 output=0 cached=0 total=0\nGenerated-By: dreamland-oneiroi-seed\n`, and runs `git commit --author "dreamland-oneiroi-seed <oneiroi-seed@github.com>" -F <tmpfile>`.
- [ ] 6.2 Wire `CommitScaffold` as the final step of `runOneiroiSeed`/`runOneiroiRevise`/`runOneiroiFork`, passing the exact file list each accumulated.
- [ ] 6.3 In `cmd/coauthor.go`'s `--trailer` handling (`appendCoauthorTrailer`/its caller), add a check at the top: if the commit message file already contains a line matching `^Generated-By:`, return immediately with no modification (skip both `appendCoauthorTrailer` and `appendTokensReport`).
- [ ] 6.4 Unit tests: `dreamland coauthor --trailer <file>` on a message containing `Generated-By: dreamland-oneiroi-seed` leaves the file byte-for-byte unchanged, even when a fake telemetry snapshot with non-zero tokens is present; a message without that trailer still gets the existing `Co-authored-by:`/`Tokens:` treatment (regression check against the base `dev-workflow-hooks` behavior).
- [ ] 6.5 Integration-style test (temp git repo, real `git commit`): after `dreamland oneiroi seed` runs, `git config --local user.name` is unchanged from whatever it was set to beforehand, and `git log -1 --format=%an` on the new commit reads `dreamland-oneiroi-seed`.

## 7. Hypnos instruction updates (all six platforms)

- [ ] 7.1 Update `internal/scaffold/templates/agents/claude-code/hypnos.md`'s numbered responsibilities per the `agent-lifecycle-management` delta spec: step 1 becomes `dreamland oneiroi seed`, persona-prose editing becomes step 2, routing-table/slash-command finalization become steps 3-4, add step 5 for `revise`/`fork` routing.
- [ ] 7.2 Apply the equivalent update to `internal/scaffold/templates/agents/{codex,cursor,kiro,antigravity,github-copilot}/hypnos.*`, preserving each platform's existing frontmatter/format conventions.
- [ ] 7.3 Grep the six updated files for any remaining language implying `hypnos` invents the new agent's name itself; remove/rewrite it.

## 8. MCP exposure

- [ ] 8.1 Add an MCP server dependency (evaluate `github.com/modelcontextprotocol/go-sdk` or an equivalent minimal Go MCP library; if none is suitable, implement the minimal stdio JSON-RPC subset needed for `tools/list`/`tools/call` directly — document the choice in a code comment).
- [ ] 8.2 Create `cmd/mcp_serve.go`: `dreamland mcp-serve` registers `oneiroi_seed`, `oneiroi_revise`, `oneiroi_fork` tools, each handler parsing its JSON arguments (`role`, `tool_tier`, `agent`, `reason`) and calling the same `internal/oneiroi` functions tasks 5.2-5.4 use, returning the generated name (and, for `fork`, the parent) as the tool result.
- [ ] 8.3 Unit test: a fake MCP client sends a `tools/call` for `oneiroi_seed` with `{"role": "example"}` over an in-process pipe; assert the response matches what calling `runOneiroiSeed` directly would produce (registry entry present, files written).

## 9. Verification

- [ ] 9.1 Run `go build ./...` and `go test ./...`; confirm no regressions in `cmd/coauthor_test.go`, `internal/agentidentity/agentidentity_test.go`, `internal/scaffold/scaffold_test.go`.
- [ ] 9.2 In a scratch repo scaffolded for Claude Code, run `dreamland oneiroi seed --role "test role"`; confirm stub files exist on all six platforms, `.dreamland/oneiroi/registry.json` has the new entry, the slash command file exists, and `git log -1` shows the `dreamland-oneiroi-seed` author with no `Co-authored-by:` trailer.
- [ ] 9.3 In the same scratch repo, run `dreamland oneiroi revise --agent <generated-name> --reason "test revision"` and `dreamland oneiroi fork --agent <generated-name> --role "fork role"`; confirm the versioning behavior in decision 3 of `design.md` (revise replaces the third word, fork creates a sibling sharing the family pair).
- [ ] 9.4 Run `openspec validate --change oneiroi-seed-script` (or equivalent) to confirm the delta specs apply cleanly against the base specs.
