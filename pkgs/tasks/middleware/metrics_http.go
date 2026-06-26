package middleware

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/logctx"
	"github.com/prometheus/client_golang/prometheus"
)

// RecordSSESubscriberGauge sets the process-wide SSE subscriber gauge (one hub per taskapi).
func RecordSSESubscriberGauge(n int) {
	slog.Debug("trace", "cmd", logctx.TraceCmd, "operation", "middleware.RecordSSESubscriberGauge", "n", n)
	taskapiSSESubscribers.Set(float64(n))
}

// SSESubscribersGauge exposes the Prometheus gauge updated by RecordSSESubscriberGauge (tests, tooling).
func SSESubscribersGauge() prometheus.Gauge {
	slog.Debug("trace", "cmd", logctx.TraceCmd, "operation", "middleware.SSESubscribersGauge")
	return taskapiSSESubscribers
}

// RecordSSEDroppedFrames bumps the dropped-frames counter by n. Called by the
// SSE hub Publish path each time a subscriber's bounded channel was full at
// fanout time. Pass the count for the entire fanout (not per-subscriber) so
// the helper stays cheap on the hot path.
func RecordSSEDroppedFrames(n int) {
	slog.Debug("trace", "cmd", logctx.TraceCmd, "operation", "middleware.RecordSSEDroppedFrames", "n", n)
	if n <= 0 {
		return
	}
	taskapiSSEDroppedFramesTotal.Add(float64(n))
}

// SSEDroppedFramesCounter exposes the Prometheus counter updated by
// RecordSSEDroppedFrames. Tests use it to assert hub fanout-pressure behavior
// without hitting the /metrics endpoint.
func SSEDroppedFramesCounter() prometheus.Counter {
	slog.Debug("trace", "cmd", logctx.TraceCmd, "operation", "middleware.SSEDroppedFramesCounter")
	return taskapiSSEDroppedFramesTotal
}

// RecordSSECoalesced bumps the coalesced-frames counter by n. Called by
// the SSE hub Publish path each time a duplicate {type,id} frame is
// dropped inside the coalesceWindow.
func RecordSSECoalesced(n int) {
	slog.Debug("trace", "cmd", logctx.TraceCmd, "operation", "middleware.RecordSSECoalesced", "n", n)
	if n <= 0 {
		return
	}
	taskapiSSECoalescedTotal.Add(float64(n))
}

// SSECoalescedCounter exposes the Prometheus counter updated by
// RecordSSECoalesced. Tests use it to pin the hub's coalescing
// behavior without hitting the /metrics endpoint.
func SSECoalescedCounter() prometheus.Counter {
	slog.Debug("trace", "cmd", logctx.TraceCmd, "operation", "middleware.SSECoalescedCounter")
	return taskapiSSECoalescedTotal
}

// RecordSSEResyncEmitted bumps the resync-directive counter by n.
// Called by the SSE handler when (a) a reconnecting client requested a
// Last-Event-ID older than the oldest retained ring entry, or (b) a
// slow-consumer subscriber was evicted and the writer goroutine sent
// the resync directive on the way out.
func RecordSSEResyncEmitted(n int) {
	slog.Debug("trace", "cmd", logctx.TraceCmd, "operation", "middleware.RecordSSEResyncEmitted", "n", n)
	if n <= 0 {
		return
	}
	taskapiSSEResyncEmittedTotal.Add(float64(n))
}

// SSEResyncEmittedCounter exposes the Prometheus counter updated by
// RecordSSEResyncEmitted. Tests use it to pin the hub's gap-handling
// and slow-consumer eviction paths without hitting /metrics.
func SSEResyncEmittedCounter() prometheus.Counter {
	slog.Debug("trace", "cmd", logctx.TraceCmd, "operation", "middleware.SSEResyncEmittedCounter")
	return taskapiSSEResyncEmittedTotal
}

// RecordSSEPublish bumps the publish counter by 1. Call once per
// SSE hub Publish() invocation regardless of how many subscribers
// fanout to — this is the denominator for slo_sse_resync_rate.
func RecordSSEPublish() {
	slog.Debug("trace", "cmd", logctx.TraceCmd, "operation", "middleware.RecordSSEPublish")
	taskapiSSEPublishTotal.Inc()
}

// SSEPublishCounter exposes the publish counter for tests that need
// to assert publish volume without hitting /metrics.
func SSEPublishCounter() prometheus.Counter {
	slog.Debug("trace", "cmd", logctx.TraceCmd, "operation", "middleware.SSEPublishCounter")
	return taskapiSSEPublishTotal
}

// RecordSSESubscriberLag observes the age (in seconds) of the oldest
// pending frame in a subscriber's bounded channel at Publish time.
// Non-positive values are clamped to zero so a clock-skew or monotonic
// mis-order never produces a bogus negative sample.
func RecordSSESubscriberLag(seconds float64) {
	slog.Debug("trace", "cmd", logctx.TraceCmd, "operation", "middleware.RecordSSESubscriberLag", "seconds", seconds)
	if seconds < 0 {
		seconds = 0
	}
	taskapiSSESubscriberLagSeconds.Observe(seconds)
}

// SSESubscriberLagHistogram exposes the lag histogram for tests.
func SSESubscriberLagHistogram() prometheus.Histogram {
	slog.Debug("trace", "cmd", logctx.TraceCmd, "operation", "middleware.SSESubscriberLagHistogram")
	return taskapiSSESubscriberLagSeconds
}

// RecordSSESubscriberEvictions bumps the slow-consumer eviction counter
// by n. Each eviction is also paired with a resync directive on the
// wire (recorded via RecordSSEResyncEmitted).
func RecordSSESubscriberEvictions(n int) {
	slog.Debug("trace", "cmd", logctx.TraceCmd, "operation", "middleware.RecordSSESubscriberEvictions", "n", n)
	if n <= 0 {
		return
	}
	taskapiSSESubscriberEvictionsTotal.Add(float64(n))
}

// SSESubscriberEvictionsCounter exposes the Prometheus counter updated by
// RecordSSESubscriberEvictions. Tests use it to assert that overflow
// causes eviction (loss-free) instead of silent frame drops.
func SSESubscriberEvictionsCounter() prometheus.Counter {
	slog.Debug("trace", "cmd", logctx.TraceCmd, "operation", "middleware.SSESubscriberEvictionsCounter")
	return taskapiSSESubscriberEvictionsTotal
}

// RUMEventsAcceptedCounter exposes the accepted-events counter for tests.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func RUMEventsAcceptedCounter() prometheus.Counter {
	return taskapiRUMEventsAcceptedTotal
}

// RUMEventsDroppedCounter exposes the dropped-events counter for tests.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func RUMEventsDroppedCounter() prometheus.Counter {
	return taskapiRUMEventsDroppedTotal
}

// RecordRUMAccepted bumps the accepted-events counter by n. Called once
// per /v1/rum batch with the number of events the handler successfully
// folded into metrics.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func RecordRUMAccepted(n int) {
	if n <= 0 {
		return
	}
	taskapiRUMEventsAcceptedTotal.Add(float64(n))
}

// RecordRUMDropped bumps the dropped-events counter by n. Called once
// per /v1/rum batch with the number of events the handler rejected
// (unknown type, invalid duration, unknown web-vital name, …). Useful
// alongside RecordRUMAccepted to spot SPA regressions that emit
// malformed payloads.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func RecordRUMDropped(n int) {
	if n <= 0 {
		return
	}
	taskapiRUMEventsDroppedTotal.Add(float64(n))
}

// RecordRUMMutationStarted bumps the mutation-started counter for the
// given kind. Denominator for both slo_optimistic_rollback_rate and
// slo_mutation_error_rate.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func RecordRUMMutationStarted(kind string) {
	taskapiRUMMutationStartedTotal.WithLabelValues(kind).Inc()
}

// RecordRUMMutationOptimisticApplied observes a click→optimistic-render
// latency for the given kind. Powers the optimistic side of
// slo_click_to_confirmed_p95_ms.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func RecordRUMMutationOptimisticApplied(kind string, seconds float64) {
	taskapiRUMMutationOptimisticAppliedSeconds.WithLabelValues(kind).Observe(seconds)
}

// RecordRUMMutationSettled observes a click→server-confirmed latency for
// the given kind, labelled by HTTP status bucket ("2xx"|"4xx"|"5xx"|
// "network"). The non-2xx series feed slo_mutation_error_rate.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func RecordRUMMutationSettled(kind, status string, seconds float64) {
	taskapiRUMMutationSettledSeconds.WithLabelValues(kind, status).Observe(seconds)
}

// RecordRUMMutationRolledBack records a rollback for the given kind and,
// if seconds > 0, also observes the click→rollback latency. Numerator
// for slo_optimistic_rollback_rate.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func RecordRUMMutationRolledBack(kind string, seconds float64) {
	taskapiRUMMutationRolledBackTotal.WithLabelValues(kind).Inc()
	if seconds > 0 {
		taskapiRUMMutationRollbackSeconds.WithLabelValues(kind).Observe(seconds)
	}
}

// RecordRUMSSEReconnected counts EventSource reconnects reported by the
// SPA. If gapSeconds > 0 the disconnect→reconnect gap is also observed
// in the gap histogram.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func RecordRUMSSEReconnected(gapSeconds float64) {
	taskapiRUMSSEReconnectedTotal.Inc()
	if gapSeconds > 0 {
		taskapiRUMSSEReconnectGapSeconds.Observe(gapSeconds)
	}
}

// RecordRUMSSEResyncReceived counts resync directives the SPA acted on.
// Dashboards pair this with sse_resync_emitted_total to verify
// "what the server emitted" matches "what the client observed".
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func RecordRUMSSEResyncReceived() {
	taskapiRUMSSEResyncReceivedTotal.Inc()
}

// RecordRUMWebVital observes a single Web Vitals measurement. The name
// label is one of the entries in handler.validWebVitalNames so the
// dashboard knows which units to apply.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func RecordRUMWebVital(name string, value float64) {
	taskapiRUMWebVitals.WithLabelValues(name).Observe(value)
}

func omitHTTPMetrics(r *http.Request) bool {
	slog.Debug("trace", "cmd", logctx.TraceCmd, "operation", "middleware.omitHTTPMetrics")
	if r.Method != http.MethodGet {
		return false
	}
	switch r.URL.Path {
	case "/health", "/health/live", "/health/ready":
		return true
	default:
		return false
	}
}

// WithHTTPMetrics records Prometheus counters and latency histograms for each request.
// GET /health, /health/live, and /health/ready are skipped to keep probe traffic out of SLI series.
// The route label is r.Pattern when set (Go 1.22+ ServeMux), else "other" to limit cardinality.
func WithHTTPMetrics(h http.Handler) http.Handler {
	slog.Debug("trace", "cmd", logctx.TraceCmd, "operation", "middleware.WithHTTPMetrics")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if omitHTTPMetrics(r) {
			h.ServeHTTP(w, r)
			return
		}
		taskapiHTTPInFlight.Inc()
		defer taskapiHTTPInFlight.Dec()
		mw := &metricsHTTPResponseWriter{ResponseWriter: w}
		start := time.Now()
		h.ServeHTTP(mw, r)
		dur := time.Since(start).Seconds()
		route := r.Pattern
		if route == "" {
			route = "other"
		}
		code := mw.statusCode()
		taskapiHTTPRequestsTotal.WithLabelValues(r.Method, route, strconv.Itoa(code)).Inc()
		taskapiHTTPRequestDuration.WithLabelValues(r.Method, route).Observe(dur)
	})
}

type metricsHTTPResponseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (mw *metricsHTTPResponseWriter) WriteHeader(code int) {
	if !mw.wroteHeader {
		mw.status = code
		mw.wroteHeader = true
	}
	mw.ResponseWriter.WriteHeader(code)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (mw *metricsHTTPResponseWriter) Write(b []byte) (int, error) {
	if !mw.wroteHeader {
		mw.status = http.StatusOK
		mw.wroteHeader = true
	}
	return mw.ResponseWriter.Write(b)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (mw *metricsHTTPResponseWriter) statusCode() int {
	if mw.status == 0 {
		return http.StatusOK
	}
	return mw.status
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (mw *metricsHTTPResponseWriter) Flush() {
	if f, ok := mw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

var _ http.Flusher = (*metricsHTTPResponseWriter)(nil)
