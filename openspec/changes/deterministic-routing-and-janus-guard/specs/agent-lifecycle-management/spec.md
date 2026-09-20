## ADDED Requirements

### Requirement: Registering or retiring an agent keeps Janus's Claude Code dispatch allowlist and the route roster coherent

Janus's Claude Code definition grants `Agent(<dispatch targets>)` (see the `janus-router-agent` capability), so the roster has one more registration point than before. The complete set of touchpoints for a roster change is:

| Touchpoint | Written by | Read by |
| --- | --- | --- |
| `.dreamland/oneiroi/registry.json` | `dreamland oneiroi seed`/`fork`/`revise` | identity resolution, `dreamland route` (valid targets, `roster`) |
| `janus.*` routing-table line, all six platforms | `dreamland oneiroi seed` stub edge, finalized by `hypnos`; removed by `mengpo` | the human-readable routing table; the drift test in the `deterministic-routing` capability |
| `Agent(...)` list in `.claude/agents/janus.md` and its template | `dreamland oneiroi seed`/`fork` (adds the new name in the same operation that adds the stub routing-table edge); `mengpo` (removes the name) | Claude Code, when Janus is the main-thread agent |
| per-agent slash command | `dreamland oneiroi seed`/`fork`; removed by `mengpo` | the user; carries `--to` semantics on Claude Code |

`dreamland oneiroi seed` and `fork` SHALL add the new agent's name to the `Agent(...)` list in the Claude Code Janus template and in the installed `.claude/agents/janus.md` in the same operation that adds the routing-table stub edge, keeping the list in registry order and never adding `janus`. `mengpo`'s archive and hard-delete modes SHALL remove the name from both. `dreamland route` needs no registration step of its own (see the `deterministic-routing` capability): it reads the registry, so a new agent is a valid `--to` target and appears in `roster` as soon as the registry write succeeds.

A Go test SHALL fail when the set of names in the `Agent(...)` list differs from the registered dispatch-target set (the registry, excluding `janus`), so a roster change that misses this touchpoint cannot ship.

`hypnos`'s step "Finalize the new agent's placement in all six `janus.*` files' routing tables" is unchanged; the `Agent(...)` entry is mechanical and is not a placement decision, so `hypnos` does not hand-edit it.

#### Scenario: Seeding an agent adds it to Janus's dispatch allowlist

- **WHEN** `dreamland oneiroi seed --role "example role"` generates `amber-falcon` on a repository scaffolded for Claude Code
- **THEN** `.claude/agents/janus.md`'s `tools` frontmatter includes `amber-falcon` inside `Agent(...)` and still excludes `janus`
- **AND** the same edit is present in the Claude Code Janus template

#### Scenario: Retiring an agent removes it from Janus's dispatch allowlist

- **WHEN** `mengpo` archives or deletes `amber-falcon`
- **THEN** `amber-falcon` no longer appears in `.claude/agents/janus.md`'s `Agent(...)` list, its routing-table line, or `dreamland route`'s `roster`

#### Scenario: A missed touchpoint fails the build

- **WHEN** an agent is present in `.dreamland/oneiroi/registry.json` but missing from the `Agent(...)` list
- **THEN** the drift test fails

#### Scenario: A seeded agent is dispatchable by a main-thread Janus

- **WHEN** a session launched with `claude --agent janus` is asked to route to `amber-falcon` (via `dreamland route --to amber-falcon`)
- **THEN** Janus's `Agent(...)` grant permits dispatching `amber-falcon` without any manual edit
