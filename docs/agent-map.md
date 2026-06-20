# Code path index

Repository paths grouped by subsystem. Read only the rows relevant to your task.

| | |
| --- | --- |
| **Applies to** | Locating code when editing the repo |
| **Audience** | Contributors and agents (after [AGENTS.md](../AGENTS.md) scoped path) |
| **Prerequisite** | Pick a scoped path in [AGENTS.md](../AGENTS.md) first |

## In this article

- [Overview](#overview)
- [Backend](#backend)
- [Web](#web)
- [Infra and test helpers](#infra-and-test-helpers)
- [See also](#see-also)

## Overview

> **Tip** — Open one to three rows below. API contracts: [api.md](./api.md). System overview: [architecture.md](./architecture.md).

## Backend

| Area | Path | Purpose | Deep dive |
| --- | --- | --- | --- |
| HTTP API + SSE | `pkgs/tasks/handler/` | REST handlers, SSE hub wiring, route registration | [handler/README.md](../pkgs/tasks/handler/README.md), [domain/sse-hub.md](./domain/sse-hub.md), [contributing.md — Handler growth](./contributing.md#when-the-handler-package-grows) |
| Domain types | `pkgs/tasks/domain/` | Task model, status/priority enums, cycles, validation, retry | [data-model.md](./data-model.md) |
| Persistence | `pkgs/tasks/store/`, `pkgs/tasks/postgres/` | Store facade, GORM migrate, dual-write to `task_events` | [domain/persistence.md](./domain/persistence.md), [store/README.md](../pkgs/tasks/store/README.md) |
| Task scheduling | `pkgs/tasks/scheduling/` | Worker readiness predicates, pickup gate, post-commit notify | [domain/task-scheduling.md](./domain/task-scheduling.md), ADR-0023 |
| Execution cycles HTTP | `pkgs/tasks/handler/handler_cycles.go` | Cycle and phase REST surface | [api.md](./api.md), [data-model.md](./data-model.md) |
| Operator retry | `handler_tasks_retry.go`, `domain/retry.go`, `harness/retry_run.go` | Start over / resume after failure | [retry-start-over.md](./domain/retry-start-over.md), [retry-resume.md](./domain/retry-resume.md) |
| Read/write policy | `pkgs/tasks/handler/readpolicy/`, `writepolicy/` | Bootstrap limits, commit-then-notify SSE enrichment | ADR-0026 |
| Workspace search | `pkgs/repo/` | Optional `@`-mentions and `/repo/*` file search | [domain/workspace-repo.md](./domain/workspace-repo.md) |
| Agent queue | `pkgs/agents/` (notifier hook) | Ready-task enqueue, reconcile tick, queue cap | [domain/agent-queue.md](./domain/agent-queue.md) |
| Agent harness | `pkgs/agents/harness/` | Execute/verify loop, criteria, git integrity, retry modes | [domain/harness.md](./domain/harness.md), [cursor-session-resume.md](./domain/cursor-session-resume.md) |
| Cycle commits | `harness/internal/git/commits.go`, `store/internal/commits/` | Agent-claimed commit ledger for verify | [cycle-commits.md](./domain/cycle-commits.md), ADR-0032 |
| Agent worker | `pkgs/agents/worker/` | Single-goroutine queue consumer; calls harness | [configuration.md](./configuration.md) |
| Worker supervisor | `internal/taskapi/agentworker/` | Boot, reload, probe, hot-swap worker from settings | [domain/agent-supervisor.md](./domain/agent-supervisor.md) |
| Agent runner stack | `pkgs/agents/runner/` | `Runner` interface, cursor adapter, registry, adapterkit, runnerfake | [domain/runner-adapters.md](./domain/runner-adapters.md) |
| App settings | `pkgs/tasks/store/internal/settings/` | Singleton `app_settings` row (id=1) | [configuration.md](./configuration.md) |
| Request logging | `pkgs/tasks/logctx/`, `pkgs/tasks/calltrace/` | `request_id`, `log_seq`, `call_path` in logs | [observability-trace-lines.md](./domain/observability-trace-lines.md) |
| JSON response helpers | `pkgs/tasks/apijson/` | Security headers, `WriteJSONError` | — |
| HTTP middleware | `pkgs/tasks/middleware/`, `internal/taskapi/` | Middleware stack, server assembly | [middleware/README.md](../pkgs/tasks/middleware/README.md) |
| Dev SSE simulation | `pkgs/tasks/devsim/` | `T2A_SSE_TEST` synthetic events for local UI | [api.md](./api.md), [configuration.md](./configuration.md) |

## Web

| Area | Path | Purpose | Deep dive |
| --- | --- | --- | --- |
| API client | `web/src/api/` | All `fetch` calls; `parseTaskApi*` parsers | [web.md](./web.md) |
| Task sync (SSE) | `web/src/tasks/sync/` | Cache invalidation, debounce, enrichment on SSE | [web.md](./web.md) §Task sync, ADR-0022 |
| Query keys / hooks | `web/src/tasks/task-query/` | TanStack Query keys, list/detail hooks, SSE bridge | [web.md](./web.md) |
| Query policy | `web/src/tasks/queryPolicy.ts`, `tasks/mutations/`, `tasks/checklist/`, `tasks/app/` | StaleTime tiers, guarded mutations, TasksAppProvider | ADR-0025 |
| Task create flow | `web/src/tasks/create/`, `components/task-create-modal/` | Compose payload, drafts, modal policy | [web.md](./web.md) §Task create, ADR-0024 |
| Task pages | `web/src/tasks/pages/` | Route-level containers (home, detail, templates, commits) | [web.md](./web.md) §Routes |
| Task list UI | `web/src/tasks/components/task-list/` | Home table, filters, bulk actions, stats | [web.md](./web.md) |
| Task detail UI | `web/src/tasks/components/task-detail/` | Header, schedule, cycles, commits, checklist panels | [web.md](./web.md) §Task detail |
| Task display helpers | `web/src/tasks/task-display/` | Shared labels, edit guards, priority display | — |
| Task templates | `web/src/api/taskTemplates.ts`, `pages/TaskTemplatesPage.tsx` | Template CRUD and batch instantiate | [api.md](./api.md), [web.md](./web.md) |
| App shell | `web/src/app/` | Router, providers, bootstrap, global styles | [web.md](./web.md) |
| Projects / settings | `web/src/projects/`, `web/src/settings/` | Non-task feature modules | [web.md](./web.md) §Routes |

## Infra and test helpers

| Area | Path | Purpose | Deep dive |
| --- | --- | --- | --- |
| Binaries | `cmd/taskapi/`, `cmd/dbcheck/` | Entrypoints only; wire deps and run | [cmd/taskapi/README.md](../cmd/taskapi/README.md) |
| Env loading | `internal/envload/` | Resolve `.env` from repo root | [configuration.md](./configuration.md) |
| taskapi config | `internal/taskapiconfig/` | Listen host, log level, queue cap, dev SSE interval | [configuration.md](./configuration.md) |
| SQLite test DB | `internal/tasktestdb/` | In-memory GORM for default Go tests | [contributing.md — Tests](./contributing.md#tests) |
| Middleware tests | `internal/middlewaretest/` | Black-box tests for middleware stack | [middleware/README.md](../pkgs/tasks/middleware/README.md) |
| Handler tests | `internal/handlertest/` | Black-box HTTP tests for exported handler API | — |
| Security header tests | `internal/httpsecurityexpect/` | Shared baseline header assertions | — |
| Agent reconcile tests | `pkgs/tasks/agentreconcile/` | SQLite integration tests; not production code | — |

## See also

- [guide.md](./guide.md) — documentation layers and learning paths
- [README.md](./README.md) — doc index by topic
- [AGENTS.md](../AGENTS.md) — scoped paths when editing
- [AGENTS.md](../AGENTS.md) §Where to find X — intent-based lookup (route, harness, sync, …)
