# Realtime smoothness — design notes

Tracks realtime smoothness decisions as they ship. This is a living document for the SSE resume, resync, and RUM/SLO work.

## Phase 2 — Lossless SSE (server)

**What shipped**

The `SSEHub` in [`pkgs/tasks/handler/sse.go`](../pkgs/tasks/handler/sse.go) now keeps a bounded ring buffer of recent events tagged with monotonically increasing 64-bit ids. The HTTP `/events` handler:

- Emits each event as `id: N\ndata: <json>\n\n` so browser `EventSource` captures the id for `Last-Event-ID`-based reconnect.
- Honors the `Last-Event-ID` request header on reconnect: events with id > the header value are replayed in publish order before the live select loop starts.
- If the client's id is older than the oldest retained ring entry, emits a single `data: {"type":"resync"}\n\n` directive (no `id:` line — the client's `Last-Event-ID` cursor stays where it was so the next reconnect will match the live tail) and the SPA drops every cached query.
- Writes a `: heartbeat\n\n` comment line every 15 s to defeat reverse-proxy idle timeouts. Browsers ignore comment lines per the SSE spec.
- Evicts subscribers whose per-connection channel fills up — the writer goroutine sends a `resync` directive on the way out and shuts the stream. The client reconnects with `Last-Event-ID`, replays from the ring, or falls through to the same resync escape hatch. Net effect: even slow consumers stay correct.
- Coalesces duplicate `{type, id}` frames inside a 50 ms window. `task_cycle_changed` carries a distinct `cycle_id` and is intentionally never coalesced (each phase transition is informationally distinct).

**Decisions worth recording**

- **Coalescing default OFF in `NewSSEHub()`, ON in production.** The plan calls for a 50 ms coalesce window. In a unit-test harness where two distinct ops complete in <50 ms, that window collapses informationally distinct frames (e.g. `task_updated:parent` from "create child" followed by `task_updated:parent` from "delete child" inside the same test). To preserve the existing trigger-surface contract without rewriting every test, `NewSSEHub()` keeps the loss-prevention machinery (ring + eviction + heartbeats) on but disables coalescing by default. Production wiring in [`cmd/taskapi/run_helpers.go`](../cmd/taskapi/run_helpers.go) opts in via `NewSSEHubWith(DefaultSSEHubOptions())`. Tests that specifically target the coalescer construct their own hub with `CoalesceWindow: 50 * time.Millisecond`.
- **Legacy `Subscribe()` keeps the 32-frame bounded buffer.** The in-process `Subscribe()` entry point (used by the dev SSE ticker, the trigger surface tests, and a handful of older integration tests) predates Last-Event-ID resume. It still drops frames silently when its 32-cap channel fills — exactly the original behavior the existing `TestSSEHub_Publish_recordsDroppedFramesCounter` pin assumes. New consumers should subscribe via the HTTP `/events` path so they get the lossless reconnect contract. The legacy path is documented as such in `sse.go`.
- **Resync directive carries no `id:` line.** EventSource captures the latest `id:` value as `Last-Event-ID` on reconnect. If we wrote `id: N` on the resync frame, the next reconnect would come back with that id — but we just told the client the gap was un-bridgeable, so the second reconnect would either still be inside the gap (resync loop) or land in a confusing state. Leaving `id:` off the resync frame means the client's cursor stays at whatever it captured last, and the next reconnect either matches the ring or triggers another resync deterministically.
- **Eviction emits a resync, then the writer returns.** When the publisher overflows a subscriber's buffer the hub closes that subscriber's `cancel` channel. The writer goroutine selects on `cancel` in its main loop, sends one resync directive, and returns — the HTTP handler's `defer cancel()` then unregisters. The client sees the directive, drops caches, reconnects with `Last-Event-ID`, and continues.
- **Three new Prometheus counters.** `taskapi_sse_coalesced_total`, `taskapi_sse_resync_emitted_total`, `taskapi_sse_subscriber_evictions_total`. The SLO `slo_sse_resync_rate ≤ 0.5%` (Phase 4a) is computed as `sse_resync_emitted_total / sse_publish_total`.

**Rollout posture**

The wire change is backward compatible: older SPAs ignore the `id:` line and the `heartbeat` comments. They still benefit from coalescing and the slow-consumer eviction (because the eviction sends them a `resync` frame, which they fall through to "unknown frame" → broad invalidation). `Last-Event-ID` resume requires the SPA-side handler shipped in [`web/src/tasks/task-query/sseInvalidate.ts`](../web/src/tasks/task-query/sseInvalidate.ts) (parses the `resync` frame) and [`web/src/tasks/hooks/useTaskEventStream.ts`](../web/src/tasks/hooks/useTaskEventStream.ts) (drops cache + refetches on resync). Both ship in the same commit.
