# Documentation index

Long-form design and contracts live here; the root [README.md](../README.md) stays commands and copy-paste.

## What to read

| Doc | Use it for |
|-----|------------|
| [../AGENTS.md](../AGENTS.md) | Short map for humans and coding agents: where code lives, what to run before finishing, link-out to rules. |
| [../CONTRIBUTING.md](../CONTRIBUTING.md) | PR checklist, `.env.example`, API/client sync with `parseTaskApi`. |
| [../SECURITY.md](../SECURITY.md) | How to report vulnerabilities privately; notes on TLS and secrets. |
| [../LICENSE](../LICENSE) | MIT license for the repository. |
| [../README.md](../README.md) | Prerequisites, build/test, run `dbcheck` / `taskapi`, dev scripts, npm commands for `web/`. |
| [PRODUCT.md](./PRODUCT.md) | Product context: control-plane positioning (agent workflows vs IDE), what T2A provides, horizons, and how we choose scope (complements `DESIGN.md` hub). |
| [DESIGN.md](./DESIGN.md) | `taskapi` **hub**: goals, architecture Mermaid, limitations, out of scope; links to contract docs below. |
| [API-HTTP.md](./API-HTTP.md) | **Contract:** REST (`/tasks`, `/repo`, health, metrics), rate limits, idempotency, documented `400` strings. |
| [API-SSE.md](./API-SSE.md) | **Contract:** `GET /events`, SSE wire format, dev-only `T2A_SSE_TEST` vars. |
| [RUNTIME-ENV.md](./RUNTIME-ENV.md) | **Contract:** env var table, `dbcheck`, startup/shutdown, HTTP timeout constants. |
| [AGENT-QUEUE.md](./AGENT-QUEUE.md) | Ready-task notifier, `MemoryQueue`, reconcile loop, fairness ordering. |
| [AGENTIC-LAYER-PLAN.md](./AGENTIC-LAYER-PLAN.md) | **Long-term roadmap** (V2–V4) for evolving the in-process Cursor CLI worker into a reliable, multi-runner, multi-replica execution runtime. V0/V1 have shipped — see contract docs below. |
| [AGENT-WORKER.md](./AGENT-WORKER.md) | **Contract:** V1 in-process Cursor CLI worker — lifecycle, runner abstraction, env vars (`T2A_AGENT_WORKER_*`), security model (env allowlist, secret redaction, prompt hashing), audit shape, orphan sweep, and explicit V2/V3/V4 deferrals. Opt-in via `T2A_AGENT_WORKER_ENABLED`. |
| [EXECUTION-CYCLES.md](./EXECUTION-CYCLES.md) | **Contract:** `task_cycles` / `task_cycle_phases` substrate, dual-write invariant to `task_events`, phase state machine, "where reads go" table, what's intentionally out. |
| [proposals/](./proposals/) | **Forward-looking design docs** for features that have not yet shipped. Read `proposals/README.md` for what goes here. |
| [PERSISTENCE.md](./PERSISTENCE.md) | GORM store, `task_events`, concurrency, AutoMigrate scope. |
| [EXTENSIBILITY.md](./EXTENSIBILITY.md) | Vertical slice: domain → store → handler → `web/`. |
| [WEB.md](./WEB.md) | `web/` SPA: React Query, SSE invalidation, `parseTaskApi`, `web/src` layout, tests. |
| [TROUBLESHOOTING.md](./TROUBLESHOOTING.md) | Dev-only: Vite `/tasks` refresh, SSE dev mode, `REPO_ROOT`, CI/local check failures. |
| [OBSERVABILITY.md](./OBSERVABILITY.md) | How we standardize, measure, and extend logging and correlation for `taskapi` (checklists, coverage script, **Grafana / PromQL**, **SLIs / SLOs** starter table for `taskapi`). |
| [OBSERVABILITY-ROADMAP.md](./OBSERVABILITY-ROADMAP.md) | **Todos:** Prometheus/runtime/DB pool metrics, SLOs, alerts, OTel — execution order and principles. |
| [runbooks/](./runbooks/) | **Operator:** short notes for Prometheus alerts (`TaskAPIHighHTTP5xxRate`, latency, in-flight, DB pool, readiness); expand in roadmap B3. |
| [../deploy/prometheus/README.md](../deploy/prometheus/README.md) | **Prometheus:** `rule_files` for `t2a-taskapi-rules.yaml` (recording + alerting rules for `taskapi` metrics). |
| [REORGANIZATION-PLAN.md](./REORGANIZATION-PLAN.md) | **Principles** for keeping the codebase + docs layout consistent (dependency rules, non-goals, "what not to do"). The original phased reorg has shipped. |
| [HANDLER-SCALE.md](./HANDLER-SCALE.md) | **Maintainability:** why `handler` is large, what already moved out (`middleware`, `calltrace`, `middlewaretest`, `handlertest`, `httpsecurityexpect`), conventions for new tests, ordered next extractions. |

Go: route lists and behavior next to code — `go doc` on `pkgs/tasks/...`, `pkgs/repo`, `internal/envload`, `cmd/taskapi`, `cmd/dbcheck`.

## Where to put updates

| Change | Update |
|--------|--------|
| Product direction: who T2A is for, outcomes, horizons, explicit non-goals | `docs/PRODUCT.md`; keep `docs/DESIGN.md` (hub) Limitations / Out of scope in sync when strategy changes. |
| Flags, env, `taskapi` startup/shutdown | `docs/RUNTIME-ENV.md` + `docs/DESIGN.md` (hub) if limitations change; `internal/taskapiconfig` for taskapi-only parsed env (listen host, log level, agent queue/reconcile, dev SSE interval); `cmd/taskapi/README.md` for binary file layout; relevant `doc.go`; root `README` only if command-line examples change. |
| REST routes, bodies, query limits, `/repo` HTTP | `docs/API-HTTP.md` + `docs/DESIGN.md` (hub) if limitations change; contract changes also touch `web/src/api` / `parseTaskApi` per CONTRIBUTING. Handler layout: `pkgs/tasks/handler/README.md`; scaling/split conventions: `docs/HANDLER-SCALE.md`; `taskapi` middleware assembly: `internal/taskapi`. |
| SSE (`GET /events`), synthetic dev SSE | `docs/API-SSE.md`. |
| New tasks API behavior (domain / store / handler / web) | `docs/EXTENSIBILITY.md` + `.cursor/rules/13-tasks-stack-extensibility.mdc`; HTTP/SSE contract files above; client sync per CONTRIBUTING. |
| Task DB schema (GORM models, `postgres` migrate, SQLite test helpers, `dbcheck -migrate`) | `docs/PERSISTENCE.md` + `docs/DESIGN.md` (hub limitations as needed) + `.cursor/rules/15-database-schema.mdc`. |
| `REPO_ROOT`, `/repo/*`, `pkgs/repo`, @-mention file UI | `docs/API-HTTP.md` (Optional workspace repo) + `.cursor/rules/14-repo-workspace-extensibility.mdc`; client sync if response shapes change. |
| Ready-task queue / reconcile | `docs/AGENT-QUEUE.md` + `docs/RUNTIME-ENV.md` (`T2A_USER_TASK_AGENT_*`). |
| Agent worker behavior (Cursor CLI runner, lifecycle, security, audit) | `docs/AGENT-WORKER.md` (contract) + `docs/RUNTIME-ENV.md` (`T2A_AGENT_WORKER_*`) + `docs/AGENTIC-LAYER-PLAN.md` (V2–V4 roadmap). |
| Operator-run real-cursor smoke test for the V1 worker | `docs/AGENT-WORKER.md` "Smoke run" (operator runbook). |
| Execution cycles substrate (cycle/phase domain types, store entrypoints, `/tasks/{id}/cycles…` HTTP, `task_cycle_changed` SSE) | `docs/EXECUTION-CYCLES.md` (design + dual-write contract) + `docs/API-HTTP.md` (routes + 400 strings) + `docs/API-SSE.md` (event payload + trigger table). |
| Agentic worker lifecycle / future versions | `docs/AGENTIC-LAYER-PLAN.md` (versioned roadmap V2–V4) + `docs/AGENT-QUEUE.md` (when queue semantics change) + `docs/RUNTIME-ENV.md` (new worker env vars). New version-scoped execution playbooks land as `docs/<FEATURE>-PLAN.md` next to their contract doc once a `proposals/` design is accepted; delete the playbook after the version ships. |
| New feature proposals (designs not yet shipped) | `docs/proposals/<FEATURE>.md`. Once accepted and execution starts, promote the contract to `docs/<FEATURE>.md` and (optionally) add `docs/<FEATURE>-PLAN.md` for the per-stage execution. |
| `web/` only (components, hooks, no API contract change) | `docs/WEB.md`; root `README` only if npm scripts or env vars for Vite change. |
| Observability standard, measurement scripts, or `taskapi` log/checklist behavior | `docs/OBSERVABILITY.md`; touch `scripts/measure-func-slog.*` / `cmd/funclogmeasure` for the per-function `slog` audit, or `scripts/measure-observability.*` for test coverage scope. |
| New Prometheus recording/alert rules or runbook links for `taskapi` | `deploy/prometheus/t2a-taskapi-rules.yaml` + `deploy/prometheus/README.md`; alert text in `docs/runbooks/`; cross-link from `docs/OBSERVABILITY.md`. |
| `dbcheck` | Root `README` + `cmd/dbcheck` doc if flags change. |

Cursor rules (`.cursor/rules/`) are for tooling, not operators.
