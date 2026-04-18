package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/kernel"
	"gorm.io/gorm"
)

func deleteTaskInTx(tx *gorm.DB, id string, by domain.Actor) (parentToNotify string, err error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.deleteTaskInTx")
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
