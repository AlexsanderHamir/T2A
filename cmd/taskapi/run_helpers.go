package main

import (
	"context"
	"errors"
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
	"gorm.io/gorm"
)

func emitTaskAPIFileLoggingConfig(minLevel slog.Level) {
	slog.Log(context.Background(), minLevel, "logging config",
		"cmd", cmdName, "operation", "taskapi.logging",
		"min_level", minLevel.String(), "json_file", true)
}

func installDefaultSlog(logFile *os.File, minimized bool, minLevel slog.Level, processLogSeq *atomic.Uint64) {
	var baseHandler slog.Handler
	if minimized {
		baseHandler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})
	} else {
		baseHandler = slog.NewJSONHandler(logFile, &slog.HandlerOptions{Level: minLevel})
	}
	slog.SetDefault(slog.New(logctx.WrapSlogHandlerWithLogSequence(
		logctx.WrapSlogHandlerWithRequestContext(baseHandler),
		processLogSeq,
	)))
}

func migrateDBAndRegisterMetrics(db *gorm.DB) error {
	migrateCtx, migrateCancel := context.WithTimeout(context.Background(), postgres.DefaultMigrateTimeout)
	defer migrateCancel()
	if err := postgres.Migrate(migrateCtx, db); err != nil {
		slog.Error("migrate failed", "cmd", cmdName, "operation", "taskapi.migrate",
			"err", err,
			"deadline_exceeded", errors.Is(err, context.DeadlineExceeded),
			"timeout_sec", int(postgres.DefaultMigrateTimeout/time.Second))
		return err
	}
	slog.Info("migrate ok", "cmd", cmdName, "operation", "taskapi.migrate",
		"timeout_sec", int(postgres.DefaultMigrateTimeout/time.Second))
	postgres.LogStartupDBConfig(slog.Default(), cmdName, db)
	taskapi.RegisterSQLDBPoolCollector(db)
	return nil
}

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

func startReadyTaskAgents(ctx context.Context, taskStore *store.Store) (context.CancelFunc, *agents.MemoryQueue) {
	qcap := taskapiconfig.UserTaskAgentQueueCap()
	agentQueue := agents.NewMemoryQueue(qcap)
	taskStore.SetReadyTaskNotifier(agentQueue)
	iv := taskapiconfig.UserTaskAgentReconcileInterval()
	slog.Info("ready task agent queue", "cmd", cmdName, "operation", "taskapi.agent_queue", "cap", qcap)
	slog.Info("ready task agent reconcile", "cmd", cmdName, "operation", "taskapi.agent_reconcile",
		"tick_interval", iv.String(), "periodic", iv > 0)

	reconcileCtx, reconcileCancel := context.WithCancel(ctx)
	go agents.RunReconcileLoop(reconcileCtx, taskStore, agentQueue, iv)
	return reconcileCancel, agentQueue
}

func mountTaskAPIMux(api http.Handler, hub *handler.SSEHub, taskStore *store.Store, agentQueue *agents.MemoryQueue) *http.ServeMux {
	taskapi.RegisterDefaultPrometheusCollectors()
	taskapi.RegisterBuildInfoGauge()
	taskapi.RegisterAgentQueueMetrics(agentQueue)
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", handler.WrapPrometheusHandler(promhttp.Handler()))
	if devsim.Enabled() {
		maybeRunSSEDevTicker(taskStore, hub)
	} else {
		slog.Info("sse dev config", "cmd", cmdName, "operation", "taskapi.sse_dev", "enabled", false)
	}
	mux.Handle("/", api)
	return mux
}

func maybeRunSSEDevTicker(taskStore *store.Store, hub *handler.SSEHub) {
	d := taskapiconfig.SSETestTickerInterval()
	if d < time.Second {
		slog.Info("sse dev env on, ticker off", "cmd", cmdName, "operation", "taskapi.sse_dev",
			"interval", d.String(), "hint", "set T2A_SSE_TEST_INTERVAL to 1s or more to run the ticker")
		return
	}
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
}

func serveUntilShutdown(srv *http.Server, ln net.Listener) (shutdownViaSignal bool, err error) {
	serveDone := make(chan error, 1)
	go func() {
		serveDone <- srv.Serve(ln)
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sig)

	select {
	case err := <-serveDone:
		if err != nil && err != http.ErrServerClosed {
			return false, err
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
			return shutdownViaSignal, shutdownErr
		}
		slog.Info("http server drained", "cmd", cmdName, "operation", "taskapi.shutdown", "phase", "http_done")
		if err := <-serveDone; err != nil && err != http.ErrServerClosed {
			return shutdownViaSignal, err
		}
	}
	return shutdownViaSignal, nil
}

func closeSQLDBOrLog(db *gorm.DB) (dbClosed bool, err error) {
	sqlDB, err := db.DB()
	if err != nil {
		slog.Error("database close skipped", "cmd", cmdName, "operation", "taskapi.db_close", "err", err)
		return false, err
	}
	if err := sqlDB.Close(); err != nil {
		slog.Error("database close", "cmd", cmdName, "operation", "taskapi.db_close", "err", err)
		return false, err
	}
	slog.Info("database pool closed", "cmd", cmdName, "operation", "taskapi.shutdown", "phase", "db_done")
	return true, nil
}

func openTaskAPILogging(logDir, logLevelFlag string, disableLogging bool) (minLevel slog.Level, logFile *os.File, logPath string, minimized bool, err error) {
	minLevel, err = taskapiconfig.ResolveLogLevel(logLevelFlag)
	if err != nil {
		return minLevel, nil, "", false, err
	}
	minimized = taskapiconfig.LoggingMinimized(disableLogging)
	if minimized {
		return minLevel, nil, "", minimized, nil
	}
	var openErr error
	logFile, logPath, openErr = openTaskAPILogFile(logDir, minLevel)
	if openErr != nil {
		return minLevel, nil, "", false, openErr
	}
	return minLevel, logFile, logPath, minimized, nil
}

func deferCloseTaskAPILogFile(logFile *os.File) {
	if logFile == nil {
		return
	}
	if err := logFile.Sync(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: log file sync: %v\n", cmdName, err)
	}
	if err := logFile.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: log file close: %v\n", cmdName, err)
	}
}

func installTaskAPIDefaultLogger(logFile *os.File, minimized bool, minLevel slog.Level, processLogSeq *atomic.Uint64, logPath string) {
	if minimized {
		fmt.Fprintf(os.Stderr, "%s: logging minimized (no log file; errors only to stderr); set by -disable-logging or %s\n", cmdName, taskapiconfig.EnvDisableLogging)
	} else {
		fmt.Fprintf(os.Stderr, "%s: writing structured logs to %s (min level %s)\n", cmdName, logPath, minLevel.String())
	}
	installDefaultSlog(logFile, minimized, minLevel, processLogSeq)
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.run")
	if !minimized {
		emitTaskAPIFileLoggingConfig(minLevel)
	}
}

func loadEnvAndOpenDatabase(envPath string) (*gorm.DB, error) {
	envLoadedPath, err := envload.Load(envPath)
	if err != nil {
		return nil, err
	}
	slog.Info("env loaded", "cmd", cmdName, "operation", "taskapi.startup", "path", envLoadedPath)

	db, err := postgres.Open(
		os.Getenv("DATABASE_URL"),
		postgres.ConfigWithSlogLogger(slog.Default()),
	)
	if err != nil {
		return nil, err
	}
	if err := migrateDBAndRegisterMetrics(db); err != nil {
		return nil, err
	}
	logHTTPTimeoutsAndShutdown()
	return db, nil
}

type taskAPIApp struct {
	taskStore  *store.Store
	hub        *handler.SSEHub
	rep        *repo.Root
	agentQueue *agents.MemoryQueue
}

func buildTaskAPIApp(db *gorm.DB) (*taskAPIApp, context.CancelFunc, error) {
	taskStore := store.NewStore(db)
	hub := handler.NewSSEHub()
	rep, err := openOptionalRepoRoot()
	if err != nil {
		return nil, nil, err
	}
	logHandlerMiddlewareConfig()
	cancel, q := startReadyTaskAgents(context.Background(), taskStore)
	return &taskAPIApp{taskStore: taskStore, hub: hub, rep: rep, agentQueue: q}, cancel, nil
}

func runTaskAPIHTTPServer(port, host string, app *taskAPIApp) (shutdownViaSignal bool, err error) {
	api := taskapi.NewHTTPHandler(app.taskStore, app.hub, app.rep)
	mux := mountTaskAPIMux(api, app.hub, app.taskStore, app.agentQueue)

	listenHost := taskapiconfig.ListenHost(host)
	ln, err := net.Listen("tcp", net.JoinHostPort(listenHost, port))
	if err != nil {
		return false, err
	}

	baseURL := fmt.Sprintf("http://localhost:%s/", port)
	slog.Info("listening", "cmd", cmdName, "operation", "taskapi.serve",
		"version", handler.ServerVersion(),
		"addr", ln.Addr().String(), "listen_host", listenHost, "url", baseURL,
		"metrics_url", fmt.Sprintf("http://localhost:%s/metrics", port))

	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxRequestHeaders,
	}

	return serveUntilShutdown(srv, ln)
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

	app, stopAgents, err := buildTaskAPIApp(db)
	if err != nil {
		slog.Error("startup failed", "cmd", cmdName, "operation", "taskapi.repo_root", "err", err)
		return 1
	}
	defer stopAgents()

	shutdownViaSignal, serveErr := runTaskAPIHTTPServer(port, host, app)
	if serveErr != nil {
		slog.Error("server error", "cmd", cmdName, "operation", "taskapi.serve", "err", serveErr)
		return 1
	}

	dbClosed, closeErr := closeSQLDBOrLog(db)
	if closeErr != nil {
		return 1
	}
	slog.Info("process exit", "cmd", cmdName, "operation", "taskapi.shutdown", "phase", "exit",
		"db_closed", dbClosed, "signal_shutdown", shutdownViaSignal)
	return 0
}
