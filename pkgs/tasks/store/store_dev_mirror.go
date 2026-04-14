package store

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/gorm"
)

// ApplyDevTaskRowMirror updates the task row to reflect a synthetic audit event without
// appending further audit rows. For development simulation only (see pkgs/tasks/devsim).
func (s *Store) ApplyDevTaskRowMirror(ctx context.Context, taskID string, typ domain.EventType, data []byte) error {
	defer deferStoreLatency(storeOpApplyDevTaskRowMirror)()
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ApplyDevTaskRowMirror")
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	var prevStatus domain.Status
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var t domain.Task
		if err := tx.Where("id = ?", taskID).First(&t).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return domain.ErrNotFound
			}
			return fmt.Errorf("load task: %w", err)
		}
		prevStatus = t.Status
		up, uerr := devMirrorRowUpdates(tx, taskID, &t, typ, data)
		if uerr != nil {
			return uerr
		}
		if up == nil || len(up) == 0 {
			return nil
		}
		if err := tx.Model(&domain.Task{}).Where("id = ?", taskID).Updates(up).Error; err != nil {
			return fmt.Errorf("mirror task row: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	nt, gerr := s.Get(ctx, taskID)
	if gerr != nil {
		return fmt.Errorf("reload after mirror: %w", gerr)
	}
	if nt.Status == domain.StatusReady && prevStatus != domain.StatusReady {
		s.notifyReadyTask(ctx, *nt)
	}
	return nil
}

func pairFromJSON(data []byte) (map[string]string, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.pairFromJSON")
	var m map[string]string
	if len(data) == 0 || string(data) == "null" {
		return m, nil
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("decode pair json: %w", err)
	}
	return m, nil
}
