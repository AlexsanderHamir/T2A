package main

import (
	"context"
	"log/slog"
)

// emitTaskAPIFileLoggingConfig logs effective JSON file logging settings (call only when not in minimized logging mode).
// The record uses minLevel as its severity so it is never filtered out by the configured handler minimum.
func emitTaskAPIFileLoggingConfig(minLevel slog.Level) {
	slog.Log(context.Background(), minLevel, "logging config",
		"cmd", cmdName, "operation", "taskapi.logging",
		"min_level", minLevel.String(), "json_file", true)
}
