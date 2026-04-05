package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/AlexsanderHamir/T2A/internal/version"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/postgres"
	"gorm.io/gorm"
)

const (
	dbPingTimeout = 30 * time.Second
	cmdName       = "dbcheck"
)

type options struct {
	migrate bool
	envPath string
}

func parseFlags() options {
	slog.Debug("trace", "cmd", cmdName, "operation", "dbcheck.parseFlags")
	var o options
	flag.BoolVar(&o.migrate, "migrate", false, "run GORM AutoMigrate after connecting")
	flag.StringVar(&o.envPath, "env", "", "path to .env (default: <repo-root>/.env)")
	flag.Parse()
	return o
}

func loadRepoDotenv(o options) error {
	slog.Debug("trace", "cmd", cmdName, "operation", "dbcheck.loadRepoDotenv")
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
	pingSec := int(dbPingTimeout / time.Second)
	startArgs := []any{
		"cmd", cmdName, "operation", "dbcheck.start",
		"version", version.String(), "migrate", o.migrate,
		"ping_timeout_sec", pingSec,
	}
	if o.migrate {
		startArgs = append(startArgs, "migrate_timeout_sec", int(postgres.DefaultMigrateTimeout/time.Second))
	}
	slog.Info("dbcheck starting", startArgs...)

	pingCtx, pingCancel := context.WithTimeout(context.Background(), dbPingTimeout)
	defer pingCancel()

	db, err := connectAndPing(pingCtx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	slog.Info("database reachable", "cmd", cmdName, "operation", "ping")
	postgres.LogStartupDBConfig(slog.Default(), cmdName, db)

	if err := migrateIfRequested(db, o.migrate); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	if o.migrate {
		slog.Info("schema migrated", "cmd", cmdName, "operation", "automigrate")
	}
	slog.Info("dbcheck ok", "cmd", cmdName, "operation", "dbcheck.done", "migrate_ran", o.migrate)
	return nil
}

func migrateIfRequested(db *gorm.DB, want bool) error {
	slog.Debug("trace", "cmd", cmdName, "operation", "dbcheck.migrateIfRequested")
	if !want {
		return nil
	}
	// Dedicated deadline: migrate can exceed pingTimeout; same bound as taskapi startup (postgres.DefaultMigrateTimeout).
	migrateCtx, migrateCancel := context.WithTimeout(context.Background(), postgres.DefaultMigrateTimeout)
	defer migrateCancel()
	if err := postgres.Migrate(migrateCtx, db); err != nil {
		return fmt.Errorf("postgres.Migrate: %w", err)
	}
	return nil
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))
	if err := run(parseFlags()); err != nil {
		slog.Error("dbcheck failed", "cmd", cmdName, "operation", "dbcheck.failed", "err", err,
			"deadline_exceeded", errors.Is(err, context.DeadlineExceeded))
		os.Exit(1)
	}
}
