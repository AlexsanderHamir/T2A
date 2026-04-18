package store

import (
	"context"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/events"
)

// TaskEventsPage is one window of audit events (newest first) plus stable paging metadata.
// Aliased to internal/events so additions stay in one place; see events.Page.
type TaskEventsPage = events.Page

// ThreadEntriesForDisplay returns the conversation for API/list UI. Re-exported from
// internal/events so the handler and devsim test harness keep saying
// store.ThreadEntriesForDisplay unchanged.
func ThreadEntriesForDisplay(ev *domain.TaskEvent) []domain.ResponseThreadEntry {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ThreadEntriesForDisplay")
	return events.ThreadEntriesForDisplay(ev)
}

// AppendTaskEvent appends one task_events row if the task exists.
func (s *Store) AppendTaskEvent(ctx context.Context, taskID string, typ domain.EventType, by domain.Actor, data []byte) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.AppendTaskEvent")
	return events.Append(ctx, s.db, taskID, typ, by, data)
}

// ListTaskEvents returns audit events for a task in ascending sequence order.
func (s *Store) ListTaskEvents(ctx context.Context, taskID string) ([]domain.TaskEvent, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListTaskEvents")
	return events.List(ctx, s.db, taskID)
}

// TaskEventCount returns how many audit rows exist for the task.
func (s *Store) TaskEventCount(ctx context.Context, taskID string) (int64, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.TaskEventCount")
	return events.Count(ctx, s.db, taskID)
}

// LastEventSeq returns the highest seq for the task, or 0 when there are no events.
func (s *Store) LastEventSeq(ctx context.Context, taskID string) (int64, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.LastEventSeq")
	return events.LastSeq(ctx, s.db, taskID)
}

// GetTaskEvent returns one task_events row by composite key, or ErrNotFound.
func (s *Store) GetTaskEvent(ctx context.Context, taskID string, seq int64) (*domain.TaskEvent, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.GetTaskEvent")
	return events.Get(ctx, s.db, taskID, seq)
}

// ListTaskEventsPageCursor returns events in descending seq using keyset paging
// (see internal/events for cursor semantics).
func (s *Store) ListTaskEventsPageCursor(ctx context.Context, taskID string, limit int, beforeSeq, afterSeq *int64) (*TaskEventsPage, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListTaskEventsPageCursor")
	return events.PageCursor(ctx, s.db, taskID, limit, beforeSeq, afterSeq)
}

// ApprovalPending reports whether an approval is outstanding for the task.
func (s *Store) ApprovalPending(ctx context.Context, taskID string) (bool, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ApprovalPending")
	return events.ApprovalPending(ctx, s.db, taskID)
}

// AppendTaskEventResponseMessage appends one message (user or agent) to the
// event thread (see internal/events for validation rules).
func (s *Store) AppendTaskEventResponseMessage(ctx context.Context, taskID string, seq int64, text string, by domain.Actor) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.AppendTaskEventResponseMessage")
	return events.AppendResponseMessage(ctx, s.db, taskID, seq, text, by)
}
