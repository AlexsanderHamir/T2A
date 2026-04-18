package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/AlexsanderHamir/T2A/internal/envload"
	"github.com/AlexsanderHamir/T2A/internal/taskapi"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/postgres"
	"gorm.io/gorm"
)

// run_db.go owns the database lifecycle for taskapi: env load, GORM
// open, migrate (with timeout), pool-collector registration, and the
// designated close+log site. Split off run_helpers.go per
// backend-engineering-bar.mdc §2 / §16.

func migrateDBAndRegisterMetrics(db *gorm.DB) error {
	migrateCtx, migrateCancel := context.WithTimeout(context.Background(), postgres.DefaultMigrateTimeout)
	defer migrateCancel()
	if err := postgres.Migrate(migrateCtx, db); err != nil {
		// Bar §4: never log AND return the same error. runTaskAPIService
		// logs the wrapped error at Error level (taskapi.startup_db) so
		// callers see the failure exactly once.
		return fmt.Errorf("migrate (timeout %ds, deadline_exceeded=%t): %w",
			int(postgres.DefaultMigrateTimeout/time.Second),
			errors.Is(err, context.DeadlineExceeded),
			err)
	}
	slog.Info("migrate ok", "cmd", cmdName, "operation", "taskapi.migrate",
		"timeout_sec", int(postgres.DefaultMigrateTimeout/time.Second))
	postgres.LogStartupDBConfig(slog.Default(), cmdName, db)
	taskapi.RegisterSQLDBPoolCollector(db)
	return nil
}

func loadEnvAndOpenDatabase(envPath string) (*gorm.DB, error) {
	envLoadedPath, err := envload.Load(envPath)
	if err != nil {
		return nil, err
	}
	slog.Info("env loaded", "cmd", cmdName, "operation", "taskapi.startup", "path", envLoadedPath)

	db, err := postgres.Open(
		os.Getenv("DATABASE_URL"),
		postgres.ConfigWithSlogLogger(slog.Default()),
	)
	if err != nil {
		return nil, err
	}
	if err := migrateDBAndRegisterMetrics(db); err != nil {
		return nil, err
	}
	logHTTPTimeoutsAndShutdown()
	return db, nil
}

// closeSQLDBOrLog closes the GORM-owned *sql.DB pool and logs the
// outcome. The function is the designated log site (per bar §4: never
// log AND return the same error — pick one); callers take the returned
// bool as the success signal and never re-log.
func closeSQLDBOrLog(db *gorm.DB) (dbClosed bool) {
	sqlDB, err := db.DB()
	if err != nil {
		slog.Error("database close skipped", "cmd", cmdName, "operation", "taskapi.db_close", "err", err)
		return false
	}
	if err := sqlDB.Close(); err != nil {
		slog.Error("database close", "cmd", cmdName, "operation", "taskapi.db_close", "err", err)
		return false
	}
	slog.Info("database pool closed", "cmd", cmdName, "operation", "taskapi.shutdown", "phase", "db_done")
	return true
}
