package main

import (
	"context"
	"fmt"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks"
	"gorm.io/gorm"
)

func connectAndPing(ctx context.Context, dsn string) (*gorm.DB, error) {
	db, err := tasks.OpenPostgres(dsn, nil)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("sql.DB: %w", err)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}
	return db, nil
}
