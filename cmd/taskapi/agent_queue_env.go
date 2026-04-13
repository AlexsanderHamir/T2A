package main

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	userTaskAgentQueueCapEnv          = "T2A_USER_TASK_AGENT_QUEUE_CAP"
	userTaskAgentReconcileIntervalEnv = "T2A_USER_TASK_AGENT_RECONCILE_INTERVAL"
)

// userTaskAgentQueueCap returns a positive buffer size when the env var is set to a valid integer, else 0 (queue disabled).
func userTaskAgentQueueCap() int {
	s := strings.TrimSpace(os.Getenv(userTaskAgentQueueCapEnv))
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		slog.Warn("invalid env", "cmd", cmdName, "operation", "taskapi.agent_queue_env", "var", userTaskAgentQueueCapEnv, "value", s)
		return 0
	}
	return n
}

// userTaskAgentReconcileInterval returns a positive tick interval for background reconcile, or 0 when unset/invalid/zero.
func userTaskAgentReconcileInterval() time.Duration {
	raw := strings.TrimSpace(os.Getenv(userTaskAgentReconcileIntervalEnv))
	if raw == "" {
		return 0
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		slog.Warn("invalid env", "cmd", cmdName, "operation", "taskapi.agent_reconcile_interval", "var", userTaskAgentReconcileIntervalEnv, "value", raw, "err", err)
		return 0
	}
	return d
}
