# taskapi — Server-Sent Events (`GET /events`)

SSE contract for live hints when tasks change. **REST** contracts: [API-HTTP.md](./API-HTTP.md). **Runtime / logging env:** [RUNTIME-ENV.md](./RUNTIME-ENV.md).

Connected clients receive `text/event-stream`. The stream tells them a task id changed so they can call REST again for full rows.

Responses also set `Cache-Control: no-store` (same baseline as JSON API responses), `Connection: keep-alive`, and `X-Accel-Buffering: no` so reverse proxies (e.g. nginx) disable response buffering for SSE.

Failure modes: if the handler was constructed with a nil hub, the server returns 503 `event stream unavailable`. If the `ResponseWriter` does not implement `http.Flusher`, the server returns 500 `streaming unsupported` (unusual with `net/http` defaults).

## Wire format

- `Content-Type: text/event-stream`
- First frame: `retry: 3000` (reconnect hint, ms)
- Each event: one `id: N` line followed by one `data:` line with JSON, separated by a blank line. Browser `EventSource` automatically captures the `id:` value as `Last-Event-ID` and replays it on reconnect (see [Lossless reconnects](#lossless-reconnects-last-event-id--ring-buffer) below):

```text
id: 42
data: {"type":"task_updated","id":"<task-uuid>"}

```

Most frames use `{type,id[,cycle_id]}`. Older clients that ignore the `id:` line keep working — they just lose the loss-free reconnect property.

```json
{"type":"task_created|task_updated|task_deleted|project_created|project_updated|project_deleted|project_context_changed","id":"<task-or-project-uuid>"}
```

`task_cycle_changed` payloads carry an extra `cycle_id` field so the SPA can scope its invalidation to the affected execution cycle subtree rather than the whole task:

```json
{"type":"task_cycle_changed","id":"<task-uuid>","cycle_id":"<cycle-uuid>"}
```

`cycle_id` is present on `task_cycle_changed` and `agent_run_progress` lines. It is omitted from task CRUD, settings, cancel, and resync frames. See [EXECUTION-CYCLES.md](./EXECUTION-CYCLES.md) for the underlying primitive.

`agent_run_progress` payloads are live progress hints from the in-process agent worker while a phase is still running. They are not persisted in `task_events`; REST remains authoritative for cycles, phases, and terminal outcomes. Durable per-attempt stream history is exposed through `GET /tasks/{id}/cycles/{cycleId}/stream`.

```json
{
  "type": "agent_run_progress",
  "id": "<task-uuid>",
  "cycle_id": "<cycle-uuid>",
  "phase_seq": 2,
  "progress": {
    "kind": "tool_call|assistant|system",
    "subtype": "started|completed|...",
    "message": "Started ReadFile",
    "tool": "ReadFile"
  }
}
```

The server normalizes Cursor CLI `stream-json` events before publishing them. It sends short human-readable messages only; raw Cursor JSON, stderr, and secrets are not streamed to browsers. The worker throttles progress fanout to at most one frame per running phase every 750 ms.

`settings_changed` and `agent_run_cancelled` are id-less notifications fired by the [App settings](./API-HTTP.md#app-settings) routes — there is no `id` (or `cycle_id`) field, only `type`:

```json
{"type":"settings_changed"}
{"type":"agent_run_cancelled"}
```

Consumers refetch `GET /settings` (and clear any "cancelling…" UI state for `agent_run_cancelled`) on receipt; both are documented in [SETTINGS.md](./SETTINGS.md).

`resync` is a hub-emitted directive (no `id`/`cycle_id`) that tells the client its reconnect cursor is outside the ring buffer or it was forcibly disconnected as a slow consumer. The frame deliberately has no `id:` line (`id:` is omitted on the wire) so browser `EventSource` does not advance its `Last-Event-ID` cursor — the next reconnect will then come back without a header and the server will admit the client at the live tail. Consumers MUST drop every cached query and refetch from REST:

```json
{"type":"resync"}
```

## Lossless reconnects (Last-Event-ID + ring buffer)

Each `Publish` call allocates a strictly-increasing event id and stores the marshalled frame in an in-memory ring buffer (default 1024 entries, ~125 KB). On reconnect the browser's `EventSource` sends `Last-Event-ID: <last seen id>`; the handler replays every retained frame with id > that value before entering the live loop. If the requested id is older than the oldest retained ring entry, the handler emits one `resync` directive and the client is expected to drop caches and refetch from REST.

The hub also evicts subscribers whose per-connection channel fills up (slow consumer / blocked HTTP write) instead of silently dropping frames: each eviction sends a `resync` directive and closes the stream so the client reconnects with its `Last-Event-ID` and either replays from the ring or falls through to the same resync escape hatch.

A `: heartbeat` comment line is written every 15 s so reverse proxies and corporate VPN gateways do not idle-kill the TCP connection. Browsers ignore comment lines per the SSE spec; no client work is required.

Identical `{type,id}` frames published inside a 50 ms window are coalesced (dropped before fanout) so a burst of supervisor reloads or duplicate `task_updated` events does not spam every connected client. `task_cycle_changed` and `agent_run_progress` are intentionally **never** coalesced by the hub; progress is rate-limited before publish and cycle transitions are informationally distinct.

Operators monitor the resync rate (`taskapi_sse_resync_emitted_total / taskapi_sse_publish_total`), the coalesced-drop rate (`taskapi_sse_coalesced_total`), and the slow-consumer eviction rate (`taskapi_sse_subscriber_evictions_total`). All three are documented in [`pkgs/tasks/middleware/metrics_http.go`](../pkgs/tasks/middleware/metrics_http.go).

Each successful write may publish more than one event so SSE clients can refresh the affected row(s) without server-side joins:

| Trigger                                                | `type`(s) emitted                                                          |
| ------------------------------------------------------ | -------------------------------------------------------------------------- |
| `POST /projects`                                      | `project_created` for the new project                                      |
| `PATCH /projects/{id}`                                | `project_updated` for the patched project                                  |
| `DELETE /projects/{id}`                               | `project_deleted` for the removed project                                  |
| `POST /projects/{id}/context`                         | `project_context_changed` for `{id}`                                       |
| `PATCH /projects/{id}/context/{contextId}`            | `project_context_changed` for `{id}`                                       |
| `DELETE /projects/{id}/context/{contextId}`           | `project_context_changed` for `{id}`                                       |
| `POST /tasks`                                          | `task_created` for the new task; plus `task_updated` for `parent_id` when the task is created under a parent |
| `PATCH /tasks/{id}`                                    | `task_updated` for the patched task                                        |
| `DELETE /tasks/{id}`                                   | `task_deleted` for the deleted task; plus `task_updated` for the parent when the deleted task had one |
| `POST /tasks/{id}/checklist/items`                     | `task_updated` for `{id}`                                                  |
| `PATCH /tasks/{id}/checklist/items/{itemId}`           | `task_updated` for `{id}`                                                  |
| `DELETE /tasks/{id}/checklist/items/{itemId}`          | `task_updated` for `{id}`                                                  |
| `PATCH /tasks/{id}/events/{seq}` (user-response thread)| `task_updated` for `{id}`                                                  |
| `POST /tasks/{id}/cycles`                              | `task_cycle_changed` for the new cycle (`id`=task, `cycle_id`=new cycle)   |
| `PATCH /tasks/{id}/cycles/{cycleId}`                   | `task_cycle_changed` for the terminated cycle                              |
| `POST /tasks/{id}/cycles/{cycleId}/phases`             | `task_cycle_changed` for the cycle whose phase started                     |
| `PATCH /tasks/{id}/cycles/{cycleId}/phases/{phaseSeq}` | `task_cycle_changed` for the cycle whose phase transitioned                |
| In-process agent runner progress                       | `agent_run_progress` for the running cycle/phase (live hint; durable history via `GET /tasks/{id}/cycles/{cycleId}/stream`) |
| `PATCH /settings`                                      | `settings_changed` (no id) after the supervisor finishes its in-process reload |
| `POST /settings/cancel-current-run`                    | `agent_run_cancelled` (no id) when a run was actually cancelled (`{"cancelled": true}`); no event when nothing was running |

Read-only `GET` routes never publish. Failed writes (any non-2xx) never publish. Task drafts (`/task-drafts/*`), the draft scorer (`POST /tasks/evaluate`), and the cursor binary probe (`POST /settings/probe-cursor`) are not part of the SSE surface — `task_updated` only fires once a `POST /tasks` actually creates the underlying row, and the probe is a best-effort one-shot read against the runner that never mutates app state.

## Dev-only: SSE “cron” (`T2A_SSE_TEST=1`)

For local UI work, `taskapi` can start a background ticker (no extra HTTP routes). Set `T2A_SSE_TEST=1` (never enable in production without intent). Every 3s by default (override with `T2A_SSE_TEST_INTERVAL`, or `0` to disable the ticker), the process:

1. Optionally runs **lifecycle simulation** when `T2A_SSE_TEST_LIFECYCLE=1`: every `T2A_SSE_TEST_LIFECYCLE_EVERY` ticker fires (default `5`), creates a task with id prefix `t2a-devsim-` or deletes one such task (no subtasks), then publishes `task_created` or `task_deleted` on the SSE hub.
2. Pages through `store.ListFlat` with limit 200 and increasing offset (`id ASC` over all tasks).
3. For each task row, calls `store.AppendTaskEvent` with actor `agent` up to **`T2A_SSE_TEST_EVENTS_PER_TICK`** times per tick (default `1`, max `50`) using the next `domain.EventType` in a fixed rotation that includes every `domain.EventType` once per cycle (order: `pkgs/tasks/devsim` `EventCycle`). The next type is chosen from `len(task_events) mod len(cycle)` so successive appends walk through all types. Sample JSON `data` is attached (realistic shapes for plans, artifacts, checklist rows, approvals, etc.; `from`/`to` for status, priority, prompt, and title/message events).
4. If `T2A_SSE_TEST_SYNC_ROW=1`, after each append `store.ApplyDevTaskRowMirror` updates the task row when the synthetic event maps to fields (status, priority, title, initial prompt, terminal completed/failed). Marking **done** uses the same checklist/subtask rules as `PATCH`; mirror steps that violate those rules are skipped (debug log only).
5. If `T2A_SSE_TEST_USER_RESPONSE=1`, after an `approval_requested` or `task_failed` append, `store.AppendTaskEventResponseMessage` adds a synthetic user thread line (same path as `PATCH /tasks/{id}/events/{seq}`).
6. Publishes `task_updated` on the SSE hub after each simulated task’s append cycle (and lifecycle publishes as above). Clients still treat REST/DB as authoritative.

| Env | Meaning |
| --- | --- |
| `T2A_SSE_TEST=1` | Enable ticker (required). |
| `T2A_SSE_TEST_INTERVAL` | Tick interval (e.g. `3s`); `0` disables the ticker. |
| `T2A_SSE_TEST_EVENTS_PER_TICK` | Appends per task per tick (`1`–`50`, default `1`). |
| `T2A_SSE_TEST_SYNC_ROW=1` | Mirror task row after each synthetic append when applicable. |
| `T2A_SSE_TEST_USER_RESPONSE=1` | Synthetic user message on `approval_requested` / `task_failed` rows. |
| `T2A_SSE_TEST_LIFECYCLE=1` | Random create/delete of `t2a-devsim-*` tasks. |
| `T2A_SSE_TEST_LIFECYCLE_EVERY` | Run lifecycle every N ticks (default `5` when lifecycle is on). |

There are no extra dev-only HTTP paths; only normal REST + `GET /events` apply.

Clients typically use `EventSource` in the browser (or any SSE-capable client), parse each `data` line, then call `GET /tasks` or `GET /tasks/{id}`. Treat REST and the database as authoritative. The SPA debounces bursts, then routes each frame to the narrowest cache slot that can hold the change:

- `task_created` / `task_updated` / `task_deleted` invalidate the cached **list** queries plus the affected **detail** subtree (`["tasks","detail",id,…]`).
- `task_cycle_changed` invalidates the **list** plus the affected task's full **detail** subtree (`["tasks","detail",id,…]`). The agent worker is the primary emitter of this frame and never publishes `task_updated`, so worker-driven status flips (running → done), audit-event appends, and checklist toggles must be reflected via the cycle frame or the open task detail page would silently go stale until the user manually refreshed. The cycle id is still bucketed for analytics, but a standalone `["tasks","detail",id,"cycles"]` invalidation is suppressed because the broader `detail` invalidation already covers it.
- `agent_run_progress` updates bounded in-memory live progress keyed by `{task_id, cycle_id, phase_seq}` and does **not** invalidate REST queries. The next `task_cycle_changed` or terminal phase row remains the authoritative cache refresh for persisted stream rows.
- `settings_changed` and `agent_run_cancelled` invalidate **only** the settings cache slot (`["settings","app"]`) so the SPA Settings page reflects new values instantly without disturbing task caches; they bypass the debounce batch.
- A frame with no recognisable `id` (and no recognised id-less type above) falls back to invalidating all task queries so nothing goes stale.

This is implemented in `web/src/tasks/task-query/sseInvalidate.ts` (`parseTaskChangeFrame`) and `web/src/tasks/hooks/useTaskEventStream.ts`, with granularity pinned by tests.

## Related

- [WEB.md](./WEB.md) — React Query + SSE invalidation in the SPA.
