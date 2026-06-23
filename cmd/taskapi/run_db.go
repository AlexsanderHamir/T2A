package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/AlexsanderHamir/Hamix/internal/envload"
	"github.com/AlexsanderHamir/Hamix/internal/taskapi"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/postgres"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
	"gorm.io/gorm"
)

// run_db.go owns the database lifecycle for taskapi: env load, GORM
// open, migrate (with timeout), pool-collector registration, and the
// designated close+log site. Split off run_helpers.go per
// backend-engineering-bar.mdc §2 / §16.

type dbStartupResult struct {
	db          *gorm.DB
	schemaDrift postgres.SchemaDriftReport
}

func migrateDBAndRegisterMetrics(db *gorm.DB) error {
	migrateCtx, migrateCancel := context.WithTimeout(context.Background(), postgres.DefaultMigrateTimeout)
	defer migrateCancel()
	if err := postgres.Migrate(migrateCtx, db); err != nil {
		return fmt.Errorf("migrate (timeout %ds, deadline_exceeded=%t): %w",
			int(postgres.DefaultMigrateTimeout/time.Second),
			errors.Is(err, context.DeadlineExceeded),
			err)
	}
	if err := store.BackfillCriteriaSatisfiedAt(migrateCtx, db); err != nil {
		return fmt.Errorf("backfill criteria_satisfied_at: %w", err)
	}
	slog.Info("migrate ok", "cmd", cmdName, "operation", "taskapi.migrate",
		"timeout_sec", int(postgres.DefaultMigrateTimeout/time.Second),
		"schema_revision", postgres.SchemaRevision)
	return nil
}

func registerDBMetrics(db *gorm.DB) {
	postgres.LogStartupDBConfig(slog.Default(), cmdName, db)
	taskapi.RegisterSQLDBPoolCollector(db)
}

func emitSchemaDriftAlerts(report postgres.SchemaDriftReport) {
	if report.Status != postgres.SchemaDriftPending {
		return
	}
	fmt.Fprintf(os.Stderr,
		"%s: SCHEMA MIGRATION REQUIRED (code revision %d, database revision %d).\n"+
			"         Run: .\\scripts\\migrate.ps1   or   go run ./cmd/dbcheck -migrate\n",
		cmdName, report.CodeRevision, report.DBRevision)
	slog.Error("schema migration required", "cmd", cmdName, "operation", "taskapi.schema_drift",
		"status", string(report.Status),
		"code_revision", report.CodeRevision,
		"db_revision", report.DBRevision,
		"remediation", report.Remediation())
}

func loadEnvAndOpenDatabase(envPath string, migrateEnabled bool) (dbStartupResult, error) {
	var out dbStartupResult
	envLoadedPath, err := envload.Load(envPath)
	if err != nil {
		return out, err
	}
	slog.Info("env loaded", "cmd", cmdName, "operation", "taskapi.startup", "path", envLoadedPath)

	db, err := postgres.Open(
		os.Getenv("DATABASE_URL"),
		postgres.ConfigWithSlogLogger(slog.Default()),
	)
	if err != nil {
		return out, err
	}
	out.db = db

	checkCtx, checkCancel := context.WithTimeout(context.Background(), postgres.DefaultPingTimeout)
	defer checkCancel()
	drift, err := postgres.CheckSchemaDrift(checkCtx, db)
	if err != nil {
		return out, fmt.Errorf("schema drift check: %w", err)
	}

	if migrateEnabled {
		if err := migrateDBAndRegisterMetrics(db); err != nil {
			return out, err
		}
		drift, err = postgres.CheckSchemaDrift(checkCtx, db)
		if err != nil {
			return out, fmt.Errorf("schema drift check after migrate: %w", err)
		}
	} else {
		slog.Info("migrate skipped", "cmd", cmdName, "operation", "taskapi.migrate",
			"reason", "not_requested",
			"hint", "run scripts/migrate.* or pass -migrate / set HAMIX_MIGRATE=1")
		emitSchemaDriftAlerts(drift)
		if drift.Status == postgres.SchemaDriftDowngrade {
			slog.Warn("schema downgrade detected", "cmd", cmdName, "operation", "taskapi.schema_drift",
				"code_revision", drift.CodeRevision,
				"db_revision", drift.DBRevision,
				"remediation", "deploy a binary matching the database schema revision")
		}
	}

	out.schemaDrift = drift
	registerDBMetrics(db)
	logHTTPTimeoutsAndShutdown()
	return out, nil
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
