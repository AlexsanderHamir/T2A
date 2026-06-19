# ADR-0020: Realtime SSE Layout

**Date:** 2026-06-19
**Status:** Accepted
**Deciders:** Engineering

## Context

The SSE hub lived in a single red-zone file [`pkgs/tasks/handler/sse.go`](../../pkgs/tasks/handler/sse.go) (~660 lines) mixing wire types, hub concurrency, HTTP streaming, and handler notify glue. The agent worker supervisor imported `*handler.SSEHub` and handler wire types, coupling process assembly to the HTTP package.

[`docs/domain/sse-hub.md`](../domain/sse-hub.md) already documents logical domains; code layout did not match. Coalesce policy was embedded in the hub with no pure tests outside integration suites.

## Decision

Introduce **`pkgs/tasks/realtime`** as the stable import surface for wire types, coalesce policy, and the `Publisher` port. Split handler transport across focused files; keep hub implementation in `handler` (HTTP stack owns `GET /events`).

| Package / file | Responsibility |
|----------------|----------------|
| `pkgs/tasks/realtime` | `ChangeType`, `Event`, `RunProgressPayload`, `CoalesceKey`, `Publisher` interface |
| `handler/sse_types.go` | Type aliases to `realtime` for backward-compatible handler API |
| `handler/sse_hub.go` | `SSEHub`, ring buffer, publish fanout, eviction, legacy `Subscribe` |
| `handler/sse_stream.go` | `streamEvents`, frame writers, reconnect replay |
| `handler/sse_notify.go` | Handler `notify*` helpers + store hydration |

**`internal/taskapi/agentworker`** accepts `realtime.Publisher` instead of `*handler.SSEHub`. Production wiring still constructs `handler.NewSSEHubWith(...)` in `cmd/taskapi/run_helpers.go`.

### Dependency rules

```
realtime     → stdlib only
handler      → realtime, middleware, calltrace
agentworker  → realtime (Publisher), not handler for publish paths
cmd/taskapi  → handler (concrete hub), agentworker
```

**Forbidden:** `realtime` importing `handler`. **Deferred:** store-origin change notifier (publish after commit from store facade).

### Non-goals (this ADR)

- No microservices, Kafka, or external pub/sub
- No Postgres outbox (future ADR when multi-replica)
- No moving all handler `notify*` call sites to store in this change
- No frontend `useTaskEventStream` split (separate track)

## Consequences

### Positive

- Hub files under file-size green/yellow bar
- Coalesce policy table-tested without hub mutex
- Supervisor decoupled from HTTP handler package for publish
- Clear seam for future store-backed `ChangeNotifier`

### Negative / Trade-offs

- Type aliases in handler preserve API but duplicate names (`TaskChangeEvent` vs `realtime.Event`)
- `funclogmeasure` path updates when symbols move between files

## Related

- [ADR-0019](ADR-0019-agentworker-internal-layout.md) — supervisor layout; deferred SSE hub ports
- [docs/domain/sse-hub.md](../domain/sse-hub.md) — behavioral reference
