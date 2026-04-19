package taskapiconfig

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	cmdLog = "taskapi"

	// EnvUserTaskAgentQueueCap is T2A_USER_TASK_AGENT_QUEUE_CAP (bounded ready-task queue depth).
	EnvUserTaskAgentQueueCap = "T2A_USER_TASK_AGENT_QUEUE_CAP"
	// EnvUserTaskAgentReconcileInterval is T2A_USER_TASK_AGENT_RECONCILE_INTERVAL (reconcile tick; "0" = startup only).
	EnvUserTaskAgentReconcileInterval = "T2A_USER_TASK_AGENT_RECONCILE_INTERVAL"
	// EnvSSETestInterval is T2A_SSE_TEST_INTERVAL (dev synthetic SSE ticker).
	EnvSSETestInterval = "T2A_SSE_TEST_INTERVAL"
	// EnvDisableLogging is T2A_DISABLE_LOGGING (truthy values minimize logging).
	EnvDisableLogging = "T2A_DISABLE_LOGGING"
	// EnvListenHost is T2A_LISTEN_HOST (HTTP bind address).
	EnvListenHost = "T2A_LISTEN_HOST"
	// EnvLogLevel is T2A_LOG_LEVEL (minimum JSON file log level when -loglevel is unset).
	EnvLogLevel                           = "T2A_LOG_LEVEL"
	defaultUserTaskAgentQueueCap          = 256
	defaultUserTaskAgentReconcileInterval = 5 * time.Minute
	defaultSSETestInterval                = 3 * time.Second
)

// DefaultUserTaskAgentQueueCap is used when T2A_USER_TASK_AGENT_QUEUE_CAP is unset, invalid, or < 1.
const DefaultUserTaskAgentQueueCap = defaultUserTaskAgentQueueCap

// DefaultUserTaskAgentReconcileInterval is used when T2A_USER_TASK_AGENT_RECONCILE_INTERVAL is unset or invalid.
const DefaultUserTaskAgentReconcileInterval = defaultUserTaskAgentReconcileInterval

// DefaultSSETestTickerInterval is used when T2A_SSE_TEST_INTERVAL is unset or below 1s (dev only).
const DefaultSSETestTickerInterval = defaultSSETestInterval

// EnvTruthy reports whether key is set to a common “true” value (1, true, yes, on; case-insensitive).
func EnvTruthy(key string) bool {
	slog.Debug("trace", "cmd", cmdLog, "operation", "taskapiconfig.EnvTruthy", "key", key)
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch v {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// LoggingMinimized returns true when file logging and most slog output should be off:
// disableFlag, or T2A_DISABLE_LOGGING truthy. Only slog.Error is emitted (to stderr) in that mode.
func LoggingMinimized(disableFlag bool) bool {
	slog.Debug("trace", "cmd", cmdLog, "operation", "taskapiconfig.LoggingMinimized")
	if disableFlag {
		return true
	}
	return EnvTruthy(EnvDisableLogging)
}

// ResolveLogLevel returns the minimum slog level for the JSON log file.
// If flagLevel is non-empty after TrimSpace, it wins; otherwise T2A_LOG_LEVEL is used.
// When both are empty, the default is info.
func ResolveLogLevel(flagLevel string) (slog.Level, error) {
	slog.Debug("trace", "cmd", cmdLog, "operation", "taskapiconfig.ResolveLogLevel")
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

// ListenHost returns the HTTP bind host: flagHost if set, else T2A_LISTEN_HOST, else 127.0.0.1.
func ListenHost(flagHost string) string {
	slog.Debug("trace", "cmd", cmdLog, "operation", "taskapiconfig.ListenHost")
	s := strings.TrimSpace(flagHost)
	if s == "" {
		s = strings.TrimSpace(os.Getenv(EnvListenHost))
	}
	if s == "" {
		return "127.0.0.1"
	}
	return s
}

// SSETestTickerInterval returns how often the SSE dev ticker runs (T2A_SSE_TEST_INTERVAL).
// Default is 3s when unset. Set to 0 to disable the ticker. Values below 1s fall back to the default.
func SSETestTickerInterval() time.Duration {
	slog.Debug("trace", "cmd", cmdLog, "operation", "taskapiconfig.SSETestTickerInterval")
	raw := strings.TrimSpace(os.Getenv(EnvSSETestInterval))
	if raw == "" {
		return defaultSSETestInterval
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		slog.Warn("invalid T2A_SSE_TEST_INTERVAL, using default", "cmd", cmdLog, "operation", "taskapiconfig.sse_test",
			"default", defaultSSETestInterval.String(), "err", err)
		return defaultSSETestInterval
	}
	if d == 0 {
		return 0
	}
	if d < time.Second {
		slog.Warn("T2A_SSE_TEST_INTERVAL below 1s, using default", "cmd", cmdLog, "operation", "taskapiconfig.sse_test",
			"default", defaultSSETestInterval.String(), "value", raw)
		return defaultSSETestInterval
	}
	return d
}

// UserTaskAgentQueueCap returns the in-memory ready-task queue depth.
// When the env var is unset, invalid, or non-positive, the default (256) is used.
func UserTaskAgentQueueCap() int {
	slog.Debug("trace", "cmd", cmdLog, "operation", "taskapiconfig.UserTaskAgentQueueCap")
	s := strings.TrimSpace(os.Getenv(EnvUserTaskAgentQueueCap))
	if s == "" {
		return defaultUserTaskAgentQueueCap
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		slog.Warn("invalid env, using default ready-task queue cap", "cmd", cmdLog, "operation", "taskapiconfig.agent_queue_env",
			"var", EnvUserTaskAgentQueueCap, "value", s, "default", defaultUserTaskAgentQueueCap)
		return defaultUserTaskAgentQueueCap
	}
	return n
}

// UserTaskAgentReconcileInterval returns the background reconcile tick interval.
// When unset or invalid, defaults to 5m. Explicit "0" means startup reconcile only (no periodic ticker).
func UserTaskAgentReconcileInterval() time.Duration {
	slog.Debug("trace", "cmd", cmdLog, "operation", "taskapiconfig.UserTaskAgentReconcileInterval")
	raw := strings.TrimSpace(os.Getenv(EnvUserTaskAgentReconcileInterval))
	if raw == "" {
		return defaultUserTaskAgentReconcileInterval
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		slog.Warn("invalid env, using default reconcile interval", "cmd", cmdLog, "operation", "taskapiconfig.agent_reconcile_interval",
			"var", EnvUserTaskAgentReconcileInterval, "value", raw, "err", err, "default", defaultUserTaskAgentReconcileInterval.String())
		return defaultUserTaskAgentReconcileInterval
	}
	if d < 0 {
		slog.Warn("invalid env, using default reconcile interval", "cmd", cmdLog, "operation", "taskapiconfig.agent_reconcile_interval",
			"var", EnvUserTaskAgentReconcileInterval, "value", raw, "default", defaultUserTaskAgentReconcileInterval.String())
		return defaultUserTaskAgentReconcileInterval
	}
	return d
}
