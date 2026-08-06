## ADDED Requirements

### Requirement: Bare, unprefixed slash commands are installed alongside the drmlnd-prefixed set

In addition to the `drmlnd`-prefixed generic routing command and the nine `drmlnd`-prefixed per-agent commands (see the existing requirements in this capability), the scaffold installer SHALL install a second, bare-named (unprefixed) command for the same targets on every supported platform:

- Two bare generic entry points, `/dreamland` and `/janus`, both equivalent in content and behavior to the `drmlnd`-prefixed generic routing command (routes to Janus, which decides the target agent). Janus has no `drmlnd`-prefixed "force route directly to janus" counterpart to alias — forcing the destination to Janus is what the generic routing command already does — so both bare names are generated from the generic routing template, not a per-agent direct-route template.
- One bare per-agent command per non-router agent — `/phantasos`, `/nyx`, `/morpheus`, `/phobetor`, `/baku`, `/iktomi`, `/zhougong`, `/hypnos`, `/mengpo` — equivalent in content and behavior to its `drmlnd`-prefixed counterpart.

This is an additive alias, not a replacement: the `drmlnd`-prefixed set (and the "No unprefixed dreamland command artifacts remain after install" requirement governing it) is unchanged and continues to be installed. This intentionally reintroduces the possibility of a naming collision with a user's own pre-existing, unrelated command of the same bare name — see the next requirement for how that collision is handled.

Each bare command file is generated from the same template as its `drmlnd`-prefixed counterpart (the generic routing template for `/dreamland` and `/janus`, the matching per-agent template for the other nine), differing only in file location/frontmatter `name` (whichever the platform uses to determine invocation name) so there is exactly one prompt body per target, not two to keep in sync.

#### Scenario: Bare /dreamland command installed on Claude Code

- **WHEN** the selected coding tool is "Claude Code" and `dreamland init` completes successfully
- **THEN** `.claude/commands/dreamland.md` exists with the same routing instructions as `.claude/commands/drmlnd/route.md`

#### Scenario: Bare per-agent commands installed on Claude Code

- **WHEN** the selected coding tool is "Claude Code" and `dreamland init` completes successfully
- **THEN** `.claude/commands/nyx.md`, `.claude/commands/phobetor.md`, and the remaining seven bare per-agent command files all exist, each with the same explicit "route to `<agent>`" instruction as its `.claude/commands/drmlnd/<agent>.md` counterpart

#### Scenario: Bare /janus command installed on Claude Code

- **WHEN** the selected coding tool is "Claude Code" and `dreamland init` completes successfully
- **THEN** `.claude/commands/janus.md` exists with the same generic routing instructions as `.claude/commands/dreamland.md` and `.claude/commands/drmlnd/route.md`

#### Scenario: Bare commands installed alongside, not instead of, the drmlnd-prefixed set

- **WHEN** `dreamland init` completes successfully for any supported platform
- **THEN** both the bare command files and their `drmlnd`-prefixed equivalents exist — neither set is omitted

### Requirement: A pre-existing bare command not authored by dreamland is left untouched

Because bare command names are not namespaced, `dreamland init` SHALL NOT overwrite a bare command file that already exists at the target path and was not itself written by a prior `dreamland init` run. Detection: a dreamland-authored bare command file carries a recognizable marker (e.g. a generated-file comment or frontmatter field consistent with the rest of that platform's dreamland-authored files); a file lacking that marker is treated as user-owned and skipped, with a warning printed to stderr naming the skipped path and the agent it would have installed.

#### Scenario: User's own /janus command is not overwritten

- **WHEN** `.claude/commands/janus.md` already exists and does not carry the dreamland-authored marker, and `dreamland init` runs
- **THEN** the file is left unchanged and a warning is printed naming `.claude/commands/janus.md` as skipped

#### Scenario: A dreamland-authored bare command is safely updated on re-init

- **WHEN** `.claude/commands/janus.md` already exists and does carry the dreamland-authored marker (installed by a prior `dreamland init` run), and `dreamland init` runs again
- **THEN** the file is overwritten with the current template content, same as any other dreamland-managed file
