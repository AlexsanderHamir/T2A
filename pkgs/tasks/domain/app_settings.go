package domain

import (
	"context"
	"log/slog"
	"time"
)

// AppSettings is the singleton row (id=1) holding all UI-configurable
// app-level settings. There is intentionally only one row: every PATCH
// upserts onto id=1 and every GET reads id=1, optionally creating it
// with defaults on first read.
//
// This row replaces the historical T2A_AGENT_WORKER_* env vars and the
// REPO_ROOT env var. Env vars are no longer read at runtime — the row
// is the only source of truth and is "saved until changed".
//
// Field semantics:
//   - WorkerEnabled: master switch for the in-process agent worker.
//     Default true.
//   - Runner: id of the runner registered in pkgs/agents/runner/registry
//     (today only "cursor"). Default "cursor".
//   - RepoRoot: absolute or process-relative path used for both the
//     agent worker WorkingDir and the global repo file picker / @-mention
//     autocomplete. Empty means "not configured": worker stays idle and
//     repo endpoints respond 409 repo_root_not_configured.
//   - CursorBin: cursor binary path. Empty means "auto-detect from PATH"
//     (the supervisor probes `cursor --version` at boot).
//   - MaxRunDurationSeconds: per-run wall-clock cap in seconds. 0 means
//     "no limit" — the worker does not wrap runner.Run with a timeout.
type AppSettings struct {
	ID                    uint      `gorm:"primaryKey;autoIncrement:false;check:chk_app_settings_singleton,id = 1"`
	WorkerEnabled         bool      `gorm:"not null;default:true"`
	Runner                string    `gorm:"not null;default:'cursor'"`
	RepoRoot              string    `gorm:"not null;default:''"`
	CursorBin             string    `gorm:"not null;default:''"`
	MaxRunDurationSeconds int       `gorm:"not null;default:0;check:chk_app_settings_max_run_duration_seconds,max_run_duration_seconds >= 0"`
	UpdatedAt             time.Time `gorm:"not null"`
}

// AppSettingsRowID is the singleton primary key. Every read/write of
// app_settings uses this id; alternative ids are not allowed (the CHECK
// constraint above enforces it at the DB level).
const AppSettingsRowID uint = 1

// DefaultRunner is the seed value for AppSettings.Runner on first boot.
// Mirrors the only registered runner today (pkgs/agents/runner/cursor).
const DefaultRunner = "cursor"

// DefaultAppSettings returns the hard-coded first-boot defaults. Used
// by the store's Get path when the row doesn't exist yet, so callers
// always observe a fully populated value.
func DefaultAppSettings() AppSettings {
	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		slog.Debug("trace", "operation", "domain.DefaultAppSettings")
	}
	return AppSettings{
		ID:                    AppSettingsRowID,
		WorkerEnabled:         true,
		Runner:                DefaultRunner,
		RepoRoot:              "",
		CursorBin:             "",
		MaxRunDurationSeconds: 0,
	}
}

// TableName pins the table name so Postgres migrations match between
// dialects (gorm pluralizer would emit "app_settings" anyway, but
// pinning avoids surprises if the struct is ever renamed).
func (AppSettings) TableName() string {
	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		slog.Debug("trace", "operation", "domain.AppSettings.TableName")
	}
	return "app_settings"
}
