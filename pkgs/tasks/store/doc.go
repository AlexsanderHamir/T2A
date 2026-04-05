// Package store provides GORM-backed persistence for domain.Task and domain.TaskEvent
// (pkgs/tasks/domain), including an append-only audit log on create and field changes.
//
// Prefer adding behavior as named use-case methods (small input structs, one transaction,
// explicit audit events) rather than ad hoc SQL from callers. The handler should stay thin;
// see docs/DESIGN.md "Extensibility" and .cursor/rules/13-tasks-stack-extensibility.mdc.
//
// List applies a maximum page size: limit ≤ 0 becomes 50; limit > 200 is capped at 200;
// negative offset becomes 0.
//
// Create records EventTaskCreated; Update appends events when title, prompt, status, or
// priority actually change. ListTaskEvents returns all audit rows in ascending seq order;
// ListTaskEventsPageCursor returns a descending-seq keyset page with total and navigation flags.
// Sentinel errors are domain.ErrNotFound and domain.ErrInvalidInput; the store does not log.
//
// [DefaultReadyTimeout] is the recommended context deadline for (*Store).Ready from GET /health/ready.
package store
