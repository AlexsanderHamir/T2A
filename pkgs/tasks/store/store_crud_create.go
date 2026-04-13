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
	if t.Status == domain.StatusReady {
		s.notifyReadyTask(ctx, *t)
	}
	return t, nil
}
