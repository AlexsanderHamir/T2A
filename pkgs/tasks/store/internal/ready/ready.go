// Package ready owns the agent-queue read paths: keyset pagination
// over StatusReady tasks ordered for fair FIFO scheduling, plus the
// user-scoped ready list. The public store facade re-exports
// QueueCursor / QueueCandidate as ReadyTaskQueueCursor /
// ReadyTaskQueueCandidate (pkgs/agents/reconcile.go is a documented
// caller).
package ready

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/kernel"
	"gorm.io/gorm"
)

const logCmd = "taskapi"

// QueueCursor is a keyset cursor for ListQueueCandidates. Nil means
// the first page. On SQLite, AfterEventRowID is the joined task_events
// rowid for stable FIFO when task_created timestamps tie.
type QueueCursor struct {
	AfterTaskCreatedAt time.Time
	AfterTaskID        string
	AfterEventRowID    int64
}

// QueueCandidate is one ready task plus scheduling metadata for the
// agent queue. EventRowID is the SQLite rowid of the seq=1
// task_created row (0 on other dialects).
type QueueCandidate struct {
	Task          domain.Task
	TaskCreatedAt time.Time
	EventRowID    int64
}

// ListQueueCandidates returns ready tasks ordered for fair scheduling:
// oldest task_created first (task_events seq 1), then a
// dialect-specific tie-breaker (SQLite: event rowid insertion order),
// then task id. Pagination is keyset; pass the cursor from the last
// row of the previous page.
func ListQueueCandidates(ctx context.Context, db *gorm.DB, limit int, cursor *QueueCursor) ([]QueueCandidate, error) {
	defer kernel.DeferLatency(kernel.OpListReadyQueue)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.ready.ListQueueCandidates")
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

	useRow := UseSQLiteEventRowID(db)
	sel := "tasks.*, te.at AS task_created_at"
	if useRow {
		sel += ", te.rowid AS sched_te_rowid"
	}
	order := "te.at ASC, tasks.id ASC"
	if useRow {
		order = "te.at ASC, te.rowid ASC, tasks.id ASC"
	}

	q := db.WithContext(ctx).Model(&domain.Task{}).
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

	var rows []joinScan
	if err := q.Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list ready task queue candidates: %w", err)
	}
	out := make([]QueueCandidate, 0, len(rows))
	for i := range rows {
		out = append(out, QueueCandidate{
			Task:          rows[i].Task,
			TaskCreatedAt: rows[i].TaskCreatedAt,
			EventRowID:    rows[i].SchedEventRowID,
		})
	}
	return out, nil
}

// ListUserCreated returns tasks with status ready whose first audit
// row is task_created by user (matches the user-task agent queue
// policy). Results are ordered by id ascending. afterID, when non-empty
// after trim, restricts to tasks.id > afterID for pagination.
func ListUserCreated(ctx context.Context, db *gorm.DB, limit int, afterID string) ([]domain.Task, error) {
	defer kernel.DeferLatency(kernel.OpListReadyUserCreated)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.ready.ListUserCreated")
	if limit <= 0 {
		limit = 200
	}
	if limit > 500 {
		limit = 500
	}
	afterID = strings.TrimSpace(afterID)
	q := db.WithContext(ctx).Model(&domain.Task{}).
		Joins(`INNER JOIN task_events te ON te.task_id = tasks.id AND te.seq = ? AND te.type = ? AND te.by = ?`,
			int64(1), domain.EventTaskCreated, domain.ActorUser).
		Where("tasks.status = ?", domain.StatusReady).
		Order("tasks.id ASC").
		Limit(limit)
	if afterID != "" {
		q = q.Where("tasks.id > ?", afterID)
	}
	var out []domain.Task
	if err := q.Find(&out).Error; err != nil {
		return nil, fmt.Errorf("list ready tasks user created: %w", err)
	}
	return out, nil
}

// UseSQLiteEventRowID reports whether the active dialect is SQLite,
// in which case ListQueueCandidates can use the task_events rowid as
// a stable FIFO tie-breaker. Postgres has no equivalent stable
// physical row id, so the fallback is purely (te.at, tasks.id).
func UseSQLiteEventRowID(db *gorm.DB) bool {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.ready.UseSQLiteEventRowID")
	if db == nil {
		return false
	}
	n := strings.ToLower(db.Dialector.Name())
	return strings.Contains(n, "sqlite")
}

// joinScan maps tasks.* plus scheduling columns from the join (package-local scan DTO).
type joinScan struct {
	domain.Task     `gorm:"embedded"`
	TaskCreatedAt   time.Time `gorm:"column:task_created_at"`
	SchedEventRowID int64     `gorm:"column:sched_te_rowid"`
}
