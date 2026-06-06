# Configuration

Two surfaces:

- **Environment variables** — process-level knobs (logging, listen host, HTTP limits, agent queue capacity, idempotency cache). Authoritative source: `internal/taskapiconfig` and `pkgs/tasks/middleware`.
- **`app_settings` DB row** — UI-driven runtime config (workspace repo, agent worker, runner, verify loop). Singleton row (`id=1`) authored from the SPA Settings page or `PATCH /settings`. Authoritative source: `pkgs/tasks/domain/app_settings.go`.

The two surfaces do not overlap. Anything in `app_settings` is **not** driven by env vars (and historical env vars like `T2A_AGENT_WORKER_*` and `REPO_ROOT` are silently ignored).

## Environment variables

`taskapi` loads `.env` from the repo root via `internal/envload.Load`. `dbcheck` follows the same discovery rule for `DATABASE_URL`.

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `DATABASE_URL` | Yes (after env load) | — | Postgres connection string for GORM. |
| `T2A_LISTEN_HOST` | No | `127.0.0.1` | HTTP bind host. `0.0.0.0` for all interfaces. `taskapi -host` flag overrides. |
| `T2A_API_TOKEN` | No | — | When set, `Authorization: Bearer <token>` required on every route except `/health*` and `/metrics`. |
| `T2A_HTTP_REQUEST_TIMEOUT` | No | `30s` | Go duration. Request execution timeout for non-SSE routes via context deadline. `0` disables. `GET /events` is exempt. |
| `T2A_LOG_DIR` | No | `./logs` | Directory for JSON log files. `taskapi -logdir` flag overrides. |
| `T2A_LOG_LEVEL` | No | `info` | Minimum `slog` level (`debug` / `info` / `warn` / `error`). `taskapi -loglevel` flag overrides. |
| `T2A_DISABLE_LOGGING` | No | — | `1`/`true`/`yes`/`on`: no JSONL file; only `slog.Error` to stderr. Same as `taskapi -disable-logging`. |
| `T2A_GORM_SLOW_QUERY_MS` | No | `200` | Statements slower than this log at `Warn`. `0` disables slow-SQL branch. |
| `T2A_RATE_LIMIT_PER_MIN` | No | `120` | Per-IP token bucket. `0` disables. Key is `RemoteAddr` host (no trusted `X-Forwarded-For`). Exempt: `/health*`, `/metrics`. Over limit: `429 rate limit exceeded` with `Retry-After: 60`. |
| `T2A_IDEMPOTENCY_TTL` | No | `24h` | Idempotency cache TTL for `Idempotency-Key`. `0` disables. In-process only — not shared across replicas. |
| `T2A_IDEMPOTENCY_MAX_ENTRIES` | No | `2048` | Max idempotency cache entries. `0` disables entry-count bounding. |
| `T2A_IDEMPOTENCY_MAX_BYTES` | No | `8388608` (8 MiB) | Max idempotency cache memory. `0` disables byte bounding. |
| `T2A_MAX_REQUEST_BODY_BYTES` | No | `1048576` (1 MiB) | Reject larger bodies with `413 request body too large`. `0` disables. |
| `T2A_USER_TASK_AGENT_QUEUE_CAP` | No | `256` | Bounded depth of `pkgs/agents.MemoryQueue`. Not durable, not shared. |
| `T2A_WORKER_REPORT_DIR` | No | `<os.TempDir()>/t2a-worker` | Worker-managed scratch root for the agent ↔ worker side-channel report files (`criteria-report.json`, `verify-report.json`). Lives outside `app_settings.repo_root` so customer working trees stay clean. The supervisor probes writability at startup; failure logs a `report_dir_not_writable` warn and the worker still starts (verify will fail loudly on the first run instead of silently). The per-cycle `<dir>/<cycle_id>/` subdirectory is GC'd at cycle terminate so disk use stays bounded. |
| `T2A_SSE_TEST` | No | — | Dev: enable synthetic SSE ticker. See [api.md](./api.md). |
| `T2A_SSE_TEST_*` | No | — | Dev tuning (interval, events per tick, lifecycle simulation). See [api.md](./api.md) and `.env.example`. |

Reconcile tick interval is fixed in code (`pkgs/agents.ReconcileTickInterval`, 2 minutes), not an env var.

### Startup sequence (`taskapi`)

1. Resolve `.env` (repo-root or `-env`), overlay logging env vars first so `T2A_LOG_*` apply before the log file is opened, then `envload.Load` (requires `DATABASE_URL`).
2. Open the log file (`taskapi-YYYY-MM-DD-HHMMSS-<nanos>.jsonl` under `T2A_LOG_DIR`). When `T2A_DISABLE_LOGGING` is set, only `slog.Error` goes to stderr (text handler).
3. `postgres.Open` — GORM connection. Configures `database/sql` pool (max open/idle, lifetime). No startup `Ping`.
4. `postgres.Migrate` — `AutoMigrate` for every domain model under `postgres.DefaultMigrateTimeout` (120s).
5. `store.NewStore`, `(*store.Store).SetReadyTaskNotifier` (in-process queue), `(*store.Store).SetPickupWake` (deferred ready), `handler.NewSSEHub`.
6. Agent worker supervisor (`cmd/taskapi/run_agentworker.go`) reads `app_settings`, builds the runner via `pkgs/agents/runner/registry`, probes the binary, and starts the worker when conditions are met.
7. `internal/taskapi.NewHTTPHandler` wires store + hub + repo into `handler.NewHandler`, then applies `pkgs/tasks/middleware.Stack` (recovery, metrics, access logging, rate limit, optional auth, timeouts, body cap, idempotency).
8. `http.Server` on `-port` (default 8080). `ReadHeaderTimeout` / `ReadTimeout` / `IdleTimeout` / `MaxHeaderBytes` are set; `WriteTimeout` is intentionally **not** set so SSE streams are not cut off.

### Graceful shutdown

On `SIGINT`/`SIGTERM`: `http.Server.Shutdown` with a 10s deadline, then close the SQL pool, sync and close the log file. Exit code `0` on clean shutdown, `1` if `Close` errors.

### `dbcheck`

`go run ./cmd/dbcheck` connects, pings (`postgres.DefaultPingTimeout` = 30s), optionally migrates (`-migrate`, 120s), and exits. Does not serve HTTP.

### Build identity

`/health`, `/health/live`, `/health/ready` return `version` (from `runtime/debug.ReadBuildInfo`). `taskapi` logs the same value on the `listening` line; `dbcheck` logs it on `dbcheck.start`. Use it to confirm which binary handled traffic.

### Request correlation

Every request gets a `request_id` (from `X-Request-ID` or a generated UUID). The handler wraps `slog` so access logs, GORM SQL traces, and JSON error bodies (`request_id` field) all share that value. JSON error bodies may include `request_id`; the SPA's `readError` appends it to error messages.

## App settings (`app_settings` row)

Singleton row in Postgres (CHECK enforces `id=1`). AutoMigrate creates the table; first read seeds it with `domain.DefaultAppSettings`. Authored via the SPA Settings page (gear icon → `/settings`) or `PATCH /settings`.

| Field | Type | Default | Effect |
|---|---|---|---|
| `agent_paused` | bool | `false` | Operator-facing soft pause exposed as a one-click toggle in the SPA header chip. The agent worker always starts at boot; pause is the only "stop dequeuing" knob. Idle reason: `paused_by_operator`. Surfaces in `GET /system/health`. |
| `runner` | string | `"cursor"` | Identifier from `pkgs/agents/runner/registry`. Only `cursor` is registered today. |
| `repo_root` | string | `""` | Absolute path to the workspace the worker and `/repo/*` operate against. **Empty = not configured**: supervisor stays idle, repo routes respond `409 repo_root_not_configured`, `@`-mention validation is skipped. |
| `cursor_bin` | string | `""` | Cursor CLI binary path. Empty = `PATH` lookup of `cursor`. Absolute paths pin a build. |
| `cursor_model` | string | `""` | Optional Cursor model forwarded to the runner. Empty = omit the model flag (Cursor uses account default). |
| `max_run_duration_seconds` | int (≥0) | `0` | Per-run wall-clock cap on `runner.Request.Timeout`. `0` = no limit. |
| `agent_pickup_delay_seconds` | int (≥0) | `5` | Delay applied to new ready tasks before the worker can dequeue them. `0` disables. |
| `display_timezone` | string | `""` | IANA timezone for SPA timestamps. Empty = browser auto-detect. Validated via `time.LoadLocation`. |
| `optimistic_mutations_enabled` | bool | `true` | Always-on compatibility field. |
| `sse_replay_enabled` | bool | `true` | Always-on compatibility field. |
| `verify_max_retries` | int (0–10) | `2` | Max execute↔verify retry loops per cycle. |
| `verify_runner_name` | string | `""` | Adversarial verify runner id. Empty = reuse execute runner. When set to a different id (e.g. `claudecode`), the supervisor builds and probes that runner separately at startup and on every `PATCH /settings`; build/probe failure logs `verify_runner_probe_failed` / `verify_runner_build_failed` and demotes verify to "reuse execute runner" so the worker keeps running. Setting it equal to `runner` is equivalent to leaving it empty. |
| `verify_runner_model` | string | `""` | Optional model for the verify runner. Changing this triggers a worker restart on `PATCH /settings`. |
| `check_command_timeout_seconds` | int (1–600) | `120` | Wall-clock cap for each deterministic `check` command. |
| `updated_at` | RFC3339 (response only) | server clock | Last successful upsert. SPA shows "last changed N ago". |

### Validation

`store.UpdateSettings` rejects (`400`) when:

- `runner` is non-empty and not in `pkgs/agents/runner/registry`.
- `max_run_duration_seconds` is negative.
- `verify_max_retries` is outside `0..10`.
- `check_command_timeout_seconds` is outside `1..600`.
- `repo_root` contains a NUL byte.

`repo_root` is **not** validated for "directory exists" on `PATCH` — the supervisor reports `repo_root_open_failed` on the next reload, surfaced via `/health/ready` (`workspace_repo: fail`).

### Lifecycle on `PATCH /settings`

```mermaid
sequenceDiagram
  participant SPA
  participant API as PATCH /settings
  participant Store
  participant Sup as agent worker supervisor
  participant Hub as SSEHub

  SPA->>API: partial fields
  API->>Store: UpdateSettings (locks id=1)
  Store-->>API: updated row
  API->>Sup: Reload(ctx)
  Sup-->>API: ok / err
  alt reload ok
    API->>Hub: Publish(settings_changed)
    Hub-->>SPA: SSE: settings_changed
    API-->>SPA: 200 with new row
  else reload err
    API-->>SPA: 500 settings saved but worker reload failed
  end
```

`Reload` is idempotent: when no material field changed, the supervisor leaves the in-flight worker alone.

### Migration from env vars

The variables below are silently ignored if still present in `.env`. Move the values into `app_settings` via the SPA or `PATCH /settings`.

| Old env var | Replacement |
|---|---|
| `T2A_AGENT_WORKER_ENABLED` | Deprecated. The agent worker always starts; use the header pause toggle (`agent_paused`) for a runtime stop. |
| `T2A_AGENT_WORKER_CURSOR_BIN` | `app_settings.cursor_bin`. |
| `T2A_AGENT_WORKER_RUN_TIMEOUT` | `app_settings.max_run_duration_seconds` (default `0` = no limit, not 5m). |
| `T2A_AGENT_WORKER_WORKING_DIR` | `app_settings.repo_root`. |
| `REPO_ROOT` | `app_settings.repo_root`. |

### Test-only override

Real-cursor smoke tests honour `T2A_TEST_CURSOR_BIN` to point at a specific binary path. This is unrelated to production `app_settings.cursor_bin`; it only wires test runs.
