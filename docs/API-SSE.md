# taskapi — Server-Sent Events (`GET /events`)

SSE contract for live hints when tasks change. **REST** contracts: [API-HTTP.md](./API-HTTP.md). **Runtime / logging env:** [RUNTIME-ENV.md](./RUNTIME-ENV.md).

Connected clients receive `text/event-stream`. The stream tells them a task id changed so they can call REST again for full rows.

Responses also set `Cache-Control: no-store` (same baseline as JSON API responses), `Connection: keep-alive`, and `X-Accel-Buffering: no` so reverse proxies (e.g. nginx) disable response buffering for SSE.

Failure modes: if the handler was constructed with a nil hub, the server returns 503 `event stream unavailable`. If the `ResponseWriter` does not implement `http.Flusher`, the server returns 500 `streaming unsupported` (unusual with `net/http` defaults).

## Wire format

- `Content-Type: text/event-stream`
- First frame: `retry: 3000` (reconnect hint, ms)
- Each event: one `data:` line with JSON:

```json
{"type":"task_created|task_updated|task_deleted","id":"<task-uuid>"}
```

Each successful write may publish more than one event so SSE clients can refresh the affected row(s) without server-side joins:

| Trigger                                                | `type`(s) emitted                                                          |
| ------------------------------------------------------ | -------------------------------------------------------------------------- |
| `POST /tasks`                                          | `task_created` for the new task; plus `task_updated` for `parent_id` when the task is created under a parent |
| `PATCH /tasks/{id}`                                    | `task_updated` for the patched task                                        |
| `DELETE /tasks/{id}`                                   | `task_deleted` for the deleted task; plus `task_updated` for the parent when the deleted task had one |
| `POST /tasks/{id}/checklist/items`                     | `task_updated` for `{id}`                                                  |
| `PATCH /tasks/{id}/checklist/items/{itemId}`           | `task_updated` for `{id}`                                                  |
| `DELETE /tasks/{id}/checklist/items/{itemId}`          | `task_updated` for `{id}`                                                  |
| `PATCH /tasks/{id}/events/{seq}` (user-response thread)| `task_updated` for `{id}`                                                  |

Read-only `GET` routes never publish. Failed writes (any non-2xx) never publish. Task drafts (`/task-drafts/*`) are not part of the SSE surface.

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

Clients typically use `EventSource` in the browser (or any SSE-capable client), parse each `data` line, then call `GET /tasks` or `GET /tasks/{id}`. Treat REST and the database as authoritative. The SPA debounces bursts, then invalidates cached **list** queries and only **detail** subtrees for task ids present on the `data` lines (falling back to invalidating all task queries if no id could be parsed), so open pages for unrelated tasks are not refetched on every event.

## Related

- [WEB.md](./WEB.md) — React Query + SSE invalidation in the SPA.
