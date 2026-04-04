package devsim

import (
	"log/slog"
	"os"
	"strings"
)

const envSSETest = "T2A_SSE_TEST"

// Enabled reports whether T2A_SSE_TEST=1 (dev-only simulation enabled).
func Enabled() bool {
	slog.Debug("trace", "cmd", logCmd, "operation", "devsim.Enabled")
	return strings.TrimSpace(os.Getenv(envSSETest)) == "1"
}
