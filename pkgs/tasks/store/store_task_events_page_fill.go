package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

func (s *Store) fillTaskEventsPageBounds(ctx context.Context, taskID string, out *TaskEventsPage, events []domain.TaskEvent) error {
	if len(events) == 0 {
		return nil
	}
	maxSeq := events[0].Seq
	minSeq := events[len(events)-1].Seq

	var newerThanMax int64
	if err := s.db.WithContext(ctx).Model(&domain.TaskEvent{}).
		Where("task_id = ? AND seq > ?", taskID, maxSeq).
		Count(&newerThanMax).Error; err != nil {
		return fmt.Errorf("count newer task events: %w", err)
	}
	var olderThanMin int64
	if err := s.db.WithContext(ctx).Model(&domain.TaskEvent{}).
		Where("task_id = ? AND seq < ?", taskID, minSeq).
		Count(&olderThanMin).Error; err != nil {
		return fmt.Errorf("count older task events: %w", err)
	}

	out.RangeStart = newerThanMax + 1
	out.RangeEnd = newerThanMax + int64(len(events))
	out.HasMoreNewer = newerThanMax > 0
	out.HasMoreOlder = olderThanMin > 0
	return nil
}

func validateListTaskEventsPageInputs(taskID string, limit int, beforeSeq, afterSeq *int64) (string, int, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return "", 0, fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	if beforeSeq != nil && afterSeq != nil {
		return "", 0, fmt.Errorf("%w: before_seq and after_seq are mutually exclusive", domain.ErrInvalidInput)
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if beforeSeq != nil && *beforeSeq < 1 {
		return "", 0, fmt.Errorf("%w: before_seq must be a positive integer", domain.ErrInvalidInput)
	}
	if afterSeq != nil && *afterSeq < 1 {
		return "", 0, fmt.Errorf("%w: after_seq must be a positive integer", domain.ErrInvalidInput)
	}
	return taskID, limit, nil
}
