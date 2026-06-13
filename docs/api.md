# API

Minimal reference for the `taskapi` HTTP surface (REST) and `GET /events` (SSE). Endpoint behavior is documented in source: error strings, status codes, validation rules, and rate-limit specifics live in `pkgs/tasks/handler/` (godoc) and `pkgs/tasks/middleware/` (godoc).

Data model semantics: [data-model.md](./data-model.md). Configuration: [configuration.md](./configuration.md).

## Conventions

- Mux is mounted at `/` (no `/api` prefix).
- All routes return `application/json`. Error bodies are `{"error":"<message>"}`; some responses include `request_id` for correlation with `X-Request-ID` / `http.access` logs.
- Cacheable read routes (`GET /tasks`, `GET /tasks/{id}`, `GET /tasks/stats`, `GET /tasks/{id}/checklist`, `GET /tasks/{id}/dependencies`, `GET /tasks/{id}/cycles`, `GET /tasks/{id}/cycles/{cycleId}`, `GET /projects`, `GET /projects/{id}`, `GET /projects/{id}/context`, `GET /settings`) emit a strong `ETag` header and `Cache-Control: private, no-cache, must-revalidate`; the server returns `304 Not Modified` with no body when `If-None-Match` matches the current ETag. All other endpoints (mutations, SSE, `/metrics`, `/health*`, `/system/health`, `/repo/*`, `/tasks/cycle-failures`, drafts, runners, evaluate) return `Cache-Control: no-store` and do not participate in revalidation.
- `X-Actor` header: `user` (default) or `agent`. The handler ignores any body `triggered_by` and uses this header.
- `Idempotency-Key` (≤ 128 bytes) caches successful (2xx) `POST`/`PATCH`/`DELETE` responses for `T2A_IDEMPOTENCY_TTL` (default 24h, in-process only). Replays are byte-identical.
- Rate limit: `T2A_RATE_LIMIT_PER_MIN` per `RemoteAddr` (default 120; `0` disables). `429` returns `Retry-After: 60`.
- Request body cap: `T2A_MAX_REQUEST_BODY_BYTES` (default 1 MiB; `0` disables).
- `T2A_API_TOKEN`, when set, requires `Authorization: Bearer <token>` on all routes except `/health*` and `/metrics`.

## Health and metrics

| Method | Path | Notes |
|---|---|---|
| GET | `/health` | Liveness; returns `version` from `runtime/debug.ReadBuildInfo`. No DB probe. |
| GET | `/health/live` | Same shape as `/health`. |
| GET | `/health/ready` | Readiness; DB ping + `SELECT 1` + workspace directory stat when `app_settings.repo_root` is set. `503` on failure. |
| GET | `/metrics` | Prometheus text. Standard Go / process collectors + `taskapi_build_info` + `taskapi_db_pool_*` + `taskapi_http_*` + `t2a_agent_runs_*` + `taskapi_sse_*` + `taskapi_agent_queue_*`. |
| GET | `/system/health` | Aggregated JSON for the SPA observability page: build, DB pool gauges, HTTP totals, SSE totals, agent queue + runs + paused. |
| POST | `/v1/rum` | Browser RUM ingest; one batched line per call, capped fields. |
| GET | `/v1/bootstrap` | Cold-start aggregate. Returns `{ settings, tasks: {tasks, limit, offset, has_more}, stats, projects: {projects, limit}, drafts: {drafts} }` in a single round trip; each field mirrors the corresponding per-endpoint wire shape. Honors `ETag` / `If-None-Match` (`304` on match). 5xx on any sub-call failure; clients must tolerate absence and fall back to per-endpoint fan-out. |

## Projects

| Method | Path | Notes |
|---|---|---|
| POST | `/projects` | Create. Body `{ id?, name, description?, context_summary? }`. Publishes `project_created`. |
| GET | `/projects` | List. `?limit` (0–100, default 50), `?include_archived=true`. |
| GET | `/projects/{id}` | Single project. |
| PATCH | `/projects/{id}` | Partial. At least one of `name`, `description`, `status`, `context_summary`. Default project (`00000000-0000-4000-8000-000000000001`) cannot be renamed / archived (409). Publishes `project_updated`. |
| DELETE | `/projects/{id}` | `204`. Blocked while tasks reference it (409). Default project cannot be deleted. Publishes `project_deleted`. |
| GET | `/projects/{id}/context` | List context items + edges. `?limit`, `?pinned_only=true`. |
| POST | `/projects/{id}/context` | Create context item. Publishes `project_context_changed`. |
| PATCH | `/projects/{id}/context/{contextId}` | Partial. Publishes `project_context_changed`. |
| DELETE | `/projects/{id}/context/{contextId}` | `204`. Publishes `project_context_changed`. |
| POST | `/projects/{id}/context/edges` | Create edge between two items. `relation ∈ supports | blocks | refines | depends_on | related`, `strength 1..5`. Publishes `project_context_changed`. |
| PATCH | `/projects/{id}/context/edges/{edgeId}` | Partial. Publishes `project_context_changed`. |
| DELETE | `/projects/{id}/context/edges/{edgeId}` | `204`. Publishes `project_context_changed`. |

## Tasks

Model semantics (tags, milestone, `depends_on`, gate, worker readiness): [data-model.md](./data-model.md).

| Method | Path | Notes |
|---|---|---|
| POST | `/tasks` | Create. Title required; `priority` required; `checklist_items` required — `[{ "text": "..." }]`, at least one non-empty `text` (persisted atomically with the task row). `400` `at least one done criterion required` when missing, empty, or all-blank. Optional `id`, `draft_id`, `project_id`, `pickup_not_before`, `cursor_model`, `tags`, `milestone`, `depends_on` (string[] legacy or `{ task_id, satisfies }[]` with `satisfies: done`). Returns flat `domain.Task`. `409` on duplicate `id`. Publishes `task_created`. |
| POST | `/tasks/evaluate` | Score a draft payload; persist snapshot. Never publishes on SSE. |
| GET | `/tasks` | List all tasks (flat). Pagination: `?limit` (0–200, default 50) + `?offset` (≥ 0) **or** `?after_id` (keyset, mutually exclusive with offset). Envelope `{ tasks, limit, offset, has_more }`. Each element is a flat `domain.Task` (no nested `children`). |
| GET | `/tasks/stats` | Counters: `total`, `ready`, `critical`, `scheduled`, `by_status`, `by_priority`, `cycles`, `phases`, `runner`, `recent_failures`. |
| GET | `/tasks/cycle-failures` | Paginated terminal cycle failures. `?limit`, `?offset`, `?sort ∈ at_desc | at_asc | reason_asc | reason_desc`. |
| GET | `/tasks/{id}` | Single flat `domain.Task`. |
| PATCH | `/tasks/{id}` | At least one of: `title`, `initial_prompt`, `status`, `priority`, `project_id`, `project_context_item_ids`, `pickup_not_before`, `cursor_model`, `tags`, `milestone`, `gate`, `depends_on`. Publishes `task_updated` (+ `task_gate_changed` / `task_dependency_changed` when those fields change). Writable `status` values for `X-Actor: user`: `ready`, `running`, `blocked`, `review`, `done`, `failed`, `on_hold`. See [data-model.md](./data-model.md). |
| DELETE | `/tasks/{id}` | `204` empty body. Publishes `task_deleted`. |
| GET | `/tasks/{id}/events` | Audit log. Default: ascending all rows. With `limit` / `before_seq` / `after_seq`: keyset-paged newest-first slice with `range_*`, `has_more_*`, `approval_pending`. |
| GET | `/tasks/{id}/events/{seq}` | Single event row. |
| PATCH | `/tasks/{id}/events/{seq}` | Append a user-response message (max 10 000 bytes after trim, thread cap 200). Only for `approval_requested` and `task_failed`. Publishes `task_updated`. |
| GET | `/tasks/{id}/dependencies` | `{ depends_on: [{ task_id, satisfies }] }`. |
| POST | `/tasks/{id}/dependencies` | Body `{ depends_on_task_id, satisfies? }` (default `done`). Cycles / self-deps rejected. Publishes `task_dependency_changed`. |
| DELETE | `/tasks/{id}/dependencies/{depId}` | `204`. Publishes `task_dependency_changed`. |
| PATCH | `/tasks/{id}/gate` | Body `{ action: release | hold | clear_hold }`. Publishes `task_gate_changed` and `task_updated`. |

### Checklist

| Method | Path | Notes |
|---|---|---|
| GET | `/tasks/{id}/checklist` | `{ items: [...] }` ordered by `sort_order`. |
| POST | `/tasks/{id}/checklist/items` | Body `{ text }`. Rejected `409` when the task is `running` or `done`, or when a cycle is running. Publishes `task_updated`. |
| PATCH | `/tasks/{id}/checklist/items/{itemId}` | Body: exactly one of `{ text }` or `{ done: true|false }`. `done:true` requires `X-Actor: agent` plus `evidence` + optional `verified_by`. Publishes `task_updated`. |
| DELETE | `/tasks/{id}/checklist/items/{itemId}` | `204`. Publishes `task_updated`. |

### Task drafts

| Method | Path | Notes |
|---|---|---|
| POST | `/task-drafts` | Upsert. Body `{ id?, name, payload }`. Never publishes on SSE. |
| GET | `/task-drafts` | List summaries (without `payload`). `?limit` (0–100). |
| GET | `/task-drafts/{id}` | Full draft with `payload` defaulted to `{}`. |
| DELETE | `/task-drafts/{id}` | `204`. |

### Execution cycles

See [data-model.md](./data-model.md) for state machine and substrate semantics.

| Method | Path | Notes |
|---|---|---|
| POST | `/tasks/{id}/cycles` | Start a cycle. Body `{ parent_cycle_id?, meta? }`. Returns `taskCycleResponse` (with typed `cycle_meta` projection). Publishes `task_cycle_changed`. |
| GET | `/tasks/{id}/cycles` | List. `?limit` (1–200), `?before_attempt_seq` keyset cursor. Newest first. |
| GET | `/tasks/{id}/cycles/{cycleId}` | One cycle with `phases[]` ordered ascending. |
| PATCH | `/tasks/{id}/cycles/{cycleId}` | Terminate. Body `{ status: succeeded|failed|aborted, reason? }`. Publishes `task_cycle_changed`. The agent worker emits `verification_failed:<id>,<id>,…` on terminal verify failure (sorted, deduped failing criterion IDs); the `verification_failed` prefix is contract-stable — clients MUST use prefix matching. Bare `verification_failed` (older cycles) remains a valid value. The reason column is 256 chars; long lists are truncated with `…` while the prefix stays intact. |
| GET | `/tasks/{id}/cycles/{cycleId}/stream` | Normalized Cursor live-run history. `?limit` (1–500), `?after_seq` keyset. |
| GET | `/tasks/{id}/cycles/{cycleId}/verdicts` | Per-criterion verdict evidence for one cycle. Returns `{ task_id, cycle_id, criteria_reports: [...], verify_reports: [...] }`. Both arrays are non-null (empty when no verdicts have been mirrored — pre-PR2 cycles, or cycles whose verify phase hasn't run yet); rows are ordered `(attempt_seq ASC, criterion_id ASC)`. Each criteria row carries `claimed_done` + `evidence` from the execute agent's self-report; each verify row carries `verified` + `verifier_kind` + `reasoning`. `verifier_kind` is the same enum as `task_checklist_completions.verified_by`. |
| POST | `/tasks/{id}/cycles/{cycleId}/phases` | Start a phase. Body `{ phase: execute|verify }`. Transitions follow `domain.ValidPhaseTransition`. Publishes `task_cycle_changed`. |
| PATCH | `/tasks/{id}/cycles/{cycleId}/phases/{phaseSeq}` | Terminate a phase. Body `{ status: succeeded|failed|skipped, summary?, details? }`. Publishes `task_cycle_changed`. |

## App settings

Singleton row (`id=1`) seeded on first read with `domain.DefaultAppSettings`. Full field reference: [configuration.md](./configuration.md).

| Method | Path | Notes |
|---|---|---|
| GET | `/settings` | Returns the full `AppSettings` row. Always available. |
| PATCH | `/settings` | Partial; pointer fields distinguish "not provided" from explicit zero. On success, supervisor reloads in-process and SSE publishes `settings_changed`. |
| POST | `/settings/probe-cursor` | Body `{ runner?, binary_path? }`. Probe failures return `200 { ok: false, error }` so the SPA renders inline. |
| POST | `/settings/list-cursor-models` | Same fallback semantics as probe. CLI failures return `200 { ok: false }`. |
| POST | `/settings/cancel-current-run` | Cancels any in-flight `runner.Run`. Cycle terminated with `cancelled_by_operator`. Publishes `agent_run_cancelled` when something was running. |

## Workspace repo

Wired only when `app_settings.repo_root` is set. When unset, every `/repo/*` route returns `409 { error: "repo root is not configured", reason: "repo_root_not_configured" }`. When `OpenRoot` rejects the path (missing, not a directory, symlink loop), routes return `500 { reason: "repo_root_open_failed", error }`.

| Method | Path | Notes |
|---|---|---|
| GET | `/repo/search?q=` | Capped list of repo-relative paths; `q` ≤ 512 bytes. |
| GET | `/repo/file?path=` | `{ path, content, binary, truncated, size_bytes, line_count, warning? }`. Binary or invalid UTF-8 returns `binary: true` with empty `content`. Files over 32 MiB are truncated. |
| GET | `/repo/validate-range?path=&start=&end=` | `{ ok, line_count?, warning? }`. Used by the SPA to warn about invalid `@`-mentions inline. |

`POST /tasks` and `PATCH /tasks/{id}` validate `@`-mentions in `initial_prompt` against the configured repo. Failures return `400` with the offending mention in the error message (`@<path>` or `@<path> (<start>-<end>)`). Validation is skipped when `repo_root` is unset, `initial_prompt` is omitted, or `initial_prompt = ""`.

## SSE — `GET /events`

`text/event-stream`. First frame: `retry: 3000`. Frames are id + JSON:

```text
id: 42
data: {"type":"task_updated","id":"<task-uuid>"}

```

Lossless reconnects via `Last-Event-ID`: a ring buffer (default 1024 entries) replays unseen frames on reconnect. Out-of-window reconnects emit one `resync` frame and the client drops caches. Slow consumers (full per-connection buffer) are evicted with a `resync` frame. Heartbeat `: heartbeat` comment every 15s. Identical `{type,id}` frames within 50ms are coalesced (except `task_cycle_changed` and `agent_run_progress`).

### Event types

| Type | When | Payload |
|---|---|---|
| `task_created` | `POST /tasks` succeeds. | `{ type, id, data: <task> }` |
| `task_updated` | Any task mutation (PATCH, checklist, event response, etc.). `data` carries the full flat task for `PATCH /tasks/{id}`; other publishers emit hint-only frames (no `data`). | `{ type, id, data?: <task> }` |
| `task_deleted` | `DELETE /tasks/{id}`. | `{ type, id }` |
| `task_dependency_changed` | Dependency add/remove/replace. | `{ type, id }` |
| `task_gate_changed` | Gate create/patch/action. | `{ type, id }` |
| `task_cycle_changed` | Cycle/phase mutation. | `{ type, id, cycle_id, data?: <cycle detail> }` |
| `agent_run_progress` | Live Cursor activity hint while a phase runs. Not persisted in `task_events`; durable history via `GET /tasks/{id}/cycles/{cycleId}/stream`. Throttled to one frame per 750ms per running phase. | `{ type, id, cycle_id, phase_seq, progress: { kind, subtype, message, tool } }` |
| `project_created` / `project_updated` / `project_deleted` | Project CRUD. | `{ type, id }` |
| `project_context_changed` | Context item / edge mutation. | `{ type, id }` (project id) |
| `settings_changed` | `PATCH /settings` after supervisor reload. | `{ type }` (no id) |
| `agent_run_cancelled` | `POST /settings/cancel-current-run` actually cancelled something. | `{ type }` (no id) |
| `resync` | Hub-emitted. Out-of-window reconnect or slow-consumer eviction. No `id:` line on wire (preserves `Last-Event-ID` cursor). | `{ type }` |

Read-only GETs never publish. Failed writes never publish. Drafts (`/task-drafts/*`), the evaluator (`POST /tasks/evaluate`), and `POST /settings/probe-cursor` are not part of the SSE surface.

### Dev synthetic SSE (`T2A_SSE_TEST=1`)

For local UI work, `taskapi` can start a background ticker (no extra routes). Set `T2A_SSE_TEST=1`; interval via `T2A_SSE_TEST_INTERVAL` (default `3s`; `0` disables). Tunables: `T2A_SSE_TEST_EVENTS_PER_TICK`, `T2A_SSE_TEST_SYNC_ROW`, `T2A_SSE_TEST_USER_RESPONSE`, `T2A_SSE_TEST_LIFECYCLE`, `T2A_SSE_TEST_LIFECYCLE_EVERY`. Never enable in production without intent. Source: `pkgs/tasks/devsim/`.
