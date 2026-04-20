package systemhealth

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// TestRead_emptyGatherer pins the "freshly-booted process" invariant:
// every nested map is non-nil and seeded with the documented enum
// keys, so the SPA never has to branch on missing fields.
func TestRead_emptyGatherer(t *testing.T) {
	reg := prometheus.NewPedanticRegistry()
	now := time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC)
	snap := Read(reg, now)

	if snap.Now != now {
		t.Errorf("Now: got %v, want %v", snap.Now, now)
	}
	if snap.UptimeSeconds != 0 {
		t.Errorf("UptimeSeconds: got %v, want 0", snap.UptimeSeconds)
	}
	for _, k := range []string{"2xx", "3xx", "4xx", "5xx", "other"} {
		if _, ok := snap.HTTP.RequestsByClass[k]; !ok {
			t.Errorf("RequestsByClass missing key %q", k)
		}
	}
	for _, k := range []string{"succeeded", "failed", "aborted"} {
		if _, ok := snap.Agent.RunsByTerminalStatus[k]; !ok {
			t.Errorf("RunsByTerminalStatus missing key %q", k)
		}
	}
	if snap.Build.Version == "" || snap.Build.GoVersion == "" {
		t.Errorf("Build labels not populated: %+v", snap.Build)
	}
}

// TestRead_nilGatherer guards against a nil registry blowing up the
// handler — the function logs and returns the zeroed-but-shaped
// snapshot.
func TestRead_nilGatherer(t *testing.T) {
	snap := Read(nil, time.Now())
	if snap.HTTP.RequestsByClass == nil {
		t.Fatal("expected non-nil RequestsByClass with nil gatherer")
	}
}

// TestRead_populatedHTTPCountersAndHistogram exercises the full
// counter + histogram aggregation path: classify by status code,
// sum across method/route labels, and interpolate p50/p95.
func TestRead_populatedHTTPCountersAndHistogram(t *testing.T) {
	reg := prometheus.NewPedanticRegistry()

	requestsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{Namespace: "taskapi", Name: "http_requests_total", Help: "."},
		[]string{"method", "route", "code"},
	)
	durationVec := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "taskapi",
			Name:      "http_request_duration_seconds",
			Help:      ".",
			Buckets:   []float64{0.05, 0.1, 0.25, 0.5, 1.0},
		},
		[]string{"method", "route"},
	)
	inFlight := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "taskapi", Name: "http_in_flight", Help: ".",
	})
	reg.MustRegister(requestsTotal, durationVec, inFlight)

	requestsTotal.WithLabelValues("GET", "/tasks", "200").Add(80)
	requestsTotal.WithLabelValues("GET", "/tasks", "200").Add(20)
	requestsTotal.WithLabelValues("POST", "/tasks", "201").Add(15)
	requestsTotal.WithLabelValues("GET", "/tasks/{id}", "404").Add(5)
	requestsTotal.WithLabelValues("POST", "/tasks", "500").Add(2)
	inFlight.Set(3)

	for i := 0; i < 80; i++ {
		durationVec.WithLabelValues("GET", "/tasks").Observe(0.04)
	}
	for i := 0; i < 18; i++ {
		durationVec.WithLabelValues("GET", "/tasks").Observe(0.08)
	}
	for i := 0; i < 2; i++ {
		durationVec.WithLabelValues("POST", "/tasks").Observe(0.4)
	}

	snap := Read(reg, time.Now())

	if got, want := snap.HTTP.InFlight, int64(3); got != want {
		t.Errorf("InFlight: got %d, want %d", got, want)
	}
	if got, want := snap.HTTP.RequestsTotal, uint64(122); got != want {
		t.Errorf("RequestsTotal: got %d, want %d", got, want)
	}
	wantClass := map[string]uint64{
		"2xx": 115, "3xx": 0, "4xx": 5, "5xx": 2, "other": 0,
	}
	for k, v := range wantClass {
		if snap.HTTP.RequestsByClass[k] != v {
			t.Errorf("RequestsByClass[%s]: got %d, want %d", k, snap.HTTP.RequestsByClass[k], v)
		}
	}
	if got := snap.HTTP.DurationSeconds.Count; got != 100 {
		t.Errorf("DurationSeconds.Count: got %d, want 100", got)
	}
	// p50 should land in the 0.05 bucket (50 samples ≤ 0.05). p95
	// should land somewhere ≤ 0.5 because 98 samples are ≤ 0.1.
	if snap.HTTP.DurationSeconds.P50 < 0 || snap.HTTP.DurationSeconds.P50 > 0.05 {
		t.Errorf("P50 out of expected range (0,0.05]: got %v", snap.HTTP.DurationSeconds.P50)
	}
	if snap.HTTP.DurationSeconds.P95 <= 0 || snap.HTTP.DurationSeconds.P95 > 0.5 {
		t.Errorf("P95 out of expected range (0,0.5]: got %v", snap.HTTP.DurationSeconds.P95)
	}
}

// TestRead_populatedSSEAndDBPoolAndAgent exercises the rest of the
// envelope so a wiring regression (e.g. wrong metric name) gets
// caught here, not in the SPA.
func TestRead_populatedSSEAndDBPoolAndAgent(t *testing.T) {
	reg := prometheus.NewPedanticRegistry()

	subs := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "taskapi", Name: "sse_subscribers", Help: ".",
	})
	dropped := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "taskapi", Name: "sse_dropped_frames_total", Help: ".",
	})
	maxOpen := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "taskapi", Subsystem: "db_pool", Name: "max_open_connections", Help: ".",
	})
	open := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "taskapi", Subsystem: "db_pool", Name: "open_connections", Help: ".",
	})
	inUse := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "taskapi", Subsystem: "db_pool", Name: "in_use_connections", Help: ".",
	})
	idle := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "taskapi", Subsystem: "db_pool", Name: "idle_connections", Help: ".",
	})
	waitCount := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "taskapi", Subsystem: "db_pool", Name: "wait_count_total", Help: ".",
	})
	waitDur := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "taskapi", Subsystem: "db_pool", Name: "wait_duration_seconds_total", Help: ".",
	})
	queueDepth := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "taskapi", Name: "agent_queue_depth", Help: ".",
	})
	queueCap := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "taskapi", Name: "agent_queue_capacity", Help: ".",
	})
	agentRuns := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "t2a", Name: "agent_runs_total", Help: ".",
	}, []string{"runner", "terminal_status"})
	reg.MustRegister(subs, dropped, maxOpen, open, inUse, idle, waitCount, waitDur, queueDepth, queueCap, agentRuns)

	subs.Set(4)
	dropped.Add(7)
	maxOpen.Set(20)
	open.Set(5)
	inUse.Set(2)
	idle.Set(3)
	waitCount.Add(1)
	waitDur.Add(0.42)
	queueDepth.Set(2)
	queueCap.Set(64)
	agentRuns.WithLabelValues("cursor", "succeeded").Add(10)
	agentRuns.WithLabelValues("cursor", "failed").Add(3)
	agentRuns.WithLabelValues("fake", "aborted").Add(1)

	snap := Read(reg, time.Now())

	if snap.SSE.Subscribers != 4 || snap.SSE.DroppedFramesTotal != 7 {
		t.Errorf("SSE: got %+v, want subs=4 dropped=7", snap.SSE)
	}
	wantPool := DBPool{
		MaxOpenConnections: 20, OpenConnections: 5, InUseConnections: 2, IdleConnections: 3,
		WaitCountTotal: 1, WaitDurationSecondsTotal: 0.42,
	}
	if snap.DBPool != wantPool {
		t.Errorf("DBPool: got %+v, want %+v", snap.DBPool, wantPool)
	}
	if snap.Agent.QueueDepth != 2 || snap.Agent.QueueCapacity != 64 {
		t.Errorf("Agent queue: got depth=%d cap=%d", snap.Agent.QueueDepth, snap.Agent.QueueCapacity)
	}
	if snap.Agent.RunsTotal != 14 {
		t.Errorf("Agent.RunsTotal: got %d, want 14", snap.Agent.RunsTotal)
	}
	wantStatus := map[string]uint64{"succeeded": 10, "failed": 3, "aborted": 1}
	for k, v := range wantStatus {
		if snap.Agent.RunsByTerminalStatus[k] != v {
			t.Errorf("RunsByTerminalStatus[%s]: got %d, want %d", k, snap.Agent.RunsByTerminalStatus[k], v)
		}
	}
}

// TestRead_uptimeFromProcessStart asserts uptime is derived from
// process_start_time_seconds (the standard ProcessCollector gauge).
func TestRead_uptimeFromProcessStart(t *testing.T) {
	reg := prometheus.NewPedanticRegistry()
	startGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "process_start_time_seconds", Help: ".",
	})
	reg.MustRegister(startGauge)

	startWall := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := startWall.Add(123 * time.Second)
	startGauge.Set(float64(startWall.Unix()))

	snap := Read(reg, now)
	if math.Abs(snap.UptimeSeconds-123) > 0.5 {
		t.Errorf("UptimeSeconds: got %v, want ~123", snap.UptimeSeconds)
	}
}

// TestRead_gatherError keeps the response shaped even when the
// registry refuses to gather (would normally be a programming error;
// the handler still gets a usable envelope to serialise).
func TestRead_gatherError(t *testing.T) {
	snap := Read(brokenGatherer{}, time.Now())
	if snap.HTTP.RequestsByClass == nil {
		t.Fatal("expected non-nil RequestsByClass even on gather error")
	}
	if snap.Agent.RunsByTerminalStatus == nil {
		t.Fatal("expected non-nil RunsByTerminalStatus even on gather error")
	}
}

type brokenGatherer struct{}

func (brokenGatherer) Gather() ([]*dto.MetricFamily, error) {
	return nil, errors.New("boom")
}
