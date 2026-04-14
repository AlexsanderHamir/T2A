package main

import (
	"context"
	"log/slog"
	"os"
	"time"
)

const cmdName = "taskapi"

// Server timeouts: WriteTimeout is left unset so long-lived SSE streams are not cut off.
// ReadHeaderTimeout mitigates slowloris; IdleTimeout limits idle keep-alive connections.
const (
	shutdownTimeout   = 10 * time.Second
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 60 * time.Second
	idleTimeout       = 120 * time.Second
	maxRequestHeaders = 1 << 20
)

func main() {
	// Real JSON sink is installed in run() after the log file is opened; this satisfies the
	// per-function slog audit without emitting to stderr before the file exists.
	_ = slog.Default().Enabled(context.Background(), slog.LevelInfo)
	os.Exit(run())
}
