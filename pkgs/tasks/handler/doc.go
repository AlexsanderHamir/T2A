// Package handler exposes REST JSON CRUD for tasks backed by a store.Store (pkgs/tasks/store).
// Wiring and shared HTTP helpers: handler.go. Task routes and DTOs: handler_tasks.go.
// GET /repo/*: repo_handlers.go. GET /events: sse.go. Prometheus HTTP metrics: metrics_http.go (WithHTTPMetrics; GET /metrics is mounted on the outer mux in cmd/taskapi).
// Per-IP rate limiting: rate_limit.go (WithRateLimit; T2A_RATE_LIMIT_PER_MIN in docs/DESIGN.md).
// Idempotency: idempotency.go (WithIdempotency; optional Idempotency-Key on POST/PATCH/DELETE; T2A_IDEMPOTENCY_TTL in docs/DESIGN.md).
// Max request body: max_body.go (WithMaxRequestBody; optional T2A_MAX_REQUEST_BODY_BYTES in docs/DESIGN.md).
// Request/response IO summaries (Debug): httplog_io.go.
// Nested call stack for logs (call_path, helper.io): calllog.go — use withCallRoot on each handler, PushCall inside helpers.
// JSONL order: WrapSlogHandlerWithLogSequence (taskapi outer) + ContextWithLogSeq in access middleware → log_seq, log_seq_scope; RunObserved for explicit helper in/out pairs.
//
// Mutating routes should follow: decode and validate the request, call the store, map errors
// to HTTP status, then call notifyChange after a successful write. Keep domain rules in
// store/domain, not in HTTP adapters (see docs/DESIGN.md "Extensibility").
//
// # Routes (Go 1.22 patterns on the returned mux)
//
//   - GET    /events          — Server-Sent Events stream (text/event-stream); JSON lines with
//     type task_created | task_updated | task_deleted and id (UUID)
//   - POST   /tasks           — create; 201 + JSON task tree (same shape as GET)
//   - GET    /tasks           — list root tasks only (parent_id null); query limit (0–200, default 50), offset (≥ 0) or keyset after_id (UUID, mutually exclusive with offset); response includes has_more; each element includes nested children[]
//   - GET    /tasks/{id}/checklist — 200 + JSON { items: [{ id, sort_order, text, done }] } for this task (definition from self or inherited ancestor)
//   - POST   /tasks/{id}/checklist/items — body { text }; 201 + checklist item row; 400 if checklist_inherit
//   - PATCH  /tasks/{id}/checklist/items/{itemId} — exactly one of { text } (non-empty) or { done: bool }; 200 + full { items }; done requires X-Actor agent; text allowed for user or agent; 400 if checklist_inherit
//   - DELETE /tasks/{id}/checklist/items/{itemId} — 204; 400 if checklist_inherit
//   - GET    /tasks/{id}/events/{seq} — 200 + JSON { task_id, seq, at, type, by, data }; 404 if no such row; 400 if seq invalid
//   - GET    /tasks/{id}/events — 200 + JSON { task_id, events[], approval_pending }; optional query limit (0–200) with keyset cursors before_seq / after_seq (positive ints, mutually exclusive) for paging (newest first; stable under concurrent inserts). offset is rejected. Unpaged full list when limit, before_seq, and after_seq are all omitted; 404 if task missing
//   - GET    /tasks/{id}      — 200 + task tree (nested children[])
//   - PATCH  /tasks/{id}      — partial update; 200 + task tree
//   - DELETE /tasks/{id}      — 204, no body; 400 if the task still has subtasks
//   - GET    /repo/search     — optional; JSON paths (q=); 503 if REPO_ROOT unset
//   - GET    /repo/validate-range — optional; JSON ok/warning (path, start, end); 503 if unset
//
// Dev-only: when taskapi sets T2A_SSE_TEST=1, pkgs/tasks/devsim runs a background ticker (store.ListFlat + AppendTaskEvent,
// rotates all EventType, ActorAgent) per task then notifies the SSE hub (see DESIGN.md). No extra HTTP routes.
//
// Header X-Actor: "user" (default) or "agent"; passed to the store for audit events.
//
// Optional header Idempotency-Key (non-empty, max 128 bytes after trim): mutating requests with the same
// method, path, key, and (for POST/PATCH) request body replay the first successful 200/201/204 response
// for the configured TTL (in-process cache; see DESIGN.md).
//
// JSON bodies disallow unknown fields; trailing data after the top-level value is rejected.
//
// POST body: id (optional; default new UUID), title (required, non-empty after trim),
// initial_prompt, status, priority (see domain package for enums; defaults ready; priority required),
// optional parent_id (existing task UUID), optional checklist_inherit (bool; requires parent_id when true).
//
// PATCH body: optional title, initial_prompt, status, priority, checklist_inherit, parent_id
// (JSON null clears parent). At least one field must be present. See store.UpdateTaskInput.
//
// Errors: domain.ErrNotFound → 404, domain.ErrInvalidInput → 400, domain.ErrConflict → 409 (duplicate client id on POST /tasks);
// other store errors → 500. Response bodies are JSON {"error":"..."} (same shape as writeJSONError). Failures are logged once
// at the handler with structured fields (including http_status); 4xx → Warn, 5xx → Error.
package handler
