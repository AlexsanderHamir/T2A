# ADR-0039: Separate GORM persistence models from domain types

**Date:** 2026-06-25  
**Status:** Proposed  
**Deciders:** Hamix maintainers  
**Tracking:** Issue #61 (follow-up to PR #56 engineering-structure work)

## Context

`pkgs/tasks/domain/models.go` (~400 lines) defines the task bounded-context entities
used by handlers, the store facade, harness, and the SPA wire contract. The same
structs carry:

- **Transport tags** — `json:"..."` for REST/SSE serialization.
- **GORM tags** — `gorm:"primaryKey"`, `check:...`, `serializer:json;type:jsonb`,
  `foreignKey` / `constraint:OnDelete:...`, composite `uniqueIndex`, custom
  `TableName()` methods.
- **GORM-only fields** — `gorm:"-"` hydration columns (`DependsOn`, `CreatedAt`),
  nested `*Project` preload associations, and `PendingRetry` with `json:"-"`.
- **Driver types** — `RunnerConfig datatypes.JSON` imports `gorm.io/datatypes`.

`postgres.Migrate` passes `&domain.Project{}`, `&domain.Task{}`, and ~20 sibling
domain structs directly to `AutoMigrate` ([`postgres.go`](../../pkgs/tasks/postgres/postgres.go)).
Store subpackages accept and return `domain.*` while issuing GORM queries against
those tagged structs.

Project standards require the opposite dependency direction:

| Rule | Source |
| --- | --- |
| `domain` imports stdlib only — no DB drivers, no HTTP | `CODE_STANDARDS.mdc` Part 8, `backend-engineering-bar.mdc` §2 |
| Stores implement domain contracts; schema lives outside domain | `CODE_STANDARDS.mdc` Part 3, `docs/domain/persistence.md` |
| Schema changes bump `SchemaRevision` and run through opt-in migrate | [ADR-0034](./ADR-0034-opt-in-schema-migration.md) |

The mismatch is longstanding pragmatism: one struct set avoids mapping boilerplate
and keeps JSON API shapes aligned with DB columns. As `models.go` grows (issue #59
split it by entity file but did not remove tags) and more packages import `domain`,
the leak of `gorm.io/datatypes` and check constraints into the inner ring becomes
harder to justify.

Enriched SSE and bootstrap reads already treat `domain.Task` as the public
post-commit snapshot ([ADR-0026](./ADR-0026-backend-data-coherence.md)). Any
separation must preserve that wire shape — persistence models are not a new
public API.

## Decision

**Recommend Option B (separate persistence models)** for acceptance and a
follow-up implementation issue. This ADR records the decision only; **no
production code changes** land in issue #61.

When Accepted:

1. **Domain structs become persistence-agnostic.** Remove all `gorm:"..."` tags,
   GORM association fields, `TableName()` methods, and `gorm.io/datatypes`
   from `pkgs/tasks/domain/`. Keep `json` tags, validation helpers, and enum
   `Scan`/`Value` in domain (stdlib `database/sql/driver` only).
2. **Store owns GORM models** under `pkgs/tasks/store/` (e.g.
   `store/models/` or `store/gormmodel/`), one file per table group mirroring
   today's `domain/models_*.go` split from issue #59.
3. **Explicit mapping** at the store boundary: `domainToModel` / `modelToDomain`
   (or small per-entity functions) in the store package. Handlers and harness
   continue to use `domain.*` only.
4. **`postgres.Migrate` AutoMigrate targets store models**, not `domain.*`.
   Post-migrate SQL steps in `postgres.go` remain; revision bump follows
   [ADR-0034](./ADR-0034-opt-in-schema-migration.md).
5. **JSON columns** on persistence models use `datatypes.JSON` or `[]byte` in
   store only; domain uses `[]string`, `map[string]any`, or typed structs.

### Implementation sketch (deferred)

| Area | Today | After Option B |
| --- | --- | --- |
| Entity definitions | `domain/models_*.go` with json+gorm | `domain/models_*.go` json only; `store/models/*.go` gorm only |
| `RunnerConfig` | `datatypes.JSON` on `domain.Task` | `json.RawMessage` or typed map in domain; `datatypes.JSON` on store `TaskModel` |
| Hydration (`DependsOn`, `CreatedAt`) | `gorm:"-"` on domain | Populated in `modelToDomain` after store reads/joins |
| Preload associations | `Project *Project` on domain with gorm FK tags | `TaskModel` carries GORM associations; mapper sets nested domain pointers if needed |
| Store internals | `db.Create(&task)` with `domain.Task` | `db.Create(domainToModel(task))`; reads `modelToDomain` |
| Tests / SQLite | `tasktestdb` AutoMigrate `domain.*` | AutoMigrate `store` models; helpers return `domain.*` |
| CI schema gate | Watches domain + postgres migrate files | Extend watch list to `store/models/` |

**Phasing suggestion:**

1. Introduce store models + mappers for leaf tables (e.g. `TaskEvent`,
   `AppSettings`) to prove the pattern.
2. Migrate high-traffic entities (`Task`, `Project`, cycles/phases) with
   table-driven round-trip tests.
3. Remove gorm tags from domain in the same PR series once all `AutoMigrate` and
   store paths use models.
4. Coordinate with any open work touching `domain/models_*.go` — do not
   double-edit across branches.

**Non-goals for the implementation issue:**

- Replacing GORM AutoMigrate with numbered SQL migrations (remains deferred per
  ADR-0034).
- Changing REST JSON field names or SSE enriched payload shape.

## Consequences

### Positive

- Restores documented dependency direction: domain is importable without GORM.
- Schema metadata (checks, FK `OnDelete`, jsonb serializers) colocated with
  store code that already owns transactions and error translation.
- Domain unit tests no longer pull `gorm.io/datatypes` transitively.
- Clear seam for future alternate persistence (read replicas, projection tables)
  without touching handler contracts.

### Negative / Trade-offs

- **Mapping boilerplate** for ~25 entities and hydration fields; must stay
  mechanical to avoid drift.
- **Dual struct maintenance** — new columns require domain field + model field +
  mapper updates (CI and code review discipline).
- **Large implementation PR** — likely multi-commit series; temporary duplication
  if migrated incrementally.
- **`SchemaRevision` / migrate CI** must include store model paths ([ADR-0034](./ADR-0034-opt-in-schema-migration.md)).

### If Proposed is rejected (Option A)

No change; document the pragmatic exception in `domain/doc.go` and relax the
lint/rule wording to "domain avoids persistence where possible."

## Alternatives Considered

| Alternative | Summary | Reason |
| --- | --- | --- |
| **A. Status quo** | Keep GORM tags on `domain` structs | Zero mapping cost; violates stated architecture; `gorm.io/datatypes` in domain; schema constraints live in inner ring |
| **B. Separate persistence models** (recommended) | Domain pure; store models + mappers; AutoMigrate store types | Matches CODE_STANDARDS; up-front mapper cost; industry-standard bounded-context split |
| **C. Tag-only split / code generation** | Single struct; tags injected via build tag or codegen | GORM reflects on struct tags at runtime — tags must exist on the type passed to `AutoMigrate`; codegen adds toolchain complexity without removing mapping for hydration fields |

## Related

- [ADR-0026: Backend data coherence](./ADR-0026-backend-data-coherence.md) — enriched SSE uses `domain.Task`; mappers must preserve wire fields.
- [ADR-0034: Opt-in schema migration](./ADR-0034-opt-in-schema-migration.md) — `SchemaRevision` bump when store models replace domain in `AutoMigrate`.
- [docs/domain/persistence.md](../domain/persistence.md) — store facade workflow after separation.
- Issue #59 — `models.go` file split (structure only; does not remove GORM tags).

## Acceptance (issue #61)

- [ ] ADR-0039 merged with clear recommendation (Option B) and consequences.
- [ ] No production code change in the ADR PR.
- [ ] Open implementation issue only if status moves to **Accepted**.
