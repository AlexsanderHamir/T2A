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

	defaultUserTaskAgentQueueCap          = 256
	defaultUserTaskAgentReconcileInterval = 5 * time.Minute
)

// userTaskAgentQueueCap returns the in-memory ready-task queue depth.
// When the env var is unset, invalid, or non-positive, the default (256) is used.
func userTaskAgentQueueCap() int {
	s := strings.TrimSpace(os.Getenv(userTaskAgentQueueCapEnv))
	if s == "" {
		return defaultUserTaskAgentQueueCap
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		slog.Warn("invalid env, using default ready-task queue cap", "cmd", cmdName, "operation", "taskapi.agent_queue_env",
			"var", userTaskAgentQueueCapEnv, "value", s, "default", defaultUserTaskAgentQueueCap)
		return defaultUserTaskAgentQueueCap
	}
	return n
}

// userTaskAgentReconcileInterval returns the background reconcile tick interval.
// When unset or invalid, defaults to 5m. Explicit "0" means startup reconcile only (no periodic ticker).
func userTaskAgentReconcileInterval() time.Duration {
	raw := strings.TrimSpace(os.Getenv(userTaskAgentReconcileIntervalEnv))
	if raw == "" {
		return defaultUserTaskAgentReconcileInterval
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		slog.Warn("invalid env, using default reconcile interval", "cmd", cmdName, "operation", "taskapi.agent_reconcile_interval",
			"var", userTaskAgentReconcileIntervalEnv, "value", raw, "err", err, "default", defaultUserTaskAgentReconcileInterval.String())
		return defaultUserTaskAgentReconcileInterval
	}
	if d < 0 {
		slog.Warn("invalid env, using default reconcile interval", "cmd", cmdName, "operation", "taskapi.agent_reconcile_interval",
			"var", userTaskAgentReconcileIntervalEnv, "value", raw, "default", defaultUserTaskAgentReconcileInterval.String())
		return defaultUserTaskAgentReconcileInterval
	}
	return d
}
