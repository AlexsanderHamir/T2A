package devmirror

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/checklist"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/kernel"
	"gorm.io/gorm"
)

const logCmd = "taskapi"

// ApplyTaskRowMirror updates the task row to reflect a synthetic
// audit event without appending further audit rows. Returns the
// reloaded task and its previous status so the caller can fire the
// ready-task notifier exactly once when the row transitioned into
// StatusReady. For development simulation only (see
// pkgs/tasks/devsim).
func ApplyTaskRowMirror(ctx context.Context, db *gorm.DB, taskID string, typ domain.EventType, data []byte) (*domain.Task, domain.Status, error) {
	defer kernel.DeferLatency(kernel.OpApplyDevTaskRowMirror)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.devmirror.ApplyTaskRowMirror")
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, "", fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	var prevStatus domain.Status
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var t domain.Task
		if err := tx.Where("id = ?", taskID).First(&t).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("load task: %w", err)
		}
		prevStatus = t.Status
		up, uerr := rowUpdates(tx, taskID, &t, typ, data)
		if uerr != nil {
			return uerr
		}
		if len(up) == 0 {
			return nil
		}
		if err := tx.Model(&domain.Task{}).Where("id = ?", taskID).Updates(up).Error; err != nil {
			return fmt.Errorf("mirror task row: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	var nt domain.Task
	if err := db.WithContext(ctx).Where("id = ?", taskID).First(&nt).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, "", domain.ErrNotFound
		}
		return nil, "", fmt.Errorf("reload after mirror: %w", err)
	}
	return &nt, prevStatus, nil
}

// ListDevsimTasks returns tasks whose id matches a SQL LIKE pattern
// (dev simulation only). Empty pattern returns ErrInvalidInput so
// callers cannot accidentally enumerate the entire task table.
func ListDevsimTasks(ctx context.Context, db *gorm.DB, idLikePattern string) ([]domain.Task, error) {
	defer kernel.DeferLatency(kernel.OpListDevsimTasks)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.devmirror.ListDevsimTasks")
	p := strings.TrimSpace(idLikePattern)
	if p == "" {
		return nil, fmt.Errorf("%w: pattern", domain.ErrInvalidInput)
	}
	var out []domain.Task
	if err := db.WithContext(ctx).Where("id LIKE ?", p).Order("id ASC").Find(&out).Error; err != nil {
		return nil, fmt.Errorf("list devsim tasks: %w", err)
	}
	return out, nil
}

func rowUpdates(tx *gorm.DB, taskID string, t *domain.Task, typ domain.EventType, data []byte) (map[string]any, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.devmirror.rowUpdates")
	switch typ {
	case domain.EventStatusChanged:
		return statusChanged(tx, taskID, t, data)
	case domain.EventPriorityChanged:
		return priorityChanged(t, data)
	case domain.EventPromptAppended:
		return promptOrTitle(t, data, "prompt")
	case domain.EventMessageAdded:
		return promptOrTitle(t, data, "title")
	case domain.EventTaskCompleted:
		return taskCompleted(tx, taskID, t)
	case domain.EventTaskFailed:
		return taskFailed(t), nil
	default:
		return nil, nil
	}
}

func statusChanged(tx *gorm.DB, taskID string, t *domain.Task, data []byte) (map[string]any, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.devmirror.statusChanged")
	m, err := pairFromJSON(data)
	if err != nil {
		return nil, err
	}
	up := map[string]any{}
	st := domain.Status(m["to"])
	if kernel.ValidStatus(st) && st != t.Status {
		if st == domain.StatusDone {
			if err := checklist.ValidateCanMarkDoneInTx(tx, taskID); err != nil {
				return nil, err
			}
		}
		up["status"] = string(st)
	}
	return up, nil
}

func priorityChanged(t *domain.Task, data []byte) (map[string]any, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.devmirror.priorityChanged")
	m, err := pairFromJSON(data)
	if err != nil {
		return nil, err
	}
	up := map[string]any{}
	pr := domain.Priority(m["to"])
	if kernel.ValidPriority(pr) && pr != t.Priority {
		up["priority"] = string(pr)
	}
	return up, nil
}

func promptOrTitle(t *domain.Task, data []byte, field string) (map[string]any, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.devmirror.promptOrTitle")
	m, err := pairFromJSON(data)
	if err != nil {
		return nil, err
	}
	up := map[string]any{}
	if field == "prompt" {
		to := m["to"]
		if to != "" && to != t.InitialPrompt {
			up["initial_prompt"] = to
		}
		return up, nil
	}
	to := strings.TrimSpace(m["to"])
	if to != "" && to != t.Title {
		up["title"] = to
	}
	return up, nil
}

func taskCompleted(tx *gorm.DB, taskID string, t *domain.Task) (map[string]any, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.devmirror.taskCompleted")
	if err := checklist.ValidateCanMarkDoneInTx(tx, taskID); err != nil {
		return nil, err
	}
	up := map[string]any{}
	if t.Status != domain.StatusDone {
		up["status"] = string(domain.StatusDone)
	}
	return up, nil
}

func taskFailed(t *domain.Task) map[string]any {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.devmirror.taskFailed")
	up := map[string]any{}
	if t.Status != domain.StatusFailed {
		up["status"] = string(domain.StatusFailed)
	}
	return up
}

func pairFromJSON(data []byte) (map[string]string, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.devmirror.pairFromJSON")
	var m map[string]string
	if len(data) == 0 || string(data) == "null" {
		return m, nil
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("decode pair json: %w", err)
	}
	return m, nil
}
