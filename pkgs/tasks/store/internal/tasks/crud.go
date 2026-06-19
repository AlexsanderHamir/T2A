package tasks

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/checklist"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/drafts"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/eval"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/kernel"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Get loads a task by id. Trimmed empty id is rejected with
// domain.ErrInvalidInput; missing rows surface as
// domain.ErrNotFound.
func Get(ctx context.Context, db *gorm.DB, id string) (*domain.Task, error) {
	defer kernel.DeferLatency(kernel.OpGetTask)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.Get")
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	var t domain.Task
	err := db.WithContext(ctx).Where("id = ?", id).First(&t).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get task: %w", err)
	}
	if err := hydrateDependsOn(ctx, db, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// Create inserts a new task row, links any draft evaluations, deletes
// the source draft (if any), appends the task_created (and parent
// subtask_added) audit events, and runs the checklist guard when the
// initial status is StatusDone — all in one transaction. The caller
// is responsible for firing the ready-task notifier when the returned
// task has Status == StatusReady (the facade does this).
func Create(ctx context.Context, db *gorm.DB, in CreateInput, by domain.Actor) (*domain.Task, error) {
	defer kernel.DeferLatency(kernel.OpCreateTask)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.Create")
	t, title, st, err := buildCreateTaskFromInput(in, by)
	if err != nil {
		return nil, err
	}
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return createTaskInTx(tx, t, in, by, title, st)
	})
	if err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}
	if err := hydrateDependsOn(ctx, db, t); err != nil {
		return nil, err
	}
	return t, nil
}

// Update applies the patch and returns (updated, prevStatus, err).
// prevStatus is the status before the patch was applied; the facade
// uses (updated.Status == StatusReady && prevStatus != StatusReady)
// to decide whether to notify the ready-task channel.
func Update(ctx context.Context, db *gorm.DB, id string, in UpdateInput, by domain.Actor) (*domain.Task, domain.Status, error) {
	defer kernel.DeferLatency(kernel.OpUpdateTask)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.Update")
	if err := kernel.ValidateActor(by); err != nil {
		return nil, "", err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, "", fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	if in.Title == nil && in.InitialPrompt == nil && in.Status == nil && in.Priority == nil && in.Project == nil && in.ProjectContextItemIDs == nil && in.AutomationSelections == nil && in.PickupNotBefore == nil && in.CursorModel == nil && in.Tags == nil && in.Milestone == nil && in.Gate == nil && in.DependsOn == nil && in.PendingRetry == nil && !in.ClearPendingRetry {
		return nil, "", fmt.Errorf("%w: no fields to update", domain.ErrInvalidInput)
	}
	var updated *domain.Task
	var origStatus domain.Status
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var cur domain.Task
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&cur).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("load task: %w", err)
		}
		origStatus = cur.Status
		nextSeq, err := kernel.NextEventSeq(tx, id)
		if err != nil {
			return err
		}
		if err := applyTaskPatches(tx, id, &cur, in, by, nextSeq); err != nil {
			return err
		}
		if err := tx.Save(&cur).Error; err != nil {
			return fmt.Errorf("save task: %w", err)
		}
		if err := hydrateDependsOn(ctx, tx, &cur); err != nil {
			return err
		}
		updated = &cur
		return nil
	})
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, "", domain.ErrNotFound
		}
		return nil, "", fmt.Errorf("update task: %w", err)
	}
	return updated, origStatus, nil
}

// Delete removes the task at id in one transaction.
func Delete(ctx context.Context, db *gorm.DB, id string, by domain.Actor) (deletedIDs []string, err error) {
	defer kernel.DeferLatency(kernel.OpDeleteTask)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.Delete")
	if err := kernel.ValidateActor(by); err != nil {
		return nil, err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Where("id = ?", id).Delete(&domain.Task{})
		if res.Error != nil {
			return fmt.Errorf("delete task: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return domain.ErrNotFound
		}
		deletedIDs = []string{id}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return deletedIDs, nil
}

func buildCreateTaskFromInput(in CreateInput, by domain.Actor) (t *domain.Task, title string, st domain.Status, err error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.buildCreateTaskFromInput")
	if err := kernel.ValidateActor(by); err != nil {
		return nil, "", "", err
	}
	title = strings.TrimSpace(in.Title)
	if title == "" {
		return nil, "", "", fmt.Errorf("%w: title required", domain.ErrInvalidInput)
	}
	st = in.Status
	if st == "" {
		st = domain.StatusReady
	}
	if !kernel.ValidClientWritableStatus(st) {
		return nil, "", "", fmt.Errorf("%w: status", domain.ErrInvalidInput)
	}
	pr := in.Priority
	if pr == "" {
		return nil, "", "", fmt.Errorf("%w: priority required", domain.ErrInvalidInput)
	}
	if !kernel.ValidPriority(pr) {
		return nil, "", "", fmt.Errorf("%w: priority", domain.ErrInvalidInput)
	}
	id := strings.TrimSpace(in.ID)
	if id == "" {
		id = uuid.NewString()
	}
	projectID := in.ProjectID
	if projectID != nil {
		p := strings.TrimSpace(*projectID)
		if p == "" {
			projectID = nil
		} else {
			projectID = &p
		}
	}
	runner := strings.TrimSpace(in.Runner)
	if runner == "" {
		runner = domain.DefaultRunner
	}
	t = &domain.Task{
		ID:                    id,
		Title:                 title,
		InitialPrompt:         in.InitialPrompt,
		Status:                st,
		Priority:              pr,
		ProjectID:             projectID,
		ProjectContextItemIDs: nil,
		Runner:                runner,
		CursorModel:           in.CursorModel,
		PickupNotBefore:       in.PickupNotBefore,
	}
	if err := normalizeCreateTaskModelFields(t, in); err != nil {
		return nil, "", "", err
	}
	return t, title, st, nil
}

func createTaskInTx(tx *gorm.DB, t *domain.Task, in CreateInput, by domain.Actor, title string, st domain.Status) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.createTaskInTx")
	if t.ProjectID != nil {
		var n int64
		if err := tx.Model(&domain.Project{}).Where("id = ? AND status = ?", *t.ProjectID, domain.ProjectStatusActive).Count(&n).Error; err != nil {
			return fmt.Errorf("project lookup: %w", err)
		}
		if n == 0 {
			return fmt.Errorf("%w: project not found", domain.ErrInvalidInput)
		}
	}
	contextIDs, err := normalizeProjectContextItemIDs(in.ProjectContextItemIDs)
	if err != nil {
		return err
	}
	if len(contextIDs) > 0 {
		if t.ProjectID == nil || strings.TrimSpace(*t.ProjectID) == "" {
			return fmt.Errorf("%w: project_id required for project context selection", domain.ErrInvalidInput)
		}
		if err := validateProjectContextSelection(tx, *t.ProjectID, contextIDs); err != nil {
			return err
		}
	}
	t.ProjectContextItemIDs = contextIDs
	if err := applyAutomationSelectionsOnCreate(tx, t, in.AutomationSelections); err != nil {
		return err
	}
	if err := tx.Create(t).Error; err != nil {
		if isDuplicatePrimaryKey(err) {
			return fmt.Errorf("%w: task id already exists", domain.ErrConflict)
		}
		return fmt.Errorf("insert task: %w", err)
	}
	if len(in.DependsOn) > 0 {
		if err := setDependenciesInTx(tx, t.ID, in.DependsOn); err != nil {
			return err
		}
		t.DependsOn = append([]domain.DependencyEdge(nil), in.DependsOn...)
	}
	seq := int64(1)
	if err := eval.AttachDraftEvaluationsInTx(tx, in.DraftID, t.ID); err != nil {
		return err
	}
	if err := drafts.DeleteByIDInTx(tx, in.DraftID); err != nil {
		return err
	}
	if err := kernel.AppendEvent(tx, t.ID, seq, domain.EventTaskCreated, by, nil); err != nil {
		return err
	}
	if len(in.ChecklistItems) > 0 {
		if err := checklist.SeedDefinitionItemsAtCreateInTx(tx, t.ID, in.ChecklistItems, by); err != nil {
			return err
		}
	}
	if st == domain.StatusDone {
		if err := checklist.ValidateCanMarkDoneInTx(tx, t.ID); err != nil {
			return err
		}
	}
	return nil
}

// isDuplicatePrimaryKey detects unique/PK violations on task insert
// across GORM + SQLite + Postgres drivers. Kept private because it
// only matters inside Create's transaction.
func isDuplicatePrimaryKey(err error) bool {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.isDuplicatePrimaryKey")
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "unique constraint failed") {
		return strings.Contains(msg, "tasks") && strings.Contains(msg, "id")
	}
	if strings.Contains(msg, "duplicate key value violates unique constraint") {
		return strings.Contains(msg, "tasks_pkey")
	}
	return false
}
