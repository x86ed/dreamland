## Context

`hypnos` (`internal/scaffold/templates/agents/claude-code/hypnos.md` and its five siblings) already has a five-step process for authoring a new oneiroi: create six platform template files, pick a tool tier, register the agent in every `janus.*` routing table, add a per-agent slash command, hand off to `phobetor`. Every part of that process is currently manual/judgment-based, including the new agent's *name* — the ten existing agents (`janus`, `phantasos`, `nyx`, `morpheus`, `phobetor`, `baku`, `iktomi`, `zhougong`, `hypnos`, `mengpo`) are all mythological/folkloric names chosen individually, with no generator and no registry.

Two structural facts constrain this change:

1. `internal/agentidentity.Registered` is a hardcoded `map[string]bool` of exactly those ten names (`internal/agentidentity/agentidentity.go`). `IsRegistered` gates every place identity is trusted: `cmd/coauthor.go`'s git-identity resolution, the telemetry collectors' agent field, and (per `harden-commit-hook-enforcement`) the commit-subject line. A name not in that set is *not* used verbatim — it's discarded and the caller falls back to `janus`. A new oneiroi minted today would be invisible to all of that enforcement until someone manually edited the Go source and rebuilt the binary.
2. `internal/scaffold`'s installers (`installAgents`, `bindHooks`, `installCommands` in `internal/scaffold/scaffold.go`) iterate a fixed template set — one embedded template file per platform per one of the ten names. There is no code path today that installs a template for an arbitrary eleventh name; `hypnos` produces that file by hand-writing content, not by calling into `internal/scaffold`.

This change's job is narrower than "fully automate agent authoring": `hypnos` still writes the persona prose (its judgment is exactly what makes it useful — see the `agent-lifecycle-management` capability's description of its role). What this change automates is everything mechanical and identical across every oneiroi: the name, the registry entry that makes the name a *real* identity to the rest of the system, and the boilerplate hook/frontmatter block every existing agent template already carries verbatim (e.g. `hypnos.md`'s own frontmatter: `dreamland coauthor --hook --agent-name hypnos`, `dreamland telemetry write --tool claude-code`, `dreamland version-bump --patch`, `dreamland commit --reason handoff --agent-name hypnos`).

## Goals / Non-Goals

**Goals:**
- A deterministic, repeatable naming algorithm sourced from a real word list (DoD/partner code names), not invented per-change.
- A 2-word "family" name at creation, with a 3rd word added only for a model revision or a fork — explicitly modeled as a semver-like major.minor(.patch) scheme.
- Extend `agentidentity`'s closed ten-name set to an open set (ten built-ins ∪ a file-backed registry) with no behavior change for the existing ten.
- A CLI command (`dreamland oneiroi seed|revise|fork`) that produces the same six-platform boilerplate scaffold `hypnos` currently hand-writes, so `hypnos`'s own job shrinks to the persona-prose part it's actually suited for.
- A distinct, self-attributed commit convention for scaffold-generated code that does not pollute the invoking agent's authorship or the model's Co-authored-by trailer, and does not require skipping the `prepare-commit-msg` hook.
- A thin, optional MCP exposure of the same three operations, delegating to identical Go logic.

**Non-Goals:**
- Automating persona-prose authoring (the instruction body of a new agent's template files) — that remains `hypnos`'s job.
- Automating the routing-table *placement decision* (which tier, which edges) — `dreamland oneiroi seed` writes a stub edge marked for `hypnos` to finalize; it does not decide tool tier or broad-vs-narrow routing itself.
- A live network fetch of the Wikipedia word list at runtime. The list is generated once, offline, and checked in — see the "Word list is generated once, offline, and checked in" decision below.
- Changing the existing ten agents' names, identities, or template content in any way.
- Building a general plugin system for non-oneiroi third-party agents (same non-goal `claude-code-self-hosting` already established).

## Decisions

**1. Word list is generated once, offline, and checked in — not fetched at runtime.**

`internal/oneiroi/seedwords/words.json` is a checked-in JSON file: `{"words": ["ablaze", "acid", "adobe", ...]}`, a deduplicated, lowercased, hyphen/space-normalized set of every individual token that appears in a two-word (or occasionally longer) DoD/partner operation code name on the Wikipedia page, split on whitespace (so e.g. "IRAQI FREEDOM" contributes both `iraqi` and `freedom` to the pool; "JUST CAUSE" contributes `just` and `cause`). A separate, one-off Go program, `internal/oneiroi/seedwords/gen/main.go`, fetches and parses the Wikipedia page's tables and writes this file; it is **not** wired into `dreamland`'s command tree, is **not** run in CI, and is **not** invoked by `dreamland oneiroi seed` — it exists purely as the documented, reproducible mechanism for regenerating `words.json` if the word pool ever needs refreshing (e.g. the Wikipedia page grows new entries). This keeps `dreamland oneiroi seed` fully offline and deterministic-modulo-randomness, with no network dependency and no risk of a stale/unreachable Wikipedia page blocking `hypnos` mid-task. Alternative considered: fetch live and cache — rejected because a hook/agent-authoring flow blocking on an external HTTP call (and needing to handle Wikipedia markup changes) is a worse failure mode than an occasionally-stale-but-always-available checked-in list.

**2. Name generation: single flat word pool, two independent draws, retry on collision.**

Rather than modeling NICKA's real (and today, largely obsolete) alphabetic-block-per-command assignment, `dreamland oneiroi seed` treats the pool as one flat set and draws two *distinct* words uniformly at random (`word1`, `word2`) using `crypto/rand`-seeded `math/rand/v2`. The resulting family name is `word1-word2` (kebab-case, matching the existing agent-name convention of short lowercase identifiers used in `--agent-name`, file names, and slash commands). If `word1-word2` already exists as a family in the registry (case-insensitive), it re-draws (both words, not just one, to avoid low-entropy retries clustering around one popular word) up to 50 attempts, then fails loudly (`Blocking` exit) telling the caller the pool is exhausted for 2-word combinations — an intentionally loud failure rather than falling back to a lower-quality generation strategy silently. Alternative considered: bucket first-word by a "series" the caller supplies (closer to real NICKA, where a requesting command is pre-assigned an alphabetic block) — rejected as unnecessary complexity for this repo's scale (tens, not thousands, of agents) and reintroduces a scarce-resource allocation problem (which command owns which letters) this system has no need for.

**3. Up to three words = family (major.minor) + version (patch), assigned by trigger, not by count.**

- `dreamland oneiroi seed` always produces a **new family**: a fresh `word1-word2` pair, registered with no third word. This is the "major.minor" identity — analogous to a fresh `v1.0` release.
- `dreamland oneiroi revise --agent <name> --reason "<reason>"` draws **one new third word** (from the same flat pool, excluding the family's own two words) and either appends it (family currently 2-word) or replaces the existing third word (family already had one from a prior revision) — a family only ever carries at most one "current" third word, it does not accumulate a growing chain, mirroring how a semver patch number is replaced, not appended to, on each new patch release. The registry keeps prior third words in a `revisions` history array for audit, but the *live* name (used for git identity, hook `--agent-name`, slash command, file names) is always `word1-word2` or `word1-word2-word3` (current), never longer.
- `dreamland oneiroi fork --agent <parent> --role "<role>"` creates a **new registry entry** that inherits the parent's `word1-word2` pair verbatim (same family — the fork is recognizably related to its origin) plus a freshly drawn third word distinct from the parent's own current third word (if any) and from any sibling fork's third word for that family. The parent's own entry and files are untouched. This directly implements "a third name is added ... if a fork is made": the fork itself is what carries the third, distinguishing word; the family (major.minor) is shared and legible as a lineage.

This gives `word1-word2` a stable, human-recognizable "family" meaning (a set of agents descended from the same original persona) while `word3` distinguishes revisions/forks within that family — the semver analogy the acceptance criteria call for.

**4. Registry: `.dreamland/oneiroi/registry.json`, additive to the closed ten, not a replacement.**

```json
{
  "agents": [
    {
      "name": "amber-falcon",
      "words": ["amber", "falcon"],
      "role": "one-line role description",
      "tool_tier": "full-edit",
      "parent": null,
      "created": "2026-08-17T00:00:00Z",
      "revisions": []
    },
    {
      "name": "amber-falcon-onyx",
      "words": ["amber", "falcon", "onyx"],
      "role": "variant role description",
      "tool_tier": "full-edit",
      "parent": "amber-falcon",
      "created": "2026-08-20T00:00:00Z",
      "revisions": []
    }
  ]
}
```

`internal/agentidentity.Registered` becomes a function of two sources: the existing compiled-in map (unchanged, always present, works with no repo/file present — e.g. in a fresh `dreamland init` before any oneiroi is seeded) union with `name` entries read from `.dreamland/oneiroi/registry.json` if that file exists in the current repo (resolved via the same `config.FindRepoRoot` every other command already uses). `IsRegistered(name)` and any exported iteration (`Registered` as a map literal is replaced by a `Registered() map[string]bool` **function** — a source-breaking rename within this repo's own package, acceptable since nothing outside this repo consumes `internal/`) check both. This keeps the existing ten's behavior byte-identical when no registry file exists (every current caller, none of which pass a registry-aware context, keeps working), and makes a freshly seeded oneiroi immediately real to `coauthor`/telemetry/commit the moment `dreamland oneiroi seed` finishes — no rebuild required, unlike the ten built-ins. Alternative considered: keep `Registered` a package-level `var` and mutate it at `init()` time by reading the registry file eagerly — rejected because package `init()` performing repo-root discovery and file I/O is surprising/untestable global state; an explicit function call resolved at each use site (already how `IsRegistered` is called, once per invocation) is simpler to reason about and to test.

**5. `dreamland oneiroi seed` generalizes `internal/scaffold`'s installers rather than duplicating them.**

`installAgents`/`bindHooks`/`installCommands` (`internal/scaffold/scaffold.go`) are refactored so their platform-spec-driven core (`platformAgentSpec`, `platformCommandSpec`, the per-platform `installFlatAgents`/`installFlatCommands`/`installSkills` functions) accepts an explicit `(name, role, toolTier string)` in addition to today's fixed ten-name iteration — the fixed-ten call site becomes one caller of the now-generalized function, passing the ten names/tiers from the existing table; `dreamland oneiroi seed` is a second caller, passing the freshly generated name/role/tier. Neither caller's *content* generation changes: the fixed ten still render from their hand-authored embedded template files (`internal/scaffold/templates/agents/<platform>/<name>.<ext>`), because that content is persona prose no algorithm should invent. `dreamland oneiroi seed` instead renders a **stub** template per platform — the same frontmatter/hook-block shape (see decision 6) as an existing agent's file, but with an instruction body of a single placeholder comment (`<!-- TODO(hypnos): persona-specific instructions for <name> -->` or the platform's native comment syntax) for `hypnos` to fill in with `Edit`. This keeps "boilerplate is generated, prose is authored" as a clean division without inventing a second scaffolding code path.

**6. The generated stub's hook/frontmatter block is copied verbatim from the fixed-tier template, name-substituted.**

Per the `agent-scaffolding` capability's tool-tier matrix, the stub's `tools:`/capability grant and its `hooks:`/binding block (Claude Code `Stop` hooks calling `dreamland coauthor --hook --agent-name <name>`, `dreamland telemetry write --tool claude-code`, `dreamland version-bump --patch`, `dreamland commit --reason handoff --agent-name <name>`, and the Codex/Cursor/Kiro/Antigravity/GitHub-Copilot equivalents already defined per platform in `internal/scaffold/templates/hooks/bindings/`) are generated by substituting `<name>` into the exact same block shape an existing full-edit-tier agent's file already carries — satisfying the acceptance criteria's "adds its generated name every time routed to," "adds a coauthor of the model it uses" (unchanged `prepare-commit-msg`/`--trailer` mechanism, agent-agnostic), and "tracks and commits token burn" (unchanged `telemetry write`/`Tokens:` mechanism) requirements with no new logic — every oneiroi, old or new, goes through the identical `dev-workflow-hooks` machinery; only the `--agent-name`/frontmatter `name:` value differs, exactly as it already does across the existing ten.

**7. Self-authored commit provenance: a `Generated-By:` trailer, not a hook bypass.**

`dreamland oneiroi seed`/`revise`/`fork` each end by staging exactly the files they wrote/modified (an explicit path list, not `git add -A` — see Risks) and running `git commit --author "dreamland-oneiroi-seed <oneiroi-seed@github.com>" -F <tmpfile>`, where the message file already contains:

```text
oneiroi: seed amber-falcon (full-edit tier)

Tokens: input=0 output=0 cached=0 total=0
Generated-By: dreamland-oneiroi-seed
```

The installed `prepare-commit-msg` hook still fires unconditionally (no `--no-verify` — this repo's own conventions and the `harden-commit-hook-enforcement` change both treat skipping hooks as the wrong tool). `dreamland coauthor --trailer <file>` (`cmd/coauthor.go`) gains one new check, ordered before its existing `Co-authored-by:`-already-present idempotency check: if the message already contains a `Generated-By:` trailer, both the `Co-authored-by:` append and the `Tokens:` append are skipped entirely — the message is treated as complete and self-describing, the same "already present, don't duplicate/alter" posture the hook already takes for an existing `Co-authored-by:` line, just triggered by a different, purpose-built marker instead of relying on trailer-text sniffing that could accidentally match unrelated content. `--author` (not `git config user.name`) is used specifically so this never touches the repo-local identity `coauthor` set earlier in the session for the invoking agent (e.g. `hypnos`) — that identity must still be correct for `hypnos`'s *own* subsequent commits in the same turn. Alternative considered: temporarily swap `git config user.name`/`user.email`, commit, swap back — rejected as a needless global-mutable-state dance when `--author` does the same thing per-commit with no window where concurrent/hook-triggered git operations could observe the wrong identity.

**8. `dreamland mcp-serve` is a thin, additive wrapper — no new business logic.**

A minimal MCP server (stdio transport, JSON-RPC per the MCP spec) exposes three tools — `oneiroi_seed`, `oneiroi_revise`, `oneiroi_fork` — whose handlers call the exact same `internal/oneiroi` functions the `cmd/oneiroi.go` Cobra commands call, with tool-call arguments mapped 1:1 to the CLI flags. This satisfies "if needed, create the ability to access it as an MCP action" without introducing a second implementation to keep in sync: `cmd/mcp_serve.go`'s tool handlers are integration-only glue (arg parsing → `internal/oneiroi` call → JSON result), matching the same "logic lives in `internal/`, `cmd/` is a thin binding" pattern the rest of this codebase already follows for its hook commands. This is genuinely optional at the CLI-usage level — nothing in the seed/revise/fork flow requires MCP — so it ships as a separate, independently testable command that can be deferred without blocking the core scaffold flow; scoped last in `tasks.md` for that reason.

## Risks / Trade-offs

- [`git add -A` would be simpler but risks committing unrelated in-progress work under the script's non-agent authorship, misattributing a human/agent's pending edits] → Mitigated by staging an explicit path list (the registry file, the six new/updated per-platform stub or reference-update files, the routing-table edits, the slash command file) built from what the command itself wrote, never a blanket `-A`.
- [A 2-word pool exhausting collision-free combinations as the roster grows] → The DoD/partner code-name page has on the order of several hundred to low thousands of two-word entries, so the flat-token pool is large (many hundreds of distinct words); at this repo's realistic roster scale (tens of agents, not thousands), collision probability stays low and the 50-retry-then-loud-failure behavior (decision 2) surfaces exhaustion explicitly rather than degrading silently.
- [Making `agentidentity.Registered` a function instead of a `var` is a breaking change to that package's own exported surface] → Scoped and accepted: `internal/` packages have no consumers outside this repo; every in-repo call site is updated as part of this change's tasks, and existing tests (`agentidentity_test.go`'s `TestIsRegistered`) are updated alongside.
- [The Wikipedia source table's structure or content changes after `words.json` is generated, so re-running the generator later produces a different pool than what an already-seeded repo's collision history was checked against] → Not a correctness problem: collision checking is always against the *current repo's* `.dreamland/oneiroi/registry.json`, not against the word list's structure; a refreshed `words.json` only affects future draws, and stale-but-valid prior names remain valid.
- [Stub template files `hypnos` fills in could ship to a real repo half-finished if `hypnos`'s persona-prose step is skipped] → Out of scope for this change to prevent structurally; `hypnos`'s own updated instructions (this change's `agent-lifecycle-management` delta) state the stub is not a finished agent and its next deterministic step is filling in the body before handing off to `phobetor` for validation, consistent with its existing responsibility.

## Open Questions

- Should `dreamland oneiroi revise`/`fork` also require the caller to state *which platform template files changed persona-wise* (vs. just the name/hook boilerplate), or is that entirely `hypnos`'s judgment call to make in a follow-up edit after the command returns? This design assumes the latter (revise/fork touch only name/registry/hook-boilerplate; persona-prose edits, if any, are a separate `hypnos` edit step) — flagging in case a future change wants the CLI to also diff/carry-forward prose.
- Is a 50-attempt collision retry ceiling the right threshold, or should it be configurable via `.dreamland.json` for very large rosters? Left as a hardcoded constant for this change; revisit if the open ceiling ever actually triggers in practice.
