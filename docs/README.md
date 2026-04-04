# Documentation index

Long-form design and contracts live here; the root [README.md](../README.md) stays commands and copy-paste.

## What to read

| Doc | Use it for |
|-----|------------|
| [../AGENTS.md](../AGENTS.md) | Short map for humans and coding agents: where code lives, what to run before finishing, link-out to rules. |
| [../CONTRIBUTING.md](../CONTRIBUTING.md) | PR checklist, `.env.example`, API/client sync with `parseTaskApi`. |
| [../README.md](../README.md) | Prerequisites, build/test, run `dbcheck` / `taskapi`, dev scripts, npm commands for `web/`. |
| [DESIGN.md](./DESIGN.md) | `taskapi`: HTTP + SSE, env vars, `REPO_ROOT` / `/repo`, persistence, limits, Mermaid, and how to extend the stack (section Extensibility). |
| [WEB.md](./WEB.md) | `web/` SPA: React Query, SSE invalidation, `parseTaskApi`, `web/src` layout, tests. |
| [TROUBLESHOOTING.md](./TROUBLESHOOTING.md) | Dev-only: Vite `/tasks` refresh, SSE dev mode, `REPO_ROOT`, CI/local check failures. |
| [OBSERVABILITY.md](./OBSERVABILITY.md) | How we standardize, measure, and extend logging and correlation for `taskapi` (checklists, coverage script). |

Go: route lists and behavior next to code — `go doc` on `pkgs/tasks/...`, `pkgs/repo`, `internal/envload`, `cmd/taskapi`, `cmd/dbcheck`.

## Where to put updates

| Change | Update |
|--------|--------|
| Flags, env, `taskapi` routes or timeouts | `docs/DESIGN.md` + relevant `doc.go`; root `README` only if command-line examples change. |
| New tasks API behavior (domain / store / handler / web) | `docs/DESIGN.md` (Extensibility) + `.cursor/rules/13-tasks-stack-extensibility.mdc`; contract changes also `11-api-contracts`. |
| Task DB schema (GORM models, `postgres` migrate, SQLite test helpers, `dbcheck -migrate`) | `docs/DESIGN.md` (persistence) + `.cursor/rules/15-database-schema.mdc`. |
| `REPO_ROOT`, `/repo/*`, `pkgs/repo`, @-mention file UI | `docs/DESIGN.md` (Optional workspace repo) + `.cursor/rules/14-repo-workspace-extensibility.mdc`; contract changes also `11-api-contracts`. |
| `web/` only (components, hooks, no API contract change) | `docs/WEB.md`; root `README` only if npm scripts or env vars for Vite change. |
| Observability standard, measurement scripts, or `taskapi` log/checklist behavior | `docs/OBSERVABILITY.md`; touch `scripts/measure-func-slog.*` / `cmd/funclogmeasure` for the per-function `slog` audit, or `scripts/measure-observability.*` for test coverage scope. |
| `dbcheck` | Root `README` + `cmd/dbcheck` doc if flags change. |

Cursor rules (`.cursor/rules/`) are for tooling, not operators.
