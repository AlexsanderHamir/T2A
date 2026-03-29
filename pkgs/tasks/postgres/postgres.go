package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

func configureSQLPool(sqldb *sql.DB) {
	sqldb.SetMaxOpenConns(defaultMaxOpenConns)
	sqldb.SetMaxIdleConns(defaultMaxIdleConns)
	sqldb.SetConnMaxLifetime(defaultConnMaxLifetime)
	sqldb.SetConnMaxIdleTime(defaultConnMaxIdleTime)
}

// Open returns a GORM DB connected to PostgreSQL using the given DSN.
func Open(dsn string, cfg *gorm.Config) (*gorm.DB, error) {
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
	if err := db.WithContext(ctx).AutoMigrate(&domain.Task{}, &domain.TaskEvent{}); err != nil {
		return fmt.Errorf("automigrate task models: %w", err)
	}
	return nil
}

var errEmptyDSN = errors.New("database DSN is empty")
