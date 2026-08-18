## ADDED Requirements

### Requirement: Per-platform installers accept an externally supplied agent name and tool tier

In addition to installing the fixed ten built-in agents from their hand-authored embedded templates, `internal/scaffold`'s per-platform installer functions SHALL expose an entry point that accepts an arbitrary `(name, role, toolTier)` triple supplied by the caller at run time, and renders a stub template (frontmatter/capability grant matching the requested tier's row in the tool-binding matrix below, hook/binding block substituted with the supplied name, and a placeholder instruction body) at the same standard per-platform agent-file path and format the fixed ten use, for all six supported platforms. This is the mechanism `dreamland oneiroi seed` (see the `oneiroi-seed-naming` capability) uses to scaffold a newly seeded agent; it does not alter how the fixed ten are installed, and the fixed ten's hand-authored template content is unaffected.

`toolTier` SHALL accept one of the four tiers already defined by the tool-binding matrix (`router`, `read-dispatch-only`, `full-edit`, `write-only-no-edit`) and SHALL apply the corresponding `Edit`/`Write`/`Read`+`Bash` grant exactly as that matrix specifies for the fixed ten.

#### Scenario: Externally supplied name renders a stub on every platform

- **WHEN** the generalized installer entry point is called with `name="amber-falcon"`, `role="example role"`, `toolTier="full-edit"`
- **THEN** a stub agent file is written for `amber-falcon` at the standard path on all six supported platforms, each with `Edit`/`Write`/`Read`/`Bash` granted per the `full-edit` tier row

#### Scenario: Fixed-ten install path is unchanged

- **WHEN** `dreamland init` installs the fixed ten built-in agents
- **THEN** their template content is still rendered from `internal/scaffold/templates/agents/<platform>/<name>.<ext>`, not from the generalized stub-rendering path
