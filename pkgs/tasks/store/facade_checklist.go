package store

import (
	"context"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/checklist"
)

// ChecklistItemView is the public re-export of the per-task checklist
// row shape returned by ListChecklistForSubject. The alias keeps the
// JSON field tags stable on the wire.
type ChecklistItemView = checklist.ItemView

// DefinitionSourceTaskID returns the task id that owns checklist item
// definitions for the given task; see internal/checklist for the
// inheritance walk.
func (s *Store) DefinitionSourceTaskID(ctx context.Context, taskID string) (string, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.DefinitionSourceTaskID")
	return checklist.DefinitionSourceTaskID(ctx, s.db, taskID)
}

// ListChecklistForSubject returns definition items for taskID with
// done flags for that same task.
func (s *Store) ListChecklistForSubject(ctx context.Context, taskID string) ([]ChecklistItemView, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListChecklistForSubject")
	return checklist.List(ctx, s.db, taskID)
}

// AddChecklistItem appends a definition row; the task must exist and
// not use checklist_inherit.
func (s *Store) AddChecklistItem(ctx context.Context, taskID, text string, by domain.Actor) (*domain.TaskChecklistItem, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.AddChecklistItem")
	return checklist.Add(ctx, s.db, taskID, text, by)
}

// DeleteChecklistItem removes a definition row owned by taskID.
func (s *Store) DeleteChecklistItem(ctx context.Context, taskID, itemID string, by domain.Actor) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.DeleteChecklistItem")
	return checklist.Delete(ctx, s.db, taskID, itemID, by)
}

// UpdateChecklistItemText updates the definition text for an item
// owned by taskID.
func (s *Store) UpdateChecklistItemText(ctx context.Context, taskID, itemID, text string, by domain.Actor) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.UpdateChecklistItemText")
	return checklist.UpdateText(ctx, s.db, taskID, itemID, text, by)
}

// SetChecklistItemDone sets or clears completion for subjectTaskID on
// an item from its definition source. Only [domain.ActorAgent] may
// change completion.
func (s *Store) SetChecklistItemDone(ctx context.Context, subjectTaskID, itemID string, done bool, by domain.Actor) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.SetChecklistItemDone")
	return checklist.SetDone(ctx, s.db, subjectTaskID, itemID, done, by)
}
