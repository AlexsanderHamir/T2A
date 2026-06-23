// Command taskapi is an HTTP server for task CRUD backed by Postgres.
// File map for this directory: README.md in the same folder.
//
// Taskapi-specific startup env parsing (listen host, log level, agent queue cap, dev SSE interval)
// lives in package github.com/AlexsanderHamir/Hamix/internal/taskapiconfig; shared .env discovery is internal/envload.
// The instrumented HTTP stack (middleware wrapping handler.NewHandler) is github.com/AlexsanderHamir/Hamix/internal/taskapi.
//
// It loads environment with envload.Load (repo-root .env or -env path), opens the database with
// pkgs/tasks/postgres.Open, optionally runs postgres.Migrate when -migrate or HAMIX_MIGRATE is set,
// checks schema revision drift on every startup, constructs handler.NewSSEHub for
// task change notifications, optionally opens pkgs/repo from app_settings.repo_root for GET /repo/* and prompt
// validation, then mounts handler.NewHandler (REST + GET /events SSE + optional /repo) on / with
// WithRecovery, WithHTTPMetrics, WithAccessLog, WithRateLimit, WithAPIAuth, WithRequestTimeout, WithMaxRequestBody, and WithIdempotency; GET /metrics (Prometheus text) is registered separately on the mux behind handler.WrapPrometheusHandler (baseline security headers).
//
// Flags (see also -h):
//
//	-host string     listen host/IP (default: HAMIX_LISTEN_HOST env or 127.0.0.1)
//	-port string     listen port (default "8080")
//	-env string      path to .env (default: <repo-root>/.env)
//	-logdir string   directory for JSON log files (default: HAMIX_LOG_DIR env or ./logs)
//	-loglevel string minimum level for the JSON log file: debug, info, warn, error (default: HAMIX_LOG_LEVEL env or info)
//	-disable-logging  no JSON log file; only slog.Error to stderr (default: HAMIX_DISABLE_LOGGING=1|true|yes|on)
//	-migrate         run GORM AutoMigrate before serving (default: HAMIX_MIGRATE=1|true|yes|on, otherwise skip)
//
// Each process start creates a new file named taskapi-YYYY-MM-DD-HHMMSS-<nanos>.jsonl (local time) under
// the log directory; records are JSON objects, one per line (slog JSON handler). One line is printed to
// stderr with the log file path. GORM SQL traces (duration, rows, parameterized SQL) use the same sink.
// SIGINT/SIGTERM trigger graceful shutdown with a 10s timeout, then the database pool is closed and the
// log file is synced and closed.
//
// The HTTP server sets read header/read/idle timeouts and a max header size; WriteTimeout is left unset so GET /events (SSE) can stay open.
//
// REST contract: see package github.com/AlexsanderHamir/Hamix/pkgs/tasks/handler and domain types
// in github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain.
package main
