package main

import (
	"context"
	"errors"
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
	"github.com/AlexsanderHamir/T2A/internal/taskapi"
	"github.com/AlexsanderHamir/T2A/internal/taskapiconfig"
	"github.com/AlexsanderHamir/T2A/pkgs/agents"
	"github.com/AlexsanderHamir/T2A/pkgs/repo"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/devsim"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/logctx"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/postgres"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func run() int {
	port := flag.String("port", "8080", "HTTP listen port")
	host := flag.String("host", "", "HTTP listen host/IP (default: T2A_LISTEN_HOST or 127.0.0.1)")
	envPath := flag.String("env", "", "path to .env (default: <repo-root>/.env)")
	logDir := flag.String("logdir", "", "directory for JSON log files (default: T2A_LOG_DIR or ./logs)")
	logLevelFlag := flag.String("loglevel", "", "minimum log level for JSON file: debug, info, warn, error (default: T2A_LOG_LEVEL or info)")
	disableLoggingFlag := flag.Bool("disable-logging", false, "no log file; only errors to stderr (default: T2A_DISABLE_LOGGING)")
	flag.Parse()

	if _, err := envload.OverloadDotenvIfPresent(*envPath); err != nil {
		fmt.Fprintf(os.Stderr, "%s: preload .env: %v\n", cmdName, err)
		return 1
	}

	minLevel, err := taskapiconfig.ResolveLogLevel(*logLevelFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", cmdName, err)
		return 1
	}

	minimized := taskapiconfig.LoggingMinimized(*disableLoggingFlag)
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
		if err := logFile.Sync(); err != nil {
			fmt.Fprintf(os.Stderr, "%s: log file sync: %v\n", cmdName, err)
		}
		if err := logFile.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "%s: log file close: %v\n", cmdName, err)
		}
	}()

	var baseHandler slog.Handler
	if minimized {
		fmt.Fprintf(os.Stderr, "%s: logging minimized (no log file; errors only to stderr); set by -disable-logging or %s\n", cmdName, taskapiconfig.EnvDisableLogging)
		baseHandler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})
	} else {
		fmt.Fprintf(os.Stderr, "%s: writing structured logs to %s (min level %s)\n", cmdName, logPath, minLevel.String())
		baseHandler = slog.NewJSONHandler(logFile, &slog.HandlerOptions{Level: minLevel})
	}
	var processLogSeq atomic.Uint64
	slog.SetDefault(slog.New(logctx.WrapSlogHandlerWithLogSequence(
		logctx.WrapSlogHandlerWithRequestContext(baseHandler),
		&processLogSeq,
	)))
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.run")
	if !minimized {
		emitTaskAPIFileLoggingConfig(minLevel)
	}

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

	migrateCtx, migrateCancel := context.WithTimeout(context.Background(), postgres.DefaultMigrateTimeout)
	defer migrateCancel()
	if err := postgres.Migrate(migrateCtx, db); err != nil {
		slog.Error("migrate failed", "cmd", cmdName, "operation", "taskapi.migrate",
			"err", err,
			"deadline_exceeded", errors.Is(err, context.DeadlineExceeded),
			"timeout_sec", int(postgres.DefaultMigrateTimeout/time.Second))
		return 1
	}
	slog.Info("migrate ok", "cmd", cmdName, "operation", "taskapi.migrate",
		"timeout_sec", int(postgres.DefaultMigrateTimeout/time.Second))

	postgres.LogStartupDBConfig(slog.Default(), cmdName, db)
	taskapi.RegisterSQLDBPoolCollector(db)

	slog.Info("http server limits", "cmd", cmdName, "operation", "taskapi.http_limits",
		"read_header_timeout_sec", int(readHeaderTimeout.Seconds()),
		"read_timeout_sec", int(readTimeout.Seconds()),
		"idle_timeout_sec", int(idleTimeout.Seconds()),
		"write_timeout_disabled", true,
		"max_header_bytes", maxRequestHeaders,
		"shutdown_timeout_sec", int(shutdownTimeout.Seconds()),
	)

	taskStore := store.NewStore(db)
	hub := handler.NewSSEHub()
	var rep *repo.Root
	root := strings.TrimSpace(os.Getenv("REPO_ROOT"))
	if root != "" {
		r, err := repo.OpenRoot(root)
		if err != nil {
			slog.Error("startup failed", "cmd", cmdName, "operation", "taskapi.repo_root", "err", err)
			return 1
		}
		rep = r
		slog.Info("repo root config", "cmd", cmdName, "operation", "taskapi.repo_root",
			"enabled", true, "path", rep.Abs())
	} else {
		slog.Info("repo root config", "cmd", cmdName, "operation", "taskapi.repo_root", "enabled", false)
	}
	rlim := handler.RateLimitPerMinuteConfigured()
	slog.Info("rate limit config", "cmd", cmdName, "operation", "taskapi.rate_limit",
		"enabled", rlim > 0, "per_ip_per_min", rlim)
	slog.Info("api auth config", "cmd", cmdName, "operation", "taskapi.api_auth",
		"enabled", handler.APIAuthEnabled())

	mb := handler.MaxRequestBodyBytesConfigured()
	slog.Info("max request body config", "cmd", cmdName, "operation", "taskapi.max_body",
		"enabled", mb > 0, "max_bytes", mb)
	reqTimeout := handler.RequestTimeout()
	reqTimeoutSec := int(reqTimeout / time.Second)
	if reqTimeout > 0 && reqTimeoutSec == 0 {
		reqTimeoutSec = 1
	}
	slog.Info("request timeout config", "cmd", cmdName, "operation", "taskapi.request_timeout",
		"enabled", reqTimeout > 0, "timeout_sec", reqTimeoutSec)
	idemTTL := handler.IdempotencyTTL()
	idemMaxEntries, idemMaxBytes := handler.IdempotencyCacheLimits()
	idemSec := int(idemTTL / time.Second)
	if idemTTL > 0 && idemSec == 0 {
		idemSec = 1
	}
	slog.Info("idempotency config", "cmd", cmdName, "operation", "taskapi.idempotency",
		"enabled", idemTTL > 0, "ttl_sec", idemSec,
		"max_entries", idemMaxEntries, "max_bytes", idemMaxBytes)

	qcap := taskapiconfig.UserTaskAgentQueueCap()
	agentQueue := agents.NewMemoryQueue(qcap)
	taskStore.SetReadyTaskNotifier(agentQueue)
	iv := taskapiconfig.UserTaskAgentReconcileInterval()
	slog.Info("ready task agent queue", "cmd", cmdName, "operation", "taskapi.agent_queue", "cap", qcap)
	slog.Info("ready task agent reconcile", "cmd", cmdName, "operation", "taskapi.agent_reconcile",
		"tick_interval", iv.String(), "periodic", iv > 0)

	reconcileCtx, reconcileCancel := context.WithCancel(context.Background())
	defer reconcileCancel()
	go agents.RunReconcileLoop(reconcileCtx, taskStore, agentQueue, iv)

	api := taskapi.NewHTTPHandler(taskStore, hub, rep)
	taskapi.RegisterDefaultPrometheusCollectors()
	taskapi.RegisterBuildInfoGauge()
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", handler.WrapPrometheusHandler(promhttp.Handler()))
	if devsim.Enabled() {
		d := taskapiconfig.SSETestTickerInterval()
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
	} else {
		slog.Info("sse dev config", "cmd", cmdName, "operation", "taskapi.sse_dev", "enabled", false)
	}
	mux.Handle("/", api)

	listenHost := taskapiconfig.ListenHost(*host)
	ln, err := net.Listen("tcp", net.JoinHostPort(listenHost, *port))
	if err != nil {
		slog.Error("startup failed", "cmd", cmdName, "operation", "taskapi.listen", "err", err)
		return 1
	}

	baseURL := fmt.Sprintf("http://localhost:%s/", *port)
	slog.Info("listening", "cmd", cmdName, "operation", "taskapi.serve",
		"version", handler.ServerVersion(),
		"addr", ln.Addr().String(), "listen_host", listenHost, "url", baseURL,
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
	defer signal.Stop(sig)

	shutdownViaSignal := false
	select {
	case err := <-serveDone:
		if err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "cmd", cmdName, "operation", "taskapi.serve", "err", err)
			return 1
		}
	case s := <-sig:
		shutdownViaSignal = true
		slog.Info("shutdown signal received", "cmd", cmdName, "operation", "taskapi.shutdown", "signal", s.String())
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		shutdownErr := srv.Shutdown(shutdownCtx)
		cancel()
		if shutdownErr != nil {
			slog.Error("shutdown", "cmd", cmdName, "operation", "taskapi.shutdown",
				"err", shutdownErr,
				"deadline_exceeded", errors.Is(shutdownErr, context.DeadlineExceeded))
			return 1
		}
		slog.Info("http server drained", "cmd", cmdName, "operation", "taskapi.shutdown", "phase", "http_done")
		if err := <-serveDone; err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "cmd", cmdName, "operation", "taskapi.serve", "err", err)
			return 1
		}
	}
	dbClosed := false
	if sqlDB, err := db.DB(); err != nil {
		slog.Error("database close skipped", "cmd", cmdName, "operation", "taskapi.db_close", "err", err)
		return 1
	} else if err := sqlDB.Close(); err != nil {
		slog.Error("database close", "cmd", cmdName, "operation", "taskapi.db_close", "err", err)
		return 1
	} else {
		slog.Info("database pool closed", "cmd", cmdName, "operation", "taskapi.shutdown", "phase", "db_done")
		dbClosed = true
	}
	slog.Info("process exit", "cmd", cmdName, "operation", "taskapi.shutdown", "phase", "exit",
		"db_closed", dbClosed, "signal_shutdown", shutdownViaSignal)
	return 0
}
