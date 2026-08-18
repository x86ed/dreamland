# oneiroi-seed-naming Specification

## Purpose
TBD - created by archiving change oneiroi-seed-script. Update Purpose after archive.
## Requirements
### Requirement: A checked-in word list, generated offline from the DoD/partner code-name page, backs name generation

`internal/oneiroi/seedwords/words.json` SHALL exist as a checked-in file containing a deduplicated, lowercased set of individual word tokens derived by splitting every two-word (or longer) operation code name listed on https://en.wikipedia.org/wiki/List_of_U.S._Department_of_Defense_and_partner_code_names on whitespace. This file SHALL be produced by a standalone generator program (`internal/oneiroi/seedwords/gen/main.go`) that is not part of the `dreamland` command tree and is not invoked at runtime by `dreamland oneiroi seed`/`revise`/`fork`. `dreamland oneiroi seed`/`revise`/`fork` SHALL read only the checked-in `words.json` and SHALL NOT perform a network request.

#### Scenario: words.json contains tokens split from multi-word code names

- **WHEN** `internal/oneiroi/seedwords/words.json` is generated from a source table containing an entry like "IRAQI FREEDOM"
- **THEN** the resulting word set contains both `iraqi` and `freedom` as separate lowercase entries

#### Scenario: Name generation never performs a network request

- **WHEN** `dreamland oneiroi seed` runs with no network connectivity available
- **THEN** it still completes successfully, reading only the checked-in `words.json`

### Requirement: dreamland oneiroi seed generates a collision-free 2-word family name

`dreamland oneiroi seed --role "<one-line role>" [--tool-tier <tier>]` SHALL draw two distinct words uniformly at random from the checked-in word pool and combine them as `<word1>-<word2>` (kebab-case) to form a new agent family name. It SHALL check the resulting name against every `name` and `words` entry already present in `.dreamland/oneiroi/registry.json` (case-insensitive) and against the ten built-in registered agent names; on a collision, it SHALL redraw both words and retry, up to 50 attempts, after which it SHALL exit non-zero with a message stating the word pool is exhausted for new 2-word combinations.

`--tool-tier` SHALL accept one of `router`, `read-dispatch-only`, `full-edit`, or `write-only-no-edit` (the four tiers defined by the `agent-scaffolding` capability's tool-binding matrix); when omitted, it SHALL default to `full-edit`.

#### Scenario: Fresh family name generated with no collision

- **WHEN** `dreamland oneiroi seed --role "example role"` runs and the drawn 2-word combination does not already exist in the registry
- **THEN** the command succeeds, printing the generated name, and adds a new entry to `.dreamland/oneiroi/registry.json` with `words` containing exactly those two words and no third word

#### Scenario: Collision triggers a full two-word redraw, not a single-word swap

- **WHEN** the first drawn 2-word combination already exists in the registry
- **THEN** the retry draws two new words, not just a replacement for one of the two original words

#### Scenario: Exhausted pool fails loudly

- **WHEN** 50 consecutive draws all collide with existing registry entries
- **THEN** `dreamland oneiroi seed` exits non-zero with a message naming the exhaustion condition, and no registry entry or scaffold file is written

### Requirement: A third word is added only on explicit revision or fork, never accumulated beyond one

`dreamland oneiroi revise --agent <name> --reason "<reason>"` SHALL draw one new word (distinct from the family's own two words and from any word already recorded for that family) and set it as that agent's current third word, replacing any prior third word (which SHALL be preserved in the registry entry's `revisions` history, not discarded). `dreamland oneiroi fork --agent <parent> --role "<one-line role>"` SHALL create a **new** registry entry inheriting the parent's two-word `words[0]`/`words[1]` pair verbatim, plus a freshly drawn third word distinct from the parent's own current third word (if any) and from any other fork of the same parent, and SHALL record `parent: <parent-name>` on the new entry. Neither operation SHALL ever produce a name with more than three words, and neither modifies the entry it does not target (`revise` never creates a new registry entry; `fork` never modifies the parent's entry).

#### Scenario: Initial seed has no third word

- **WHEN** `dreamland oneiroi seed` creates a new family
- **THEN** the registry entry's `words` array has exactly 2 elements and the agent's live name (used for git identity, hook bindings, and slash command) is `<word1>-<word2>`

#### Scenario: Revision adds a third word to an existing 2-word family

- **WHEN** `dreamland oneiroi revise --agent amber-falcon --reason "model swap"` runs
- **THEN** the registry entry's `words` array gains a third element and the agent's live name becomes `amber-falcon-<word3>`

#### Scenario: Re-revision replaces, not appends, the third word

- **WHEN** `dreamland oneiroi revise` runs again for an agent that already has a third word
- **THEN** the live `words` array still has exactly 3 elements (the old third word moves to the `revisions` history, a new one takes its place), never 4

#### Scenario: Fork creates a sibling sharing the family pair with its own third word

- **WHEN** `dreamland oneiroi fork --agent amber-falcon --role "variant role"` runs
- **THEN** a new registry entry is created with `words` `["amber", "falcon", "<new-word3>"]`, `parent: "amber-falcon"`, and the `amber-falcon` entry itself is unmodified

#### Scenario: Fork's third word never collides with a sibling fork's third word

- **WHEN** `amber-falcon` has already been forked once (producing `amber-falcon-onyx`) and is forked again
- **THEN** the new fork's third word is not `onyx` and is not one of `amber`/`falcon`

### Requirement: The open agent registry makes seeded oneiroi first-class registered identities

`.dreamland/oneiroi/registry.json` SHALL be created (if absent) on the first `dreamland oneiroi seed`/`revise`/`fork` invocation in a repository, and every subsequent invocation SHALL read and atomically rewrite it (temp file + rename, matching the atomic-merge convention already used for platform hook-binding files per the `dev-workflow-hooks` capability). `internal/agentidentity.IsRegistered` SHALL treat a name as registered if it is one of the ten built-in agent names **or** matches a `name` field in this registry file (when present in the current repository) — with no change in behavior for the built-in ten and no error when the registry file does not exist (a repo that has never seeded an oneiroi behaves exactly as it does today).

#### Scenario: Freshly seeded agent is immediately a registered identity

- **WHEN** `dreamland oneiroi seed --role "example"` completes and generates the name `amber-falcon`
- **THEN** `internal/agentidentity.IsRegistered("amber-falcon")` returns `true` in that repository, with no rebuild of the `dreamland` binary required

#### Scenario: Built-in ten remain registered with no registry file present

- **WHEN** `.dreamland/oneiroi/registry.json` does not exist in a repository
- **THEN** `internal/agentidentity.IsRegistered("janus")` still returns `true`, and `internal/agentidentity.IsRegistered("amber-falcon")` returns `false`

#### Scenario: coauthor resolves a seeded oneiroi's git identity correctly

- **WHEN** `dreamland coauthor --hook` runs with a hook payload identifying the acting agent as `amber-falcon`, and `amber-falcon` is present in `.dreamland/oneiroi/registry.json`
- **THEN** `git config --local user.name` is set to `"amber-falcon"`, not `"janus"`

### Requirement: dreamland oneiroi seed scaffolds the standard hook/identity boilerplate for every supported platform

For each of the six supported platforms (Claude Code, Codex CLI, Cursor, Kiro, Antigravity, GitHub Copilot), `dreamland oneiroi seed` SHALL write a stub agent template file at that platform's standard agent-file path and format (per the `agent-scaffolding` capability's per-platform path table, substituting the generated name for the fixed ten's names), containing:

- Frontmatter/capability declarations reflecting the requested (or default `full-edit`) tool tier, in the same shape the `agent-scaffolding` capability's tool-binding matrix already defines for that tier.
- The identical hook/binding block an existing same-tier agent's template carries (Claude Code `Stop` hooks: `dreamland coauthor --hook --agent-name <name>`, `dreamland telemetry write --tool claude-code --agent-name <name>`, `dreamland version-bump --patch`, `dreamland commit --reason handoff --agent-name <name>`; the equivalent bindings for the other five platforms), with `<name>` substituted for the generated name.
- An instruction-body placeholder (e.g. `<!-- TODO(hypnos): persona-specific instructions for <name> -->`, or the platform's native comment syntax) in place of persona prose.

It SHALL also add a stub delegation-target line to all six `janus.*` routing-table files (marked for `hypnos` to finalize the tier/placement) and a per-agent slash command file (per the `router-slash-commands` capability's per-platform command conventions) for the generated name, on every platform.

#### Scenario: Stub files exist on all six platforms after seeding

- **WHEN** `dreamland oneiroi seed --role "example role"` generates the name `amber-falcon`
- **THEN** a stub template file exists for `amber-falcon` at the standard agent-file path on Claude Code, Codex CLI, Cursor, Kiro, Antigravity, and GitHub Copilot

#### Scenario: Stub carries the same hook block shape as an existing agent of the same tier

- **WHEN** `dreamland oneiroi seed` generates `amber-falcon` with the default `full-edit` tier on Claude Code
- **THEN** its frontmatter `hooks.Stop` block matches the shape of an existing full-edit-tier agent's `Stop` block (e.g. `hypnos.md`'s), with `--agent-name amber-falcon` substituted for the existing agent's name

#### Scenario: Stub instruction body is a placeholder, not authored prose

- **WHEN** any of the six stub files for `amber-falcon` is read immediately after `dreamland oneiroi seed` completes
- **THEN** its instruction body contains a placeholder comment naming `hypnos` as the next editor, and no role-specific persona prose

#### Scenario: Slash command installed for the generated name

- **WHEN** `dreamland oneiroi seed` generates `amber-falcon` on Claude Code
- **THEN** `.claude/commands/drmlnd/amber-falcon.md` exists, following the same per-agent direct-invoke convention the `router-slash-commands` capability defines for the existing nine agents

### Requirement: Scaffold-generated commits are self-authored, zero-token, and coauthor-free

`dreamland oneiroi seed`/`revise`/`fork` SHALL, on successful completion, stage exactly the set of files that invocation created or modified (an explicit path list, never `git add -A`) and create one commit authored as `dreamland-oneiroi-seed <oneiroi-seed@github.com>` via `git commit --author`, without modifying the repository-local `git config user.name`/`user.email` the invoking session's `coauthor` may have already set. The commit message SHALL include a `Tokens: input=0 output=0 cached=0 total=0` line and a `Generated-By: dreamland-oneiroi-seed` trailer, and SHALL NOT include a `Co-authored-by:` line.

#### Scenario: Commit author is the script, not the invoking agent

- **WHEN** `hypnos` (git identity already set to `hypnos` by `coauthor`) invokes `dreamland oneiroi seed` mid-turn
- **THEN** the resulting commit's author is `dreamland-oneiroi-seed <oneiroi-seed@github.com>`, and `git config --local user.name` remains `"hypnos"` immediately afterward

#### Scenario: Commit message declares zero tokens and no coauthor

- **WHEN** `dreamland oneiroi seed` completes and its commit is created
- **THEN** `git log -1 --format=%B` contains `Tokens: input=0 output=0 cached=0 total=0` and `Generated-By: dreamland-oneiroi-seed`, and contains no `Co-authored-by:` line

#### Scenario: Only the invocation's own files are staged

- **WHEN** a repository has unrelated pending changes in the working tree at the time `dreamland oneiroi seed` runs
- **THEN** the resulting commit contains only the registry entry, the six stub template files, the routing-table edits, and the slash command file this invocation produced — the unrelated pending changes remain uncommitted afterward

### Requirement: oneiroi seed/revise/fork are exposed identically via CLI and an optional MCP server

`dreamland oneiroi seed`, `dreamland oneiroi revise`, and `dreamland oneiroi fork` SHALL be Cobra subcommands under a `dreamland oneiroi` command group. A separate command, `dreamland mcp-serve`, SHALL run a stdio MCP server exposing `oneiroi_seed`, `oneiroi_revise`, and `oneiroi_fork` as MCP tools, each delegating to the identical `internal/oneiroi` functions the CLI subcommands call, with tool-call arguments mapping 1:1 to the CLI flags (`role`, `tool-tier`, `agent`, `reason`).

#### Scenario: CLI seed command available

- **WHEN** `dreamland --help` is run after this change lands
- **THEN** `oneiroi` appears in the command list, and `dreamland oneiroi seed --help` shows `--role` and `--tool-tier` flags

#### Scenario: MCP tool produces the same result as the equivalent CLI invocation

- **WHEN** `dreamland mcp-serve` is running and an MCP client calls the `oneiroi_seed` tool with `{"role": "example role"}`
- **THEN** the resulting registry entry, stub files, and commit are indistinguishable in shape from those produced by `dreamland oneiroi seed --role "example role"` run directly

