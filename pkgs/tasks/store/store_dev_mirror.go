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
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ApplyDevTaskRowMirror")
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var t domain.Task
		if err := tx.Where("id = ?", taskID).First(&t).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return domain.ErrNotFound
			}
			return fmt.Errorf("load task: %w", err)
		}
		up := map[string]any{}
		switch typ {
		case domain.EventStatusChanged:
			m, err := pairFromJSON(data)
			if err != nil {
				return err
			}
			st := domain.Status(m["to"])
			if validStatus(st) && st != t.Status {
				if st == domain.StatusDone {
					if err := validateCanMarkDoneTx(tx, taskID); err != nil {
						return err
					}
				}
				up["status"] = string(st)
			}
		case domain.EventPriorityChanged:
			m, err := pairFromJSON(data)
			if err != nil {
				return err
			}
			pr := domain.Priority(m["to"])
			if validPriority(pr) && pr != t.Priority {
				up["priority"] = string(pr)
			}
		case domain.EventPromptAppended:
			m, err := pairFromJSON(data)
			if err != nil {
				return err
			}
			to := m["to"]
			if to != "" && to != t.InitialPrompt {
				up["initial_prompt"] = to
			}
		case domain.EventMessageAdded:
			m, err := pairFromJSON(data)
			if err != nil {
				return err
			}
			to := strings.TrimSpace(m["to"])
			if to != "" && to != t.Title {
				up["title"] = to
			}
		case domain.EventTaskCompleted:
			if err := validateCanMarkDoneTx(tx, taskID); err != nil {
				return err
			}
			if t.Status != domain.StatusDone {
				up["status"] = string(domain.StatusDone)
			}
		case domain.EventTaskFailed:
			if t.Status != domain.StatusFailed {
				up["status"] = string(domain.StatusFailed)
			}
		default:
			return nil
		}
		if len(up) == 0 {
			return nil
		}
		if err := tx.Model(&domain.Task{}).Where("id = ?", taskID).Updates(up).Error; err != nil {
			return fmt.Errorf("mirror task row: %w", err)
		}
		return nil
	})
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
