package main

import (
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

// main is intentionally a thin wrapper around run(). The slog JSON sink is
// installed inside run() after the log file is opened, so logging here would
// emit to stderr before the file exists. Skip-listed in
// cmd/funclogmeasure/analyze.go for the same reason.
func main() {
	os.Exit(run())
}
