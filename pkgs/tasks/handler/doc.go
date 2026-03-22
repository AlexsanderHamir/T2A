// Package handler exposes REST JSON CRUD for tasks backed by a store.Store (pkgs/tasks/store).
//
// # Routes (Go 1.22 patterns on the returned mux)
//
//   - POST   /tasks           — create; 201 + JSON task
//   - GET    /tasks           — list; query limit (0–200, default 50), offset (≥ 0, default 0)
//   - GET    /tasks/{id}      — 200 + task
//   - PATCH  /tasks/{id}      — partial update; 200 + task
//   - DELETE /tasks/{id}      — 204, no body
//
// Header X-Actor: "user" (default) or "agent"; passed to the store for audit events.
//
// JSON bodies disallow unknown fields; trailing data after the top-level value is rejected.
//
// POST body: id (optional; default new UUID), title (required, non-empty after trim),
// initial_prompt, status, priority (see domain package for enums; defaults ready / medium).
//
// PATCH body: optional title, initial_prompt, status, priority (JSON null to clear optional
// pointer fields in decoding); at least one field required. See store.UpdateTaskInput.
//
// Errors: domain.ErrNotFound → 404 "not found", domain.ErrInvalidInput → 400 "bad request";
// other store errors → 500. Response bodies are plain text, not JSON.
package handler
