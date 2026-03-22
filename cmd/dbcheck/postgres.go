package main

import (
	"context"
	"fmt"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/postgres"
	"gorm.io/gorm"
)

func connectAndPing(ctx context.Context, dsn string) (*gorm.DB, error) {
	db, err := postgres.Open(dsn, nil)
	if err != nil {
		return nil, fmt.Errorf("postgres.Open: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("gorm sql.DB: %w", err)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return db, nil
}
