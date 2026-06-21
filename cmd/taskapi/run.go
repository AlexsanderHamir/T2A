package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/AlexsanderHamir/Hamix/internal/envload"
)

func run() int {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.run")
	port := flag.String("port", "8080", "HTTP listen port")
	host := flag.String("host", "", "HTTP listen host/IP (default: HAMIX_LISTEN_HOST or 127.0.0.1)")
	envPath := flag.String("env", "", "path to .env (default: <repo-root>/.env)")
	logDir := flag.String("logdir", "", "directory for JSON log files (default: HAMIX_LOG_DIR or ./logs)")
	logLevelFlag := flag.String("loglevel", "", "minimum log level for JSON file: debug, info, warn, error (default: HAMIX_LOG_LEVEL or info)")
	disableLoggingFlag := flag.Bool("disable-logging", false, "no log file; only errors to stderr (default: HAMIX_DISABLE_LOGGING)")
	flag.Parse()

	if _, err := envload.OverloadDotenvIfPresent(*envPath); err != nil {
		fmt.Fprintf(os.Stderr, "%s: preload .env: %v\n", cmdName, err)
		return 1
	}
	return runTaskAPIService(*port, *host, *envPath, *logDir, *logLevelFlag, *disableLoggingFlag)
}
