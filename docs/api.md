# API

Minimal reference for the `taskapi` HTTP surface (REST) and `GET /events` (SSE). Endpoint behavior is documented in source: error strings, status codes, validation rules, and rate-limit specifics live in `pkgs/tasks/handler/` (godoc) and `pkgs/tasks/middleware/` (godoc).

Data model semantics: [data-model.md](./data-model.md). Configuration: [configuration.md](./configuration.md).

## Conventions

- Mux is mounted at `/` (no `/api` prefix).
- All routes return `application/json`. Error bodies are `{"error":"<message>"}`; some responses include `request_id` for correlation with `X-Request-ID` / `http.access` logs.
- Cacheable read routes (`GET /tasks`, `GET /tasks/{id}`, `GET /tasks/stats`, `GET /tasks/{id}/checklist`, `GET /tasks/{id}/dependencies`, `GET /tasks/{id}/cycles`, `GET /tasks/{id}/cycles/{cycleId}`, `GET /projects`, `GET /projects/{id}`, `GET /projects/{id}/context`, `GET /settings`) emit a strong `ETag` header and `Cache-Control: private, no-cache, must-revalidate`; the server returns `304 Not Modified` with no body when `If-None-Match` matches the current ETag. All other endpoints (mutations, SSE, `/metrics`, `/health*`, `/system/health`, `/repo/*`, `/tasks/cycle-failures`, drafts, runners) return `Cache-Control: no-store` and do not participate in revalidation.
- `X-Actor` header: `user` (default) or `agent`. The handler ignores any body `triggered_by` and uses this header.
- `Idempotency-Key` (≤ 128 bytes) caches successful (2xx) `POST`/`PATCH`/`DELETE` responses for `HAMIX_IDEMPOTENCY_TTL` (default 24h, in-process only). Replays are byte-identical.
- Rate limit: `HAMIX_RATE_LIMIT_PER_MIN` per `RemoteAddr` (default 120; `0` disables). `429` returns `Retry-After: 60`.
- Request body cap: `HAMIX_MAX_REQUEST_BODY_BYTES` (default 1 MiB; `0` disables).
- `HAMIX_API_TOKEN`, when set, requires `Authorization: Bearer <token>` on all routes except `/health*` and `/metrics`.

## Health and metrics

| Method | Path | Notes |
|---|---|---|
| GET | `/health` | Liveness; returns `version` from `runtime/debug.ReadBuildInfo`. No DB probe. |
| GET | `/health/live` | Same shape as `/health`. |
| GET | `/health/ready` | Readiness; DB ping + `SELECT 1`; `checks.schema` compares code vs DB `SchemaRevision` (`503` when `pending` or `downgrade`, with `schema.code_revision`, `schema.db_revision`, `schema.remediation`); workspace/repo checks as before. |
| GET | `/metrics` | Prometheus text. Standard Go / process collectors + `taskapi_build_info` + `taskapi_db_pool_*` + `taskapi_http_*` + `hamix_agent_runs_*` + `taskapi_sse_*` + `taskapi_agent_queue_*`. |
| GET | `/system/health` | Aggregated JSON for the SPA observability page: build, DB pool gauges, HTTP totals, SSE totals, agent queue + runs + paused. |
| POST | `/v1/rum` | Browser RUM ingest; one batched line per call, capped fields. |
| GET | `/v1/bootstrap` | Cold-start aggregate. Returns `{ settings, tasks: {tasks, limit, offset, has_more}, stats, projects: {projects, limit}, drafts: {drafts} }` in a single round trip; each field mirrors the corresponding per-endpoint wire shape. Default limits match [`readpolicy`](../../pkgs/tasks/handler/readpolicy/readpolicy.go) (`BootstrapListLimit` 20, `BootstrapProjectsLimit` 100, `BootstrapDraftsLimit` 50). Honors `ETag` / `If-None-Match` (`304` on match). 5xx on any sub-call failure; clients must tolerate absence and fall back to per-endpoint fan-out. |

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

### Git repositories, worktrees, and branches

Git context follows [ADR-0037](./adr/ADR-0037-global-repos-project-tree.md) (global repositories, optional project overlay) and [ADR-0039](./adr/ADR-0039-fixed-worktree-branch.md) (one fixed `branch_id` per worktree; tasks bind `worktree_id`). Worktree JSON includes `branch_id`. Error responses use `{ "error", "code", "request_id?" }`.

**Global routes (preferred):**

| Method | Path | Notes |
|---|---|---|
| GET | `/git/repositories` | `{ repositories: [...] }`. |
| POST | `/git/repositories` | Register checkout. Body `{ path, host_path? }`. Resolves main worktree path and `git_common_dir`. **201**. Does not auto-create worktrees/branches. **409** `not_a_git_repository`, `duplicate` (same git object database). |
| GET | `/git/repositories/{repoId}` | Single repository. **404** `repository_not_found`. |
| DELETE | `/git/repositories/{repoId}` | **204**. **409** `has_running_task`. |
| GET | `/git/repositories/{repoId}/worktrees` | `{ worktrees: [...] }`. |
| GET | `/git/repositories/{repoId}/worktrees/live` | Linked worktrees from `git worktree list`: `{ worktrees: [{ path, branch, is_main, detached, registered, locked, prunable }] }`. |
| GET | `/git/repositories/{repoId}/worktrees/probe?path=` | Validate a path belongs to this repo: `{ path, linked, is_main, branch, registered }`. |
| POST | `/git/repositories/{repoId}/worktrees` | Body `{ path, name?, branch, create_branch?, start_point? }`. Creates worktree with immutable `branch_id`. **201**. **409** `branch_bound_to_worktree` when branch is already assigned. |
| POST | `/git/repositories/{repoId}/worktrees/register` | Register existing linked worktree. Body `{ path, name?, branch?: { name, create_branch?, start_point? } }`. **201**. Sets `branch_id` from `branch` or the worktree's current checkout; immutable after create. |
| POST | `/git/repositories/{repoId}/reconcile` | Repair registered repository/worktree paths against `git worktree list`. Body `{ bootstrap_path?, repair?, dry_run? }` (all optional). Does **not** insert unregistered worktrees — use **Register worktree** + `GET …/worktrees/live`. When the stored main path is missing, pass `bootstrap_path` or use **Relocate**. **202** `{ status, report }` where `status ∈ ok | needs_bootstrap_path | partial` and `report` includes path/remove/head counts plus optional `resolution_source`, `discovered_path`, `worktrees_skipped`. **409** `has_running_task` when a vanished worktree is still referenced (global route). **409** `bootstrap_mismatch` when `bootstrap_path` is not the same git object database. |
| POST | `/git/repositories/{repoId}/relocate` | Operator alias: body `{ path }` runs reconcile with `bootstrap_path=path`, `repair=true`. **202** same shape as reconcile. |
| POST | `/git/worktrees/{worktreeId}/relocate` | Manual path fix for one registered worktree. Body `{ path }`. **200** worktree JSON after probe + UPDATE. **409** `bootstrap_mismatch` when path belongs to a different repo. |
| DELETE | `/git/worktrees/{worktreeId}` | **204**. Query `?force=true`. **409** `has_running_task`. |
| GET | `/git/repositories/{repoId}/branches` | Registered branches `{ branches: [...] }`. |
| GET | `/git/repositories/{repoId}/branches/live` | Live refs from `git branch` `{ branches: [{ name, head_sha }] }`. |
| GET | `/git/repositories/{repoId}/projects` | Projects tied to this repo `{ projects, limit }`. |

**Legacy per-project routes** (deprecated; removed after contract migration):

| Method | Path | Notes |
|---|---|---|
| GET | `/projects/{id}/git/repositories` | `{ repositories: [...] }`. |
| POST | `/projects/{id}/git/repositories` | Register main checkout. Body `{ path, host_path?, default_branch? }`. **201** repository. **409** `not_a_git_repository`, `duplicate`. |
| GET | `/projects/{id}/git/repositories/{repoId}` | Single repository. **404** `repository_not_found`. |
| DELETE | `/projects/{id}/git/repositories/{repoId}` | **204**. **409** `has_running_task` when a `running` task references the repo, a worktree, or a branch under it. |
| GET | `/projects/{id}/git/repositories/{repoId}/worktrees` | `{ worktrees: [...] }`. |
| POST | `/projects/{id}/git/repositories/{repoId}/worktrees` | Body `{ path, name?, branch, create_branch?, start_point? }`. **201**. **409** `path_exists`, `branch_checked_out`. |
| DELETE | `/projects/{id}/git/worktrees/{worktreeId}` | **204**. Query `?force=true` for dirty trees. **409** `has_running_task`. Main worktree cannot be deleted (**400**). |
| GET | `/projects/{id}/git/repositories/{repoId}/branches` | `{ branches: [...] }`. |
| POST | `/projects/{id}/git/repositories/{repoId}/branches` | Body `{ name, start_point? }`. **201**. **409** `branch_exists`. |
| DELETE | `/projects/{id}/git/branches/{branchId}` | **204**. Query `?force=true` for unmerged. **409** `has_running_task`, `branch_checked_out`. |
| POST | `/projects/{id}/git/repositories/{repoId}/reconcile` | Same body/response as global reconcile. **409** when a missing worktree is still referenced by tasks. |

**Projects:** `POST /projects` accepts optional `repository_id` (repo must exist). Tasks accept optional `worktree_id` (required for agent runs; branch is derived from the worktree row).

Stable error codes: `not_a_git_repository`, `path_exists`, `branch_exists`, `branch_checked_out`, `branch_bound_to_worktree`, `project_repo_mismatch`, `has_running_task`, `bootstrap_mismatch`, `repository_not_found`, `worktree_not_found`, `branch_not_found`, `duplicate`.

## Tasks

Model semantics (tags, milestone, `depends_on`, gate, worker readiness): [data-model.md](./data-model.md).

| Method | Path | Notes |
|---|---|---|
| POST | `/tasks` | Create. Title required; `priority` required; `checklist_items` required — `[{ "text": "..." , "verify_commands"?: [{ "command": "...", "expected_outcome"?: "..." }] }]`, at least one non-empty `text` (persisted atomically with the task row). `400` `at least one done criterion required` when missing, empty, or all-blank. Optional `id`, `draft_id`, `project_id`, `worktree_id`, `pickup_not_before`, `cursor_model`, `tags`, `milestone`, `depends_on` (string[] legacy or `{ task_id, satisfies }[]` with `satisfies: done`). Returns flat `domain.Task`. `409` on duplicate `id`. Publishes `task_created`. |
| GET | `/tasks` | List all tasks (flat). Pagination: `?limit` (0–200, default 50) + `?offset` (≥ 0) **or** `?after_id` (keyset, mutually exclusive with offset). Envelope `{ tasks, limit, offset, has_more }`. Each element is a flat `domain.Task` (no nested `children`). Rows are ordered **newest first** by `created_at` (from the `task_created` audit event). |
| GET | `/tasks/stats` | Counters: `total`, `ready`, `critical`, `scheduled`, `by_status`, `by_priority`, `cycles`, `phases`, `runner`, `recent_failures`. |
| GET | `/tasks/cycle-failures` | Paginated terminal cycle failures. `?limit`, `?offset`, `?sort ∈ at_desc | at_asc | reason_asc | reason_desc`. |
| GET | `/tasks/{id}` | Single flat `domain.Task`. |
| PATCH | `/tasks/{id}` | At least one of: `title`, `initial_prompt`, `status`, `priority`, `project_id`, `worktree_id`, `project_context_item_ids`, `pickup_not_before`, `cursor_model`, `tags`, `milestone`, `gate`, `depends_on`. Publishes `task_updated` (+ `task_gate_changed` / `task_dependency_changed` when those fields change). Writable `status` values for `X-Actor: user`: `ready`, `running`, `blocked`, `review`, `done`, `failed`, `on_hold`. See [data-model.md](./data-model.md). |
| DELETE | `/tasks/{id}` | `204` empty body. Publishes `task_deleted`. |
| GET | `/tasks/{id}/events` | Audit log. Default: ascending all rows. With `limit` / `before_seq` / `after_seq`: keyset-paged newest-first slice with `range_*`, `has_more_*`, `approval_pending`. Deep dive: [domain/task-events.md](./domain/task-events.md). |
| GET | `/tasks/{id}/events/{seq}` | Single event row. |
| PATCH | `/tasks/{id}/events/{seq}` | Append a user-response message (max 10 000 bytes after trim, thread cap 200). Only for `approval_requested` and `task_failed`. Publishes `task_updated`. |
| GET | `/tasks/{id}/dependencies` | `{ depends_on: [{ task_id, satisfies }] }`. |
| POST | `/tasks/{id}/dependencies` | Body `{ depends_on_task_id, satisfies? }` (default `done`). Cycles / self-deps rejected. Publishes `task_dependency_changed`. |
| DELETE | `/tasks/{id}/dependencies/{depId}` | `204`. Publishes `task_dependency_changed`. |
| PATCH | `/tasks/{id}/gate` | Body `{ action: release | hold | clear_hold }`. Publishes `task_gate_changed` and `task_updated`. |
| POST | `/tasks/{id}/retry` | Operator retry after task `failed`. Body `{ mode: fresh|resume, parent_cycle_id? }`. Requires `X-Actor: user`. Resolves `parent_cycle_id` to the latest terminal cycle (`failed` or `aborted`, max `attempt_seq`) when omitted. Sets ephemeral `pending_retry` on the task row and `status=ready`. `409` when already `ready` with a different pending intent; idempotent `200` when the same mode+parent is re-posted. Appends `task_retry_requested` audit event. Publishes `task_updated`. Bare `PATCH failed→ready` without this route leaves `pending_retry` null (legacy run). |

### Checklist

| Method | Path | Notes |
|---|---|---|
| GET | `/tasks/{id}/checklist` | `{ items: [...] }` ordered by `sort_order`. Each item includes optional `verify_commands: [{ sort_order, command, expected_outcome }]`. |
| POST | `/tasks/{id}/checklist/items` | Body `{ text, verify_commands? }`. Rejected `409` when the task is `running` or a cycle is running. Allowed on `done` tasks for post-completion edits. Publishes `task_updated`. |
| PATCH | `/tasks/{id}/checklist/items/{itemId}` | Body: exactly one of `{ text }`, `{ verify_commands }`, or `{ done: true|false }`. Rejected `409` when the task is `running` or a cycle is running. Satisfied criteria remain locked until the task is `done`. `done:true` requires `X-Actor: agent` plus `evidence` + optional `verified_by`. Publishes `task_updated`. |
| DELETE | `/tasks/{id}/checklist/items/{itemId}` | `204`. Rejected `409` when the task is `running` or a cycle is running. Publishes `task_updated`. |

### Task drafts

| Method | Path | Notes |
|---|---|---|
| POST | `/task-drafts` | Upsert. Body `{ id?, name, payload }`. Never publishes on SSE. |
| GET | `/task-drafts` | List summaries (without `payload`). `?limit` (0–100). |
| GET | `/task-drafts/{id}` | Full draft with `payload` defaulted to `{}`. |
| DELETE | `/task-drafts/{id}` | `204`. |

### Task templates

Named, durable task compose blueprints. Payload shape matches task create fields (title, prompt, status, priority, checklist, runner, project, schedule, tags, milestone, `depends_on`). Never publishes on SSE for CRUD; instantiate publishes `task_created` per success (same as `POST /tasks`).

| Method | Path | Notes |
|---|---|---|
| POST | `/task-templates` | Upsert. Body `{ id?, name?, payload }`. `name` defaults to trimmed `payload.title`. Validates like `POST /tasks` (title, priority, checklist, runner/model, prompt @-mentions when repo enabled). **201** summary `{ id, name, created_at, updated_at }`. |
| GET | `/task-templates` | List summaries (without `payload`). `?limit` (0–100, default 50). `?q=` ILIKE search on `name`. |
| GET | `/task-templates/{id}` | Full template with `payload`. |
| PATCH | `/task-templates/{id}` | Partial `{ name?, payload? }`. **200** full detail. |
| DELETE | `/task-templates/{id}` | `204`. |
| POST | `/task-templates/instantiate` | Body `{ template_ids: string[], count?: number }` **or** `{ items: { template_id, count? }[] }`. When `items` is non-empty it takes precedence over `template_ids` / top-level `count`. Omitted `count` defaults to **1** per template. Per-item and top-level `count` must be **1..25**; total creates (`sum(counts)`) must not exceed **100**. Duplicate `template_id` in `items` is **400**. Processes each item in order, creating `count` tasks per item. **200** `{ tasks: Task[], errors: { template_id, error }[] }`. Strips `depends_on`; omits past `pickup_not_before`. **400** when neither `template_ids` nor `items` is provided. |

### Execution cycles

See [data-model.md](./data-model.md) for state machine and substrate semantics.

| Method | Path | Notes |
|---|---|---|
| POST | `/tasks/{id}/cycles` | Start a cycle. Body `{ parent_cycle_id?, meta? }`. Returns `taskCycleResponse` (with typed `cycle_meta` projection). Publishes `task_cycle_changed`. |
| GET | `/tasks/{id}/cycles` | List. `?limit` (1–200), `?before_attempt_seq` keyset cursor. Newest first. |
| GET | `/tasks/{id}/cycles/{cycleId}` | One cycle with `phases[]` ordered ascending. |
| PATCH | `/tasks/{id}/cycles/{cycleId}` | Terminate. Body `{ status: succeeded|failed|aborted, reason? }`. Publishes `task_cycle_changed`. The agent worker emits `verification_failed:<id>,<id>,…` on terminal verify failure (sorted, deduped failing criterion IDs); the `verification_failed` prefix is contract-stable — clients MUST use prefix matching. Bare `verification_failed` (older cycles) remains a valid value. The reason column is 256 chars; long lists are truncated with `…` while the prefix stays intact. |
| GET | `/tasks/{id}/cycles/{cycleId}/stream` | Normalized Cursor live-run history. `?limit` (1–500), `?after_seq` keyset. |
| GET | `/tasks/{id}/commits` | Task-wide git commits (deduped by SHA, earliest `committed_at` wins). Returns `{ task_id, commits: [{ cycle_id, attempt_seq, seq, repo, worktree, branch, sha, committed_at, message }] }`. Refetch on `task_cycle_changed` after execute ingest. |
| GET | `/tasks/{id}/cycles/{cycleId}/verdicts` | Per-criterion verdict evidence for one cycle. Returns `{ task_id, cycle_id, git_context?, commits: [...], criteria_reports: [...], verify_reports: [...], command_runs: [...] }`. `git_context` (`{ repo, worktree, branch }`) is omitted when no commits were indexed; when present, `repo`/`worktree` come from the first commit and `branch` from the last commit with a non-empty branch (fallback: first). `commits[]` is always non-null (empty when no rows); entries are `{ seq, repo, worktree, branch, sha, committed_at, message }` ordered `seq ASC`. Criteria, verify, and command arrays are non-null (empty when no rows mirrored); those rows are ordered `(attempt_seq ASC, criterion_id ASC)` (command runs also by `command_seq ASC`). Each criteria row carries `claimed_done` + `evidence` from the execute agent's self-report; each verify row carries `verified` + `verifier_kind` + `reasoning`; each command run carries `exit_code` + `meta_path` for worker-executed verify shell checks. `verifier_kind` is the same enum as `task_checklist_completions.verified_by`. Pre-ADR-0014 cycles return empty `commits` and omit `git_context`. |
| POST | `/tasks/{id}/cycles/{cycleId}/phases` | Start a phase. Body `{ phase: execute|verify }`. Transitions follow `domain.ValidPhaseTransition`. Publishes `task_cycle_changed`. |
| PATCH | `/tasks/{id}/cycles/{cycleId}/phases/{phaseSeq}` | Terminate a phase. Body `{ status: succeeded|failed|skipped, summary?, details? }`. Publishes `task_cycle_changed`. |

## Runners

Runner adapters register at compile time via `pkgs/agents/runner/registry`. The SPA Settings page discovers available runners through these routes. Full plug-in model: [domain/runner-adapters.md](./domain/runner-adapters.md).

| Method | Path | Notes |
|---|---|---|
| GET | `/runners` | Array of `{ id, label, default_binary_hint, config_schema? }`. `config_schema` present when the adapter implements `ConfigSchemaProvider`. |
| GET | `/runners/{id}/config-schema` | Returns the adapter config schema. **404** unknown runner. **501** runner does not expose a schema. |
| POST | `/runners/{id}/validate-config` | Body: opaque JSON config blob. **200** `{ valid: true }` or **422** `{ valid: false, error }`. **404** unknown runner. **501** no validator. |
| POST | `/runners/{id}/probe` | Body `{ binary_path? }`. When `binary_path` is omitted, falls back to `app_settings.cursor_bin` for `cursor` only. Probe/CLI failures return **200** `{ ok: false, error, runner, binary_path? }` so the SPA renders inline. **404** unknown runner. **501** adapter does not implement `Prober`. Success: **200** `{ ok: true, version, binary_path, runner }`. |
| POST | `/runners/{id}/list-models` | Same body and soft-failure semantics as probe. Success: **200** `{ ok: true, models: [{ id, label }], binary_path, runner }`. **501** when `ModelLister` is not implemented. Timeout: 30s. |

Legacy cursor-named routes under `/settings` (`probe-cursor`, `list-cursor-models`) remain; prefer `/runners/*` for new UI work.

## App settings

Singleton row (`id=1`) seeded on first read with `domain.DefaultAppSettings`. Full field reference: [configuration.md](./configuration.md).

| Method | Path | Notes |
|---|---|---|
| GET | `/settings` | Returns the full `AppSettings` row. Always available. |
| GET | `/settings/workspace-roots` | `{ roots: [{ id, path, label, category?, available, unavailable_reason? }], environment: "native"\|"docker" }`. Browse roots for the workspace folder picker. `category` is one of `install`, `home`, `documents`, `desktop`, `downloads`, `pictures`, `music`, `videos`, or `custom`. Does not require `repo_root`. `Cache-Control: no-store`. |
| GET | `/settings/browse-dirs?path=` | `{ path?, parent_path?, is_git_repo?, entries: [{ name, path, has_children, is_git_repo }] }`. Lists immediate subdirectories under allowed browse roots. When `path` is set, `is_git_repo` reflects whether that directory is a git checkout. Empty `path` lists available roots. Does not require `repo_root`. **400** when path escapes roots. `Cache-Control: no-store`. |
| GET | `/settings/git-probe?path=` | `{ path, is_git_repository, current_branch?, branches: [{ name, head_sha }] }`. Opens the path with git and lists local branches without registering a repository. `is_git_repository: false` and empty `branches` when the path is not a checkout. Does not require `repo_root`. **400** when `path` is missing. `Cache-Control: no-store`. |
| PATCH | `/settings` | Partial; pointer fields distinguish "not provided" from explicit zero. On success, supervisor reloads in-process and SSE publishes `settings_changed`. |
| POST | `/settings/probe-cursor` | Body `{ runner?, binary_path? }`. Probe failures return `200 { ok: false, error }` so the SPA renders inline. |
| POST | `/settings/list-cursor-models` | Same fallback semantics as probe. CLI failures return `200 { ok: false }`. |
| POST | `/settings/cancel-current-run` | Cancels any in-flight `runner.Run`. Cycle terminated with `cancelled_by_operator`. Publishes `agent_run_cancelled` when something was running. |

## Workspace repo

Deep dive: [domain/workspace-repo.md](./domain/workspace-repo.md). Wired only when `app_settings.repo_root` is set. When unset, every `/repo/*` route returns `409 { error: "repo root is not configured", reason: "repo_root_not_configured" }`. When `OpenRoot` rejects the path (missing, not a directory, symlink loop), routes return `500 { reason: "repo_root_open_failed", error }`.

| Method | Path | Notes |
|---|---|---|
| GET | `/repo/search?q=` | Capped list of repo-relative paths; `q` ≤ 512 bytes. |
| GET | `/repo/file?path=` | `{ path, content, binary, truncated, size_bytes, line_count, warning? }`. Binary or invalid UTF-8 returns `binary: true` with empty `content`. Files over 32 MiB are truncated. |
| GET | `/repo/validate-range?path=&start=&end=` | `{ ok, line_count?, warning? }`. Used by the SPA to warn about invalid `@`-mentions inline. |
| GET | `/repo/diff?sha=` | `{ sha, patch, truncated, size_bytes, author?, author_email?, parent_sha?, files_changed?, insertions?, deletions? }`. Unified diff for one commit via `git show` in the configured `repo_root`; `sha` is 7–40 hex chars (≤ 64 bytes query). Patch capped at 512 KiB (`truncated: true` when clipped). Author and shortstat come from `git show --format` / `--shortstat`. **404** when SHA is absent from the repo. |

`POST /tasks` and `PATCH /tasks/{id}` validate `@`-mentions in `initial_prompt` against the configured repo. Failures return `400` with the offending mention in the error message (`@<path>` or `@<path> (<start>-<end>)`). Validation is skipped when `repo_root` is unset, `initial_prompt` is omitted, or `initial_prompt = ""`.

## SSE — `GET /events`

Deep dive: [domain/sse-hub.md](./domain/sse-hub.md). `text/event-stream`. First frame: `retry: 3000`. Frames are id + JSON:

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

Read-only GETs never publish. Failed writes never publish. Drafts (`/task-drafts/*`), task templates CRUD (`/task-templates` except instantiate), and `POST /settings/probe-cursor` are not part of the SSE surface.

### Dev synthetic SSE (`HAMIX_SSE_TEST=1`)

For local UI work, `taskapi` can start a background ticker (no extra routes). Set `HAMIX_SSE_TEST=1`; interval via `HAMIX_SSE_TEST_INTERVAL` (default `3s`; `0` disables). Tunables: `HAMIX_SSE_TEST_EVENTS_PER_TICK`, `HAMIX_SSE_TEST_SYNC_ROW`, `HAMIX_SSE_TEST_USER_RESPONSE`, `HAMIX_SSE_TEST_LIFECYCLE`, `HAMIX_SSE_TEST_LIFECYCLE_EVERY`. Never enable in production without intent. Source: `pkgs/tasks/devsim/`.
