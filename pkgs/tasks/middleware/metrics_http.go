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
