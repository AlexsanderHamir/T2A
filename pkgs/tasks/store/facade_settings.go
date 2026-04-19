package store

import (
	"context"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/settings"
)

// AppSettings is the singleton runtime-settings row returned by
// GetSettings / UpdateSettings. See pkgs/tasks/domain.AppSettings for
// field semantics.
type AppSettings = domain.AppSettings

// SettingsPatch is the partial-update payload for UpdateSettings.
// Pointer-typed fields distinguish "not provided" (nil) from an
// explicit zero value (e.g. *int = 0 means "no limit").
type SettingsPatch = settings.Patch

// GetSettings returns the singleton app_settings row, creating it from
// hard-coded defaults on first read so callers always see a populated
// value. There is no env-var fallback — the DB row is the only source
// of truth.
func (s *Store) GetSettings(ctx context.Context) (AppSettings, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.GetSettings")
	return settings.Get(ctx, s.db)
}

// UpdateSettings applies a partial patch to the singleton row inside a
// transaction. Returns the post-update value so the caller (the HTTP
// handler) can echo the canonical row in the PATCH response without an
// extra round-trip.
func (s *Store) UpdateSettings(ctx context.Context, patch SettingsPatch) (AppSettings, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.UpdateSettings")
	return settings.Update(ctx, s.db, patch)
}
