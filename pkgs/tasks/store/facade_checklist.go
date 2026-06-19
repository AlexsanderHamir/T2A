package store

import (
	"context"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/checklist"
	"gorm.io/gorm"
)

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// BackfillCriteriaSatisfiedAt sets criteria_satisfied_at for tasks whose
// checklist is already complete. Idempotent migration helper.
func BackfillCriteriaSatisfiedAt(ctx context.Context, db *gorm.DB) error {
	return checklist.BackfillCriteriaSatisfiedAt(ctx, db)
}

// ChecklistItemView is the public re-export of the per-task checklist
// row shape returned by ListChecklistForSubject. The alias keeps the
// JSON field tags stable on the wire.
type ChecklistItemView = checklist.ItemView

// ChecklistVerifyItem is a criterion row for worker verification.
type ChecklistVerifyItem = checklist.VerifyItem

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

// AddChecklistItem appends a definition row when the task is not running or done.
func (s *Store) AddChecklistItem(ctx context.Context, taskID, text string, verifyCommands []checklist.VerifyCommandInput, by domain.Actor) (*domain.TaskChecklistItem, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.AddChecklistItem")
	return checklist.Add(ctx, s.db, taskID, text, verifyCommands, by)
}

// ReplaceChecklistVerifyCommands replaces optional verify commands on a criterion.
func (s *Store) ReplaceChecklistVerifyCommands(ctx context.Context, taskID, itemID string, cmds []checklist.VerifyCommandInput, by domain.Actor) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ReplaceChecklistVerifyCommands")
	return checklist.ReplaceVerifyCommands(ctx, s.db, taskID, itemID, cmds, by)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// NormalizeVerifyCommands validates optional verify command inputs.
func NormalizeVerifyCommands(in []VerifyCommandInput) ([]VerifyCommandInput, error) {
	return checklist.NormalizeVerifyCommandInputs(in)
}

// CreateChecklistItemInput is the public re-export for task-create checklist rows.
type CreateChecklistItemInput = checklist.CreateChecklistItemInput

// VerifyCommandInput is the public re-export for checklist verify command wire shape.
type VerifyCommandInput = checklist.VerifyCommandInput

// ListChecklistForVerify returns criteria rows for worker verification.
func (s *Store) ListChecklistForVerify(ctx context.Context, taskID string) ([]ChecklistVerifyItem, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListChecklistForVerify")
	return checklist.ListForVerify(ctx, s.db, taskID)
}

// IsTaskCycleRunning reports whether the task or an inherit ancestor has a running cycle.
func (s *Store) IsTaskCycleRunning(ctx context.Context, taskID string) (bool, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.IsTaskCycleRunning")
	return checklist.IsTaskCycleRunning(ctx, s.db, taskID)
}

// SetChecklistItemDoneWithEvidence records agent completion with proof metadata.
func (s *Store) SetChecklistItemDoneWithEvidence(
	ctx context.Context,
	subjectTaskID, itemID string,
	evidence string,
	verifier domain.VerifierKind,
	reasoning, cycleID string,
	by domain.Actor,
) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.SetChecklistItemDoneWithEvidence")
	flag, err := checklist.SetDoneWithEvidence(ctx, s.db, subjectTaskID, itemID, evidence, verifier, reasoning, cycleID, by)
	if err != nil {
		return err
	}
	if flag.BecameComplete {
		s.notifyUnblockedDependents(ctx, subjectTaskID)
	}
	return nil
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
	before, _ := s.Get(ctx, subjectTaskID)
	if err := checklist.SetDone(ctx, s.db, subjectTaskID, itemID, done, by); err != nil {
		return err
	}
	after, _ := s.Get(ctx, subjectTaskID)
	if before != nil && after != nil && before.CriteriaSatisfiedAt == nil && after.CriteriaSatisfiedAt != nil {
		s.notifyUnblockedDependents(ctx, subjectTaskID)
	}
	return nil
}
