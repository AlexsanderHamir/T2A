package store

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/kernel"
)

// Ping checks that the database session is reachable (e.g. for HTTP readiness probes).
func (s *Store) Ping(ctx context.Context) error {
	defer kernel.DeferLatency(kernel.OpPing)()
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.Ping")
	if s == nil || s.db == nil {
		return errors.New("tasks store: nil database")
	}
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

// DefaultReadyTimeout is the recommended upper bound for [context.Context] passed to (*Store).Ready
// from HTTP readiness probes (GET /health/ready).
const DefaultReadyTimeout = 2 * time.Second

// Ready checks Ping plus a trivial SQL round-trip (readiness beyond the pool ping).
func (s *Store) Ready(ctx context.Context) error {
	defer kernel.DeferLatency(kernel.OpReady)()
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.Ready")
	if s == nil || s.db == nil {
		return errors.New("tasks store: nil database")
	}
	if err := s.Ping(ctx); err != nil {
		return err
	}
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	var n int64
	if err := sqlDB.QueryRowContext(ctx, "SELECT 1").Scan(&n); err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("tasks store: ready check: want 1, got %d", n)
	}
	return nil
}
