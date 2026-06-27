package taskapiconfig

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
)

const (
	cmdLog = calltrace.LogCmd

	// EnvUserTaskAgentQueueCap is HAMIX_USER_TASK_AGENT_QUEUE_CAP (bounded ready-task queue depth).
	EnvUserTaskAgentQueueCap = "HAMIX_USER_TASK_AGENT_QUEUE_CAP"
	// EnvSSETestInterval is HAMIX_SSE_TEST_INTERVAL (dev synthetic SSE ticker).
	EnvSSETestInterval = "HAMIX_SSE_TEST_INTERVAL"
	// EnvDisableLogging is HAMIX_DISABLE_LOGGING (truthy values minimize logging).
	EnvDisableLogging = "HAMIX_DISABLE_LOGGING"
	// EnvMigrate is HAMIX_MIGRATE (truthy values run postgres.Migrate at taskapi startup).
	EnvMigrate = "HAMIX_MIGRATE"
	// EnvListenHost is HAMIX_LISTEN_HOST (HTTP bind address).
	EnvListenHost = "HAMIX_LISTEN_HOST"
	// EnvLogLevel is HAMIX_LOG_LEVEL (minimum JSON file log level when -loglevel is unset).
	EnvLogLevel = "HAMIX_LOG_LEVEL"
	// EnvWorkerReportDir is HAMIX_WORKER_REPORT_DIR. Overrides the default
	// worker-managed scratch directory (<os.TempDir()>/hamix-worker)
	// where the agent CLI writes criteria-report.json /
	// verify-report.json. Lives outside the operator's RepoRoot so
	// customer working trees stay clean. The supervisor validates the
	// path is writable at startup; failure logs a warn and falls back
	// to the default rather than blocking the worker.
	EnvWorkerReportDir = "HAMIX_WORKER_REPORT_DIR"
	// EnvAgentWorkerConcurrency is HAMIX_AGENT_WORKER_CONCURRENCY (in-process worker pool size).
	EnvAgentWorkerConcurrency = "HAMIX_AGENT_WORKER_CONCURRENCY"
	// EnvGitReconcileOnStartup is HAMIX_GIT_RECONCILE_ON_STARTUP (conservative git reconcile at boot).
	EnvGitReconcileOnStartup     = "HAMIX_GIT_RECONCILE_ON_STARTUP"
	defaultUserTaskAgentQueueCap = 256
	defaultSSETestInterval       = 3 * time.Second
	// defaultWorkerReportDirSubdir matches worker.DefaultReportDirSubdir;
	// duplicated here so the env layer does not depend on the worker
	// package. The Worker package is the source of truth for the leaf
	// name; supervising code that wants the resolved path should call
	// WorkerReportDir() rather than recomputing.
	defaultWorkerReportDirSubdir = "hamix-worker"
)

// DefaultAgentWorkerConcurrency is used when HAMIX_AGENT_WORKER_CONCURRENCY is unset, invalid, or out of range.
const DefaultAgentWorkerConcurrency = defaultAgentWorkerConcurrency

const defaultAgentWorkerConcurrency = 4

// DefaultSSETestTickerInterval is used when HAMIX_SSE_TEST_INTERVAL is unset or below 1s (dev only).
const DefaultSSETestTickerInterval = defaultSSETestInterval

// EnvTruthy reports whether key is set to a common “true” value (1, true, yes, on; case-insensitive).
func EnvTruthy(key string) bool {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "taskapiconfig.EnvTruthy", "key", key)
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch v {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// MigrateEnabled reports whether taskapi should run postgres.Migrate at startup.
// The -migrate flag wins when true; otherwise HAMIX_MIGRATE is consulted.
func MigrateEnabled(migrateFlag bool) bool {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "taskapiconfig.MigrateEnabled")
	if migrateFlag {
		return true
	}
	return EnvTruthy(EnvMigrate)
}

// LoggingMinimized returns true when file logging and most slog output should be off:
// disableFlag, or HAMIX_DISABLE_LOGGING truthy. Only slog.Error is emitted (to stderr) in that mode.
func LoggingMinimized(disableFlag bool) bool {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "taskapiconfig.LoggingMinimized")
	if disableFlag {
		return true
	}
	return EnvTruthy(EnvDisableLogging)
}

// ResolveLogLevel returns the minimum slog level for the JSON log file.
// If flagLevel is non-empty after TrimSpace, it wins; otherwise HAMIX_LOG_LEVEL is used.
// When both are empty, the default is info.
func ResolveLogLevel(flagLevel string) (slog.Level, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "taskapiconfig.ResolveLogLevel")
	s := strings.TrimSpace(flagLevel)
	if s == "" {
		s = strings.TrimSpace(os.Getenv(EnvLogLevel))
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
		return slog.LevelInfo, fmt.Errorf("invalid -loglevel / %s %q (want debug, info, warn, error)", EnvLogLevel, s)
	}
}

// ListenHost returns the HTTP bind host: flagHost if set, else HAMIX_LISTEN_HOST, else 127.0.0.1.
func ListenHost(flagHost string) string {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "taskapiconfig.ListenHost")
	s := strings.TrimSpace(flagHost)
	if s == "" {
		s = strings.TrimSpace(os.Getenv(EnvListenHost))
	}
	if s == "" {
		return "127.0.0.1"
	}
	return s
}

// SSETestTickerInterval returns how often the SSE dev ticker runs (HAMIX_SSE_TEST_INTERVAL).
// Default is 3s when unset. Set to 0 to disable the ticker. Values below 1s fall back to the default.
func SSETestTickerInterval() time.Duration {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "taskapiconfig.SSETestTickerInterval")
	raw := strings.TrimSpace(os.Getenv(EnvSSETestInterval))
	if raw == "" {
		return defaultSSETestInterval
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		slog.Warn("invalid HAMIX_SSE_TEST_INTERVAL, using default", "cmd", calltrace.LogCmd, "operation", "taskapiconfig.sse_test",
			"default", defaultSSETestInterval.String(), "err", err)
		return defaultSSETestInterval
	}
	if d == 0 {
		return 0
	}
	if d < time.Second {
		slog.Warn("HAMIX_SSE_TEST_INTERVAL below 1s, using default", "cmd", calltrace.LogCmd, "operation", "taskapiconfig.sse_test",
			"default", defaultSSETestInterval.String(), "value", raw)
		return defaultSSETestInterval
	}
	return d
}

// WorkerReportDir resolves the worker-managed scratch root for the
// agent <-> worker side-channel report files. Returns the value of
// HAMIX_WORKER_REPORT_DIR when set (after TrimSpace); otherwise
// <os.TempDir()>/hamix-worker. Never returns an empty string — callers
// can pass the result straight into worker.Options.ReportDir without
// a nil/empty guard.
func WorkerReportDir() string {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "taskapiconfig.WorkerReportDir")
	s := strings.TrimSpace(os.Getenv(EnvWorkerReportDir))
	if s != "" {
		return s
	}
	return filepath.Join(os.TempDir(), defaultWorkerReportDirSubdir)
}

// UserTaskAgentQueueCap returns the in-memory ready-task queue depth.
// When the env var is unset, invalid, or non-positive, the default (256) is used.
func UserTaskAgentQueueCap() int {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "taskapiconfig.UserTaskAgentQueueCap")
	s := strings.TrimSpace(os.Getenv(EnvUserTaskAgentQueueCap))
	if s == "" {
		return defaultUserTaskAgentQueueCap
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		slog.Warn("invalid env, using default ready-task queue cap", "cmd", calltrace.LogCmd, "operation", "taskapiconfig.agent_queue_env",
			"var", EnvUserTaskAgentQueueCap, "value", s, "default", defaultUserTaskAgentQueueCap)
		return defaultUserTaskAgentQueueCap
	}
	return n
}

// AgentWorkerConcurrency returns the in-process agent worker pool size.
// Clamped to [1, 32]; default is 4 when unset or invalid.
func AgentWorkerConcurrency() int {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "taskapiconfig.AgentWorkerConcurrency")
	s := strings.TrimSpace(os.Getenv(EnvAgentWorkerConcurrency))
	if s == "" {
		return defaultAgentWorkerConcurrency
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		slog.Warn("invalid env, using default agent worker concurrency", "cmd", calltrace.LogCmd,
			"operation", "taskapiconfig.agent_worker_concurrency_env",
			"var", EnvAgentWorkerConcurrency, "value", s, "default", defaultAgentWorkerConcurrency)
		return defaultAgentWorkerConcurrency
	}
	if n > 32 {
		n = 32
	}
	return n
}

// GitReconcileOnStartupMode returns the startup reconcile mode when enabled.
// Only "repair-only" is supported today; empty string means disabled.
func GitReconcileOnStartupMode() string {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "taskapiconfig.GitReconcileOnStartupMode")
	v := strings.ToLower(strings.TrimSpace(os.Getenv(EnvGitReconcileOnStartup)))
	if v == "repair-only" {
		return v
	}
	if v != "" {
		slog.Warn("unsupported git startup reconcile mode, ignoring", "cmd", calltrace.LogCmd,
			"operation", "taskapiconfig.git_reconcile_on_startup", "value", v)
	}
	return ""
}
