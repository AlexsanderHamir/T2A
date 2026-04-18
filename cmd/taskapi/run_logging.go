package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"

	"github.com/AlexsanderHamir/T2A/internal/taskapiconfig"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/logctx"
)

// run_logging.go owns slog/log-file setup for taskapi: opening the
// daily JSON log file, wrapping the slog handler with request and
// per-process sequence context, and the deferred sync+close site.
// Split off run_helpers.go per backend-engineering-bar.mdc §2 / §16
// (do not grow a single file into a junk drawer of unrelated concerns).

func emitTaskAPIFileLoggingConfig(minLevel slog.Level) {
	slog.Log(context.Background(), minLevel, "logging config",
		"cmd", cmdName, "operation", "taskapi.logging",
		"min_level", minLevel.String(), "json_file", true)
}

func installDefaultSlog(logFile *os.File, minimized bool, minLevel slog.Level, processLogSeq *atomic.Uint64) {
	var baseHandler slog.Handler
	if minimized {
		baseHandler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})
	} else {
		baseHandler = slog.NewJSONHandler(logFile, &slog.HandlerOptions{Level: minLevel})
	}
	slog.SetDefault(slog.New(logctx.WrapSlogHandlerWithLogSequence(
		logctx.WrapSlogHandlerWithRequestContext(baseHandler),
		processLogSeq,
	)))
}

func openTaskAPILogging(logDir, logLevelFlag string, disableLogging bool) (minLevel slog.Level, logFile *os.File, logPath string, minimized bool, err error) {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.openTaskAPILogging")
	minLevel, err = taskapiconfig.ResolveLogLevel(logLevelFlag)
	if err != nil {
		return minLevel, nil, "", false, err
	}
	minimized = taskapiconfig.LoggingMinimized(disableLogging)
	if minimized {
		return minLevel, nil, "", minimized, nil
	}
	var openErr error
	logFile, logPath, openErr = openTaskAPILogFile(logDir, minLevel)
	if openErr != nil {
		return minLevel, nil, "", false, openErr
	}
	return minLevel, logFile, logPath, minimized, nil
}

func deferCloseTaskAPILogFile(logFile *os.File) {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.deferCloseTaskAPILogFile")
	if logFile == nil {
		return
	}
	if err := logFile.Sync(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: log file sync: %v\n", cmdName, err)
	}
	if err := logFile.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: log file close: %v\n", cmdName, err)
	}
}

func installTaskAPIDefaultLogger(logFile *os.File, minimized bool, minLevel slog.Level, processLogSeq *atomic.Uint64, logPath string) {
	if minimized {
		fmt.Fprintf(os.Stderr, "%s: logging minimized (no log file; errors only to stderr); set by -disable-logging or %s\n", cmdName, taskapiconfig.EnvDisableLogging)
	} else {
		fmt.Fprintf(os.Stderr, "%s: writing structured logs to %s (min level %s)\n", cmdName, logPath, minLevel.String())
	}
	installDefaultSlog(logFile, minimized, minLevel, processLogSeq)
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.run")
	if !minimized {
		emitTaskAPIFileLoggingConfig(minLevel)
	}
}
