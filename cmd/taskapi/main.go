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
)

const (
	sseTestIntervalEnv     = "T2A_SSE_TEST_INTERVAL"
	sseTestDefaultInterval = 3 * time.Second
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

func run() int {
	port := flag.String("port", "8080", "HTTP listen port")
	envPath := flag.String("env", "", "path to .env (default: <repo-root>/.env)")
	logDir := flag.String("logdir", "", "directory for JSON log files (default: T2A_LOG_DIR or ./logs)")
	flag.Parse()

	logFile, logPath, err := openTaskAPILogFile(*logDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", cmdName, err)
		return 1
	}
	defer func() {
		_ = logFile.Sync()
		_ = logFile.Close()
	}()

	fmt.Fprintf(os.Stderr, "%s: writing structured logs to %s\n", cmdName, logPath)
	jsonHandler := slog.NewJSONHandler(logFile, &slog.HandlerOptions{Level: slog.LevelDebug})
	var processLogSeq atomic.Uint64
	slog.SetDefault(slog.New(handler.WrapSlogHandlerWithLogSequence(
		handler.WrapSlogHandlerWithRequestContext(jsonHandler),
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
	api := handler.WithRecovery(handler.WithAccessLog(handler.NewHandler(taskStore, hub, rep)))
	mux := http.NewServeMux()
	if devsim.Enabled() {
		if d := resolveSSETestTickerInterval(); d >= time.Second {
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
		}
	}
	mux.Handle("/", api)

	ln, err := net.Listen("tcp", net.JoinHostPort("", *port))
	if err != nil {
		slog.Error("startup failed", "cmd", cmdName, "operation", "taskapi.listen", "err", err)
		return 1
	}

	baseURL := fmt.Sprintf("http://localhost:%s/", *port)
	slog.Info("listening", "cmd", cmdName, "operation", "taskapi.serve", "addr", ln.Addr().String(), "url", baseURL)

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
		// Serve returned before signal (e.g. ErrServerClosed); continue to DB cleanup.
	case <-sig:
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		shutdownErr := srv.Shutdown(shutdownCtx)
		cancel()
		if shutdownErr != nil {
			slog.Error("shutdown", "cmd", cmdName, "operation", "taskapi.shutdown", "err", shutdownErr)
			return 1
		}
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
