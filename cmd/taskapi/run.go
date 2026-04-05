package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/AlexsanderHamir/T2A/internal/envload"
	"github.com/AlexsanderHamir/T2A/pkgs/repo"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/devsim"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/postgres"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func run() int {
	port := flag.String("port", "8080", "HTTP listen port")
	envPath := flag.String("env", "", "path to .env (default: <repo-root>/.env)")
	logDir := flag.String("logdir", "", "directory for JSON log files (default: T2A_LOG_DIR or ./logs)")
	logLevelFlag := flag.String("loglevel", "", "minimum log level for JSON file: debug, info, warn, error (default: T2A_LOG_LEVEL or info)")
	disableLoggingFlag := flag.Bool("disable-logging", false, "no log file; only errors to stderr (default: T2A_DISABLE_LOGGING)")
	flag.Parse()

	if _, err := envload.OverloadDotenvIfPresent(*envPath); err != nil {
		fmt.Fprintf(os.Stderr, "%s: preload .env: %v\n", cmdName, err)
		return 1
	}

	minLevel, err := resolveTaskAPILogLevel(*logLevelFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", cmdName, err)
		return 1
	}

	minimized := taskAPILoggingMinimized(*disableLoggingFlag)
	var logFile *os.File
	var logPath string
	if !minimized {
		var openErr error
		logFile, logPath, openErr = openTaskAPILogFile(*logDir, minLevel)
		if openErr != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", cmdName, openErr)
			return 1
		}
	}
	defer func() {
		if logFile == nil {
			return
		}
		_ = logFile.Sync()
		_ = logFile.Close()
	}()

	var baseHandler slog.Handler
	if minimized {
		fmt.Fprintf(os.Stderr, "%s: logging minimized (no log file; errors only to stderr); set by -disable-logging or %s\n", cmdName, disableLoggingEnv)
		baseHandler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})
	} else {
		fmt.Fprintf(os.Stderr, "%s: writing structured logs to %s (min level %s)\n", cmdName, logPath, minLevel.String())
		baseHandler = slog.NewJSONHandler(logFile, &slog.HandlerOptions{Level: minLevel})
	}
	var processLogSeq atomic.Uint64
	slog.SetDefault(slog.New(handler.WrapSlogHandlerWithLogSequence(
		handler.WrapSlogHandlerWithRequestContext(baseHandler),
		&processLogSeq,
	)))
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.run")

	path, err := envload.Load(*envPath)
	if err != nil {
		slog.Error("startup failed", "cmd", cmdName, "operation", "taskapi.env", "err", err)
		return 1
	}
	slog.Info("env loaded", "cmd", cmdName, "operation", "taskapi.startup", "path", path)

	db, err := postgres.Open(
		os.Getenv("DATABASE_URL"),
		postgres.ConfigWithSlogLogger(slog.Default()),
	)
	if err != nil {
		slog.Error("startup failed", "cmd", cmdName, "operation", "taskapi.db", "err", err)
		return 1
	}

	ctx := context.Background()
	if err := postgres.Migrate(ctx, db); err != nil {
		slog.Error("migrate failed", "cmd", cmdName, "operation", "taskapi.migrate", "err", err)
		return 1
	}
	slog.Info("migrate ok", "cmd", cmdName, "operation", "taskapi.migrate")

	postgres.LogStartupDBConfig(slog.Default(), cmdName, db)

	slog.Info("http server limits", "cmd", cmdName, "operation", "taskapi.http_limits",
		"read_header_timeout_sec", int(readHeaderTimeout.Seconds()),
		"read_timeout_sec", int(readTimeout.Seconds()),
		"idle_timeout_sec", int(idleTimeout.Seconds()),
		"max_header_bytes", maxRequestHeaders,
		"shutdown_timeout_sec", int(shutdownTimeout.Seconds()),
	)

	taskStore := store.NewStore(db)
	hub := handler.NewSSEHub()
	var rep *repo.Root
	if root := strings.TrimSpace(os.Getenv("REPO_ROOT")); root != "" {
		r, err := repo.OpenRoot(root)
		if err != nil {
			slog.Error("startup failed", "cmd", cmdName, "operation", "taskapi.repo_root", "err", err)
			return 1
		}
		rep = r
		slog.Info("repo root configured", "cmd", cmdName, "operation", "taskapi.startup", "path", rep.Abs())
	}
	rlim := handler.RateLimitPerMinuteConfigured()
	slog.Info("rate limit config", "cmd", cmdName, "operation", "taskapi.rate_limit",
		"enabled", rlim > 0, "per_ip_per_min", rlim)

	mb := handler.MaxRequestBodyBytesConfigured()
	slog.Info("max request body config", "cmd", cmdName, "operation", "taskapi.max_body",
		"enabled", mb > 0, "max_bytes", mb)
	idemTTL := handler.IdempotencyTTL()
	idemSec := int(idemTTL / time.Second)
	if idemTTL > 0 && idemSec == 0 {
		idemSec = 1
	}
	slog.Info("idempotency config", "cmd", cmdName, "operation", "taskapi.idempotency",
		"enabled", idemTTL > 0, "ttl_sec", idemSec)

	api := handler.WithRecovery(handler.WithHTTPMetrics(handler.WithAccessLog(handler.WithRateLimit(handler.WithMaxRequestBody(handler.WithIdempotency(handler.NewHandler(taskStore, hub, rep)))))))
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.Handler())
	if devsim.Enabled() {
		d := resolveSSETestTickerInterval()
		if d >= time.Second {
			slog.Info("sse dev ticker enabled", "cmd", cmdName, "operation", "taskapi.sse_dev", "interval", d.String())
			opts := devsim.LoadOptions()
			devsim.RunTicker(taskStore, d, opts, func(kind devsim.ChangeKind, id string) {
				var typ handler.TaskChangeType
				switch kind {
				case devsim.ChangeCreated:
					typ = handler.TaskCreated
				case devsim.ChangeDeleted:
					typ = handler.TaskDeleted
				default:
					typ = handler.TaskUpdated
				}
				hub.Publish(handler.TaskChangeEvent{Type: typ, ID: id})
			})
		} else {
			slog.Info("sse dev env on, ticker off", "cmd", cmdName, "operation", "taskapi.sse_dev",
				"interval", d.String(), "hint", "set T2A_SSE_TEST_INTERVAL to 1s or more to run the ticker")
		}
	}
	mux.Handle("/", api)

	ln, err := net.Listen("tcp", net.JoinHostPort("", *port))
	if err != nil {
		slog.Error("startup failed", "cmd", cmdName, "operation", "taskapi.listen", "err", err)
		return 1
	}

	baseURL := fmt.Sprintf("http://localhost:%s/", *port)
	slog.Info("listening", "cmd", cmdName, "operation", "taskapi.serve",
		"version", handler.ServerVersion(),
		"addr", ln.Addr().String(), "url", baseURL,
		"metrics_url", fmt.Sprintf("http://localhost:%s/metrics", *port))

	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxRequestHeaders,
	}

	serveDone := make(chan error, 1)
	go func() {
		serveDone <- srv.Serve(ln)
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serveDone:
		if err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "cmd", cmdName, "operation", "taskapi.serve", "err", err)
			return 1
		}
	case s := <-sig:
		slog.Info("shutdown signal received", "cmd", cmdName, "operation", "taskapi.shutdown", "signal", s.String())
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		shutdownErr := srv.Shutdown(shutdownCtx)
		cancel()
		if shutdownErr != nil {
			slog.Error("shutdown", "cmd", cmdName, "operation", "taskapi.shutdown", "err", shutdownErr)
			return 1
		}
		slog.Info("http server drained", "cmd", cmdName, "operation", "taskapi.shutdown", "phase", "http_done")
		if err := <-serveDone; err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "cmd", cmdName, "operation", "taskapi.serve", "err", err)
			return 1
		}
	}
	if sqlDB, err := db.DB(); err != nil {
		slog.Error("database close skipped", "cmd", cmdName, "operation", "taskapi.db_close", "err", err)
	} else if err := sqlDB.Close(); err != nil {
		slog.Error("database close", "cmd", cmdName, "operation", "taskapi.db_close", "err", err)
		return 1
	} else {
		slog.Info("database pool closed", "cmd", cmdName, "operation", "taskapi.shutdown", "phase", "db_done")
	}
	return 0
}

// resolveSSETestTickerInterval returns how often the SSE dev ticker runs store.List + AppendTaskEvent per task.
// Default is 3s when T2A_SSE_TEST_INTERVAL is unset. Set to 0 to disable the ticker.
func resolveSSETestTickerInterval() time.Duration {
	raw := strings.TrimSpace(os.Getenv(sseTestIntervalEnv))
	if raw == "" {
		return sseTestDefaultInterval
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		slog.Warn("invalid T2A_SSE_TEST_INTERVAL, using default", "cmd", cmdName, "operation", "taskapi.sse_test",
			"default", sseTestDefaultInterval.String(), "err", err)
		return sseTestDefaultInterval
	}
	if d == 0 {
		return 0
	}
	if d < time.Second {
		slog.Warn("T2A_SSE_TEST_INTERVAL below 1s, using default", "cmd", cmdName, "operation", "taskapi.sse_test",
			"default", sseTestDefaultInterval.String(), "value", raw)
		return sseTestDefaultInterval
	}
	return d
}

// resolveTaskAPILogLevel returns the minimum slog level for the JSON log file.
// If flagLevel is non-empty after TrimSpace, it wins; otherwise T2A_LOG_LEVEL is used.
// When both are empty, the default is info (no Debug trace lines; lighter for production).
func resolveTaskAPILogLevel(flagLevel string) (slog.Level, error) {
	s := strings.TrimSpace(flagLevel)
	if s == "" {
		s = strings.TrimSpace(os.Getenv("T2A_LOG_LEVEL"))
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
		return slog.LevelInfo, fmt.Errorf("invalid -loglevel / T2A_LOG_LEVEL %q (want debug, info, warn, error)", s)
	}
}
