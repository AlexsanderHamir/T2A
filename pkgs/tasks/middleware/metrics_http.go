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
)

// RecordSSESubscriberGauge sets the process-wide SSE subscriber gauge (one hub per taskapi).
func RecordSSESubscriberGauge(n int) {
	slog.Debug("trace", "cmd", logctx.TraceCmd, "operation", "middleware.RecordSSESubscriberGauge", "n", n)
	taskapiSSESubscribers.Set(float64(n))
}

// SSESubscribersGauge exposes the Prometheus gauge updated by RecordSSESubscriberGauge (tests, tooling).
func SSESubscribersGauge() prometheus.Gauge {
	return taskapiSSESubscribers
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
