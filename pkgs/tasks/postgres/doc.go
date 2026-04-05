// Package postgres opens a PostgreSQL pool via GORM and migrates task schema models from package domain.
//
// Open rejects an empty or whitespace-only DSN and configures the underlying [database/sql.DB] pool
// (limits and connection lifetime) after a successful dial.
//
// For long-lived servers, use [ConfigWithSlogLogger] with [log/slog.Default] so SQL traces share the
// process JSON log sink; one-off tools can pass [gorm.Config] with a silent logger instead.
// SQL slower than T2A_GORM_SLOW_QUERY_MS (default 200; 0 disables) is logged at Warn.
//
// [DefaultMigrateTimeout] documents the shared AutoMigrate deadline used by taskapi and dbcheck -migrate.
package postgres
