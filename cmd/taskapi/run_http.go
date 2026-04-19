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
	"syscall"
	"time"

	"github.com/AlexsanderHamir/T2A/internal/taskapi"
	"github.com/AlexsanderHamir/T2A/internal/taskapiconfig"
	"github.com/AlexsanderHamir/T2A/pkgs/agents"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/devsim"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// run_http.go owns the HTTP server lifecycle for taskapi: mux mount
// (incl. /metrics + optional SSE dev ticker), listener bind, and the
// graceful Serve+Shutdown loop. Split off run_helpers.go per
// backend-engineering-bar.mdc §2 / §16.

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
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
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
		if err := <-serveDone; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return shutdownViaSignal, err
		}
	}
	return shutdownViaSignal, nil
}

func runTaskAPIHTTPServer(ctx context.Context, port, host string, app *taskAPIApp) (shutdownViaSignal bool, err error) {
	api := taskapi.NewHTTPHandler(app.taskStore, app.hub, app.rep, app.agentWorker)
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
