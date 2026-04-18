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
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/worker"
	"github.com/AlexsanderHamir/T2A/pkgs/repo"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/devsim"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/logctx"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/postgres"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/gorm"
)

// shutdownGraceAfterRunTimeout is the headroom added to
// T2A_AGENT_WORKER_RUN_TIMEOUT when waiting for Worker.Run to drain
// during shutdown. The extra slack covers the worker's own deferred
// best-effort writes (handleShutdownAfterRun) so they can land before
// the reconcile ctx and DB pool close.
const shutdownGraceAfterRunTimeout = 10 * time.Second

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

// agentWorkerHandle bundles the per-process state for the optional
// agent worker. When the worker is disabled (the documented default),
// every field except waitDone is zero/nil and waitDone is a closed
// channel so shutdown sequencing stays uniform.
type agentWorkerHandle struct {
	worker       *worker.Worker
	cancelWorker context.CancelFunc
	waitDone     chan struct{}
	runTimeout   time.Duration
}

// drain blocks until Worker.Run returns or the bounded shutdown
// deadline trips. The deadline is RunTimeout plus a fixed grace so
// the worker's own best-effort post-cancel writes
// (handleShutdownAfterRun) can land before reconcile or the DB pool
// close. Closes the worker context first so the runner sees a
// cancelled ctx promptly.
func (h *agentWorkerHandle) drain() {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.agentWorkerHandle.drain")
	if h == nil || h.cancelWorker == nil {
		return
	}
	h.cancelWorker()
	deadline := h.runTimeout + shutdownGraceAfterRunTimeout
	select {
	case <-h.waitDone:
		slog.Info("agent worker drained", "cmd", cmdName, "operation", "taskapi.shutdown",
			"phase", "worker_done")
	case <-time.After(deadline):
		slog.Warn("agent worker drain timeout", "cmd", cmdName, "operation", "taskapi.shutdown",
			"phase", "worker_drain_timeout", "deadline", deadline.String())
	}
}

// startReadyTaskAgents wires the bounded ready-task queue, the
// reconcile loop, and (when T2A_AGENT_WORKER_ENABLED is truthy) the
// in-process Cursor CLI agent worker plus its startup orphan sweep.
// The returned cancel func tears down the reconcile goroutine; the
// worker handle is non-nil even when the worker is disabled so the
// shutdown path can call drain() unconditionally.
//
// Wiring order when the worker is enabled:
//  1. Probe `cursor --version` with a 5s budget; exit 1 on failure.
//  2. Run worker.SweepOrphanRunningCycles once on the freshly opened
//     store so any cycle/phase rows stuck in 'running' from a previous
//     crash are closed before the new worker can race them.
//  3. Build the Cursor adapter using the probed version string.
//  4. Build the SSE notifier adapter that wraps hub.Publish.
//  5. Construct + Run the Worker on a child context derived from ctx.
//
// When the worker is disabled (default), only the queue + reconcile
// loop start — no probe, no sweep, no Cursor binary required.
func startReadyTaskAgents(ctx context.Context, taskStore *store.Store, hub *handler.SSEHub) (context.CancelFunc, *agents.MemoryQueue, *agentWorkerHandle) {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.startReadyTaskAgents")
	qcap := taskapiconfig.UserTaskAgentQueueCap()
	agentQueue := agents.NewMemoryQueue(qcap)
	taskStore.SetReadyTaskNotifier(agentQueue)
	iv := taskapiconfig.UserTaskAgentReconcileInterval()
	slog.Info("ready task agent queue", "cmd", cmdName, "operation", "taskapi.agent_queue", "cap", qcap)
	slog.Info("ready task agent reconcile", "cmd", cmdName, "operation", "taskapi.agent_reconcile",
		"tick_interval", iv.String(), "periodic", iv > 0)

	reconcileCtx, reconcileCancel := context.WithCancel(ctx)
	go agents.RunReconcileLoop(reconcileCtx, taskStore, agentQueue, iv)

	handle := startAgentWorkerIfEnabled(ctx, taskStore, agentQueue, hub)
	return reconcileCancel, agentQueue, handle
}

// startAgentWorkerIfEnabled returns a populated agentWorkerHandle when
// T2A_AGENT_WORKER_ENABLED is truthy, or a no-op handle (closed
// waitDone, nil cancel) otherwise. Failures inside the enabled branch
// call os.Exit(1) per the Stage 4 "fail loudly at startup" rule —
// caller cannot recover from a missing Cursor binary.
func startAgentWorkerIfEnabled(ctx context.Context, taskStore *store.Store, agentQueue *agents.MemoryQueue, hub *handler.SSEHub) *agentWorkerHandle {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.startAgentWorkerIfEnabled")
	enabled := taskapiconfig.AgentWorkerEnabled()
	runTimeout := taskapiconfig.AgentWorkerRunTimeout()
	workingDir := taskapiconfig.AgentWorkerWorkingDir()
	cursorBin := taskapiconfig.AgentWorkerCursorBin()
	if !enabled {
		slog.Info("agent worker config", "cmd", cmdName, "operation", "taskapi.agent_worker",
			"enabled", false, "runner", "", "cursor_bin", cursorBin,
			"cursor_version", "", "run_timeout_sec", int(runTimeout/time.Second),
			"working_dir", workingDir)
		closed := make(chan struct{})
		close(closed)
		return &agentWorkerHandle{waitDone: closed, runTimeout: runTimeout}
	}

	if err := assertWorkingDirExists(workingDir); err != nil {
		slog.Error("agent worker working dir not usable, refusing to start agent worker",
			"cmd", cmdName, "operation", "taskapi.agent_worker.workdir_err",
			"working_dir", workingDir, "err", err)
		os.Exit(1)
	}

	cursorVersion, err := cursor.Probe(ctx, cursorBin, cursor.DefaultProbeTimeout, nil)
	if err != nil {
		slog.Error("cursor binary not usable, refusing to start agent worker",
			"cmd", cmdName, "operation", "taskapi.agent_worker.probe_err",
			"cursor_bin", cursorBin, "err", err)
		os.Exit(1)
	}

	sweepCtx, cancelSweep := context.WithTimeout(ctx, 30*time.Second)
	res, sweepErr := worker.SweepOrphanRunningCycles(sweepCtx, taskStore)
	cancelSweep()
	if sweepErr != nil {
		slog.Warn("agent worker startup sweep failed (continuing anyway)",
			"cmd", cmdName, "operation", "taskapi.agent_worker.sweep_err", "err", sweepErr)
	} else {
		slog.Info("agent worker startup sweep ok", "cmd", cmdName,
			"operation", "taskapi.agent_worker.sweep_ok",
			"cycles_aborted", res.CyclesAborted, "phases_failed", res.PhasesFailed,
			"tasks_failed", res.TasksFailed)
	}

	adapter := cursor.New(cursor.Options{
		BinaryPath: cursorBin,
		Version:    cursorVersion,
	})

	notifier := newCycleChangeSSEAdapter(hub)
	w := worker.NewWorker(taskStore, agentQueue, adapter, worker.Options{
		RunTimeout: runTimeout,
		WorkingDir: workingDir,
		Notifier:   notifier,
	})

	workerCtx, cancelWorker := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := w.Run(workerCtx); err != nil {
			slog.Error("agent worker exited with error", "cmd", cmdName,
				"operation", "taskapi.agent_worker.exit_err", "err", err)
		}
	}()

	slog.Info("agent worker config", "cmd", cmdName, "operation", "taskapi.agent_worker",
		"enabled", true, "runner", adapter.Name(), "cursor_bin", cursorBin,
		"cursor_version", cursorVersion, "run_timeout_sec", int(runTimeout/time.Second),
		"working_dir", workingDir)

	return &agentWorkerHandle{
		worker:       w,
		cancelWorker: cancelWorker,
		waitDone:     done,
		runTimeout:   runTimeout,
	}
}

// assertWorkingDirExists is the fail-fast guard for
// T2A_AGENT_WORKER_WORKING_DIR. Returns an error when the path is
// missing or not a directory; the caller logs+exits.
func assertWorkingDirExists(dir string) error {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.assertWorkingDirExists",
		"dir", dir)
	if dir == "" {
		return errors.New("working directory is empty")
	}
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("stat %q: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path %q is not a directory", dir)
	}
	return nil
}

// cycleChangeSSEAdapter implements worker.CycleChangeNotifier on top
// of the existing handler.SSEHub. The TaskCycleChanged event type and
// the SPA cache invalidation hook were added in
// EXECUTION-CYCLES-PLAN.md Stage 5/7; the Stage 4 worker is the first
// server-side publisher.
type cycleChangeSSEAdapter struct {
	hub *handler.SSEHub
}

func newCycleChangeSSEAdapter(hub *handler.SSEHub) *cycleChangeSSEAdapter {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.newCycleChangeSSEAdapter")
	return &cycleChangeSSEAdapter{hub: hub}
}

// PublishCycleChange satisfies worker.CycleChangeNotifier. Nil hub or
// blank ids are no-ops so the adapter is safe to wire even before the
// SSE listener is fully attached.
func (a *cycleChangeSSEAdapter) PublishCycleChange(taskID, cycleID string) {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.cycleChangeSSEAdapter.PublishCycleChange",
		"task_id", taskID, "cycle_id", cycleID)
	if a == nil || a.hub == nil || taskID == "" {
		return
	}
	a.hub.Publish(handler.TaskChangeEvent{
		Type:    handler.TaskCycleChanged,
		ID:      taskID,
		CycleID: cycleID,
	})
}

func mountTaskAPIMux(ctx context.Context, api http.Handler, hub *handler.SSEHub, taskStore *store.Store, agentQueue *agents.MemoryQueue) *http.ServeMux {
	taskapi.RegisterDefaultPrometheusCollectors()
	taskapi.RegisterBuildInfoGauge()
	taskapi.RegisterAgentQueueMetrics(agentQueue)
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", handler.WrapPrometheusHandler(promhttp.Handler()))
	if devsim.Enabled() {
		maybeRunSSEDevTicker(ctx, taskStore, hub)
	} else {
		slog.Info("sse dev config", "cmd", cmdName, "operation", "taskapi.sse_dev", "enabled", false)
	}
	mux.Handle("/", api)
	return mux
}

func maybeRunSSEDevTicker(ctx context.Context, taskStore *store.Store, hub *handler.SSEHub) {
	d := taskapiconfig.SSETestTickerInterval()
	if d < time.Second {
		slog.Info("sse dev env on, ticker off", "cmd", cmdName, "operation", "taskapi.sse_dev",
			"interval", d.String(), "hint", "set T2A_SSE_TEST_INTERVAL to 1s or more to run the ticker")
		return
	}
	slog.Info("sse dev ticker enabled", "cmd", cmdName, "operation", "taskapi.sse_dev", "interval", d.String())
	opts := devsim.LoadOptions()
	devsim.RunTicker(ctx, taskStore, d, opts, func(kind devsim.ChangeKind, id string) {
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
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.openTaskAPILogging")
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
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.deferCloseTaskAPILogFile")
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
	cancel, q, aw := startReadyTaskAgents(ctx, taskStore, hub)
	return &taskAPIApp{taskStore: taskStore, hub: hub, rep: rep, agentQueue: q, agentWorker: aw}, cancel, nil
}

func runTaskAPIHTTPServer(ctx context.Context, port, host string, app *taskAPIApp) (shutdownViaSignal bool, err error) {
	api := taskapi.NewHTTPHandler(app.taskStore, app.hub, app.rep)
	mux := mountTaskAPIMux(ctx, api, app.hub, app.taskStore, app.agentQueue)

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

	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()

	app, stopAgents, err := buildTaskAPIApp(appCtx, db)
	if err != nil {
		slog.Error("startup failed", "cmd", cmdName, "operation", "taskapi.repo_root", "err", err)
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
		_, _ = closeSQLDBOrLog(db)
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
