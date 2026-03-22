package tasks

import (
	"context"
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func OpenPostgres(dsn string, cfg *gorm.Config) (*gorm.DB, error) {
	if cfg == nil {
		cfg = &gorm.Config{}
	}
	db, err := gorm.Open(postgres.Open(dsn), cfg)
	if err != nil {
		return nil, fmt.Errorf("gorm open: %w", err)
	}
	return db, nil
}

func MigratePostgreSQL(ctx context.Context, db *gorm.DB) error {
	if err := db.WithContext(ctx).AutoMigrate(&Task{}, &TaskEvent{}); err != nil {
		return fmt.Errorf("automigrate task models: %w", err)
	}
	return nil
}
