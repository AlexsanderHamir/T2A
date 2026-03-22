// Package tasks is the root import path for the task subsystem. Implementations are split into
// subpackages:
//
//   - domain — models, enums, errors, SQL enum scanning
//   - postgres — GORM PostgreSQL open and schema migration (Migrate also used with SQLite in tests)
//   - store — CRUD and append-only task_events audit log
//   - handler — REST JSON API on net/http
//
// Typical wiring (see cmd/taskapi):
//
//	db, _ := postgres.Open(dsn, nil)
//	postgres.Migrate(ctx, db)
//	s := store.NewStore(db)
//	http.Handler = handler.NewHandler(s)
package tasks
