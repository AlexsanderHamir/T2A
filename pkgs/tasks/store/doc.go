// Package store provides GORM-backed persistence for domain.Task and domain.TaskEvent
// (pkgs/tasks/domain), including an append-only audit log on create and field changes.
//
// List applies a maximum page size: limit ≤ 0 becomes 50; limit > 200 is capped at 200;
// negative offset becomes 0.
//
// Create records EventTaskCreated; Update appends events when title, prompt, status, or
// priority actually change. ListTaskEvents returns audit rows for a task in sequence order.
// Sentinel errors are domain.ErrNotFound and domain.ErrInvalidInput; the store does not log.
package store
