## 1. Spec updates (pending, not-yet-archived janus-router-agent change)

- [ ] 1.1 Update `openspec/changes/janus-router-agent/specs/janus-router-agent/spec.md`: apply this change's MODIFIED requirements (enumerated historical command/skill spellings in the routing table; iktomi keyword-disambiguation rule) and ADDED requirement (explicit out-of-scope refusal).
- [ ] 1.2 Update `openspec/changes/janus-router-agent/specs/router-slash-commands/spec.md`: apply this change's ADDED requirements (legacy skill redirect behavior; cross-platform enumeration consistency).

## 2. Janus routing table and iktomi disambiguation (all six platforms)

- [ ] 2.1 `internal/scaffold/templates/agents/claude-code/janus.md`: extend each routing-table bullet with historical `openspec-*` skill spellings; add the "mentions openspec vs. has no openspec context" disambiguation sentence.
- [ ] 2.2 `internal/scaffold/templates/agents/claude-code/iktomi.md`: add the same disambiguation sentence so Iktomi's own definition of its trigger matches Janus's.
- [ ] 2.3 `internal/scaffold/templates/agents/codex/janus.toml` and `iktomi.toml`: apply the same two edits in TOML prose fields.
- [ ] 2.4 `internal/scaffold/templates/agents/cursor/janus.mdc` and `iktomi.mdc`: apply the same two edits.
- [ ] 2.5 `internal/scaffold/templates/agents/kiro/janus.md` and `iktomi.md`: apply the same two edits.
- [ ] 2.6 `internal/scaffold/templates/agents/antigravity/janus` and `iktomi`: apply the same two edits.
- [ ] 2.7 `internal/scaffold/templates/agents/github-copilot/janus.agent.md` and `iktomi.agent.md`: apply the same two edits, preserving the platform's structural `agents:`/frontmatter requirements.

## 3. Janus scope guardrail (all six platforms)

- [ ] 3.1 Add the "Janus refuses to act outside the dispatch role" instruction (decide-and-dispatch only; delegate rather than answer/implement/investigate beyond `openspec status`) to `claude-code/janus.md`.
- [ ] 3.2 Add the equivalent guardrail text to `codex/janus.toml`, `cursor/janus.mdc`, `kiro/janus.md`, `antigravity/janus`, and `github-copilot/janus.agent.md`.

## 4. Legacy openspec-* skill reconciliation

- [ ] 4.1 Rewrite `.claude/skills/openspec-propose/SKILL.md` as a redirect stub resolving to `phantasos`, matching `.claude/commands/opsx/propose.md`'s documented target.
- [ ] 4.2 Rewrite `.claude/skills/openspec-explore/SKILL.md` as a redirect stub resolving to `phantasos`, matching `.claude/commands/opsx/explore.md`.
- [ ] 4.3 Rewrite `.claude/skills/openspec-archive-change/SKILL.md` as a redirect stub resolving to `baku`, matching `.claude/commands/opsx/archive.md`.
- [ ] 4.4 Rewrite `.claude/skills/openspec-apply-change/SKILL.md` as a redirect stub resolving to `janus` (which then chooses `nyx`/`morpheus` per task), matching `.claude/commands/opsx/apply.md`.
- [ ] 4.5 Check whether any other supported platform has an auto-discoverable skill mechanism equivalent to Claude Code's `.claude/skills/`; if so, apply the same redirect-stub treatment there for parity.

## 5. Cross-platform command/skill enumeration parity check

- [ ] 5.1 Diff the set of per-agent commands, `/route`, and `/opsx:*` commands across all six platform template directories; confirm each of the nine per-agent commands and `/route` exists on every platform, filing follow-up edits for any gap found.
- [ ] 5.2 Confirm Janus's routing-table entries on every platform name the same target agent for each `/opsx:*` command as that command's own definition file states (per the existing "Janus's own routing table stays consistent with the slash command definitions" requirement).

## 6. Apply to this repo's own installed copies

- [ ] 6.1 Since this repo dogfoods its own scaffolding, apply the same edits made in sections 2-4 directly to this repo's currently-installed `.claude/` files (not just the `internal/scaffold/templates/` sources future `dreamland init` runs draw from), so the fix is live immediately.

## 7. Validation

- [ ] 7.1 Manually trace a request containing the literal word "openspec" that clearly asks to draft a proposal (e.g. "write an openspec change proposal for X") through the updated Janus instructions and confirm it resolves to `phantasos`, not `iktomi`.
- [ ] 7.2 Manually trace an invocation of each legacy `openspec-*` skill name and confirm it resolves to the same target as its `/opsx:*` counterpart.
- [ ] 7.3 Re-run `openspec status` on this change and on `janus-router-agent` to confirm both remain internally consistent after the spec edits.
