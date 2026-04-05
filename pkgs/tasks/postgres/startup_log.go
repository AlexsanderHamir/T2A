package postgres

import (
	"log/slog"

	"gorm.io/gorm"
)

// LogStartupDBConfig logs SQL pool caps and GORM slow-query threshold at Info.
// operation is cmd+".db_config" (e.g. taskapi.db_config, dbcheck.db_config).
// It does not log the DSN or other secrets. db must be non-nil (typically right after Open + Migrate).
func LogStartupDBConfig(lg *slog.Logger, cmd string, db *gorm.DB) {
	if lg == nil {
		lg = slog.Default()
	}
	if db == nil {
		return
	}
	lg.Info("database config",
		"cmd", cmd,
		"operation", cmd+".db_config",
		"max_open_conns", defaultMaxOpenConns,
		"max_idle_conns", defaultMaxIdleConns,
		"conn_max_lifetime_sec", int(defaultConnMaxLifetime.Seconds()),
		"conn_max_idle_time_sec", int(defaultConnMaxIdleTime.Seconds()),
		"gorm_slow_query_ms", SlowQueryThresholdMS(),
	)
}
