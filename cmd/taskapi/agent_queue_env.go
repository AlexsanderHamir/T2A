package main

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
)

const userTaskAgentQueueCapEnv = "T2A_USER_TASK_AGENT_QUEUE_CAP"

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
