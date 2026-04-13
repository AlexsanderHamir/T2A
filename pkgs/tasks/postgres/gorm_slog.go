package postgres

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

const defaultSlowQueryMS = 200

// slowQueryThresholdForGORM reads T2A_GORM_SLOW_QUERY_MS (milliseconds).
// Empty: defaultSlowQueryMS. "0": disable the slow-SQL branch (queries stay at Info when LogLevel is Info).
// Invalid or negative: defaultSlowQueryMS.
func slowQueryThresholdForGORM() time.Duration {
	slog.Debug("trace", "operation", "postgres.slowQueryThresholdForGORM")
	s := strings.TrimSpace(os.Getenv("T2A_GORM_SLOW_QUERY_MS"))
	if s == "" {
		return time.Duration(defaultSlowQueryMS) * time.Millisecond
	}
	ms, err := strconv.Atoi(s)
	if err != nil || ms < 0 {
		return time.Duration(defaultSlowQueryMS) * time.Millisecond
	}
	return time.Duration(ms) * time.Millisecond
}

// SlowQueryThresholdMS returns the effective GORM slow-SQL threshold in milliseconds
// (T2A_GORM_SLOW_QUERY_MS; default 200; 0 means the slow-SQL warn branch is off).
func SlowQueryThresholdMS() int {
	slog.Debug("trace", "operation", "postgres.SlowQueryThresholdMS")
	d := slowQueryThresholdForGORM()
	if d <= 0 {
		return 0
	}
	return int(d / time.Millisecond)
}

// ConfigWithSlogLogger returns a GORM config that records each SQL round-trip through lg
// (typically slog.Default() after taskapi attaches the JSON log handler).
// ParameterizedQueries keeps bound values out of log lines; statements slower than
// T2A_GORM_SLOW_QUERY_MS (default 200ms; 0 disables) log at Warn.
func ConfigWithSlogLogger(lg *slog.Logger) *gorm.Config {
	slog.Debug("trace", "operation", "postgres.ConfigWithSlogLogger")
	if lg == nil {
		return nil
	}
	return &gorm.Config{
		Logger: gormlogger.NewSlogLogger(lg, gormlogger.Config{
			LogLevel:                  gormlogger.Info,
			SlowThreshold:             slowQueryThresholdForGORM(),
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true,
		}),
	}
}
