// Package health holds the database liveness / readiness probes used by
// the public store facade for GET /health/live and GET /health/ready.
// The facade exposes them as (*Store).Ping and (*Store).Ready; this
// package owns the SQL implementation so the surrounding handler glue
// and the pool-vs-roundtrip distinction are not entangled with the
// rest of the store concerns.
package health

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/kernel"
	"gorm.io/gorm"
)

const logCmd = "taskapi"

// Ping checks that the database session is reachable (sql.DB.PingContext).
// Used by liveness checks and as the first hop of Ready.
func Ping(ctx context.Context, db *gorm.DB) error {
	defer kernel.DeferLatency(kernel.OpPing)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.health.Ping")
	if db == nil {
		return errors.New("tasks store: nil database")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

// Ready performs Ping plus a trivial SELECT 1 round-trip so a stalled
// query path (driver hang, broken transaction) is caught before traffic
// reaches the full read/write code paths.
func Ready(ctx context.Context, db *gorm.DB) error {
	defer kernel.DeferLatency(kernel.OpReady)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.health.Ready")
	if db == nil {
		return errors.New("tasks store: nil database")
	}
	if err := Ping(ctx, db); err != nil {
		return err
	}
	sqlDB, err := db.DB()
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
