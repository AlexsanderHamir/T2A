package store

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/gorm"
)

// ReadyTaskQueueCursor is a keyset cursor for ListReadyTaskQueueCandidates. Nil means the first page.
// On SQLite, AfterEventRowID is the joined task_events rowid for stable FIFO when task_created timestamps tie.
type ReadyTaskQueueCursor struct {
	AfterTaskCreatedAt time.Time
	AfterTaskID        string
	AfterEventRowID    int64
}

// ReadyTaskQueueCandidate is one ready task plus scheduling metadata for the agent queue.
type ReadyTaskQueueCandidate struct {
	Task          domain.Task
	TaskCreatedAt time.Time
	// EventRowID is the SQLite rowid of the seq=1 task_created row (0 on other dialects).
	EventRowID int64
}

// ListReadyTaskQueueCandidates returns ready tasks ordered for fair scheduling: oldest task_created
// first (task_events seq 1), then a dialect-specific tie-breaker (SQLite: event rowid insertion order),
// then task id. Pagination is keyset; pass the cursor from the last row of the previous page.
func (s *Store) ListReadyTaskQueueCandidates(ctx context.Context, limit int, cursor *ReadyTaskQueueCursor) ([]ReadyTaskQueueCandidate, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListReadyTaskQueueCandidates")
	if limit <= 0 {
		limit = 200
	}
	if limit > 500 {
		limit = 500
	}
	if cursor != nil {
		cursor.AfterTaskID = strings.TrimSpace(cursor.AfterTaskID)
		if cursor.AfterTaskID == "" {
			return nil, fmt.Errorf("%w: cursor requires after_task_id", domain.ErrInvalidInput)
		}
	}

	useRow := scheduleUseSQLiteEventRowID(s.db)
	sel := "tasks.*, te.at AS task_created_at"
	if useRow {
		sel += ", te.rowid AS sched_te_rowid"
	}
	order := "te.at ASC, tasks.id ASC"
	if useRow {
		order = "te.at ASC, te.rowid ASC, tasks.id ASC"
	}

	q := s.db.WithContext(ctx).Model(&domain.Task{}).
		Select(sel).
		Joins(`INNER JOIN task_events te ON te.task_id = tasks.id AND te.seq = ? AND te.type = ?`,
			int64(1), domain.EventTaskCreated).
		Where("tasks.status = ?", domain.StatusReady).
		Order(order).
		Limit(limit)

	if cursor != nil {
		if useRow && cursor.AfterEventRowID > 0 {
			q = q.Where("(te.at > ? OR (te.at = ? AND te.rowid > ?) OR (te.at = ? AND te.rowid = ? AND tasks.id > ?))",
				cursor.AfterTaskCreatedAt, cursor.AfterTaskCreatedAt, cursor.AfterEventRowID,
				cursor.AfterTaskCreatedAt, cursor.AfterEventRowID, cursor.AfterTaskID)
		} else {
			q = q.Where("(te.at > ? OR (te.at = ? AND tasks.id > ?))",
				cursor.AfterTaskCreatedAt, cursor.AfterTaskCreatedAt, cursor.AfterTaskID)
		}
	}

	var rows []readyTaskJoinScan
	if err := q.Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list ready task queue candidates: %w", err)
	}
	out := make([]ReadyTaskQueueCandidate, 0, len(rows))
	for i := range rows {
		out = append(out, ReadyTaskQueueCandidate{
			Task:          rows[i].Task,
			TaskCreatedAt: rows[i].TaskCreatedAt,
			EventRowID:    rows[i].SchedEventRowID,
		})
	}
	return out, nil
}

func scheduleUseSQLiteEventRowID(db *gorm.DB) bool {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.scheduleUseSQLiteEventRowID")
	if db == nil {
		return false
	}
	n := strings.ToLower(db.Dialector.Name())
	return strings.Contains(n, "sqlite")
}

// readyTaskJoinScan maps tasks.* plus scheduling columns from the join (package-local scan DTO).
type readyTaskJoinScan struct {
	domain.Task     `gorm:"embedded"`
	TaskCreatedAt   time.Time `gorm:"column:task_created_at"`
	SchedEventRowID int64     `gorm:"column:sched_te_rowid"`
}
