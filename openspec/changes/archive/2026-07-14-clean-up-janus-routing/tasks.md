## 1. Spec updates (pending, not-yet-archived janus-router-agent change)

- [x] 1.1 Update `openspec/changes/janus-router-agent/specs/janus-router-agent/spec.md`: apply this change's MODIFIED requirements (enumerated historical command/skill spellings in the routing table; iktomi keyword-disambiguation rule) and ADDED requirement (explicit out-of-scope refusal).
- [x] 1.2 Update `openspec/changes/janus-router-agent/specs/router-slash-commands/spec.md`: apply this change's ADDED requirements (legacy skill redirect behavior; cross-platform enumeration consistency).

## 2. Janus routing table and iktomi disambiguation (all six platforms)

- [x] 2.1 `internal/scaffold/templates/agents/claude-code/janus.md`: extend each routing-table bullet with historical `openspec-*` skill spellings; add the "mentions openspec vs. has no openspec context" disambiguation sentence.
- [x] 2.2 `internal/scaffold/templates/agents/claude-code/iktomi.md`: add the same disambiguation sentence so Iktomi's own definition of its trigger matches Janus's.
- [x] 2.3 `internal/scaffold/templates/agents/codex/janus.toml` and `iktomi.toml`: apply the same two edits in TOML prose fields.
- [x] 2.4 `internal/scaffold/templates/agents/cursor/janus.mdc` and `iktomi.mdc`: apply the same two edits.
- [x] 2.5 `internal/scaffold/templates/agents/kiro/janus.md` and `iktomi.md`: apply the same two edits.
- [x] 2.6 `internal/scaffold/templates/agents/antigravity/janus` and `iktomi`: apply the same two edits.
- [x] 2.7 `internal/scaffold/templates/agents/github-copilot/janus.agent.md` and `iktomi.agent.md`: apply the same two edits, preserving the platform's structural `agents:`/frontmatter requirements.

## 3. Janus scope guardrail (all six platforms)

- [x] 3.1 Add the "Janus refuses to act outside the dispatch role" instruction (decide-and-dispatch only; delegate rather than answer/implement/investigate beyond `openspec status`) to `claude-code/janus.md`.
- [x] 3.2 Add the equivalent guardrail text to `codex/janus.toml`, `cursor/janus.mdc`, `kiro/janus.md`, `antigravity/janus`, and `github-copilot/janus.agent.md`.

## 4. Legacy openspec-* skill reconciliation

- [x] 4.1 Rewrite `.claude/skills/openspec-propose/SKILL.md` as a redirect stub resolving to `phantasos`, matching `.claude/commands/opsx/propose.md`'s documented target.
- [x] 4.2 Rewrite `.claude/skills/openspec-explore/SKILL.md` as a redirect stub resolving to `phantasos`, matching `.claude/commands/opsx/explore.md`.
- [x] 4.3 Rewrite `.claude/skills/openspec-archive-change/SKILL.md` as a redirect stub resolving to `baku`, matching `.claude/commands/opsx/archive.md`.
- [x] 4.4 Rewrite `.claude/skills/openspec-apply-change/SKILL.md` as a redirect stub resolving to `janus` (which then chooses `nyx`/`morpheus` per task), matching `.claude/commands/opsx/apply.md`.
- [x] 4.5 Found `.github/skills/openspec-{propose,explore,archive-change,apply-change}/SKILL.md` (GitHub Copilot has the same auto-discoverable mechanism); applied the same redirect-stub treatment there, pointing at `.github/prompts/opsx-*.prompt.md`.

## 5. Cross-platform command/skill enumeration parity check

- [x] 5.1 Diffed per-agent commands/`/route`/`/opsx:*` across all six platforms. Claude Code and Cursor each have all nine per-agent commands plus `/route` (`.claude/commands/*.md`, `internal/scaffold/templates/commands/cursor/*.md`) — no gap. Codex CLI, Kiro, and Antigravity have no project-scoped command mechanism at all, by the documented design in `scaffold.go:49-58` (not a gap — a prior, deliberate architecture decision). **Gap found and left unresolved, needs a decision (see note below): GitHub Copilot.** `scaffold.go:58` states "GitHub Copilot and Antigravity have no public slash-command mechanism," but this repo has ad hoc `.github/prompts/opsx-{propose,explore,apply,archive}.prompt.md` and `.github/skills/openspec-*` (hand-authored, not templated in `internal/scaffold/templates/`), and zero per-agent (`/phantasos`, `/nyx`, etc.) or `/route` prompts for that platform. This is out of the scope this change's design.md set (design.md's goals were routing-text correctness and legacy-skill reconciliation, not building a new per-platform command surface) — flagging for the user rather than unilaterally authoring ~10 new prompt files or rewriting `scaffold.go`'s platform support.
- [x] 5.2 Confirmed: Janus's routing-table entries on every platform name the same target agent as each `/opsx:*` command's own definition file (`phantasos` for propose/explore, `baku` for archive, the `nyx`/`morpheus` two-flow for apply) — consistent across all six platforms after the section 2 edits.

## 6. Apply to this repo's own installed copies

- [x] 6.1 This repo has no `.claude/agents/` directory installed (only `.claude/commands/` and `.claude/skills/` are dogfooded here — the agent-file layer only exists under `internal/scaffold/templates/agents/`, for other repos that run `dreamland init`). So there were no live agent files in this repo to update beyond the templates already edited in sections 2-3; the only live-in-this-repo files affected by this change are the four legacy skill stubs (and their GitHub Copilot equivalents), both already rewritten directly in section 4.

## 7. Validation

- [x] 7.1 Traced "write an openspec change proposal for X" through the updated `claude-code/janus.md`: the `iktomi` bullet now reads on absence of proposal/task-list/spec-scenario context, not on the word "openspec," and the `phantasos` bullet covers proposal drafting — resolves to `phantasos`, not `iktomi`.
- [x] 7.2 Traced each rewritten legacy skill (`openspec-propose` → `phantasos`, `openspec-explore` → `phantasos`, `openspec-archive-change` → `baku`, `openspec-apply-change` → `janus`/`nyx`/`morpheus`) — each stub states the same target as its `/opsx:*` counterpart's own definition file.
- [x] 7.3 Re-ran `openspec status` on `clean-up-janus-routing` (4/4 artifacts complete) and `janus-router-agent` (4/4 artifacts complete) — both internally consistent after the spec edits.
