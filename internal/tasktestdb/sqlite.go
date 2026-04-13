package tasktestdb

import (
	"context"
	"log/slog"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/postgres"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// OpenSQLite returns an in-memory GORM DB with task schema migrated (for store/handler tests).
func OpenSQLite(t *testing.T) *gorm.DB {
	t.Helper()
	slog.Debug("trace", "operation", "tasktestdb.OpenSQLite")
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	// Single connection: in-memory DB is per-connection; also serializes writers for tests.
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close sqlite: %v", err)
		}
	})

	if err := db.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
		t.Fatal(err)
	}
	if err := postgres.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return db
}
