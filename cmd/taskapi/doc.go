// Command taskapi is an HTTP server for task CRUD backed by Postgres.
//
// It loads environment with envload.Load (repo-root .env or -env path), opens the database with
// pkgs/tasks/postgres.Open, optionally runs postgres.Migrate, registers internal/ui routes
// (GET / and GET /static/*), then mounts handler.NewHandler for the JSON task API on /.
//
// Flags (see also -h):
//
//   -port string    listen port (default "8080")
//   -env string     path to .env (default: <repo-root>/.env)
//   -migrate        run GORM AutoMigrate before serving
//
// Structured logs go to stderr. SIGINT/SIGTERM trigger graceful shutdown with a 10s timeout.
//
// REST contract: see package github.com/AlexsanderHamir/T2A/pkgs/tasks/handler and domain types
// in github.com/AlexsanderHamir/T2A/pkgs/tasks/domain.
package main
