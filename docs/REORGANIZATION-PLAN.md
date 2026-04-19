# Reorganization principles

The phased reorganization (M1–M5: split `DESIGN.md`, add per-package READMEs, extract `internal/taskapi`, `pkgs/tasks/middleware`, `pkgs/tasks/logctx`, `pkgs/tasks/apijson`, `internal/taskapiconfig`) has shipped. This doc retains only the forward-looking principles so future structural work stays consistent with the layout we landed on. Look in [`docs/proposals/`](./proposals/) for any new structural proposal before adding boxes to the model below.

## Dependency direction (must hold)

```text
cmd/taskapi, cmd/dbcheck
    → internal/envload, internal/taskapiconfig
    → pkgs/agents          (in-process queue + worker; no import of handler)
    → pkgs/tasks/handler   (HTTP + SSE; imports store, domain, repo)
    → pkgs/tasks/middleware (composable HTTP layers)
    → pkgs/tasks/store     (DB; imports domain)
    → pkgs/tasks/postgres  (dialect, migrate, open)
    → pkgs/tasks/domain    (types, sentinels; no store/handler)
    → pkgs/repo            (optional workspace)
```

**Rules**

- **`domain`** must not import `store`, `handler`, `postgres`, or `agents`.
- **`store`** must not import `handler`.
- **`agents`** may import `store` for reconcile / worker typing; must not import `handler`.

## Non-goals (do not pursue without an explicit proposal)

- Rewriting persistence to raw `database/sql` everywhere — GORM stays.
- Microservices or extracting `taskapi` into multiple binaries.
- Perfect DDD — aim for **boring, legible** Go layout.

## What not to do

- Do not rename the `module` path or move `domain` types casually — breaks importers and mental model.
- Do not split packages purely by line count mid-function.
- Do not duplicate API tables across `README`, `DESIGN.md`, and `API-HTTP.md` — keep a **single authoritative table** per concern.
- Do not add a new `handler/middleware` subpackage without a clear ownership win; the `pkgs/tasks/middleware` extraction already covers cross-cutting layers.

## Success metrics (lightweight)

- **Time-to-answer** for "where is X implemented?" stays small (informal: new contributor trial).
- **PR conflict rate** on docs stays low (do not let `DESIGN.md` regrow into a single mega-file).
- **`docs/DESIGN.md` hub** stays a short index, not a narrative.
