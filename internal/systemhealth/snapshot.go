package systemhealth

import (
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/AlexsanderHamir/T2A/internal/version"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

const logCmd = "taskapi"

// Build identifies the running binary. Mirrors the labels of the
// taskapi_build_info gauge so the JSON value matches what scrapers
// see; populated from internal/version (no Prometheus dependency on
// this field, so the response is non-empty even before the build_info
// gauge is registered).
type Build struct {
	Version   string `json:"version"`
	Revision  string `json:"revision"`
	GoVersion string `json:"go_version"`
}

// HTTPDuration summarises the taskapi_http_request_duration_seconds
// histogram with two interpolated quantiles + the total observation
// count. p50/p95 are computed from cumulative bucket counts; the
// values are 0 when count == 0 so the SPA can render "—" without
// branching on missing keys.
type HTTPDuration struct {
	P50   float64 `json:"p50"`
	P95   float64 `json:"p95"`
	Count uint64  `json:"count"`
}

// HTTP is the request-volume + latency rollup. RequestsByClass keys
// are always {"2xx","3xx","4xx","5xx","other"} — `other` collects
// requests with a non-numeric or out-of-range status code so the SPA
// table never has missing buckets.
type HTTP struct {
	InFlight        int64             `json:"in_flight"`
	RequestsTotal   uint64            `json:"requests_total"`
	RequestsByClass map[string]uint64 `json:"requests_by_class"`
	DurationSeconds HTTPDuration      `json:"duration_seconds"`
}

// SSE captures the live fanout: how many subscribers are connected
// right now and how many frames the hub has dropped because a slow
// consumer's bounded channel was full. Sustained drops are the
// canonical signal of a stuck client.
type SSE struct {
	Subscribers        int64  `json:"subscribers"`
	DroppedFramesTotal uint64 `json:"dropped_frames_total"`
}

// DBPool mirrors database/sql.DBStats projected through
// taskapi_db_pool_*. Only the fields the operator UI renders are
// included; the full set is still on /metrics for Prometheus.
type DBPool struct {
	MaxOpenConnections       int64   `json:"max_open_connections"`
	OpenConnections          int64   `json:"open_connections"`
	InUseConnections         int64   `json:"in_use_connections"`
	IdleConnections          int64   `json:"idle_connections"`
	WaitCountTotal           uint64  `json:"wait_count_total"`
	WaitDurationSecondsTotal float64 `json:"wait_duration_seconds_total"`
}

// Agent reflects the in-process worker queue + terminal-status
// counters. RunsByTerminalStatus seeds {"succeeded","failed","aborted"}
// to zero on first read so the heatmap-style UI is never blank.
//
// Paused is the operator-facing soft pause from app_settings.AgentPaused.
// It is NOT sourced from a Prometheus metric — the supervisor goes
// idle when paused, so a counter-based signal would lag the actual
// state. The handler layers this field on after Read() returns,
// reading the singleton settings row directly. Defaults to false on
// snapshots produced without a settings reader (e.g. unit tests of
// the aggregator) so old call sites stay correct.
type Agent struct {
	QueueDepth           int64             `json:"queue_depth"`
	QueueCapacity        int64             `json:"queue_capacity"`
	RunsTotal            uint64            `json:"runs_total"`
	RunsByTerminalStatus map[string]uint64 `json:"runs_by_terminal_status"`
	Paused               bool              `json:"paused"`
}

// Snapshot is the wire envelope returned by GET /system/health.
// All sub-objects and their map keys are always present (zero/empty
// rather than missing) so the SPA can render against a stable shape.
type Snapshot struct {
	Build         Build     `json:"build"`
	UptimeSeconds float64   `json:"uptime_seconds"`
	Now           time.Time `json:"now"`
	HTTP          HTTP      `json:"http"`
	SSE           SSE       `json:"sse"`
	DBPool        DBPool    `json:"db_pool"`
	Agent         Agent     `json:"agent"`
}

// Gather is the abstraction used to read metrics. prometheus.Gatherer
// satisfies it; tests pass a NewPedanticRegistry to get an isolated
// view.
type Gather interface {
	Gather() ([]*dto.MetricFamily, error)
}

// Read builds a Snapshot from the supplied Gather. It returns a
// fully-zeroed but still well-formed Snapshot when Gather fails so
// callers can log+respond rather than 500ing the operator UI.
func Read(g Gather, now time.Time) Snapshot {
	slog.Debug("trace", "cmd", logCmd, "operation", "systemhealth.Read")
	snap := newZeroSnapshot(now)
	snap.Build = readBuildFromVersion()
	if g == nil {
		return snap
	}
	mfs, err := g.Gather()
	if err != nil {
		slog.Warn("systemhealth gather failed", "cmd", logCmd, "operation", "systemhealth.Read", "err", err)
		return snap
	}
	for _, mf := range mfs {
		applyFamily(&snap, mf)
	}
	return snap
}

// ReadDefault is the production entry point: it scrapes
// prometheus.DefaultGatherer at the supplied wall clock.
func ReadDefault(now time.Time) Snapshot {
	slog.Debug("trace", "cmd", logCmd, "operation", "systemhealth.ReadDefault")
	return Read(prometheus.DefaultGatherer, now)
}

func newZeroSnapshot(now time.Time) Snapshot {
	return Snapshot{
		Now: now.UTC(),
		HTTP: HTTP{
			RequestsByClass: map[string]uint64{
				"2xx":   0,
				"3xx":   0,
				"4xx":   0,
				"5xx":   0,
				"other": 0,
			},
		},
		Agent: Agent{
			RunsByTerminalStatus: map[string]uint64{
				"succeeded": 0,
				"failed":    0,
				"aborted":   0,
			},
		},
	}
}

func readBuildFromVersion() Build {
	v, r, gv := version.PrometheusBuildInfoLabels()
	return Build{Version: v, Revision: r, GoVersion: gv}
}

// applyFamily routes one MetricFamily into the matching Snapshot
// slot. Unrelated families are ignored so /system/health stays
// independent of Go runtime metrics drift.
func applyFamily(snap *Snapshot, mf *dto.MetricFamily) {
	if mf == nil {
		return
	}
	switch mf.GetName() {
	case "taskapi_http_in_flight":
		snap.HTTP.InFlight = int64(gaugeSum(mf))
	case "taskapi_http_requests_total":
		applyHTTPRequests(&snap.HTTP, mf)
	case "taskapi_http_request_duration_seconds":
		applyHTTPDuration(&snap.HTTP, mf)
	case "taskapi_sse_subscribers":
		snap.SSE.Subscribers = int64(gaugeSum(mf))
	case "taskapi_sse_dropped_frames_total":
		snap.SSE.DroppedFramesTotal = uint64(counterSum(mf))
	case "taskapi_db_pool_max_open_connections":
		snap.DBPool.MaxOpenConnections = int64(gaugeSum(mf))
	case "taskapi_db_pool_open_connections":
		snap.DBPool.OpenConnections = int64(gaugeSum(mf))
	case "taskapi_db_pool_in_use_connections":
		snap.DBPool.InUseConnections = int64(gaugeSum(mf))
	case "taskapi_db_pool_idle_connections":
		snap.DBPool.IdleConnections = int64(gaugeSum(mf))
	case "taskapi_db_pool_wait_count_total":
		snap.DBPool.WaitCountTotal = uint64(counterSum(mf))
	case "taskapi_db_pool_wait_duration_seconds_total":
		snap.DBPool.WaitDurationSecondsTotal = counterSum(mf)
	case "taskapi_agent_queue_depth":
		snap.Agent.QueueDepth = int64(gaugeSum(mf))
	case "taskapi_agent_queue_capacity":
		snap.Agent.QueueCapacity = int64(gaugeSum(mf))
	case "t2a_agent_runs_total":
		applyAgentRuns(&snap.Agent, mf)
	case "process_start_time_seconds":
		applyUptime(snap, mf)
	}
}

func applyHTTPRequests(h *HTTP, mf *dto.MetricFamily) {
	for _, m := range mf.GetMetric() {
		c := m.GetCounter()
		if c == nil {
			continue
		}
		val := uint64(c.GetValue())
		h.RequestsTotal += val
		h.RequestsByClass[classifyStatus(labelValue(m, "code"))] += val
	}
}

func classifyStatus(code string) string {
	if len(code) == 0 {
		return "other"
	}
	switch code[0] {
	case '2':
		return "2xx"
	case '3':
		return "3xx"
	case '4':
		return "4xx"
	case '5':
		return "5xx"
	default:
		return "other"
	}
}

func applyHTTPDuration(h *HTTP, mf *dto.MetricFamily) {
	// Aggregate buckets across all label combinations so the global
	// p50/p95 reflects every request, not just one (method,route)
	// pair. Buckets in dto.Histogram are cumulative upper bounds;
	// adding them per upper bound yields the global cumulative
	// distribution we can interpolate.
	merged := mergeHistograms(mf)
	if merged.count == 0 {
		return
	}
	h.DurationSeconds = HTTPDuration{
		P50:   percentileFromBuckets(merged, 0.50),
		P95:   percentileFromBuckets(merged, 0.95),
		Count: merged.count,
	}
}

type bucket struct {
	upperBound float64
	cumulative uint64
}

type mergedHistogram struct {
	count   uint64
	buckets []bucket
}

func mergeHistograms(mf *dto.MetricFamily) mergedHistogram {
	totals := map[float64]uint64{}
	var count uint64
	for _, m := range mf.GetMetric() {
		hg := m.GetHistogram()
		if hg == nil {
			continue
		}
		count += hg.GetSampleCount()
		for _, b := range hg.GetBucket() {
			totals[b.GetUpperBound()] += b.GetCumulativeCount()
		}
	}
	bounds := make([]float64, 0, len(totals))
	for ub := range totals {
		bounds = append(bounds, ub)
	}
	sort.Float64s(bounds)
	out := make([]bucket, 0, len(bounds))
	for _, ub := range bounds {
		out = append(out, bucket{upperBound: ub, cumulative: totals[ub]})
	}
	return mergedHistogram{count: count, buckets: out}
}

// percentileFromBuckets does a Prometheus-style linear interpolation
// inside the bucket that contains the target rank. Behaviour matches
// histogram_quantile() closely enough for an operator UI: it errs on
// the side of returning the bucket's upper bound when the target sits
// inside the +Inf bucket so very-slow tails are visible.
func percentileFromBuckets(mh mergedHistogram, q float64) float64 {
	if mh.count == 0 || len(mh.buckets) == 0 {
		return 0
	}
	rank := float64(mh.count) * q
	var prevBound float64
	var prevCum uint64
	for _, b := range mh.buckets {
		if float64(b.cumulative) >= rank {
			if b.upperBound == 0 || b.cumulative == prevCum {
				return b.upperBound
			}
			width := b.upperBound - prevBound
			frac := (rank - float64(prevCum)) / float64(b.cumulative-prevCum)
			return prevBound + width*frac
		}
		prevBound = b.upperBound
		prevCum = b.cumulative
	}
	return mh.buckets[len(mh.buckets)-1].upperBound
}

func applyAgentRuns(a *Agent, mf *dto.MetricFamily) {
	for _, m := range mf.GetMetric() {
		c := m.GetCounter()
		if c == nil {
			continue
		}
		val := uint64(c.GetValue())
		a.RunsTotal += val
		status := labelValue(m, "terminal_status")
		if _, ok := a.RunsByTerminalStatus[status]; !ok {
			// Keep cardinality bounded: any unexpected terminal
			// status (would be a worker bug) lands under "other"
			// rather than silently inflating a known bucket.
			status = "other"
			if _, ok := a.RunsByTerminalStatus[status]; !ok {
				a.RunsByTerminalStatus[status] = 0
			}
		}
		a.RunsByTerminalStatus[status] += val
	}
}

func applyUptime(snap *Snapshot, mf *dto.MetricFamily) {
	start := gaugeSum(mf)
	if start <= 0 {
		return
	}
	delta := snap.Now.Sub(time.Unix(int64(start), 0)).Seconds()
	if delta < 0 {
		delta = 0
	}
	snap.UptimeSeconds = delta
}

func gaugeSum(mf *dto.MetricFamily) float64 {
	var sum float64
	for _, m := range mf.GetMetric() {
		if g := m.GetGauge(); g != nil {
			sum += g.GetValue()
			continue
		}
		if u := m.GetUntyped(); u != nil {
			sum += u.GetValue()
		}
	}
	return sum
}

func counterSum(mf *dto.MetricFamily) float64 {
	var sum float64
	for _, m := range mf.GetMetric() {
		if c := m.GetCounter(); c != nil {
			sum += c.GetValue()
		}
	}
	return sum
}

func labelValue(m *dto.Metric, name string) string {
	for _, lp := range m.GetLabel() {
		if lp.GetName() == name {
			return lp.GetValue()
		}
	}
	return ""
}

// String is a debug helper used by error paths that want to embed the
// snapshot identity in a log line without dumping the whole struct.
func (s Snapshot) String() string {
	return fmt.Sprintf("systemhealth.Snapshot{ver=%s rev=%s uptime=%.0fs http_in_flight=%d sse_subs=%d agent_q=%d/%d}",
		s.Build.Version, s.Build.Revision, s.UptimeSeconds,
		s.HTTP.InFlight, s.SSE.Subscribers,
		s.Agent.QueueDepth, s.Agent.QueueCapacity)
}
