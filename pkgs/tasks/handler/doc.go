// Package handler exposes REST JSON CRUD for tasks backed by a store.Store (pkgs/tasks/store).
//
// # Routes (Go 1.22 patterns on the returned mux)
//
//   - GET    /events          — Server-Sent Events stream (text/event-stream); JSON lines with
//     type task_created | task_updated | task_deleted and id (UUID)
//   - POST   /tasks           — create; 201 + JSON task
//   - GET    /tasks           — list; query limit (0–200, default 50), offset (≥ 0, default 0)
//   - GET    /tasks/{id}/events — 200 + JSON { task_id, events[] } (seq, at, type, by, data); 404 if task missing
//   - GET    /tasks/{id}      — 200 + task
//   - PATCH  /tasks/{id}      — partial update; 200 + task
//   - DELETE /tasks/{id}      — 204, no body
//   - GET    /repo/search     — optional; JSON paths (q=); 503 if REPO_ROOT unset
//   - GET    /repo/validate-range — optional; JSON ok/warning (path, start, end); 503 if unset
//
// Dev-only (not mounted on the returned handler; register separately on the parent mux when
// T2A_SSE_TEST=1): GET /dev/sse/ping, POST /dev/sse/publish — task_updated uses store.Update (same
// as PATCH) then SSE; task_created/task_deleted on publish are SSE-only; see DESIGN.md.
//
// Header X-Actor: "user" (default) or "agent"; passed to the store for audit events.
//
// JSON bodies disallow unknown fields; trailing data after the top-level value is rejected.
//
// POST body: id (optional; default new UUID), title (required, non-empty after trim),
// initial_prompt, status, priority (see domain package for enums; defaults ready / medium).
//
// PATCH body: optional title, initial_prompt, status, priority (pointer fields: omitted key
// or JSON null means “no change”; at least one field must be non-nil after decode). See
// store.UpdateTaskInput.
//
// Errors: domain.ErrNotFound → 404 "not found", domain.ErrInvalidInput → 400 "bad request";
// other store errors → 500. Response bodies are plain text, not JSON. Failures are logged once
// at the handler with structured fields (including http_status); 4xx → Warn, 5xx → Error.
package handler
