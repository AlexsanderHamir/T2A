package handler

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/AlexsanderHamir/T2A/internal/systemhealth"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/calltrace"
	"github.com/prometheus/client_golang/prometheus"
)

// systemHealthSnapshotter wraps the prometheus registry behind a
// closure so the handler does not need to take a hard dependency on
// the prometheus dto types. The production default scrapes
// prometheus.DefaultGatherer at the supplied wall clock; tests
// override via WithSystemHealthGatherer.
type systemHealthSnapshotter func(now time.Time) systemhealth.Snapshot

func defaultSystemHealthSnapshotter() systemHealthSnapshotter {
	return func(now time.Time) systemhealth.Snapshot {
		return systemhealth.Read(prometheus.DefaultGatherer, now)
	}
}

// systemHealth serves GET /system/health: a stable JSON envelope
// summarising build info, uptime, HTTP/SSE/DB/agent counters for the
// operator UI. Distinct from /health/live and /health/ready (those
// stay tiny for orchestrator probes); this endpoint is meant for the
// /observability page and short-lived operator scripts.
//
// The response is always 200 with a fully-shaped envelope. Even when
// the underlying gather fails the snapshot is zero-valued but
// well-formed so the SPA does not need to branch on missing keys.
func (h *Handler) systemHealth(w http.ResponseWriter, r *http.Request) {
	const op = "system.health"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.systemHealth")
	r = calltrace.WithRequestRoot(r, op)
	debugHTTPRequest(r, op)
	snap := h.snapshotSystemHealth(time.Now())
	// agent.paused is sourced from app_settings, not Prometheus —
	// the supervisor goes idle on a flip and counters would lag the
	// flag. We tolerate a transient store error by leaving the
	// field at its zero value (false) so the operator UI does not
	// 500 just because the singleton row is briefly unreadable.
	if h.store != nil {
		cfg, err := h.store.GetSettings(r.Context())
		if err != nil {
			slog.Warn("system health: read app_settings for paused flag failed",
				"cmd", calltrace.LogCmd, "operation", op, "err", err)
		} else {
			snap.Agent.Paused = cfg.AgentPaused
		}
	}
	writeJSON(w, r, op, http.StatusOK, snap)
}

func (h *Handler) snapshotSystemHealth(now time.Time) systemhealth.Snapshot {
	if h.systemHealthFn != nil {
		return h.systemHealthFn(now)
	}
	return defaultSystemHealthSnapshotter()(now)
}

// WithSystemHealthGatherer overrides the Prometheus gatherer used by
// GET /system/health. Production wiring uses the default registry;
// tests pass a NewPedanticRegistry so they can populate counters
// without leaking into the global registry.
func WithSystemHealthGatherer(g systemhealth.Gather) HandlerOption {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.WithSystemHealthGatherer")
	return func(h *Handler) {
		if g == nil {
			return
		}
		h.systemHealthFn = func(now time.Time) systemhealth.Snapshot {
			return systemhealth.Read(g, now)
		}
	}
}
