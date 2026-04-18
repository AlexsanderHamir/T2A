// Package events owns the read and write paths for task_events: the
// append-only audit log that backs every task lifecycle change. The
// public store facade re-exports AppendTaskEvent / ListTaskEvents /
// ListTaskEventsPageCursor / GetTaskEvent / TaskEventCount /
// LastEventSeq / ApprovalPending / AppendTaskEventResponseMessage as
// methods on *Store, plus the package-level ThreadEntriesForDisplay
// helper used by the handler and devsim test harness.
//
// Append paths from other store subpackages (CRUD, cycles, checklist,
// devmirror) write through kernel.NextEventSeq + kernel.AppendEvent
// directly, not through this package, so the audit log invariants
// stay enforceable in a single hot-path helper.
package events

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

const logCmd = "taskapi"

// Append inserts one task_events row for taskID after asserting that
// the parent task exists (so we never leak orphan events). Each call
// runs in its own transaction so the seq allocator and the insert
// are observed atomically.
func Append(ctx context.Context, db *gorm.DB, taskID string, typ domain.EventType, by domain.Actor, data []byte) error {
	defer kernel.DeferLatency(kernel.OpAppendTaskEvent)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.events.Append")
	if err := kernel.ValidateActor(by); err != nil {
		return err
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var n int64
		if err := tx.Model(&domain.Task{}).Where("id = ?", taskID).Count(&n).Error; err != nil {
			return fmt.Errorf("task lookup: %w", err)
		}
		if n == 0 {
			return domain.ErrNotFound
		}
		seq, err := kernel.NextEventSeq(tx, taskID)
		if err != nil {
			return err
		}
		return kernel.AppendEvent(tx, taskID, seq, typ, by, data)
	})
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("append task event: %w", err)
	}
	return nil
}
