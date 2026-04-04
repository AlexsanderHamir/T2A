package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// openTaskAPILogFile creates the log directory if needed and opens a new JSON-lines log file
// named with the current local date and time. dirFlag takes precedence over T2A_LOG_DIR;
// when both are empty, "logs" (relative to the process working directory) is used.
// minLevel is the minimum slog level written to this file (e.g. slog.LevelInfo hides Debug).
func openTaskAPILogFile(dirFlag string, minLevel slog.Level) (*os.File, string, error) {
	dir := strings.TrimSpace(dirFlag)
	if dir == "" {
		dir = strings.TrimSpace(os.Getenv("T2A_LOG_DIR"))
	}
	if dir == "" {
		dir = "logs"
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, "", fmt.Errorf("resolve log directory: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, "", fmt.Errorf("create log directory: %w", err)
	}
	now := time.Now()
	name := fmt.Sprintf("taskapi-%s-%09d.jsonl", now.Format("2006-01-02-150405"), now.Nanosecond())
	path := filepath.Join(abs, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o644)
	if err != nil {
		return nil, "", fmt.Errorf("open log file: %w", err)
	}
	// Log with a JSON handler on this file so the line lands in the same jsonl before slog.SetDefault in run().
	early := slog.New(slog.NewJSONHandler(f, &slog.HandlerOptions{Level: minLevel}))
	early.Debug("trace", "cmd", cmdName, "operation", "taskapi.openTaskAPILogFile", "path", path)
	return f, path, nil
}
