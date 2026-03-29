// Command taskapi is an HTTP server for task CRUD backed by Postgres.
//
// It loads environment with envload.Load (repo-root .env or -env path), opens the database with
// pkgs/tasks/postgres.Open, optionally runs postgres.Migrate, constructs handler.NewSSEHub for
// task change notifications, optionally opens pkgs/repo from REPO_ROOT for GET /repo/* and prompt
// validation, then mounts handler.NewHandler (REST + GET /events SSE + optional /repo) on /.
//
// Flags (see also -h):
//
//	-port string    listen port (default "8080")
//	-env string     path to .env (default: <repo-root>/.env)
//	-migrate        run GORM AutoMigrate before serving
//
// Structured logs go to stderr. SIGINT/SIGTERM trigger graceful shutdown with a 10s timeout, then the database pool is closed.
//
// The HTTP server sets read header/read/idle timeouts and a max header size; WriteTimeout is left unset so GET /events (SSE) can stay open.
//
// REST contract: see package github.com/AlexsanderHamir/T2A/pkgs/tasks/handler and domain types
// in github.com/AlexsanderHamir/T2A/pkgs/tasks/domain.
package main
