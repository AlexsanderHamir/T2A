package main

import (
	"os"
	"strings"
)

// envTruthy reports whether key is set to a common “true” value (1, true, yes, on; case-insensitive).
func envTruthy(key string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch v {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

const disableLoggingEnv = "T2A_DISABLE_LOGGING"

// taskAPILoggingMinimized returns true when file logging and most slog output should be off:
// -disable-logging flag, or T2A_DISABLE_LOGGING truthy. Only slog.Error is emitted (to stderr).
func taskAPILoggingMinimized(disableFlag bool) bool {
	if disableFlag {
		return true
	}
	return envTruthy(disableLoggingEnv)
}
