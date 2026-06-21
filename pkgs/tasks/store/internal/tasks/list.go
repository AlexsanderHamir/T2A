package tasks

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/kernel"
	"gorm.io/gorm"
)

// ListFlat returns tasks ordered by task_created time descending (newest
// first), then id descending. limit is clamped to [1, 200] (default 50)
// and offset to [0, +inf).
func ListFlat(ctx context.Context, db *gorm.DB, limit, offset int, filter *ListFilter) ([]domain.Task, error) {
	defer kernel.DeferLatency(kernel.OpListFlat)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.ListFlat")
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	q := db.WithContext(ctx).Model(&domain.Task{})
	q = applyListFilter(q, db, filter)
	q = applyTaskCreatedJoin(q)
	var rows []listRowScan
	err := q.Order(listOrderCreatedDesc).
		Limit(limit).
		Offset(offset).
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	out := tasksFromListRows(rows)
	for i := range out {
		if err := hydrateDependsOn(ctx, db, &out[i]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// ListFlatAfter is the keyset variant of ListFlat: returns tasks older
// than the task identified by afterID in the list sort order (created_at
// desc, id desc).
func ListFlatAfter(ctx context.Context, db *gorm.DB, limit int, afterID string) ([]domain.Task, bool, error) {
	defer kernel.DeferLatency(kernel.OpListFlatAfter)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.ListFlatAfter")
	afterID = strings.TrimSpace(afterID)
	if afterID == "" {
		return nil, false, fmt.Errorf("%w: after_id", domain.ErrInvalidInput)
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	cursorAt, err := loadCreatedAtCursor(ctx, db, afterID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, false, domain.ErrNotFound
		}
		return nil, false, err
	}
	q := db.WithContext(ctx).Model(&domain.Task{})
	q = applyTaskCreatedJoin(q)
	q = q.Where("(te.at < ? OR (te.at = ? AND tasks.id < ?))", cursorAt, cursorAt, afterID)
	var rows []listRowScan
	err = q.Order(listOrderCreatedDesc).Limit(limit + 1).Scan(&rows).Error
	if err != nil {
		return nil, false, fmt.Errorf("list tasks after id: %w", err)
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	out := tasksFromListRows(rows)
	for i := range out {
		if err := hydrateDependsOn(ctx, db, &out[i]); err != nil {
			return nil, false, err
		}
	}
	return out, hasMore, nil
}

// ListFlatPage returns a flat page with hasMore using limit+1 fetch.
// Rows are ordered newest-first by task_created time.
func ListFlatPage(ctx context.Context, db *gorm.DB, limit, offset int, filter *ListFilter) ([]domain.Task, bool, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.ListFlatPage")
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	q := db.WithContext(ctx).Model(&domain.Task{})
	q = applyListFilter(q, db, filter)
	q = applyTaskCreatedJoin(q)
	var rows []listRowScan
	err := q.Order(listOrderCreatedDesc).Limit(limit + 1).Offset(offset).Scan(&rows).Error
	if err != nil {
		return nil, false, fmt.Errorf("list tasks: %w", err)
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	out := tasksFromListRows(rows)
	for i := range out {
		if err := hydrateDependsOn(ctx, db, &out[i]); err != nil {
			return nil, false, err
		}
	}
	return out, hasMore, nil
}
