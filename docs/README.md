# Documentation

This folder holds product context, architecture notes, and stable contracts. The root [README.md](../README.md) stays focused on install, build, and run commands.

## Start Here

Read these first for the current MVP shape:

1. [PRODUCT.md](./PRODUCT.md) — what T2A is for and how scope is chosen.
2. [DESIGN.md](./DESIGN.md) — architecture hub, data flow, technical choices, limitations.
3. [API-HTTP.md](./API-HTTP.md) and [API-SSE.md](./API-SSE.md) — server contracts.
4. [PROJECT-CONTEXT.md](./PROJECT-CONTEXT.md) — long-lived projects, shared context, and task run snapshots.
5. [WEB.md](./WEB.md) — SPA structure and client data flow.
6. [TROUBLESHOOTING.md](./TROUBLESHOOTING.md) — common local and CI failures.

Use [AGENTS.md](../AGENTS.md) for the short repo map and verification checklist.

## Stable Contracts

These describe behavior that code and tests should keep in sync.

| Area | Docs |
| --- | --- |
| HTTP API | [API-HTTP.md](./API-HTTP.md) |
| SSE events | [API-SSE.md](./API-SSE.md) |
| Runtime config | [RUNTIME-ENV.md](./RUNTIME-ENV.md) |
| Persistence | [PERSISTENCE.md](./PERSISTENCE.md) |
| Settings | [SETTINGS.md](./SETTINGS.md) |
| Project context | [PROJECT-CONTEXT.md](./PROJECT-CONTEXT.md) |
| Execution attempts | [EXECUTION-CYCLES.md](./EXECUTION-CYCLES.md) |
| Agent queue | [AGENT-QUEUE.md](./AGENT-QUEUE.md) |
| Agent worker | [AGENT-WORKER.md](./AGENT-WORKER.md) |
| Web SPA | [WEB.md](./WEB.md) |

## Build and Extend

Use these when changing the shape of the system.

| Doc | Use it for |
| --- | --- |
| [EXTENSIBILITY.md](./EXTENSIBILITY.md) | Add a new feature slice end-to-end. |
| [HANDLER-SCALE.md](./HANDLER-SCALE.md) | Understand handler package split rules and test placement. |
| [REORGANIZATION-PLAN.md](./REORGANIZATION-PLAN.md) | Historical layout principles; keep only as a structural reference. |

## Operations

Keep this section lightweight for MVP. Runtime metrics exist, but checked-in deploy dashboards and alert rules are intentionally out of the repo.

| Doc | Use it for |
| --- | --- |
| [OBSERVABILITY.md](./OBSERVABILITY.md) | Logging, metrics, correlation, and measurement scripts. |
| [SLOs.md](./SLOs.md) | Realtime UX SLO definitions and RUM sources. |
| [REALTIME.md](./REALTIME.md) | Shipped realtime/SSE smoothness decisions. |
| [runbooks/](./runbooks/) | Optional operator playbooks if you wire alerts. |

## Future Work

These are not current runtime contracts.

| Doc | Use it for |
| --- | --- |
| [AGENTIC-LAYER-PLAN.md](./AGENTIC-LAYER-PLAN.md) | Long-term V2–V4 worker roadmap. |
| [OBSERVABILITY-ROADMAP.md](./OBSERVABILITY-ROADMAP.md) | Future observability work such as OTel. |
| [proposals/](./proposals/) | Designs that have not shipped. |
| [future-considerations/](./future-considerations/) | Scaling notes and deferred ideas. |

## Where To Update

| Change | Update |
| --- | --- |
| Product scope or non-goals | [PRODUCT.md](./PRODUCT.md), then [DESIGN.md](./DESIGN.md) if limitations change. |
| Flags, env, startup, shutdown | [RUNTIME-ENV.md](./RUNTIME-ENV.md), root [README.md](../README.md) if commands change. |
| REST routes or JSON shapes | [API-HTTP.md](./API-HTTP.md), and matching `web/src/api` parsers if the SPA consumes it. |
| SSE wire format or triggers | [API-SSE.md](./API-SSE.md). |
| Database model or migration behavior | [PERSISTENCE.md](./PERSISTENCE.md), plus [DESIGN.md](./DESIGN.md) if limitations change. |
| Settings UI or settings API | [SETTINGS.md](./SETTINGS.md), [API-HTTP.md](./API-HTTP.md), and [WEB.md](./WEB.md) if the SPA changes. |
| Agent queue or worker lifecycle | [AGENT-QUEUE.md](./AGENT-QUEUE.md), [AGENT-WORKER.md](./AGENT-WORKER.md), and [EXECUTION-CYCLES.md](./EXECUTION-CYCLES.md) when cycle semantics change. |
| Web-only UI behavior | [WEB.md](./WEB.md). |
| Observability behavior or scripts | [OBSERVABILITY.md](./OBSERVABILITY.md); use [runbooks/](./runbooks/) only for operator procedures. |
| Future designs | [proposals/](./proposals/) first; promote to a focused contract doc only when implementation starts. |

Cursor rules (`.cursor/rules/`) are for tooling guidance, not operator docs.
