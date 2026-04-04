package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/joho/godotenv"
)

func loadDotenv(path string) error {
	slog.Debug("trace", "cmd", cmdName, "operation", "dbcheck.loadDotenv")
	if err := godotenv.Overload(path); err != nil {
		return fmt.Errorf("godotenv overload %q: %w", path, err)
	}
	if os.Getenv("DATABASE_URL") == "" {
		return fmt.Errorf("DATABASE_URL is empty after loading %s", path)
	}
	return nil
}
