package store

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// ListDevsimTasks returns tasks whose id matches a SQL LIKE pattern (dev simulation only).
func (s *Store) ListDevsimTasks(ctx context.Context, idLikePattern string) ([]domain.Task, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListDevsimTasks")
	p := strings.TrimSpace(idLikePattern)
	if p == "" {
		return nil, fmt.Errorf("%w: pattern", domain.ErrInvalidInput)
	}
	var out []domain.Task
	if err := s.db.WithContext(ctx).Where("id LIKE ?", p).Order("id ASC").Find(&out).Error; err != nil {
		return nil, fmt.Errorf("list devsim tasks: %w", err)
	}
	return out, nil
}
