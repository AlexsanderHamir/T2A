# Architecture Decision Records

Historical **why** behind structural and behavioral choices. Schema, routes, and env vars stay authoritative in [data-model.md](../data-model.md), [api.md](../api.md), and [configuration.md](../configuration.md).

| ADR | Status | Topic |
| --- | --- | --- |
| [ADR-0039](./ADR-0039-domain-persistence-separation.md) | Proposed | Separate GORM models from `domain/` types (issue #61; follow-up to PR #56) |
| [ADR-0038](./ADR-0038-shared-git-exec-core.md) | Accepted | Shared `pkgs/gitcore` for git subprocess I/O |
| [ADR-0037](./ADR-0037-global-repos-project-tree.md) | Accepted | Global repos, worktree/branch tree |
| [ADR-0034](./ADR-0034-opt-in-schema-migration.md) | Accepted | Opt-in `AutoMigrate`, `SchemaRevision` |
| [ADR-0026](./ADR-0026-backend-data-coherence.md) | Accepted | Read policy + enriched SSE |

Older records: `ADR-0001` … `ADR-0036` in this directory (filename order).

## See also

- [guide.md](../guide.md) § When to read domain articles and ADRs
- [domain/README.md](../domain/README.md) — behavioral deep-dives (how), not decision log (why)
