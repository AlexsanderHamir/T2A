package middleware

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// httpRequestDurationSecondsBuckets are upper bounds (seconds) for
// taskapi_http_request_duration_seconds. They prioritize resolution between
// ~10ms and 1s for REST SLI work (p50/p95), with a tail to 10s for slow store
// paths. Documented in docs/api.md and docs/architecture.md.
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
	// docs/architecture.md the resync-rate SLI is
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
	// `slo_click_to_confirmed_p95_ms ≤ 100` documented in docs/architecture.md.
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
