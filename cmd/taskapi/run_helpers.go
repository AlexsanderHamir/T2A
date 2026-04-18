package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/agents"
	"github.com/AlexsanderHamir/T2A/pkgs/repo"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
	"gorm.io/gorm"
)

// run_helpers.go is the taskapi entrypoint orchestrator: it owns the
// taskAPIApp aggregate, buildTaskAPIApp wiring, runTaskAPIService
// (the cmd.Run body), and a handful of small startup-config log
// helpers that don't fit another concern. Per
// backend-engineering-bar.mdc §2 / §16 the lifecycle subsystems
// (logging, db, agent worker, http) live in sibling run_*.go files.

func logHTTPTimeoutsAndShutdown() {
	slog.Info("http server limits", "cmd", cmdName, "operation", "taskapi.http_limits",
		"read_header_timeout_sec", int(readHeaderTimeout.Seconds()),
		"read_timeout_sec", int(readTimeout.Seconds()),
		"idle_timeout_sec", int(idleTimeout.Seconds()),
		"write_timeout_disabled", true,
		"max_header_bytes", maxRequestHeaders,
		"shutdown_timeout_sec", int(shutdownTimeout.Seconds()),
	)
}

func openOptionalRepoRoot() (*repo.Root, error) {
	root := strings.TrimSpace(os.Getenv("REPO_ROOT"))
	if root == "" {
		slog.Info("repo root config", "cmd", cmdName, "operation", "taskapi.repo_root", "enabled", false)
		return nil, nil
	}
	r, err := repo.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	slog.Info("repo root config", "cmd", cmdName, "operation", "taskapi.repo_root",
		"enabled", true, "path", r.Abs())
	return r, nil
}

func logHandlerMiddlewareConfig() {
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
}

type taskAPIApp struct {
	taskStore   *store.Store
	hub         *handler.SSEHub
	rep         *repo.Root
	agentQueue  *agents.MemoryQueue
	agentWorker *agentWorkerHandle
}

func buildTaskAPIApp(ctx context.Context, db *gorm.DB) (*taskAPIApp, context.CancelFunc, error) {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.buildTaskAPIApp")
	taskStore := store.NewStore(db)
	hub := handler.NewSSEHub()
	rep, err := openOptionalRepoRoot()
	if err != nil {
		return nil, nil, err
	}
	logHandlerMiddlewareConfig()
	cancel, q, aw, err := startReadyTaskAgents(ctx, taskStore, hub)
	if err != nil {
		return nil, nil, err
	}
	return &taskAPIApp{taskStore: taskStore, hub: hub, rep: rep, agentQueue: q, agentWorker: aw}, cancel, nil
}

func runTaskAPIService(port, host, envPath, logDir, logLevelFlag string, disableLogging bool) int {
	minLevel, logFile, logPath, minimized, err := openTaskAPILogging(logDir, logLevelFlag, disableLogging)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", cmdName, err)
		return 1
	}
	defer deferCloseTaskAPILogFile(logFile)

	var processLogSeq atomic.Uint64
	installTaskAPIDefaultLogger(logFile, minimized, minLevel, &processLogSeq, logPath)

	db, err := loadEnvAndOpenDatabase(envPath)
	if err != nil {
		slog.Error("startup failed", "cmd", cmdName, "operation", "taskapi.startup_db", "err", err)
		return 1
	}

	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()

	app, stopAgents, err := buildTaskAPIApp(appCtx, db)
	if err != nil {
		slog.Error("startup failed", "cmd", cmdName, "operation", "taskapi.startup_app", "err", err)
		closeSQLDBOrLog(db)
		return 1
	}

	shutdownViaSignal, serveErr := runTaskAPIHTTPServer(appCtx, port, host, app)
	// Order: cancel worker ctx and wait for it to drain (best-effort
	// aborted/cycle writes need a live DB pool) → cancel reconcile →
	// close DB. Done in this strict order even on serve error so the
	// audit trail finishes before the pool closes.
	app.agentWorker.drain()
	stopAgents()

	if serveErr != nil {
		slog.Error("server error", "cmd", cmdName, "operation", "taskapi.serve", "err", serveErr)
		closeSQLDBOrLog(db)
		return 1
	}

	dbClosed := closeSQLDBOrLog(db)
	if !dbClosed {
		return 1
	}
	slog.Info("process exit", "cmd", cmdName, "operation", "taskapi.shutdown", "phase", "exit",
		"db_closed", dbClosed, "signal_shutdown", shutdownViaSignal)
	return 0
}
