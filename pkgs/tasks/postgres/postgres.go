package postgres

import (
	"context"
	"fmt"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Open returns a GORM DB connected to PostgreSQL using the given DSN.
func Open(dsn string, cfg *gorm.Config) (*gorm.DB, error) {
	if cfg == nil {
		cfg = &gorm.Config{}
	}
	db, err := gorm.Open(postgres.Open(dsn), cfg)
	if err != nil {
		return nil, fmt.Errorf("gorm open: %w", err)
	}
	return db, nil
}

// Migrate runs AutoMigrate for domain.Task and domain.TaskEvent (works with any GORM dialector, e.g. tests on SQLite).
func Migrate(ctx context.Context, db *gorm.DB) error {
	if err := db.WithContext(ctx).AutoMigrate(&domain.Task{}, &domain.TaskEvent{}); err != nil {
		return fmt.Errorf("automigrate task models: %w", err)
	}
	return nil
}
