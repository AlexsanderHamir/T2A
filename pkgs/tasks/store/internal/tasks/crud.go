package tasks

import (
	"context"
	"encoding/json"
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
	t, title, parentID, st, err := buildCreateTaskFromInput(in, by)
	if err != nil {
		return nil, err
	}
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return createTaskInTx(tx, t, in, by, title, parentID, st)
	})
	if err != nil {
		return nil, fmt.Errorf("create task: %w", err)
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
	if in.Title == nil && in.InitialPrompt == nil && in.Status == nil && in.Priority == nil && in.TaskType == nil && in.Parent == nil && in.ChecklistInherit == nil {
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

// Delete removes a leaf task (children block deletion) and, when the
// task had a parent, appends a subtask_removed audit event on the
// parent and returns that parent id (so the facade can fan out an
// SSE poke). When the task was a root, the returned parent id is "".
func Delete(ctx context.Context, db *gorm.DB, id string, by domain.Actor) (string, error) {
	defer kernel.DeferLatency(kernel.OpDeleteTask)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.Delete")
	if err := kernel.ValidateActor(by); err != nil {
		return "", err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	var parentToNotify string
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		pid, txErr := deleteTaskInTx(tx, id, by)
		if txErr != nil {
			return txErr
		}
		parentToNotify = pid
		return nil
	})
	if err != nil {
		return "", err
	}
	return parentToNotify, nil
}

func buildCreateTaskFromInput(in CreateInput, by domain.Actor) (t *domain.Task, title string, parentID *string, st domain.Status, err error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.buildCreateTaskFromInput")
	if err := kernel.ValidateActor(by); err != nil {
		return nil, "", nil, "", err
	}
	title = strings.TrimSpace(in.Title)
	if title == "" {
		return nil, "", nil, "", fmt.Errorf("%w: title required", domain.ErrInvalidInput)
	}
	st = in.Status
	if st == "" {
		st = domain.StatusReady
	}
	if !kernel.ValidStatus(st) {
		return nil, "", nil, "", fmt.Errorf("%w: status", domain.ErrInvalidInput)
	}
	pr := in.Priority
	if pr == "" {
		return nil, "", nil, "", fmt.Errorf("%w: priority required", domain.ErrInvalidInput)
	}
	if !kernel.ValidPriority(pr) {
		return nil, "", nil, "", fmt.Errorf("%w: priority", domain.ErrInvalidInput)
	}
	tt := in.TaskType
	if tt == "" {
		tt = domain.TaskTypeGeneral
	}
	if !kernel.ValidTaskType(tt) {
		return nil, "", nil, "", fmt.Errorf("%w: task_type", domain.ErrInvalidInput)
	}
	id := strings.TrimSpace(in.ID)
	if id == "" {
		id = uuid.NewString()
	}
	parentID = in.ParentID
	if parentID != nil {
		p := strings.TrimSpace(*parentID)
		if p == "" {
			parentID = nil
		} else {
			parentID = &p
		}
	}
	if in.ChecklistInherit && (parentID == nil || *parentID == "") {
		return nil, "", nil, "", fmt.Errorf("%w: checklist_inherit requires parent_id", domain.ErrInvalidInput)
	}
	t = &domain.Task{
		ID:               id,
		Title:            title,
		InitialPrompt:    in.InitialPrompt,
		Status:           st,
		Priority:         pr,
		TaskType:         tt,
		ParentID:         parentID,
		ChecklistInherit: in.ChecklistInherit,
	}
	return t, title, parentID, st, nil
}

func createTaskInTx(tx *gorm.DB, t *domain.Task, in CreateInput, by domain.Actor, title string, parentID *string, st domain.Status) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.createTaskInTx")
	if parentID != nil {
		var n int64
		if err := tx.Model(&domain.Task{}).Where("id = ?", *parentID).Count(&n).Error; err != nil {
			return fmt.Errorf("parent lookup: %w", err)
		}
		if n == 0 {
			return fmt.Errorf("%w: parent not found", domain.ErrInvalidInput)
		}
	}
	if err := tx.Create(t).Error; err != nil {
		if isDuplicatePrimaryKey(err) {
			return fmt.Errorf("%w: task id already exists", domain.ErrConflict)
		}
		return fmt.Errorf("insert task: %w", err)
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
	seq++
	if parentID != nil {
		pseq, err := kernel.NextEventSeq(tx, *parentID)
		if err != nil {
			return err
		}
		pb, err := json.Marshal(map[string]string{
			"child_task_id": t.ID,
			"title":         title,
		})
		if err != nil {
			return err
		}
		if err := kernel.AppendEvent(tx, *parentID, pseq, domain.EventSubtaskAdded, by, pb); err != nil {
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

func deleteTaskInTx(tx *gorm.DB, id string, by domain.Actor) (parentToNotify string, err error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.deleteTaskInTx")
	var t domain.Task
	if err := tx.Where("id = ?", id).First(&t).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", domain.ErrNotFound
		}
		return "", fmt.Errorf("load task: %w", err)
	}
	var childCount int64
	if err := tx.Model(&domain.Task{}).Where("parent_id = ?", id).Count(&childCount).Error; err != nil {
		return "", fmt.Errorf("delete task: %w", err)
	}
	if childCount > 0 {
		return "", fmt.Errorf("%w: delete subtasks first", domain.ErrInvalidInput)
	}
	if t.ParentID != nil {
		pid := strings.TrimSpace(*t.ParentID)
		if pid != "" {
			var pn int64
			if err := tx.Model(&domain.Task{}).Where("id = ?", pid).Count(&pn).Error; err != nil {
				return "", fmt.Errorf("parent lookup: %w", err)
			}
			if pn > 0 {
				pseq, err := kernel.NextEventSeq(tx, pid)
				if err != nil {
					return "", err
				}
				b, mErr := json.Marshal(map[string]string{
					"child_task_id": id,
					"title":         strings.TrimSpace(t.Title),
				})
				if mErr != nil {
					return "", mErr
				}
				if err := kernel.AppendEvent(tx, pid, pseq, domain.EventSubtaskRemoved, by, b); err != nil {
					return "", err
				}
				parentToNotify = pid
			}
		}
	}
	res := tx.Where("id = ?", id).Delete(&domain.Task{})
	if res.Error != nil {
		return "", fmt.Errorf("delete task: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return "", domain.ErrNotFound
	}
	return parentToNotify, nil
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
