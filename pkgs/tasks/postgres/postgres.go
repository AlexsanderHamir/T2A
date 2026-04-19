package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const (
	defaultMaxOpenConns    = 25
	defaultMaxIdleConns    = 5
	defaultConnMaxLifetime = 30 * time.Minute
	defaultConnMaxIdleTime = 5 * time.Minute
)

// DefaultMigrateTimeout is the recommended upper bound for [Migrate] from operators
// (taskapi startup, dbcheck -migrate). Tests and fast local runs may use a shorter deadline or
// [context.Background] when AutoMigrate is expected to finish quickly.
const DefaultMigrateTimeout = 2 * time.Minute

// DefaultPingTimeout is the recommended upper bound for the first successful [database/sql.DB.PingContext]
// from operator CLIs (dbcheck). Long-lived servers may omit an explicit ping or use their own probe policy.
const DefaultPingTimeout = 30 * time.Second

func configureSQLPool(sqldb *sql.DB) {
	slog.Debug("trace", "operation", "postgres.configureSQLPool")
	sqldb.SetMaxOpenConns(defaultMaxOpenConns)
	sqldb.SetMaxIdleConns(defaultMaxIdleConns)
	sqldb.SetConnMaxLifetime(defaultConnMaxLifetime)
	sqldb.SetConnMaxIdleTime(defaultConnMaxIdleTime)
}

// Open returns a GORM DB connected to PostgreSQL using the given DSN.
func Open(dsn string, cfg *gorm.Config) (*gorm.DB, error) {
	slog.Debug("trace", "operation", "postgres.Open")
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, fmt.Errorf("postgres open: %w", errEmptyDSN)
	}
	if cfg == nil {
		cfg = &gorm.Config{}
	}
	db, err := gorm.Open(postgres.Open(dsn), cfg)
	if err != nil {
		return nil, fmt.Errorf("gorm open: %w", err)
	}
	sqldb, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("gorm sql db: %w", err)
	}
	configureSQLPool(sqldb)
	return db, nil
}

// Migrate runs AutoMigrate for domain.Task and domain.TaskEvent (works with any GORM dialector, e.g. tests on SQLite).
func Migrate(ctx context.Context, db *gorm.DB) error {
	slog.Debug("trace", "operation", "postgres.Migrate")
	if err := db.WithContext(ctx).AutoMigrate(
		&domain.Task{},
		&domain.TaskEvent{},
		&domain.TaskChecklistItem{},
		&domain.TaskChecklistCompletion{},
		&domain.TaskDraftEvaluation{},
		&domain.TaskDraft{},
		&domain.TaskCycle{},
		&domain.TaskCyclePhase{},
		&domain.AppSettings{},
	); err != nil {
		return fmt.Errorf("automigrate task models: %w", err)
	}
	return nil
}

var errEmptyDSN = errors.New("database DSN is empty")
