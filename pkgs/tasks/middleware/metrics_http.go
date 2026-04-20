package middleware

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/logctx"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// httpRequestDurationSecondsBuckets are upper bounds (seconds) for
// taskapi_http_request_duration_seconds. They prioritize resolution between
// ~10ms and 1s for REST SLI work (p50/p95), with a tail to 10s for slow store
// paths. Documented in docs/API-HTTP.md and docs/OBSERVABILITY.md.
var httpRequestDurationSecondsBuckets = []float64{
	0.01, 0.025, 0.05, 0.1, 0.15, 0.25, 0.35, 0.5, 0.75, 1,
	1.5, 2.5, 5, 10,
}

var (
	taskapiHTTPInFlight = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "taskapi",
		Name:      "http_in_flight",
		Help:      "Number of HTTP requests currently being served (excludes health probes).",
	})
	taskapiHTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "taskapi",
			Name:      "http_requests_total",
			Help:      "Total HTTP requests processed (excludes health probes).",
		},
		[]string{"method", "route", "code"},
	)
	taskapiHTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "taskapi",
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds (excludes health probes).",
			Buckets:   httpRequestDurationSecondsBuckets,
		},
		[]string{"method", "route"},
	)
	taskapiSSESubscribers = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "taskapi",
		Name:      "sse_subscribers",
		Help:      "Number of connected GET /events SSE clients in this process.",
	})
	// taskapiSSEDroppedFramesTotal counts how many fanout-frame writes the
	// SSE hub had to drop because a subscriber's bounded channel was full
	// at the time of Publish (slow consumer / blocked HTTP write). This is
	// the canonical observable for fanout pressure under load — the hub
	// drops silently rather than blocking the publisher, so without this
	// counter a stuck client would only surface as missing UI updates with
	// no metric trail. Counter is process-wide (no subscriber label) to
	// keep cardinality bounded; correlate with taskapi_sse_subscribers
	// gauge to spot per-fanout drop rates.
	taskapiSSEDroppedFramesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "taskapi",
		Name:      "sse_dropped_frames_total",
		Help:      "Total SSE fanout frames dropped because a subscriber channel was full (slow consumer indicator).",
	})
	// taskapiSSECoalescedTotal counts duplicate {type,id} frames the
	// hub collapsed inside its coalesceWindow. Cycle frames carry a
	// distinct cycle id and are intentionally never coalesced; settings
	// and task_updated bursts are the primary contributors. A non-zero
	// rate is healthy (the coalescer is doing its job); a runaway rate
	// (>1 frame/s sustained) suggests an upstream caller is spamming
	// Publish in a tight loop.
	taskapiSSECoalescedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "taskapi",
		Name:      "sse_coalesced_total",
		Help:      "Duplicate SSE frames collapsed by the hub coalescer.",
	})
	// taskapiSSEResyncEmittedTotal counts `{"type":"resync"}` directives
	// the hub sent to a client — either because the client's
	// Last-Event-ID was older than the oldest retained ring entry, or
	// because the subscriber was evicted as a slow consumer. The SLO
	// `slo_sse_resync_rate ≤ 0.5%` is computed as
	// `sse_resync_emitted_total / sse_publish_total`; a sustained
	// non-trivial rate means the ring buffer is too small or
	// downstream clients are too slow.
	taskapiSSEResyncEmittedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "taskapi",
		Name:      "sse_resync_emitted_total",
		Help:      "Total resync directives emitted (Last-Event-ID gap or slow-consumer eviction).",
	})
	// taskapiSSESubscriberEvictionsTotal counts how many subscribers
	// were forcibly disconnected because their bounded channel filled
	// up. Each eviction also contributes one resync directive (above)
	// and one dropped frame. Tracked separately so an operator can
	// distinguish "one slow client got evicted once" (subscriber
	// eviction = 1) from "one slow client kept dropping frames before
	// finally being evicted" (dropped > eviction).
	taskapiSSESubscriberEvictionsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "taskapi",
		Name:      "sse_subscriber_evictions_total",
		Help:      "Total slow-consumer SSE subscriber evictions (paired with a resync directive on the wire).",
	})
	// taskapiSSEPublishTotal counts every successful Publish() call on the
	// SSE hub — the denominator for the `slo_sse_resync_rate` SLO. Per
	// docs/SLOs.md the resync-rate SLI is
	// `sse_resync_emitted_total / sse_publish_total`, so this counter
	// MUST be incremented once per Publish regardless of how many
	// subscribers fanout to.
	taskapiSSEPublishTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "taskapi",
		Name:      "sse_publish_total",
		Help:      "Total SSE events published to the hub (denominator for slo_sse_resync_rate).",
	})
	// taskapiSSESubscriberLagSeconds observes the age (in seconds) of
	// the oldest pending frame in each subscriber's bounded channel at
	// Publish time. Drives the `slo_sse_subscriber_lag_p99_seconds ≤ 2`
	// SLO. Histogram (not Gauge) so the p99 is first-class Prometheus.
	taskapiSSESubscriberLagSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "taskapi",
		Name:      "sse_subscriber_lag_seconds",
		Help:      "Age of the oldest pending frame in each subscriber's bounded channel at Publish time.",
		// 10ms..30s — the SLO target is 2s at p99; generous tail so
		// pathological backpressure doesn't saturate the last bucket.
		Buckets: []float64{0.010, 0.025, 0.050, 0.100, 0.250, 0.500, 1.0, 2.0, 5.0, 10.0, 30.0},
	})

	// rumLatencyBuckets are tuned for the click-to-confirmed SLO
	// `slo_click_to_confirmed_p95_ms ≤ 100` documented in docs/SLOs.md.
	// Concentrated below 250 ms so p95 has resolution where the SLI
	// lives, with a tail to 30s for pathological optimistic-applied
	// renders or settled latencies behind a slow store.
	rumLatencyBuckets = []float64{
		0.005, 0.010, 0.025, 0.050, 0.075,
		0.100, 0.150, 0.200, 0.300, 0.500,
		0.750, 1.0, 2.0, 5.0, 10.0, 30.0,
	}

	taskapiRUMEventsAcceptedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "taskapi",
		Name:      "rum_events_accepted_total",
		Help:      "RUM events accepted from POST /v1/rum and folded into metrics.",
	})
	taskapiRUMEventsDroppedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "taskapi",
		Name:      "rum_events_dropped_total",
		Help:      "RUM events dropped at POST /v1/rum (unknown type / invalid duration / unknown web-vital name).",
	})
	taskapiRUMMutationStartedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "taskapi",
		Name:      "rum_mutation_started_total",
		Help:      "Total mutations the SPA initiated, labelled by kind. Denominator for slo_optimistic_rollback_rate and slo_mutation_error_rate.",
	}, []string{"kind"})
	taskapiRUMMutationOptimisticAppliedSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "taskapi",
		Name:      "rum_mutation_optimistic_applied_seconds",
		Help:      "Click → optimistic-render latency. Powers slo_click_to_confirmed_p95_ms when the optimistic render fired before the server response.",
		Buckets:   rumLatencyBuckets,
	}, []string{"kind"})
	taskapiRUMMutationSettledSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "taskapi",
		Name:      "rum_mutation_settled_seconds",
		Help:      "Click → server-confirmed latency. Powers slo_click_to_confirmed_p95_ms when no optimistic render fired (or as the SLO numerator for non-optimistic kinds).",
		Buckets:   rumLatencyBuckets,
	}, []string{"kind", "status"})
	taskapiRUMMutationRolledBackTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "taskapi",
		Name:      "rum_mutation_rolled_back_total",
		Help:      "Total mutations whose optimistic apply was reverted after a server error. Numerator for slo_optimistic_rollback_rate.",
	}, []string{"kind"})
	taskapiRUMMutationRollbackSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "taskapi",
		Name:      "rum_mutation_rollback_seconds",
		Help:      "Click → rollback latency for mutations whose optimistic apply was reverted (server error path).",
		Buckets:   rumLatencyBuckets,
	}, []string{"kind"})
	taskapiRUMSSEReconnectedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "taskapi",
		Name:      "rum_sse_reconnected_total",
		Help:      "EventSource reconnect events reported by the SPA (browser auto-retry or post-resync explicit reconnect).",
	})
	taskapiRUMSSEReconnectGapSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "taskapi",
		Name:      "rum_sse_reconnect_gap_seconds",
		Help:      "Gap between EventSource disconnect and successful reconnect.",
		Buckets:   rumLatencyBuckets,
	})
	taskapiRUMSSEResyncReceivedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "taskapi",
		Name:      "rum_sse_resync_received_total",
		Help:      "Resync directives the SPA received (paired with sse_resync_emitted_total on the server side).",
	})
	taskapiRUMWebVitals = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "taskapi",
		Name:      "rum_web_vitals",
		Help:      "Browser Web Vitals reported by the SPA (LCP/INP/CLS/FCP/FID/TTFB). LCP/FCP/TTFB/INP/FID values are milliseconds; CLS is unitless layout-shift score. Pick the right unit per metric in the dashboard.",
		Buckets:   []float64{0.05, 0.1, 0.25, 0.5, 0.75, 1, 1.5, 2, 2.5, 3, 4, 5, 7.5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000},
	}, []string{"name"})
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
func RUMEventsAcceptedCounter() prometheus.Counter {
	return taskapiRUMEventsAcceptedTotal
}

// RUMEventsDroppedCounter exposes the dropped-events counter for tests.
func RUMEventsDroppedCounter() prometheus.Counter {
	return taskapiRUMEventsDroppedTotal
}

// RecordRUMAccepted bumps the accepted-events counter by n. Called once
// per /v1/rum batch with the number of events the handler successfully
// folded into metrics.
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
func RecordRUMDropped(n int) {
	if n <= 0 {
		return
	}
	taskapiRUMEventsDroppedTotal.Add(float64(n))
}

// RecordRUMMutationStarted bumps the mutation-started counter for the
// given kind. Denominator for both slo_optimistic_rollback_rate and
// slo_mutation_error_rate.
func RecordRUMMutationStarted(kind string) {
	taskapiRUMMutationStartedTotal.WithLabelValues(kind).Inc()
}

// RecordRUMMutationOptimisticApplied observes a click→optimistic-render
// latency for the given kind. Powers the optimistic side of
// slo_click_to_confirmed_p95_ms.
func RecordRUMMutationOptimisticApplied(kind string, seconds float64) {
	taskapiRUMMutationOptimisticAppliedSeconds.WithLabelValues(kind).Observe(seconds)
}

// RecordRUMMutationSettled observes a click→server-confirmed latency for
// the given kind, labelled by HTTP status bucket ("2xx"|"4xx"|"5xx"|
// "network"). The non-2xx series feed slo_mutation_error_rate.
func RecordRUMMutationSettled(kind, status string, seconds float64) {
	taskapiRUMMutationSettledSeconds.WithLabelValues(kind, status).Observe(seconds)
}

// RecordRUMMutationRolledBack records a rollback for the given kind and,
// if seconds > 0, also observes the click→rollback latency. Numerator
// for slo_optimistic_rollback_rate.
func RecordRUMMutationRolledBack(kind string, seconds float64) {
	taskapiRUMMutationRolledBackTotal.WithLabelValues(kind).Inc()
	if seconds > 0 {
		taskapiRUMMutationRollbackSeconds.WithLabelValues(kind).Observe(seconds)
	}
}

// RecordRUMSSEReconnected counts EventSource reconnects reported by the
// SPA. If gapSeconds > 0 the disconnect→reconnect gap is also observed
// in the gap histogram.
func RecordRUMSSEReconnected(gapSeconds float64) {
	taskapiRUMSSEReconnectedTotal.Inc()
	if gapSeconds > 0 {
		taskapiRUMSSEReconnectGapSeconds.Observe(gapSeconds)
	}
}

// RecordRUMSSEResyncReceived counts resync directives the SPA acted on.
// Dashboards pair this with sse_resync_emitted_total to verify
// "what the server emitted" matches "what the client observed".
func RecordRUMSSEResyncReceived() {
	taskapiRUMSSEResyncReceivedTotal.Inc()
}

// RecordRUMWebVital observes a single Web Vitals measurement. The name
// label is one of the entries in handler.validWebVitalNames so the
// dashboard knows which units to apply.
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

func (mw *metricsHTTPResponseWriter) WriteHeader(code int) {
	if !mw.wroteHeader {
		mw.status = code
		mw.wroteHeader = true
	}
	mw.ResponseWriter.WriteHeader(code)
}

func (mw *metricsHTTPResponseWriter) Write(b []byte) (int, error) {
	if !mw.wroteHeader {
		mw.status = http.StatusOK
		mw.wroteHeader = true
	}
	return mw.ResponseWriter.Write(b)
}

func (mw *metricsHTTPResponseWriter) statusCode() int {
	if mw.status == 0 {
		return http.StatusOK
	}
	return mw.status
}

func (mw *metricsHTTPResponseWriter) Flush() {
	if f, ok := mw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

var _ http.Flusher = (*metricsHTTPResponseWriter)(nil)
