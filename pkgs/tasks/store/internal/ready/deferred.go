package ready

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/kernel"
	"gorm.io/gorm"
)

// DeferredPickup is a ready task with pickup_not_before still in the future.
type DeferredPickup struct {
	ID              string
	PickupNotBefore time.Time
}

// ListDeferredReadyPickups returns ready tasks whose pickup_not_before is
// strictly after `now`, ordered by pickup time then id. Used to hydrate the
// pickup wake scheduler at startup.
func ListDeferredReadyPickups(ctx context.Context, db *gorm.DB, now time.Time, limit int) ([]DeferredPickup, error) {
	defer kernel.DeferLatency(kernel.OpListReadyQueue)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.ready.ListDeferredReadyPickups")
	if limit <= 0 {
		limit = 10_000
	}
	if limit > 50_000 {
		limit = 50_000
	}
	var rows []struct {
		ID              string
		PickupNotBefore time.Time
	}
	q := db.WithContext(ctx).Model(&domain.Task{}).
		Select("id", "pickup_not_before").
		Where("status = ?", domain.StatusReady).
		Where("pickup_not_before IS NOT NULL").
		Where("pickup_not_before > ?", now.UTC()).
		Order("pickup_not_before ASC, id ASC").
		Limit(limit)
	if err := q.Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list deferred ready pickups: %w", err)
	}
	out := make([]DeferredPickup, 0, len(rows))
	for i := range rows {
		out = append(out, DeferredPickup{
			ID:              rows[i].ID,
			PickupNotBefore: rows[i].PickupNotBefore,
		})
	}
	return out, nil
}
