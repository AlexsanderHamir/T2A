# T2A — product context

T2A is a **control plane** for agent-heavy workflows. As agents take over execution, the IDE stops being the right home for orchestration—you need durable state, a shared API, and an audit trail that every actor (human, script, agent) uses the same way.

## What T2A provides

- **One task store** (Postgres) with an append-only **audit trail** so handoffs are explainable.
- **Project context** for long-running initiatives: a project is shared memory for many tasks, while each task run keeps an auditable snapshot of the context it actually used.
- **REST** for all mutations and queries; **`GET /events`** (SSE) so clients know when to refetch instead of polling blindly.
- Optional **web UI** (`web/`) for task CRUD and live updates; same contracts as any other client.

## Scope discipline

T2A grows by making durable coordination primitives explicit before making them clever. The first project-context layer is relational, inspectable, and editable: projects own canonical shared context; tasks keep their existing subtask tree and can reference project memory; agent runs snapshot the exact context bundle they receive.

Deferred on purpose: embeddings, vector databases, hidden memory pruning, autonomous summarization daemons, auth/sharing, billing, and multi-tenant project boundaries.

## Where to go next

Technical routes: [docs/API-HTTP.md](./API-HTTP.md) and [docs/API-SSE.md](./API-SSE.md). Env and startup: [docs/RUNTIME-ENV.md](./RUNTIME-ENV.md). Architecture and limits: [docs/DESIGN.md](./DESIGN.md). Project context: [docs/PROJECT-CONTEXT.md](./PROJECT-CONTEXT.md). Browser client: [docs/WEB.md](./WEB.md). Build and run: root [README](../README.md).
