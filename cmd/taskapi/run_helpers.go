package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"
)

// resolveSSETestTickerInterval returns how often the SSE dev ticker runs store.List + AppendTaskEvent per task.
// Default is 3s when T2A_SSE_TEST_INTERVAL is unset. Set to 0 to disable the ticker.
func resolveSSETestTickerInterval() time.Duration {
	raw := strings.TrimSpace(os.Getenv(sseTestIntervalEnv))
	if raw == "" {
		return sseTestDefaultInterval
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		slog.Warn("invalid T2A_SSE_TEST_INTERVAL, using default", "cmd", cmdName, "operation", "taskapi.sse_test",
			"default", sseTestDefaultInterval.String(), "err", err)
		return sseTestDefaultInterval
	}
	if d == 0 {
		return 0
	}
	if d < time.Second {
		slog.Warn("T2A_SSE_TEST_INTERVAL below 1s, using default", "cmd", cmdName, "operation", "taskapi.sse_test",
			"default", sseTestDefaultInterval.String(), "value", raw)
		return sseTestDefaultInterval
	}
	return d
}

func resolveListenHost(flagHost string) string {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.resolveListenHost")
	s := strings.TrimSpace(flagHost)
	if s == "" {
		s = strings.TrimSpace(os.Getenv("T2A_LISTEN_HOST"))
	}
	if s == "" {
		return "127.0.0.1"
	}
	return s
}

// emitTaskAPIFileLoggingConfig logs effective JSON file logging settings (call only when not in minimized logging mode).
// The record uses minLevel as its severity so it is never filtered out by the configured handler minimum.
func emitTaskAPIFileLoggingConfig(minLevel slog.Level) {
	slog.Log(context.Background(), minLevel, "logging config",
		"cmd", cmdName, "operation", "taskapi.logging",
		"min_level", minLevel.String(), "json_file", true)
}

// resolveTaskAPILogLevel returns the minimum slog level for the JSON log file.
// If flagLevel is non-empty after TrimSpace, it wins; otherwise T2A_LOG_LEVEL is used.
// When both are empty, the default is info (no Debug trace lines; lighter for production).
func resolveTaskAPILogLevel(flagLevel string) (slog.Level, error) {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.resolveTaskAPILogLevel")
	s := strings.TrimSpace(flagLevel)
	if s == "" {
		s = strings.TrimSpace(os.Getenv("T2A_LOG_LEVEL"))
	}
	if s == "" {
		return slog.LevelInfo, nil
	}
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("invalid -loglevel / T2A_LOG_LEVEL %q (want debug, info, warn, error)", s)
	}
}
