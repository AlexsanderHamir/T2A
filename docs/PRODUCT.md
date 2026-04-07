# T2A — product context

T2A is a **control plane** for agent-heavy workflows. As agents take over execution, the IDE stops being the right home for orchestration—you need durable state, a shared API, and an audit trail that every actor (human, script, agent) uses the same way.

## What T2A provides

- **One task store** (Postgres) with an append-only **audit trail** so handoffs are explainable.
- **REST** for all mutations and queries; **`GET /events`** (SSE) so clients know when to refetch instead of polling blindly.
- Optional **web UI** (`web/`) for task CRUD and live updates; same contracts as any other client.

## Where to go next

Technical routes, env vars, and limits: [docs/DESIGN.md](./DESIGN.md). Browser client: [docs/WEB.md](./WEB.md). Build and run: root [README](../README.md).
