package store

import (
	"context"
	"log/slog"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/health"
)

// DefaultReadyTimeout is the recommended upper bound for [context.Context] passed to (*Store).Ready
// from HTTP readiness probes (GET /health/ready). Stays in the public store package so callers
// like cmd/taskapi keep referring to store.DefaultReadyTimeout unchanged.
const DefaultReadyTimeout = 2 * time.Second

// Ping checks that the database session is reachable (e.g. for HTTP readiness probes).
func (s *Store) Ping(ctx context.Context) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.Ping")
	if s == nil {
		return health.Ping(ctx, nil)
	}
	return health.Ping(ctx, s.db)
}

// Ready checks Ping plus a trivial SQL round-trip (readiness beyond the pool ping).
func (s *Store) Ready(ctx context.Context) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.Ready")
	if s == nil {
		return health.Ready(ctx, nil)
	}
	return health.Ready(ctx, s.db)
}
