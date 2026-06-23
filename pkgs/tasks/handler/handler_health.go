package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

func health(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.health")
	const op = "health"
	r = calltrace.WithRequestRoot(r, op)
	debugHTTPRequest(r, op)
	writeJSON(w, r, op, http.StatusOK, map[string]string{
		"status":  "ok",
		"version": ServerVersion(),
	})
}

func healthLive(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.health.live")
	const op = "health.live"
	r = calltrace.WithRequestRoot(r, op)
	debugHTTPRequest(r, op)
	writeJSON(w, r, op, http.StatusOK, map[string]string{
		"status":  "ok",
		"version": ServerVersion(),
	})
}

func (h *Handler) healthReady(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.health.ready")
	const op = "health.ready"
	r = calltrace.WithRequestRoot(r, op)
	debugHTTPRequest(r, op)
	ctx, cancel := context.WithTimeout(r.Context(), store.DefaultReadyTimeout)
	defer cancel()

	checks := map[string]string{}

	if err := h.store.Ready(ctx); err != nil {
		slog.Warn("readiness check failed", "cmd", calltrace.LogCmd, "operation", op, "check", "database", "err", err,
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

	if !h.gitAvailable {
		slog.Warn("readiness check failed", "cmd", calltrace.LogCmd, "operation", op, "check", "git_available")
		checks["git_available"] = "fail"
		writeJSON(w, r, op, http.StatusServiceUnavailable, map[string]any{
			"status":  "degraded",
			"checks":  checks,
			"version": ServerVersion(),
		})
		return
	}
	checks["git_available"] = "ok"

	repoCount, err := h.store.CountGitRepositories(ctx)
	if err != nil {
		slog.Warn("readiness check failed", "cmd", calltrace.LogCmd, "operation", op, "check", "registered_repositories", "err", err)
		checks["registered_repositories"] = "fail"
		writeJSON(w, r, op, http.StatusServiceUnavailable, map[string]any{
			"status":  "degraded",
			"checks":  checks,
			"version": ServerVersion(),
		})
		return
	}
	if repoCount == 0 {
		slog.Warn("readiness advisory", "cmd", calltrace.LogCmd, "operation", op, "check", "registered_repositories", "count", 0)
		checks["registered_repositories"] = "warn"
	} else {
		checks["registered_repositories"] = "ok"
	}

	writeJSON(w, r, op, http.StatusOK, map[string]any{
		"status":  "ok",
		"checks":  checks,
		"version": ServerVersion(),
	})
}
