package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

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
	var origStatus domain.Status
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var cur domain.Task
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&cur).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("load task: %w", err)
		}
		origStatus = cur.Status

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
