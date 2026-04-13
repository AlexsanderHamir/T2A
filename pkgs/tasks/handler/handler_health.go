package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

func health(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.health")
	const op = "health"
	r = withCallRoot(r, op)
	debugHTTPRequest(r, op)
	writeJSON(w, r, op, http.StatusOK, map[string]string{
		"status":  "ok",
		"version": ServerVersion(),
	})
}

func healthLive(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.health.live")
	const op = "health.live"
	r = withCallRoot(r, op)
	debugHTTPRequest(r, op)
	writeJSON(w, r, op, http.StatusOK, map[string]string{
		"status":  "ok",
		"version": ServerVersion(),
	})
}

func (h *Handler) healthReady(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.health.ready")
	const op = "health.ready"
	r = withCallRoot(r, op)
	debugHTTPRequest(r, op)
	ctx, cancel := context.WithTimeout(r.Context(), store.DefaultReadyTimeout)
	defer cancel()

	checks := map[string]string{}

	if err := h.store.Ready(ctx); err != nil {
		slog.Warn("readiness check failed", "cmd", httpLogCmd, "operation", op, "check", "database", "err", err,
			"deadline_exceeded", errors.Is(err, context.DeadlineExceeded),
			"timeout_sec", int(store.DefaultReadyTimeout/time.Second))
		checks["database"] = "fail"
		writeJSON(w, r, op, http.StatusServiceUnavailable, map[string]any{
			"status":  "degraded",
			"checks":  checks,
			"version": ServerVersion(),
		})
		return
	}
	checks["database"] = "ok"

	if h.repo != nil {
		if err := h.repo.Ready(); err != nil {
			slog.Warn("readiness check failed", "cmd", httpLogCmd, "operation", op, "check", "workspace_repo", "err", err)
			checks["workspace_repo"] = "fail"
			writeJSON(w, r, op, http.StatusServiceUnavailable, map[string]any{
				"status":  "degraded",
				"checks":  checks,
				"version": ServerVersion(),
			})
			return
		}
		checks["workspace_repo"] = "ok"
	}

	writeJSON(w, r, op, http.StatusOK, map[string]any{
		"status":  "ok",
		"checks":  checks,
		"version": ServerVersion(),
	})
}
