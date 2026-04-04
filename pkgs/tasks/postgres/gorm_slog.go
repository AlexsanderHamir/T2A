package postgres

import (
	"log/slog"
	"time"

	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// ConfigWithSlogLogger returns a GORM config that records each SQL round-trip through lg
// (typically slog.Default() after taskapi attaches the JSON log handler).
// ParameterizedQueries keeps bound values out of log lines; slow statements log at Warn.
func ConfigWithSlogLogger(lg *slog.Logger) *gorm.Config {
	if lg == nil {
		return nil
	}
	return &gorm.Config{
		Logger: gormlogger.NewSlogLogger(lg, gormlogger.Config{
			LogLevel:                  gormlogger.Info,
			SlowThreshold:             200 * time.Millisecond,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true,
		}),
	}
}
