package tasks

import (
	"context"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func OpenPostgres(dsn string, cfg *gorm.Config) (*gorm.DB, error) {
	if cfg == nil {
		cfg = &gorm.Config{}
	}
	return gorm.Open(postgres.Open(dsn), cfg)
}

func MigratePostgreSQL(ctx context.Context, db *gorm.DB) error {
	return db.WithContext(ctx).AutoMigrate(&Task{}, &TaskEvent{})
}
