package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks"
	"gorm.io/gorm"
)

const (
	dbTimeout = 30 * time.Second
	cmdName   = "dbcheck"
)

type options struct {
	migrate bool
	envPath string
}

func parseFlags() options {
	var o options
	flag.BoolVar(&o.migrate, "migrate", false, "run GORM AutoMigrate after connecting")
	flag.StringVar(&o.envPath, "env", "", "path to .env (default: <repo-root>/.env)")
	flag.Parse()
	return o
}

func loadRepoDotenv(o options) error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getwd: %w", err)
	}
	path, err := resolveDotenvPath(wd, o.envPath)
	if err != nil {
		return fmt.Errorf("resolve .env path: %w", err)
	}
	if err := loadDotenv(path); err != nil {
		return fmt.Errorf("load .env: %w", err)
	}
	return nil
}

func run(o options) error {
	if err := loadRepoDotenv(o); err != nil {
		return fmt.Errorf("env setup: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	db, err := connectAndPing(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	slog.Info("database reachable", "cmd", cmdName, "operation", "ping")

	if err := migrateIfRequested(ctx, db, o.migrate); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	if o.migrate {
		slog.Info("schema migrated", "cmd", cmdName, "operation", "automigrate")
	}
	return nil
}

func migrateIfRequested(ctx context.Context, db *gorm.DB, want bool) error {
	if !want {
		return nil
	}
	if err := tasks.MigratePostgreSQL(ctx, db); err != nil {
		return fmt.Errorf("tasks.MigratePostgreSQL: %w", err)
	}
	return nil
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))
	if err := run(parseFlags()); err != nil {
		slog.Error("dbcheck failed", "cmd", cmdName, "operation", "run", "err", err)
		os.Exit(1)
	}
}
