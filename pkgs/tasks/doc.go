// Package tasks defines task domain types, Postgres persistence via GORM, an append-only
// audit log ([TaskEvent]) on create and on field changes, and an [http.Handler] for REST CRUD.
//
// # Database
//
// At process startup, open the pool with [OpenPostgres] using a Postgres URI (for example from
// DATABASE_URL). Optionally run [MigratePostgreSQL] once to create or update the tasks and
// task_events tables. Construct a [Store] with the resulting *gorm.DB.
//
// # Store
//
// [Store] methods implement create, get, list, update, and delete. [Store.List] applies a
// maximum page size: if limit is less than or equal to zero, it defaults to 50; if greater
// than 200, it is capped at 200. Negative offset is normalized to zero.
//
// [Store.Create] records [EventTaskCreated]. [Store.Update] appends events when title,
// initial prompt, status, or priority actually change (see event kinds in this package).
// Updates require at least one patch field; empty title after trim is invalid.
//
// Errors: [ErrNotFound] and [ErrInvalidInput] are returned from store methods; callers map
// them to HTTP responses. The store does not log.
//
// # HTTP handler
//
// [NewHandler] returns a mux that registers these routes (Go 1.22 patterns):
//
//   - POST   /tasks           — create; 201 + JSON task
//   - GET    /tasks           — list; query limit (0–200, default 50), offset (≥ 0, default 0); 200 + {tasks, limit, offset}
//   - GET    /tasks/{id}      — 200 + task
//   - PATCH  /tasks/{id}      — partial update; 200 + task
//   - DELETE /tasks/{id}      — 204, no body
//
// Header X-Actor: "user" (default) or "agent"; stored on events as the acting party.
//
// JSON request bodies are decoded with disallow-unknown-fields enabled; trailing data after
// the top-level value is rejected.
//
// POST /tasks body fields: id (optional; default id is task_ + UUID), title (required,
// non-empty after trim), initial_prompt, status, priority (enums; defaults ready / medium).
//
// PATCH /tasks/{id} body: optional title, initial_prompt, status, priority (omit or JSON null
// to leave unchanged). At least one field must be present.
//
// Handler error mapping: [ErrNotFound] → 404 "not found", [ErrInvalidInput] → 400 "bad
// request", other errors → 500 "internal server error". Bodies are plain text, not JSON.
//
// Fixtures and decode tests live under testdata/ and in handler_helpers_test.go.
package tasks
