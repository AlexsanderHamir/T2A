package store

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/gorm"
)

// TaskEventsPage is one window of audit events (newest first) plus stable paging metadata.
type TaskEventsPage struct {
	Events       []domain.TaskEvent
	Total        int64
	RangeStart   int64
	RangeEnd     int64
	HasMoreNewer bool
	HasMoreOlder bool
}

// ListTaskEventsPageCursor returns events in **descending seq** (newest first) using keyset paging.
//   - Neither beforeSeq nor afterSeq: first page (newest rows).
//   - beforeSeq: rows with seq strictly less than beforeSeq (older page).
//   - afterSeq: rows with seq strictly greater than afterSeq (newer page), still returned newest first.
//
// Limit is coerced: ≤0 becomes 50; >200 capped at 200. beforeSeq and afterSeq must not both be set.
func (s *Store) ListTaskEventsPageCursor(ctx context.Context, taskID string, limit int, beforeSeq, afterSeq *int64) (*TaskEventsPage, error) {
	defer deferStoreLatency(storeOpListTaskEventsPage)()
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListTaskEventsPageCursor")
	var err error
	taskID, limit, err = validateListTaskEventsPageInputs(taskID, limit, beforeSeq, afterSeq)
	if err != nil {
		return nil, err
	}

	var total int64
	if err := s.db.WithContext(ctx).Model(&domain.TaskEvent{}).Where("task_id = ?", taskID).Count(&total).Error; err != nil {
		return nil, fmt.Errorf("count task events: %w", err)
	}

	q := s.db.WithContext(ctx).Where("task_id = ?", taskID)
	if beforeSeq != nil {
		q = q.Where("seq < ?", *beforeSeq)
	} else if afterSeq != nil {
		q = q.Where("seq > ?", *afterSeq)
	}
	var events []domain.TaskEvent
	err = q.Order("seq DESC").Limit(limit).Find(&events).Error
	if err != nil {
		return nil, fmt.Errorf("list task events page: %w", err)
	}

	out := &TaskEventsPage{Events: events, Total: total}
	if err := s.fillTaskEventsPageBounds(ctx, taskID, out, events); err != nil {
		return nil, err
	}
	return out, nil
}

// ApprovalPending reports whether an approval is outstanding: among approval-related
// events, the latest by seq decides — granted clears pending, requested sets it.
func (s *Store) ApprovalPending(ctx context.Context, taskID string) (bool, error) {
	defer deferStoreLatency(storeOpApprovalPending)()
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ApprovalPending")
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return false, fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	types := []domain.EventType{domain.EventApprovalRequested, domain.EventApprovalGranted}
	var row domain.TaskEvent
	err := s.db.WithContext(ctx).
		Where("task_id = ? AND type IN ?", taskID, types).
		Order("seq DESC").
		Limit(1).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("approval pending lookup: %w", err)
	}
	switch row.Type {
	case domain.EventApprovalGranted:
		return false, nil
	case domain.EventApprovalRequested:
		return true, nil
	default:
		return false, nil
	}
}
