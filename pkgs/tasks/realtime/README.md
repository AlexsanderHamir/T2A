# `pkgs/tasks/realtime`

SSE wire contracts and publish port for in-process fanout. Transport (ring buffer, HTTP `GET /events`) lives in [`pkgs/tasks/handler`](../handler/).

| File | Role |
|------|------|
| `wire.go` | `ChangeType`, `Event`, `RunProgressPayload` |
| `coalesce.go` | Pure `CoalesceKey` for hub dedup window |
| `publisher.go` | `Publisher` interface (`Publish(Event)`) |

**Importers:** `handler` (hub + notify), `internal/taskapi/agentworker` (supervisor adapters). Handlers should keep using `handler.TaskChangeEvent` aliases unless adding a new non-HTTP publisher.

See [docs/domain/sse-hub.md](../../docs/domain/sse-hub.md) and [ADR-0020](../../docs/adr/ADR-0020-realtime-sse-layout.md).
