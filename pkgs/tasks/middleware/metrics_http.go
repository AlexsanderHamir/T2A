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
