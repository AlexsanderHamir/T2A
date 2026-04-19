package taskapi

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/worker"
	"github.com/prometheus/client_golang/prometheus"
)

// agentRunDurationBuckets are tuned for one Cursor CLI execute attempt:
// the supervisor honours app_settings.max_run_duration_seconds (0 =
// no limit; operators typically cap at <= 30 minutes via the SPA
// Settings page — see docs/SETTINGS.md), so the buckets need sub-
// second granularity for fast failures (timeout-before-startup,
// immediate non-zero exit) and 1m–30m granularity for normal runs. Mirrors the pattern in pkgs/tasks/store/internal/kernel/metrics.go:
// a fixed []float64 declared next to the registration with a comment
// explaining the choice rather than relying on prometheus.DefBuckets.
var agentRunDurationBuckets = []float64{
	0.5, 1, 2.5, 5, 10, 30, 60, 120, 300, 600, 1200, 1800,
}

var registerAgentWorkerMetrics sync.Once

// workerMetricsAdapter satisfies worker.RunMetrics by fanning out to a
// counter and a histogram. Owned by this package so the registration is
// the single source of truth for label values; the worker package does
// not import prometheus.
type workerMetricsAdapter struct {
	runs     *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

// RecordRun increments t2a_agent_runs_total and observes
// t2a_agent_run_duration_seconds. Label values are bounded: runner is
// the adapter Name() (today only "cursor", "fake" in tests), and
// terminalStatus is one of the three terminal domain.CycleStatus
// values ("succeeded", "failed", "aborted").
func (a *workerMetricsAdapter) RecordRun(runnerName, terminalStatus string, d time.Duration) {
	slog.Debug("trace", "cmd", cmdLog, "operation", "taskapi.workerMetricsAdapter.RecordRun",
		"runner", runnerName, "terminal_status", terminalStatus, "duration_ms", d.Milliseconds())
	if a == nil {
		return
	}
	a.runs.WithLabelValues(runnerName, terminalStatus).Inc()
	a.duration.WithLabelValues(runnerName).Observe(d.Seconds())
}

// registerAgentWorkerMetricsOn registers the counter + histogram on
// reg (tests pass a NewPedanticRegistry to assert on the metric shape
// without leaking globals). Returns the adapter ready for
// worker.Options.Metrics.
func registerAgentWorkerMetricsOn(reg prometheus.Registerer) (*workerMetricsAdapter, error) {
	slog.Debug("trace", "cmd", cmdLog, "operation", "taskapi.registerAgentWorkerMetricsOn")
	runs := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "t2a",
		Name:      "agent_runs_total",
		Help:      "Count of completed agent worker attempts, labelled by runner and terminal cycle status.",
	}, []string{"runner", "terminal_status"})
	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "t2a",
		Name:      "agent_run_duration_seconds",
		Help:      "Wall-clock duration of one agent worker attempt (StartCycle → TerminateCycle), in seconds.",
		Buckets:   agentRunDurationBuckets,
	}, []string{"runner"})
	if err := reg.Register(runs); err != nil {
		return nil, fmt.Errorf("register t2a_agent_runs_total: %w", err)
	}
	if err := reg.Register(duration); err != nil {
		return nil, fmt.Errorf("register t2a_agent_run_duration_seconds: %w", err)
	}
	return &workerMetricsAdapter{runs: runs, duration: duration}, nil
}

// RegisterAgentWorkerMetricsOn is the test-friendly variant of
// RegisterAgentWorkerMetrics: it registers the worker counter +
// histogram on reg (typically a prometheus.NewPedanticRegistry built
// per-test) and returns the adapter as worker.RunMetrics so callers
// can plug it into worker.Options.Metrics without going through the
// global default registry. The returned adapter shares its full shape
// (metric names, labels, buckets) with the production wiring so an
// e2e test cannot drift from prod by silently using a different
// counter name.
//
// Errors propagate verbatim — duplicate registration on the same reg
// surfaces as a prometheus.AlreadyRegisteredError that the test can
// inspect; production callers go through RegisterAgentWorkerMetrics
// which absorbs that case via sync.Once.
func RegisterAgentWorkerMetricsOn(reg prometheus.Registerer) (worker.RunMetrics, error) {
	slog.Debug("trace", "cmd", cmdLog, "operation", "taskapi.RegisterAgentWorkerMetricsOn")
	a, err := registerAgentWorkerMetricsOn(reg)
	if err != nil {
		return nil, err
	}
	return a, nil
}

// RegisterAgentWorkerMetrics registers the worker counter + histogram
// on the default Prometheus registry exactly once and returns an
// adapter that satisfies worker.RunMetrics. Safe to call when the
// agent worker is disabled — the returned adapter is still usable for
// uniform wiring, but no observations land because the worker never
// fires RecordRun.
//
// On duplicate registration (e.g. the function is reachable from a
// re-init in tests) the call is a no-op and returns nil without
// logging at error level so taskapi can keep running.
func RegisterAgentWorkerMetrics() worker.RunMetrics {
	slog.Debug("trace", "cmd", cmdLog, "operation", "taskapi.RegisterAgentWorkerMetrics")
	var adapter *workerMetricsAdapter
	registerAgentWorkerMetrics.Do(func() {
		a, err := registerAgentWorkerMetricsOn(prometheus.DefaultRegisterer)
		if err != nil {
			var dup prometheus.AlreadyRegisteredError
			if errors.As(err, &dup) {
				return
			}
			slog.Warn("prometheus agent worker metrics register failed",
				"cmd", cmdLog, "operation", "taskapi.RegisterAgentWorkerMetrics", "err", err)
			return
		}
		adapter = a
		slog.Info("prometheus agent worker metrics registered",
			"cmd", cmdLog, "operation", "taskapi.RegisterAgentWorkerMetrics")
	})
	if adapter == nil {
		return nil
	}
	return adapter
}
