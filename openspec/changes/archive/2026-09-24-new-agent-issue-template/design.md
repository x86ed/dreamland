## Context

Recommendations from zhougong reports need a reviewable, structured, durable form. GitHub issues are already the project's collaboration surface (`gh` is in the permission allowlist).

## Decisions

1. **Issue form (YAML), not markdown template**: required fields and a tier dropdown are enforced by GitHub. Trade-off: forms cannot be prefilled by URL for all fields, so programmatic creation builds the body itself using the same section headings.
2. **One core function, two entry points** (`internal/agentissue.Create`) called by the Cobra command and the MCP tool, mirroring how oneiroi's `*Core` functions serve both CLI and MCP. `gh` is invoked through an injectable `runGh` var for tests.
3. **No default Stop hook**: firing an issue on every turn would spam the tracker. "Hook" is satisfied by the command being safely callable from any lifecycle hook (exit codes are deterministic) and by init writing the template.
4. **Template file is written by init and refreshed by `--template`** from an embedded copy in `internal/scaffold/templates`.

## Risks

- `gh` not authenticated: surfaced as a clear error, never a partial issue.
- Duplicate issues: `--create` first runs `gh issue list --label new-agent --search "<name> in:title"` and errors on an exact title match.
