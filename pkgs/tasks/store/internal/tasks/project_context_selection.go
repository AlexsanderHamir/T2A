package tasks

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/gorm"
)

const maxSelectedProjectContextItems = 20

func normalizeProjectContextItemIDs(raw []string) ([]string, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.normalizeProjectContextItemIDs")
	if len(raw) > maxSelectedProjectContextItems {
		return nil, fmt.Errorf("%w: project_context_item_ids max %d", domain.ErrInvalidInput, maxSelectedProjectContextItems)
	}
	out := make([]string, 0, len(raw))
	seen := make(map[string]bool, len(raw))
	for _, value := range raw {
		id := strings.TrimSpace(value)
		if id == "" {
			return nil, fmt.Errorf("%w: project_context_item_ids contains empty id", domain.ErrInvalidInput)
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out, nil
}

func validateProjectContextSelection(tx *gorm.DB, projectID string, ids []string) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.validateProjectContextSelection",
		"project_id", projectID, "count", len(ids))
	var count int64
	if err := tx.Model(&domain.ProjectContextItem{}).
		Where("project_id = ? AND id IN ?", strings.TrimSpace(projectID), ids).
		Count(&count).Error; err != nil {
		return fmt.Errorf("project context selection lookup: %w", err)
	}
	if count != int64(len(ids)) {
		return fmt.Errorf("%w: project context item not found", domain.ErrInvalidInput)
	}
	return nil
}
