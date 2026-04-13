package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// isDuplicateTaskPrimaryKey detects unique/PK violations on task insert across GORM + SQLite + Postgres drivers.
func isDuplicateTaskPrimaryKey(err error) bool {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.isDuplicateTaskPrimaryKey")
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

func (s *Store) Create(ctx context.Context, in CreateTaskInput, by domain.Actor) (*domain.Task, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.Create")
	if err := validateActor(by); err != nil {
		return nil, err
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return nil, fmt.Errorf("%w: title required", domain.ErrInvalidInput)
	}
	st := in.Status
	if st == "" {
		st = domain.StatusReady
	}
	if !validStatus(st) {
		return nil, fmt.Errorf("%w: status", domain.ErrInvalidInput)
	}
	pr := in.Priority
	if pr == "" {
		return nil, fmt.Errorf("%w: priority required", domain.ErrInvalidInput)
	}
	if !validPriority(pr) {
		return nil, fmt.Errorf("%w: priority", domain.ErrInvalidInput)
	}
	tt := in.TaskType
	if tt == "" {
		tt = domain.TaskTypeGeneral
	}
	if !validTaskType(tt) {
		return nil, fmt.Errorf("%w: task_type", domain.ErrInvalidInput)
	}
	id := strings.TrimSpace(in.ID)
	if id == "" {
		id = uuid.NewString()
	}

	parentID := in.ParentID
	if parentID != nil {
		p := strings.TrimSpace(*parentID)
		if p == "" {
			parentID = nil
		} else {
			parentID = &p
		}
	}
	if in.ChecklistInherit && (parentID == nil || *parentID == "") {
		return nil, fmt.Errorf("%w: checklist_inherit requires parent_id", domain.ErrInvalidInput)
	}

	t := &domain.Task{
		ID:               id,
		Title:            title,
		InitialPrompt:    in.InitialPrompt,
		Status:           st,
		Priority:         pr,
		TaskType:         tt,
		ParentID:         parentID,
		ChecklistInherit: in.ChecklistInherit,
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
			if isDuplicateTaskPrimaryKey(err) {
				return fmt.Errorf("%w: task id already exists", domain.ErrConflict)
			}
			return fmt.Errorf("insert task: %w", err)
		}
		seq := int64(1)
		if err := attachDraftEvaluationsTx(tx, in.DraftID, t.ID); err != nil {
			return err
		}
		if err := deleteDraftByIDTx(tx, in.DraftID); err != nil {
			return err
		}
		if err := appendEvent(tx, t.ID, seq, domain.EventTaskCreated, by, nil); err != nil {
			return err
		}
		seq++
		if parentID != nil {
			pseq, err := nextEventSeq(tx, *parentID)
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
			if err := appendEvent(tx, *parentID, pseq, domain.EventSubtaskAdded, by, pb); err != nil {
				return err
			}
		}
		if st == domain.StatusDone {
			if err := validateCanMarkDoneTx(tx, t.ID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}
	return t, nil
}

func (s *Store) Get(ctx context.Context, id string) (*domain.Task, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.Get")
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	var t domain.Task
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&t).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get task: %w", err)
	}
	return &t, nil
}

func (s *Store) Update(ctx context.Context, id string, in UpdateTaskInput, by domain.Actor) (*domain.Task, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.Update")
	if err := validateActor(by); err != nil {
		return nil, err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	if in.Title == nil && in.InitialPrompt == nil && in.Status == nil && in.Priority == nil && in.TaskType == nil && in.Parent == nil && in.ChecklistInherit == nil {
		return nil, fmt.Errorf("%w: no fields to update", domain.ErrInvalidInput)
	}

	var updated *domain.Task
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var cur domain.Task
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&cur).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("load task: %w", err)
		}

		nextSeq, err := nextEventSeq(tx, id)
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
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("update task: %w", err)
	}
	return updated, nil
}

// Delete removes a task with no children. When the task had a parent, appends
// subtask_removed on the parent and returns that parent id (for SSE); otherwise returns "".
func (s *Store) Delete(ctx context.Context, id string, by domain.Actor) (string, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.Delete")
	if err := validateActor(by); err != nil {
		return "", err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	var parentToNotify string
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var t domain.Task
		if err := tx.Where("id = ?", id).First(&t).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("load task: %w", err)
		}
		var childCount int64
		if err := tx.Model(&domain.Task{}).Where("parent_id = ?", id).Count(&childCount).Error; err != nil {
			return fmt.Errorf("delete task: %w", err)
		}
		if childCount > 0 {
			return fmt.Errorf("%w: delete subtasks first", domain.ErrInvalidInput)
		}
		if t.ParentID != nil {
			pid := strings.TrimSpace(*t.ParentID)
			if pid != "" {
				var pn int64
				if err := tx.Model(&domain.Task{}).Where("id = ?", pid).Count(&pn).Error; err != nil {
					return fmt.Errorf("parent lookup: %w", err)
				}
				if pn > 0 {
					pseq, err := nextEventSeq(tx, pid)
					if err != nil {
						return err
					}
					b, mErr := json.Marshal(map[string]string{
						"child_task_id": id,
						"title":         strings.TrimSpace(t.Title),
					})
					if mErr != nil {
						return mErr
					}
					if err := appendEvent(tx, pid, pseq, domain.EventSubtaskRemoved, by, b); err != nil {
						return err
					}
					parentToNotify = pid
				}
			}
		}
		res := tx.Where("id = ?", id).Delete(&domain.Task{})
		if res.Error != nil {
			return fmt.Errorf("delete task: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return domain.ErrNotFound
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return parentToNotify, nil
}
