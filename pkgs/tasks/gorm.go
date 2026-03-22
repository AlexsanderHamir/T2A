package tasks

import (
	"context"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// OpenPostgres returns a GORM DB using the PostgreSQL driver (pgx under the hood).
func OpenPostgres(dsn string, cfg *gorm.Config) (*gorm.DB, error) {
	if cfg == nil {
		cfg = &gorm.Config{}
	}
	return gorm.Open(postgres.Open(dsn), cfg)
}

// MigratePostgreSQL creates or updates tasks and task_events via AutoMigrate.
func MigratePostgreSQL(ctx context.Context, db *gorm.DB) error {
	return db.WithContext(ctx).AutoMigrate(&Task{}, &TaskEvent{})
}
