package kernel

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/gorm"
)

// LoadTask reads one domain.Task by id inside the open transaction tx and
// maps gorm.ErrRecordNotFound to domain.ErrNotFound. Used by checklist,
// cycles, and any future subpackage that needs the task row before
// branching on its current state.
func LoadTask(tx *gorm.DB, id string) (*domain.Task, error) {
	slog.Debug("trace", "cmd", LogCmd, "operation", "tasks.store.kernel.LoadTask")
	var t domain.Task
	if err := tx.Where("id = ?", id).First(&t).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("load task: %w", err)
	}
	return &t, nil
}
