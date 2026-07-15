## Context

Every implementation flow in this project except agent-roster maintenance is spec-driven: Janus dispatches an entry request to `phantasos`, which drafts `proposal.md`/`design.md`/`tasks.md`; Janus (or a direct hand-off) then dispatches the resulting tasks to an implementer (`nyx`→`morpheus` or `morpheus` directly); `phobetor` validates; `baku` closes. Agent-roster requests break this pattern today: `janus.md`'s routing table sends "author a new agent" straight to `hypnos` and "archive/delete an unneeded agent" straight to `mengpo`, with no drafted proposal in between. `zhougong.md` compounds this — when its report recommends a new agent, `zhougong` itself decides the recommendation is "specific and unambiguous enough" and hands off straight to `hypnos`, meaning the agent whose stated job is analysis (`agent-lifecycle-management` spec: "Zhou Gong generates agent-performance reports...") is also the one deciding what change to make and skipping spec drafting entirely.

This is inconsistent with the project's own dogfooded workflow (confirmed while drafting the `agentic-sdlc-readme` change's Agent Building workflow section) and was the trigger for this change.

## Goals / Non-Goals

**Goals:**
- Route agent-roster requests (create or retire) through `phantasos` for spec drafting, the same way every other change is drafted, rather than jumping straight to the implementer.
- Narrow `zhougong` back to analysis only: it produces a report and, when the report clearly recommends a new agent, hands off to `phantasos` — it no longer originates the actual change.
- Reuse the existing `nyx`/`morpheus` dispatch shape rather than inventing new machinery: `hypnos` and `mengpo` become task-implementers dispatched by Janus via `/opsx:apply`, exactly like `morpheus` is dispatched for code tasks, differing only in that their tasks describe an agent to create or retire instead of code to write.

**Non-Goals:**
- No change to `phobetor` or `baku` — both already handle hand-offs generically regardless of what kind of task was completed (`phobetor`'s "Hypnos hands off directly to Phobetor to validate a newly authored agent" scenario already exists in `agent-lifecycle-management`; `baku`'s closure logic doesn't inspect task content).
- No change to `mengpo`'s archive/hard-delete mechanics, its default report-to-Janus terminal behavior, or its broad-routing capability (it can still hand off directly to another agent when archival reveals a stale reference).
- No change to `hypnos`'s six-platform authoring steps or its deterministic hand-off to `phobetor`.
- No new CLI commands, no Go source changes.

## Decisions

**`phantasos` does not gain a new fixed hand-off edge to `hypnos`/`mengpo` — it reports to Janus, same as it does for every other change.** Alternative considered: give `phantasos` a new deterministic edge straight to `hypnos`/`mengpo` (mirroring `nyx`→`morpheus`), which was the shape floated in conversation (`janus`→`zhougong`→`phantasos`→`hypnos`→`phobetor`→`baku`). Rejected: today, `phantasos` never hands off directly to `morpheus` either — it drafts, then reports back to Janus, and Janus makes a *separate* routing decision (which flow, which implementer) per the `janus-router-agent` spec's "Janus chooses between the acceptance-test and direct-implementation flows per task" requirement. Giving `phantasos` a new fixed edge only for the agent-roster case would special-case it inconsistently with how it already works for features. Keeping `phantasos` uniform (always drafts, always reports to Janus) means the only new thing Janus's routing table needs is to recognize `hypnos`/`mengpo` as valid `/opsx:apply` targets alongside `nyx`/`morpheus` — no new agent-to-agent edge, no new frontmatter `agents:` graph changes on GitHub Copilot (`phantasos.agent.md`'s `agents: [janus]` stays exactly as-is).

**`hypnos`/`mengpo` triggers change from "a raw request" to "a task from a `phantasos`-authored change."** This is what makes `phantasos`'s involvement real rather than cosmetic — `hypnos` now reads the agent's role and rationale out of `tasks.md`/`design.md` the way `morpheus` reads implementation instructions out of `tasks.md`, instead of receiving a bare request forwarded by Janus.

**`zhougong`'s hand-off target changes from `hypnos` to `phantasos`, nothing else about `zhougong` changes.** It still writes the same report to `.dreamland/reports/`, still includes the same "recommended new agent" section, still has no `Edit` tool. Only the terminal hand-off decision (`hypnos` → `phantasos`) changes, and the accompanying prose now says explicitly that `phantasos` is the one who turns the recommendation into a drafted change — `zhougong` stops being the agent that (implicitly) decides a change should happen; it only ever proposes.

**Janus's routing table gains one new bullet (`hypnos`/`mengpo` as `/opsx:apply` targets for agent-roster tasks) and edits two existing bullets (`phantasos` now explicitly covers agent-roster specs; the old direct `hypnos`/`mengpo` bullets are replaced).** No frontmatter changes on any platform: `janus.*`'s `agents:`/tool-graph declarations (where present, GitHub Copilot) already list `hypnos` and `mengpo` as dispatch targets, since Janus already dispatches to them directly today — that stays true, only the *reason* Janus dispatches to them changes (a pending `/opsx:apply` task, not a raw request).

## Risks / Trade-offs

- [Risk] `phobetor`'s pass/fail branching (`baku` on pass / `morpheus` on impl bug / `phantasos` on spec defect) is framed around code tasks and tests; it's not obvious what "spec defect" or "implementation bug" means when validating a newly authored agent definition. → Not introduced by this change (`hypnos`→`phobetor` already exists today) — out of scope, flagged here for a future change rather than silently left undocumented.
- [Risk] Existing repos that already have `.dreamland.json` installed with the old templates won't get this change until they re-run `dreamland init --force` or are otherwise re-scaffolded. → No migration needed for this repo itself (templates only affect newly-scaffolded target repos); acceptable since this is a template-source change, not a runtime behavior change.
- [Risk] The `agentic-sdlc-readme` change's Agent Building workflow section (already drafted) describes the old shape (`phantasos`→`hypnos` as a direct hop). → Mitigation: revisit that change's workflow requirement once this change is applied, to describe `hypnos`/`mengpo` as `/opsx:apply` targets dispatched by Janus rather than a direct `phantasos` hand-off.

## Migration Plan

Template-file edits only, applied directly to the source templates under `internal/scaffold/templates/agents/`. No data migration. Existing target-repo installs are unaffected until re-scaffolded.

## Open Questions

None outstanding — proceeding to specs/tasks.
