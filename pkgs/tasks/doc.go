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
//	db, err := postgres.Open(dsn, nil)
//	if err != nil { ... }
//	if err := postgres.Migrate(ctx, db); err != nil { ... }
//	s := store.NewStore(db)
//	hub := handler.NewSSEHub()
//	http.Handler = handler.NewHandler(s, hub)
package tasks
